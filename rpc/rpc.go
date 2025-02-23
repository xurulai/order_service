package rpc

import (
	"errors"
	"fmt"
	"order_service/config"  // 导入配置模块
	"order_service/proto"   // 导入服务的 Protobuf 定义

	_ "github.com/mbobakov/grpc-consul-resolver" // 导入 Consul 的 gRPC 解析器
	"google.golang.org/grpc"                     // 导入 gRPC 库
	"google.golang.org/grpc/credentials/insecure" // 导入不安全的传输凭证（仅用于开发环境）
)

// 定义消息类型常量
const (
	MsgType = iota // iota 是 Go 语言中的一个特殊关键字，用于生成递增的常量值
	MsyType2
	MsyType3
	MsyType4 // 3 -> 4

	MsyType3_2 // 中间插了一个
)

// 初始化其他服务的 RPC 客户端
var (
	GoodsCli proto.GoodsClient // 商品服务的 gRPC 客户端
	StockCli proto.StockClient // 库存服务的 gRPC 客户端
)

// InitSrvClient 初始化 RPC 客户端
func InitSrvClient() error {
	// 检查配置中的服务名称是否有效
	if len(config.Conf.GoodsService.Name) == 0 {
		return errors.New("invalid GoodsService.Name")
	}
	if len(config.Conf.StockService.Name) == 0 {
		return errors.New("invalid StockService.Name")
	}

	// consul 实现服务发现
	// 程序启动时，通过 Consul 获取可用的商品服务地址
	goodsConn, err := grpc.Dial(
		fmt.Sprintf("consul://%s/%s?wait=14s", config.Conf.ConsulConfig.Addr, config.Conf.GoodsService.Name),
		// 指定 round_robin 负载均衡策略
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
		// 使用不安全的传输凭证（仅用于开发环境）
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("dial goods_srv failed, err:%v\n", err)
		return err
	}
	GoodsCli = proto.NewGoodsClient(goodsConn) // 创建商品服务的 gRPC 客户端

	// 同样的方式初始化库存服务的 gRPC 客户端
	stockConn, err := grpc.Dial(
		fmt.Sprintf("consul://%s/%s?wait=14s", config.Conf.ConsulConfig.Addr, config.Conf.StockService.Name),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("dial stock_srv failed, err:%v\n", err)
		return err
	}
	StockCli = proto.NewStockClient(stockConn) // 创建库存服务的 gRPC 客户端

	return nil // 初始化成功
}