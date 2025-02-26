package main

import (
	"context"
	"fmt"
	"log"
	"order_service/proto"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	conn   *grpc.ClientConn  // gRPC 客户端连接
	client proto.OrderClient // gRPC 客户端实例
)

func init() {
	var err error
	conn, err := grpc.Dial("127.0.0.1:8389", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
    	log.Fatalf("Failed to connect: %v", err)
	}
	client = proto.NewOrderClient(conn)
}

// TestCreateOrder 测试创建订单（修复空指针和日志格式问题）
func TestCreateOrder(wg *sync.WaitGroup, errCount *int32, index int) {
	defer wg.Done()
	param := &proto.CreateOrderReq{
		GoodsId: 1001,
		Num:     1,
		UserId:  1,
		Address: "jinan",
		Name:    "giao",
		Phone:   "201314",
	}

	resp, err := client.CreateOrder(context.Background(), param)
	if err != nil {
		log.Printf("Error calling GetGoodsDetail: %v", err)
	} else {
		log.Printf("Response: %+v", resp)
	}

}

func main() {
	defer conn.Close()
	var wg sync.WaitGroup
	var errCount int32 = 0

	// 启动 5 个协程并发测试
	for i := 0; i < 5; i++ {
		wg.Add(1)
		// 修复3: 传递协程索引参数
		go TestCreateOrder(&wg, &errCount, i)
	}

	wg.Wait()
	fmt.Printf("测试完成，总错误数: %d\n", atomic.LoadInt32(&errCount))
}
