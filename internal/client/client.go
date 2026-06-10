package client

import (
	"context"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type EthConfig struct {
	RPCURL       string // WebSocket 或 HTTP RPC 地址
	ContractAddr string // ERC-20 合约地址
}

// EthClient 封装了以太坊客户端和相关配置
type EthClient struct {
	Client       *ethclient.Client
	ContractAddr common.Address
	Context      context.Context
	Cancel       context.CancelFunc
}

// LoadEthConfig 从环境变量加载配置
func LoadEthConfig() (*EthConfig, error) {
	rpcURL := os.Getenv("ETH_WS_URL")
	if rpcURL == "" {
		rpcURL = os.Getenv("ETH_RPC_URL")
	}
	if rpcURL == "" {
		return nil, fmt.Errorf("ETH_WS_URL or ETH_RPC_URL must be set")
	}

	contractHex := os.Getenv("ERC20_CONTRACT")
	if contractHex == "" {
		return nil, fmt.Errorf("ERC20_CONTRACT env is not set")
	}

	return &EthConfig{
		RPCURL:       rpcURL,
		ContractAddr: contractHex,
	}, nil
}

// NewEthClient 创建并初始化以太坊客户端
func NewEthClient(cfg *EthConfig) (*EthClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client, err := ethclient.DialContext(ctx, cfg.RPCURL)
	if err != nil {
		cancel() // 清理 context
		return nil, err
	}

	return &EthClient{
		Client:       client,
		ContractAddr: common.HexToAddress(cfg.ContractAddr),
		Context:      ctx,
		Cancel:       cancel,
	}, nil
}

// Close 关闭客户端连接
func (ec *EthClient) Close() {
	ec.Cancel()       // 取消 context
	ec.Client.Close() // 关闭连接
}

// GetContractAddress 获取合约地址
func (ec *EthClient) GetContractAddress() common.Address {
	return ec.ContractAddr
}
