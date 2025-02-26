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
func Create(ctx context.Context, param *proto.CreateOrderReq) (*proto.CreateOrderRep, error) {

	// 1. 生成订单号
	orderId := snowflake.GenID()

	// 2. 查询商品金额（营销）- RPC调用
	goodsDetail, err := rpc.GoodsCli.GetGoodsDetail(ctx, &proto.GetGoodsDetailReq{
		GoodsId: param.GoodsId,
		UserId:  param.UserId,
	})
	if err != nil || goodsDetail == nil {
		zap.L().Error("商品详情获取失败", zap.Error(err), zap.Any("resp", goodsDetail))
		return &proto.CreateOrderRep{Success: false, Message: "商品服务不可用"}, nil
	}
	if goodsDetail.Price == "" {
		return &proto.CreateOrderRep{Success: false, Message: "商品价格为空"}, nil
	}

	// 3. 计算支付金额：商品单价 × 购买数量
	// 3. 计算支付金额
	price, err := strconv.ParseInt(goodsDetail.Price, 10, 64)
	if err != nil {
		zap.L().Error("价格格式错误", zap.String("price", goodsDetail.Price), zap.Error(err))
		return &proto.CreateOrderRep{Success: false, Message: "商品价格异常"}, nil
	}
	payAmount := price * int64(param.Num)

	// 4. 库存校验及扣减
	_, err = rpc.StockCli.ReduceStock(ctx, &proto.ReduceStockInfo{
		GoodsId: param.GoodsId,
		Num:     int64(param.Num),
		OrderId: orderId,
	})
	if err != nil {
		zap.L().Error("库存扣减失败", zap.Error(err))
		return &proto.CreateOrderRep{Success: false, Message: "库存不足"}, nil
	}

	// 5. 创建订单，用事务把创建订单和创建订单详情两个动作包起来
	orderData := model.Order{
		OrderId:        orderId,
		UserId:         param.UserId,
		PayAmount:      payAmount,
		ReceiveAddress: param.Address,
		ReceiveName:    param.Name,
		ReceivePhone:   param.Phone,
	}

	orderDetail := model.OrderDetail{
		OrderId:   orderId,
		UserId:    param.UserId,
		GoodsId:   param.GoodsId,
		Num:       int64(param.Num),
		Title:     goodsDetail.Title,
		Price:     price,
		Brief:     goodsDetail.Brief,
		PayAmount: payAmount,
	}

	if err := mysql.CreateOrder(ctx, &orderData); err != nil {
		zap.L().Error("订单创建失败", zap.Error(err))
		return &proto.CreateOrderRep{Success: false, Message: "订单创建失败"}, nil
	}

	if err := mysql.CreateOrderDetail(ctx, &orderDetail); err != nil {
		zap.L().Error("订单详情创建失败", zap.Error(err))
		return &proto.CreateOrderRep{Success: false, Message: "订单详情创建失败"}, nil
	}

	return &proto.CreateOrderRep{
		Success: true,
		OrderId: orderId,
		Price:   goodsDetail.Price, // 透传商品价格
	}, nil
}
