package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	rpcURL := os.Getenv("ETH_WS_URL")
	if rpcURL == "" {
		rpcURL = os.Getenv("ETH_RPC_URL")
	}
	if rpcURL == "" {
		log.Fatal("ETH_WS_URL or ETH_RPC_URL must be set")
	}

	contractHex := os.Getenv("ERC20_CONTRACT")
	if contractHex == "" {
		log.Fatal("ERC20_CONTRACT env is not set")
	}
	contractAddr := common.HexToAddress(contractHex)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("failed to connect to Ethereum node: %v", err)
	}
	defer client.Close()
	abiBytes, err := os.ReadFile("abi/MyERC20.abi.json")
	if err != nil {
		log.Fatal(err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		log.Fatalf("failed to parse ABI: %v", err)
	}

	eventStore := NewEventStore(100)
	// 1.获取ethClient

	// 2.获取parsedABI对象

	// 3.订阅事件监听

	// 4.启动web服务

	// 5.优雅退出
}
