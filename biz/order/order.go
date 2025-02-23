package order

import (
	"context"
	"strconv"

	"order_service/dao/mysql"             // 数据库操作模块
	"order_service/model"                 // 数据模型模块
	"order_service/proto"                 // gRPC 服务定义模块
	"order_service/rpc"                   // RPC 客户端初始化模块
	"order_service/third_party/snowflake" // Snowflake ID 生成模块

	"go.uber.org/zap" // 日志库
)

// biz层业务代码
// biz -> dao

// 不加分布式事务实现创建订单
func Create(ctx context.Context, param *proto.CreateOrderReq) error {
	// 1. 生成订单号
	orderId := snowflake.GenID() // 使用 Snowflake 算法生成全局唯一的订单号

	// 2. 查询商品金额（营销）- RPC调用
	// 每次请求都通过 Consul 获取一个可用的商品服务地址
	// 然后通过 RPC 客户端调用商品服务的接口
	goodsDetail, err := rpc.GoodsCli.GetGoodsDetail(ctx, &proto.GetGoodsDetailReq{
		GoodsId: param.GoodsId, // 商品 ID
		UserId:  param.UserId,  // 用户 ID
	})
	if err != nil {
		zap.L().Error("GoodsCli.GetGoodsDetail failed", zap.Error(err)) // 记录错误日志
		return err                                                      // 如果调用失败，返回错误
	}

	// 3. 计算支付金额：商品单价 × 购买数量
	// 获取商品单价（字符串格式）
	priceStr := goodsDetail.Price
	price, err := strconv.ParseInt(priceStr, 10, 64) // 将价格转换为 int64 类型
	if err != nil {
		zap.L().Error("strconv.ParseInt failed", zap.String("priceStr", priceStr), zap.Error(err))
		return err
	}

	// 计算支付金额：单价 × 数量
	payAmount := price * int64(param.Num)

	// 3. 库存校验及扣减
	// 每次请求都通过 Consul 获取一个可用的库存服务地址
	// 然后通过 RPC 客户端调用库存服务的接口
	_, err = rpc.StockCli.ReduceStock(ctx, &proto.ReduceStockInfo{
		GoodsId: param.GoodsId,    // 商品 ID
		Num:     int64(param.Num), // 扣减数量
		OrderId: orderId,          //订单号
	})
	if err != nil {
		zap.L().Error("StockCli.ReduceStock failed", zap.Error(err)) // 记录错误日志
		return err                                                   // 如果调用失败，返回错误
	}

	// 4. 创建订单，用事务把创建订单和创建订单详情两个动作包起来
	// 4.1 创建订单表
	orderData := model.Order{
		OrderId:        orderId,       // 订单号
		UserId:         param.UserId,  // 用户 ID
		PayAmount:      payAmount,     // 支付金额
		ReceiveAddress: param.Address, // 收货地址
		ReceiveName:    param.Name,    // 收货人姓名
		ReceivePhone:   param.Phone,   // 收货人电话
	}

	// 4.2 创建订单详情表
	orderDetail := model.OrderDetail{
		OrderId: orderId,          // 订单号
		UserId:  param.UserId,     // 用户 ID
		GoodsId: param.GoodsId,    // 商品 ID
		Num:     int64(param.Num), // 商品数量
		Title: goodsDetail.Title,  //商品名称
		Price: goodsDetail.Price,  //商品价格
		Brief: goodsDetail.Brief,  //商品简介
		PayAmount: payAmount,	   //支付金额
	}

	// 事务操作：同时创建订单表和订单详情表
	return mysql.CreateOrderWithTransation(ctx, &orderData, &orderDetail)
}
