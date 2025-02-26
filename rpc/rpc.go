package rpc

import (
	"errors"
	"fmt"
	"time"

	"order_service/config"
	"order_service/proto"

	_ "github.com/mbobakov/grpc-consul-resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	GoodsCli proto.GoodsClient
	StockCli proto.StockClient
)

func InitSrvClient() error {
	// 验证配置
	if len(config.Conf.GoodsService.Name) == 0 {
		return errors.New("invalid GoodsService.Name")
	}
	if len(config.Conf.StockService.Name) == 0 {
		return errors.New("invalid StockService.Name")
	}
	if len(config.Conf.ConsulConfig.Addr) == 0 {
		return errors.New("ConsulConfig.Addr is required")
	}

	// 初始化商品服务客户端
	goodsConn, err := grpc.Dial(
		fmt.Sprintf("consul://%s/%s?wait=14s", config.Conf.ConsulConfig.Addr, config.Conf.GoodsService.Name),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),                // 等待连接建立
		grpc.WithTimeout(5*time.Second), // 设置超时时间
	)
	if err != nil {
		fmt.Printf("Failed to dial goods_srv: ConsulAddr=%s, ServiceName=%s, err=%v\n",
			config.Conf.ConsulConfig.Addr, config.Conf.GoodsService.Name, err)
		return err
	}
	GoodsCli = proto.NewGoodsClient(goodsConn)

	// 初始化库存服务客户端
	stockConn, err := grpc.Dial(
		fmt.Sprintf("consul://%s/%s?wait=14s", config.Conf.ConsulConfig.Addr, config.Conf.StockService.Name),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		fmt.Printf("Failed to dial stock_srv: ConsulAddr=%s, ServiceName=%s, err=%v\n",
			config.Conf.ConsulConfig.Addr, config.Conf.StockService.Name, err)
		return err
	}
	StockCli = proto.NewStockClient(stockConn)

	return nil
}
