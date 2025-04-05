package order

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"order_service/config"
	"order_service/dao/mq"
	"order_service/dao/mysql"             // 数据库操作模块
	"order_service/model"                 // 数据模型模块
	"order_service/proto"                 // gRPC 服务定义模块
	"order_service/rpc"                   // RPC 客户端初始化模块
	"order_service/third_party/snowflake" // Snowflake ID 生成模块

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"go.uber.org/zap" // 日志库
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// biz层业务代码
// biz -> dao

// OrderEntity 自定义结构体，实现了 RocketMQ 事务消息的两个方法：
// 1. ExecuteLocalTransaction：本地事务执行逻辑
// 2. CheckLocalTransaction：事务状态回查逻辑
type OrderEntity struct {
	OrderId int64                 // 订单ID
	Param   *proto.CreateOrderReq // 订单请求参数
	Topic  string                // 事务消息的主题
	err     error                 // 本地事务执行过程中可能产生的错误
}

// 创建订单
//订单创建的入口点，负责生成订单号并发送事务消息
func Create(ctx context.Context, param *proto.CreateOrderReq) (*proto.Response, error) {

	// 1. 生成订单号
	orderId := snowflake.GenID()

	//创建OrderEntity实例，用于事务消息的上下文
	orderEntity := &OrderEntity{
		OrderId: orderId,
		Param:   param,
		Topic:   config.Conf.RocketMqConfig.Topic.CreateOrder, // 默认Topic为创建订单
	}

	//创建事务生产者
	p, err := rocketmq.NewTransactionProducer(
		orderEntity, // 将 OrderEntity 作为事务消息的上下文
		producer.WithNsResolver(primitive.NewPassthroughResolver([]string{"127.0.0.1:9876"})), // 配置 RocketMQ 名称服务器地址
		producer.WithRetry(2),                 // 设置重试次数
		producer.WithGroupName("order_srv_1"), // 设置生产者组名
	)
	if err != nil {
		// 如果创建事务生产者失败，记录日志并返回错误。
		zap.L().Error("NewTransactionProducer failed", zap.Error(err))
		return nil,status.Error(codes.Internal, "NewTransactionProducer failed")
	}
	// 启动事务生产者
	p.Start()

	//构造事务消息的内容
	data := model.OrderDetail{
		OrderId: orderId,
		GoodsId: param.GoodsId,
		Num:     int64(param.Num),
	}

	b, _ := json.Marshal(data)
	//构造消息
	msg := &primitive.Message{
		Topic: orderEntity.Topic, // 事务消息的主题，用于创建订单
		Body:  b,
	}

	//发送事务消息
	res, err := p.SendMessageInTransaction(ctx, msg)
	if err != nil {
		// 如果发送事务消息失败，记录日志并返回错误。
		zap.L().Error("SendMessageInTransaction failed", zap.Error(err))
		return nil,status.Error(codes.Internal, "create order failed")
	}
	// 根据事务消息的响应状态和Topic判断订单创建是否成功
	if res.State == primitive.CommitMessageState {
		// 如果事务消息提交成功，根据Topic返回不同的响应
		if orderEntity.Topic == config.Conf.RocketMqConfig.Topic.CreateOderSuccessfully {
			return &proto.Response{Success: true, Message: "Order created successfully"}, nil
		} else if orderEntity.Topic == config.Conf.RocketMqConfig.Topic.PayTimeOut {
			return nil, status.Error(codes.Internal, "Order creation failed due to timeout")
		} else if orderEntity.Topic == config.Conf.RocketMqConfig.Topic.StockRollback {
			return nil, status.Error(codes.Internal, "Order creation failed")
		}
	}

	if res.State == primitive.RollbackMessageState {
		return nil, status.Error(codes.Internal, "create order failed")
	}

	return nil, status.Error(codes.Internal, "unknown transaction state")

}


