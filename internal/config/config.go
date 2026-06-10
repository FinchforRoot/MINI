package config

import "01-mini/internal/client"

func LoadConfig() (*client.EthConfig, error) {
	return client.LoadEthConfig()
}
