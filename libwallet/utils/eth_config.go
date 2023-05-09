package utils

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
)

// GetGenesis returns a genesis block based on chain parameter args used.
func GetGenesis(chainParams *params.ChainConfig) (*core.Genesis, error) {
	switch chainParams {
	case ETHMainnetParams:
		return core.DefaultGenesisBlock(), nil
	case ETHGoerliParams:
		return core.DefaultGoerliGenesisBlock(), nil
	case ETHRinkebyParams:
		return core.DefaultRinkebyGenesisBlock(), nil
	case ETHSepoliaParams:
		return core.DefaultSepoliaGenesisBlock(), nil
	default:
		return nil, fmt.Errorf("no valid chain config params provided")
	}
}

// GetEthStatsURL returns the url needed to connect to an Ethstats API online.
func GetEthStatsURL(chainParams *params.ChainConfig) (string, error) {
	switch chainParams {
	case ETHMainnetParams:
		return "https://ethstats.dev/", nil
	case ETHGoerliParams:
		return "https://stats.goerli.net/", nil
	case ETHRinkebyParams:
		return "https://stats.rinkeby.io/", nil
	case ETHSepoliaParams:
		return "https://stats.noderpc.xyz/", nil
	default:
		return "", fmt.Errorf("no valid chain config params provided")
	}
}

// GetBootstrapNodes returns the nodes needed to initialize given network on ethereum.
func GetBootstrapNodes(chainParams *params.ChainConfig) ([]string, error) {
	switch chainParams {
	case ETHMainnetParams:
		return params.MainnetBootnodes, nil
	case ETHGoerliParams:
		return params.GoerliBootnodes, nil
	case ETHRinkebyParams:
		return params.RinkebyBootnodes, nil
	case ETHSepoliaParams:
		return params.SepoliaBootnodes, nil
	default:
		return nil, fmt.Errorf("no valid chain config params provided")
	}
}
