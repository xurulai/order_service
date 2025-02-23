package main

import (
	"flag"
	"fmt"
	"net"
	"order_service/config"
	"order_service/dao/mysql"
	"order_service/logger"
	"order_service/registry"
	"order_service/rpc"
	"order_service/third_party/snowflake"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	//"google.golang.org/grpc/health"
	//"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// 可以把启动时的一些列操作放到 bootstap/init 包

	var cfn string
	// 0.从命令行获取可能的conf路径
	flag.StringVar(&cfn, "conf", "./conf/config.yaml", "指定配置文件路径")
	flag.Parse()
	// 1. 加载配置文件
	err := config.Init(cfn)
	if err != nil {
		panic(err) // 程序启动时加载配置文件失败直接退出
	}
	// 2. 加载日志
	err = logger.Init(config.Conf.LogConfig, config.Conf.Mode)
	if err != nil {
		panic(err) // 程序启动时初始化日志模块失败直接退出
	}
	// 3. 初始化MySQL
	err = mysql.Init(config.Conf.MySQLConfig)
	if err != nil {
		panic(err) // 程序启动时初始化MySQL失败直接退出
	}
	// 4. 初始化Consul
	err = registry.Init(config.Conf.ConsulConfig.Addr)
	if err != nil {
		panic(err) // 程序启动时初始化注册中心失败直接退出
	}

	// 5.初始化RPC客户端
	err = rpc.InitSrvClient()
	if err != nil {
		panic(err)
	}

	// 6. 初始化snowflake
	err = snowflake.Init(config.Conf.StartTime, config.Conf.MachineID)
	if err != nil {
		panic(err)
	}

	// 监听端口
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Conf.Port))
	if err != nil {
		panic(err)
	}
	// 创建gRPC服务
	s := grpc.NewServer()
	// 健康检查
	//grpc_health_v1.RegisterHealthServer(s, health.NewServer())
	//proto.RegisterOrderServer(s, &handler.OrderSrv{})
	// 商品服务注册RPC服务
	// 启动gRPC服务
	go func() {
		err = s.Serve(lis)
		if err != nil {
			panic(err)
		}
	}()

	// 注册服务到consul
	registry.Reg.RegisterService(config.Conf.Name, config.Conf.IP, config.Conf.Port, nil)

	zap.L().Info("service start...")

	// 服务退出时要注销服务
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit // 正常会hang在此处
	// 退出时注销服务
	serviceId := fmt.Sprintf("%s-%s-%d", config.Conf.Name, config.Conf.IP, config.Conf.Port)
	registry.Reg.Deregister(serviceId)
}
