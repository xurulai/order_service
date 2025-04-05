package order

import (
	"context"
	"order_service/dao/mysql"
	"order_service/model"
	"order_service/proto"
	"order_service/rpc"
	"time"

	"go.uber.org/zap"
)

// StartTimeoutScanner 启动定时任务，扫描并处理超时订单
func StartTimeoutScanner(ctx context.Context) {
	zap.L().Info("Starting order timeout scanner")

	// 定时任务：每5分钟执行一次
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			zap.L().Info("Order timeout scanner stopped")
			return
		case <-ticker.C:
			scanAndProcessTimeoutOrders()
		}
	}
}

// scanAndProcessTimeoutOrders 扫描并处理超时订单
func scanAndProcessTimeoutOrders() {
	ctx := context.Background()

	// 计算一周前的时间
	oneWeekAgo := time.Now().Add(-7 * 24 * time.Hour)

	// 获取一周前的订单ID最小值
	minOrderIdOneWeekAgo, err := mysql.GetMinOrderIdAfterTime(ctx, oneWeekAgo)
	if err != nil {
		zap.L().Error("Failed to get min order ID one week ago", zap.Error(err))
		return
	}

	// 获取订单ID分片参数，只针对一周内的订单
	// 获取订单ID分片参数，只针对大于minOrderIdOneWeekAgo的订单
	shardParams, err := mysql.GetShardParams(ctx, minOrderIdOneWeekAgo)
	if err != nil {
		zap.L().Error("Failed to get shard parameters", zap.Error(err))
		return
	}

	for _, param := range shardParams {
		processShard(ctx, param)
	}
}

// processShard 处理单个分片
func processShard(ctx context.Context, param model.ShardParam) {
	zap.L().Info("Processing shard", zap.Int("ShardID", param.ShardID))

	// 查询当前分片的超时订单
	timeoutOrders, err := mysql.QueryTimeoutOrdersByShard(ctx, param.StartID, param.EndID, time.Now().Add(-30*time.Minute))
	if err != nil {
		zap.L().Error("Failed to query timeout orders for shard", zap.Error(err), zap.Int("ShardID", param.ShardID))
		return
	}

	if len(timeoutOrders) == 0 {
		zap.L().Info("No timeout orders found for shard", zap.Int("ShardID", param.ShardID))
		return
	}

	zap.L().Info("Found timeout orders for shard", zap.Int("count", len(timeoutOrders)), zap.Int("ShardID", param.ShardID))

	for _, order := range timeoutOrders {
		processTimeoutOrder(ctx, order)
	}
}

// processTimeoutOrder 处理单个超时订单
func processTimeoutOrder(ctx context.Context, order model.OrderDetail) {
	zap.L().Info("Processing timeout order", zap.Int64("OrderId", order.OrderId))

	// 1. 回滚库存
	_, err := rpc.StockCli.RollbackStock(ctx, &proto.ReduceStockInfo{
		GoodsId: order.GoodsId,
		Num:     order.Num,
		OrderId: order.OrderId,
	})
	if err != nil {
		zap.L().Error("Failed to rollback stock", zap.Error(err), zap.Int64("OrderId", order.OrderId))
		return
	}

	// 2. 更新订单状态为“已超时”
	order.Status = "timeout"
	err = mysql.UpdateOrderStatus(ctx, &order)
	if err != nil {
		zap.L().Error("Failed to update order status to timeout", zap.Error(err), zap.Int64("OrderId", order.OrderId))
		return
	}

	// 3. 发送超时通知（可选）
	// utils.SendOrderTimeoutNotification(order.OrderId)

	zap.L().Info("Order processed successfully", zap.Int64("OrderId", order.OrderId))
}
