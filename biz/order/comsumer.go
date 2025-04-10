package order

import (
	"context"
	"encoding/json"
	"fmt"
	"order_service/config"
	"order_service/dao/mq"
	"order_service/dao/mysql"
	"order_service/model"
	"order_service/proto"
	"order_service/rpc"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go.uber.org/zap"
)

// OrderTimeouthandle 是处理订单超时消息的回调函数
// 消费者处理订单超时消息
// 添加消费者消息重试机制，超过重试次数则会存入死信队列，后续进行人工处理。
func OrderTimeouthandle(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	for _, msg := range msgs {
		// 检查消息主题是否是订单超时主题
		if msg.Topic != config.Conf.RocketMqConfig.Topic.PayTimeOut {
			zap.L().Info("Message topic does not match order timeout topic, skipping", zap.String("topic", msg.Topic))
			continue
		}

		// 解析消息内容
		var orderDetail model.OrderDetail
		err := json.Unmarshal(msg.Body, &orderDetail)
		if err != nil {
			zap.L().Error("Failed to unmarshal order detail", zap.Error(err))
			return consumer.ConsumeRetryLater, err
		}

		// 判断订单是否超时
		switch orderDetail.Status {
		case "unpaid":
			// 如果订单状态为“未支付”，执行超时处理逻辑
			zap.L().Info("Order is unpaid, processing timeout", zap.Int64("OrderId", orderDetail.OrderId))

			// 1. 回滚库存
			_, err = rpc.StockCli.RollbackStock(context.Background(), &proto.ReduceStockInfo{
				GoodsId: orderDetail.GoodsId,
				Num:     orderDetail.Num,
				OrderId: orderDetail.OrderId,
			})
			if err != nil {
				zap.L().Error("Failed to rollback stock", zap.Error(err), zap.Int64("OrderId", orderDetail.OrderId))
				return consumer.ConsumeRetryLater, err
			}

			// 2. 更新订单状态为“已超时”
			orderDetail.Status = "timeout"
			err = mysql.UpdateOrderStatus(context.Background(), &orderDetail)
			if err != nil {
				zap.L().Error("Failed to update order status to timeout", zap.Error(err), zap.Int64("OrderId", orderDetail.OrderId))
				return consumer.ConsumeRetryLater, err
			}

			// 3. 发送超时通知（可选）
			// utils.SendOrderTimeoutNotification(orderDetail.OrderId)
		case "paid", "cancelled", "timeout":
			// 如果订单状态不是“未支付”，记录日志并忽略
			zap.L().Info("Order already processed, ignoring timeout message",
				zap.Int64("OrderId", orderDetail.OrderId),
				zap.String("status", orderDetail.Status))
		default:
			zap.L().Error("Unknown order status", zap.String("status", orderDetail.Status))
			return consumer.ConsumeRetryLater, fmt.Errorf("unknown order status: %s", orderDetail.Status)
		}

		// 检查重试次数
		reconsumeTimes := msg.ReconsumeTimes
		maxReconsumeTimes := int32(3) // 最大重试次数

		if reconsumeTimes >= maxReconsumeTimes {
			// 发送到死信队列
			deadLetterMsg := primitive.NewMessage("dead_letter_queue", msg.Body)
			_, err := mq.Producer.SendSync(context.Background(), deadLetterMsg)
			if err != nil {
				zap.L().Error("Failed to send message to dead letter queue", zap.Error(err))
				return consumer.ConsumeRetryLater, err
			}
			zap.L().Info("Message moved to dead letter queue", zap.Int64("OrderId", orderDetail.OrderId))
			return consumer.ConsumeSuccess, nil
		}
	}

	return consumer.ConsumeSuccess, nil
}
