package main

import (
	"01-mini/internal/client"
	"log"
)

func main() {

	ethConfig, err := client.LoadEthConfig()
	if err != nil {
		log.Fatalf("load config error : %v", err)
	}
	ethClient, err := client.NewEthClient(ethConfig)
	if err != nil {
		log.Fatalf("new ethClient fail ,error : %v", err)
	}

	// 1.获取ethClient

	// 2.获取parsedABI对象

	// 3.订阅事件监听

	// 4.启动web服务

	// 5.优雅退出
}
