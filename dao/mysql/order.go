package mysql

import (
	"context"
	"log"
	"order_service/errno"
	"order_service/model"
	"time"

	"gorm.io/gorm"
)

func QueryOrder(ctx context.Context, orderId int64) (model.Order, error) {
	var data model.Order
	err := db.WithContext(ctx).
		Model(&model.Order{}).
		Where("order_id = ?", orderId).
		First(&data).Error
	return data, err
}

func UpdateOrder(ctx context.Context, data model.Order) error {
	return db.WithContext(ctx).
		Model(&model.Order{}).
		Where("order_id = ?", data.OrderId).
		Updates(&data).Error
}

func CreateOrder(ctx context.Context, data *model.Order) error {
	return db.WithContext(ctx).
		Model(&model.Order{}).
		Save(data).Error
}

func CreateOrderDetail(ctx context.Context, data *model.OrderDetail) error {
	return db.WithContext(ctx).
		Model(&model.OrderDetail{}).
		Save(data).Error
}

// CreateOrderWithTransation 创建订单事务处理
func CreateOrderWithTransation(ctx context.Context, order *model.Order, orderDetail *model.OrderDetail) error {
	return db.WithContext(ctx).
		Transaction(func(tx *gorm.DB) error {
			// 在事务中执行一些 db 操作（从这里开始，您应该使用 'tx' 而不是 'db'）
			if err := tx.Create(order).Error; err != nil {
				// 返回任何错误都会回滚事务
				return err
			}

			if err := tx.Create(orderDetail).Error; err != nil {
				return err
			}
			// 返回 nil 提交事务
			return nil
		})
}

func UpdateOrderStatus(ctx context.Context, order *model.OrderDetail) error {
	// 使用 gorm 的 WithContext 方法，将上下文传递给数据库操作
	result := db.WithContext(ctx).
		// 指定操作的模型，这里操作的是 model.Order 表
		Model(&model.OrderDetail{}).
		// 指定更新条件，根据 order_id 更新
		Where("order_id = ?", order.OrderId).
		// 更新订单状态
		Updates(map[string]interface{}{
			"status": order.Status,
		})

	// 检查更新是否成功
	if result.Error != nil {
		log.Printf("Failed to update order status: %v", result.Error)
		return errno.ErrUpdateFailed
	}

	// 如果没有行被更新，返回错误
	if result.RowsAffected == 0 {
		log.Printf("No rows affected for orderId: %d", order.OrderId)
		return errno.ErrOrderNotFound
	}

	// 更新成功，返回 nil
	return nil
}


// GetMinOrderIdAfterTime 获取指定时间后的最小订单ID
func GetMinOrderIdAfterTime(ctx context.Context, timestamp time.Time) (int64, error) {
	var minID int64
	err := db.WithContext(ctx).
		Model(&model.Order{}).
		Where("created_at > ?", timestamp).
		Select("MIN(id)").
		Row().
		Scan(&minID)
	if err != nil {
		return 0, err
	}
	return minID, nil
}

// GetShardParams 获取订单ID分片参数，只针对大于minOrderId的订单
// GetShardParams 获取订单ID分片参数，只针对大于minOrderId的订单
func GetShardParams(ctx context.Context, minOrderId int64) ([]model.ShardParam, error) {
	// 查询订单表中大于minOrderId的最小和最大ID
	var minID, maxID int64
	err := db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id > ?", minOrderId).
		Select("MIN(id), MAX(id)").
		Row().
		Scan(&minID, &maxID)
	if err != nil {
		return nil, err
	}

	// 如果没有符合条件的订单，返回空分片
	if minID == 0 && maxID == 0 {
		return nil, nil
	}

	// 计算分片数量
	shardCount := int64(10) // 将 shardCount 转换为 int64
	totalRange := maxID - minID + 1
	shardSize := totalRange / shardCount

	var shardParams []model.ShardParam
	for i := int64(0); i < shardCount; i++ {
		startID := minID + i*shardSize
		endID := minID + (i+1)*shardSize - 1
		if i == shardCount-1 {
			endID = maxID
		}
		shardParams = append(shardParams, model.ShardParam{
			StartID: startID,
			EndID:   endID,
			ShardID: int(i),
		})
	}

	return shardParams, nil
}

// QueryTimeoutOrdersByShard 按分片查询超时未支付的订单
func QueryTimeoutOrdersByShard(ctx context.Context, startID, endID int64, timeoutTime time.Time) ([]model.OrderDetail, error) {
	var orders []model.OrderDetail
	err := db.WithContext(ctx).
		Where("id BETWEEN ? AND ? AND status = ? AND created_at < ?", startID, endID, "unpaid", timeoutTime).
		Find(&orders).
		Error
	if err != nil {
		return nil, err
	}
	return orders, nil
}
