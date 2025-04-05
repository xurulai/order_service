package main

import (
	"flag"
	"fmt"
	"net"
	"order_service/biz/order"
	"order_service/config"
	"order_service/dao/mq"
	"order_service/dao/mysql"
	"order_service/dao/redis"
	"order_service/handler"
	"order_service/logger"
	"order_service/proto"
	"order_service/registry"
	"order_service/third_party/snowflake"
	"os"
	"os/signal"
	"syscall"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	var cfn string
	// 0. 从命令行获取配置文件路径，默认值为 "./conf/config.yaml"
	// 例如：stock_service -conf="./conf/config_qa.yaml"
	flag.StringVar(&cfn, "conf", "./conf/config.yaml", "指定配置文件路径")
	flag.Parse()

	// 1. 加载配置文件
	err := config.Init(cfn)
	if err != nil {
		panic(err) // 如果加载配置文件失败，直接退出程�?
	}

	// 2. 初始化日志模�?
	err = logger.Init(config.Conf.LogConfig, config.Conf.Mode)
	if err != nil {
		panic(err) // 如果初始化日志模块失败，直接退出程�?
	}

	// 3. 初始�? MySQL 数据库连�?
	err = mysql.Init(config.Conf.MySQLConfig)
	if err != nil {
		panic(err) // 如果初始�? MySQL 数据库失败，直接退出程�?
	}

	// 初始�? Redis 连接
	err = redis.Init(config.Conf.RedisConfig)
	if err != nil {
		panic(err) // 如果初始�? Redis 失败，直接退出程�?
	}
	// 6. 初始化snowflake
	err = snowflake.Init(config.Conf.StartTime, config.Conf.MachineID)
	if err != nil {
		panic(err)
	}
	// 7. 初始化rocketmq
	err = mq.Init()
	if err != nil {
		panic(err)
	}
	// 监听订单超时的消�?
	c, _ := rocketmq.NewPushConsumer(
		consumer.WithGroupName("order_srv_1"),
		consumer.WithNsResolver(primitive.NewPassthroughResolver([]string{"127.0.0.1:9876"})),
	)
	// 订阅topic
	err = c.Subscribe("xx_pay_timeout", consumer.MessageSelector{}, order.OrderTimeouthandle)
	if err != nil {
		fmt.Println(err.Error())
	}
	// Note: start after subscribe
	err = c.Start()
	if err != nil {
		panic(err)
	}

	err = registry.Init(config.Conf.ConsulConfig.Addr)
	if err != nil {
		zap.L().Error("Failed to initialize Consul", zap.Error(err))
		// 可以选择退出或继续运行，取决于业务需�?
		panic(err)
	}

	// 监听端口
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Conf.Port))
	if err != nil {
		panic(err)
	}

	// 创建 gRPC 服务
	s := grpc.NewServer()
	// 注册健康检查服�?
	grpc_health_v1.RegisterHealthServer(s, health.NewServer())
	proto.RegisterOrderServer(s, &handler.OrderSrv{})

	// 启动 gRPC 服务
	go func() {
		err = s.Serve(lis)
		if err != nil {
			panic(err)
		}
	}()

	// 注册服务�? Consul
	err = registry.Reg.RegisterService(config.Conf.Name, config.Conf.IP, config.Conf.Port, nil)
	if err != nil {
		zap.L().Error("Failed to register service to Consul", zap.Error(err))
		// 可以选择退出或继续运行，取决于业务需�?
		panic(err)

	}
	// 打印 gRPC 服务启动日志
	zap.L().Info(
		"rpc server start",
		zap.String("ip", config.Conf.IP),
		zap.Int("port", config.Conf.Port),
	)

	// 服务退出时注销服务
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit // 等待退出信�?

	// 注销服务
	serviceId := fmt.Sprintf("%s-%s-%d", config.Conf.Name, config.Conf.IP, config.Conf.Port)
	registry.Reg.Deregister(serviceId)
}