// ExecuteLocalTransaction 是 RocketMQ 事务消息的本地事务执行逻辑。
// 当发送事务消息（half-message）成功后，RocketMQ 会调用此方法。
func (o *OrderEntity) ExecuteLocalTransaction(*primitive.Message) primitive.LocalTransactionState {
	fmt.Println("in ExecuteLocalTransaction...")

	// 参数校验：如果 Param 为空，说明事务消息的上下文不完整，直接返回 Rollback 状态。
	if o.Param == nil {
		zap.L().Error("ExecuteLocalTransaction param is nil")
		o.err = status.Error(codes.Internal, "invalid OrderEntity")
		return primitive.RollbackMessageState
	}

	// 获取订单请求参数
	param := o.Param
	ctx := context.Background()

	// 1. 查询商品金额（营销）--> RPC连接 goods_service
	// 调用 goods_service 获取商品详情，包括价格。
	goodsDetail, err := rpc.GoodsCli.GetGoodsDetail(ctx, &proto.GetGoodsDetailReq{
		GoodsId: param.GoodsId,
		UserId:  param.UserId,
	})
	if err != nil {
		// 如果查询商品失败，记录日志并返回 Rollback 状态，表示本地事务失败。
		zap.L().Error("GoodsCli.GetGoodsDetail failed", zap.Error(err))
		o.err = status.Error(codes.Internal, err.Error())
		return primitive.RollbackMessageState
	}

	// 将价格字符串转换为整型
	payAmountStr := goodsDetail.Price
	payAmount, _ := strconv.ParseInt(payAmountStr, 10, 64)

	// 2. 库存校验及扣减  --> RPC连接 stock_service
	// 调用 stock_service 扣减库存。
	_, err = rpc.StockCli.ReduceStock(ctx, &proto.ReduceStockInfo{
		GoodsId: o.Param.GoodsId,
		Num:     int64(o.Param.Num),
		OrderId: o.OrderId,
	})

	if err != nil {
		// 如果库存扣减失败，记录日志并返回 Rollback 状态，表示本地事务失败。
		zap.L().Error("StockCli.ReduceStock failed", zap.Error(err))
		o.err = status.Error(codes.Internal, "ReduceStock failed")
		return primitive.RollbackMessageState
	}

	// 代码能执行到这里说明 扣减库存成功了，
	// 从这里开始如果本地事务执行失败就需要回滚库存

	// 3. 创建订单
	// 构建订单数据并写入 MySQL 数据库。
	orderData := model.Order{
		OrderId:        o.OrderId,
		UserId:         param.UserId,
		PayAmount:      payAmount,
		ReceiveAddress: param.Address,
		ReceiveName:    param.Name,
		ReceivePhone:   param.Phone,
		//Status:         100, // 待支付 用于支付服务
	}
	orderDetail := model.OrderDetail{
		OrderId: o.OrderId,
		UserId:  param.UserId,
		GoodsId: param.GoodsId,
		Num:     int64(param.Num),
		Title:     goodsDetail.Title,
		Status: 	"pending",  //待支付
		Price:     payAmount,
		Brief:     goodsDetail.Brief,
		PayAmount: payAmount,
	}

	// 使用事务创建订单和订单详情记录。
	err = mysql.CreateOrderWithTransation(ctx, &orderData, &orderDetail)
	if err != nil {
		// 如果订单创建失败，发送一条状态为“order_failed”的消息
		// 记录日志并返回 Rollback 状态，表示本地事务失败。
		o.Topic = config.Conf.RocketMqConfig.Topic.StockRollback
		msg := primitive.NewMessage(o.Topic, []byte(fmt.Sprintf(`{"orderId":%d,"reason":"order creation failed"}`, o.OrderId)))
		_, errSend := mq.Producer.SendSync(context.Background(), msg)
		if errSend != nil {
			zap.L().Error("send order_failed msg failed", zap.Error(errSend))
		}
		zap.L().Error("CreateOrderWithTransation failed", zap.Error(err))
		return primitive.RollbackMessageState
	}

	// 发送延迟消息，用于订单超时处理。
	// 如果用户在一定时间内未支付，将触发库存回滚。
	data := model.OrderDetail{
		OrderId: o.OrderId,
		GoodsId: param.GoodsId,
		Num:     int64(param.Num),
		Status:  "pending",  //待支付    //实际应该根据支付服务的反馈修改
	}
	b, _ := json.Marshal(data)
	o.Topic = config.Conf.RocketMqConfig.Topic.PayTimeOut
	msgTimeout := primitive.NewMessage(o.Topic, b) // 设置消息的Topic: order_timeout
	msgTimeout.WithDelayTimeLevel(3) // 设置延迟级别（例如 10s）
	//同步发送延迟消息,会阻塞当前线程，知道消息发送成功或失败

	//订单超时的检测通过rocketmq自带的延迟消息机制完成，在订单创建成功后，系统会发布一条延迟信息，用于在指定时间后检查订单是否超时
	//这条消息会在设定的时间后被 RocketMQ 触发，发送到指定的消费者（例如订单服务的超时处理模块）。
	//发送一条状态为“timeout”的消息
	// 同步发送延迟消息，会阻塞当前线程，直到消息发送成功或失败
	_, err = mq.Producer.SendSync(context.Background(), msgTimeout)
	if err != nil {
		// 如果发送延迟消息失败，发送一条状态为“order_failed”的消息
		// 记录日志并返回 Rollback 状态。
		o.Topic = config.Conf.RocketMqConfig.Topic.PayTimeOut
		msg := primitive.NewMessage(o.Topic, []byte(fmt.Sprintf(`{"orderId":%d,"reason":"timeout message send failed"}`, o.OrderId)))
		_, errSend := mq.Producer.SendSync(context.Background(), msg)
		if errSend != nil {
			zap.L().Error("send order_failed msg failed", zap.Error(errSend))
		}
		zap.L().Error("send delay msg failed", zap.Error(err))
		return primitive.RollbackMessageState
	}

	//如果本地事务成功，提交订单创建成功的消息到Rocketmq
	//发送一条状态为“success”的消息
	// 如果本地事务成功，发送一条状态为“order_success”的消息
	o.Topic = config.Conf.RocketMqConfig.Topic.CreateOderSuccessfully
	msgSuccess := primitive.NewMessage(o.Topic, []byte(fmt.Sprintf(`{"orderId":%d,"status":"success"}`, o.OrderId)))
	_, errSuccess := mq.Producer.SendSync(context.Background(), msgSuccess)
	if errSuccess != nil {
		zap.L().Error("send order_success msg failed", zap.Error(errSuccess))
		return primitive.RollbackMessageState
	}

	// 如果本地事务成功，返回 Commit 状态，表示事务消息可以提交。
	return primitive.CommitMessageState
}


// CheckLocalTransaction 是 RocketMQ 事务消息的状态回查逻辑。
// 当 RocketMQ 在发送事务消息后未收到明确的提交或回滚响应时，会调用此方法回查本地事务的状态。
func (o *OrderEntity) CheckLocalTransaction(*primitive.MessageExt) primitive.LocalTransactionState {
	// 查询订单是否创建成功。
	_, err := mysql.QueryOrder(context.Background(), o.OrderId)
	if err == gorm.ErrRecordNotFound {
		// 如果订单未创建成功，返回 Commit 状态，表示需要回滚库存。
		return primitive.CommitMessageState
	}
	// 如果订单已创建成功，返回 Rollback 状态，表示不需要回滚库存。
	return primitive.RollbackMessageState
}


