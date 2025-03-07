package mq

import (
	"fmt"                  // 标准库，用于格式化输入输出
	"order_service/config" // 自定义配置包，可能包含 RocketMQ 的配置信息

	"github.com/apache/rocketmq-client-go/v2"           // RocketMQ Go 客户端主包
	"github.com/apache/rocketmq-client-go/v2/primitive" // 包含 RocketMQ 的基本数据结构，如消息体
	"github.com/apache/rocketmq-client-go/v2/producer"  // 包含生产者相关功能
)

var (
	Producer rocketmq.Producer // 全局变量，用于存储 RocketMQ 生产者实例
)

// Init 初始化 RocketMQ 生产者
func Init() (err error) {
	// 创建 RocketMQ 生产者实例
	Producer, err = rocketmq.NewProducer(
		// 配置名称服务器地址解析器
		producer.WithNsResolver(primitive.NewPassthroughResolver([]string{config.Conf.RocketMqConfig.Addr})),
		// 设置消息发送失败时的重试次数为 2 次
		producer.WithRetry(2),
		// 设置生产者所属的组名
		producer.WithGroupName(config.Conf.RocketMqConfig.GroupId),
	)
	if err != nil {
		// 如果创建生产者失败，打印错误信息并返回
		fmt.Println(err)
		return err
	}
	// 启动生产者
	err = Producer.Start()
	if err != nil {
		// 如果启动失败，打印错误信息并返回
		fmt.Println(err)
		return
	}
	// 初始化成功，返回 nil
	return nil
}

// Exit 关闭 RocketMQ 生产者
func Exit() error {
	// 调用 Shutdown 方法关闭生产者
	err := Producer.Shutdown()
	if err != nil {
		// 如果关闭失败，打印错误信息
		fmt.Printf("shutdown producer error: %s", err.Error())
	}
	// 返回关闭操作的结果
	return err
}
