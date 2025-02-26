package handler

import (
	"context"
	"fmt"
	"order_service/biz/order"
	"order_service/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderSrv struct {
	proto.UnimplementedOrderServer // 嵌入未实现的 OrderServer 接口，用于兼容 gRPC 的接口实现
}

// CreateOrder 创建订单
// 生成订单号 查询商品信息（营销中心算价） 扣库存 生成支付信息 调用收货地址 通知商家
// 简化版：生成订单号 查询商品信息 扣库存
// 1. 生成订单号 2.查询商品信息 3.扣库存
func (s *OrderSrv) CreateOrder(ctx context.Context, req *proto.CreateOrderReq) (*proto.CreateOrderRep, error) {
	fmt.Println("in CreateOrder ... ") // 打印进入方法的日志

	// 参数处理
	if req.GetUserId() <= 0 { // 检查请求中的用户ID是否有效
		// 无效的请求
		return nil, status.Error(codes.InvalidArgument, "请求参数有误") // 返回 gRPC 的 InvalidArgument 错误
	}

	// 业务处理
	resp, err := order.Create(ctx, req) // 调用业务逻辑层的 Create 方法处理订单创建
	if err != nil {
		zap.L().Error("order.Create failed", zap.Error(err)) // 记录错误日志
		return nil, status.Error(codes.Internal, "内部错误")     // 返回 gRPC 的 Internal 错误
	}

	return resp, nil // 返回空响应，表示操作成功
}
