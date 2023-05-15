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
		return "wss://ethstats.dev", nil
	case ETHGoerliParams:
		return "https://stats.goerli.net/", nil
	case ETHRinkebyParams:
		return "https://stats.rinkeby.io/", nil
	case ETHSepoliaParams:
		return `sepolia-instance:5cc663205a01d0bb80933f4b5d48d300c55be0b5@wss://stats.noderpc.xyz`, nil
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
		bootnodes := params.SepoliaBootnodes
		// Extra boot nodes documented here: https://github.com/eth-clients/sepolia#meta-data-sepolia
		bootnodes = append(bootnodes, []string{
			"enode://9246d00bc8fd1742e5ad2428b80fc4dc45d786283e05ef6edbd9002cbc335d40998444732fbe921cb88e1d2c73d1b1de53bae6a2237996e9bfe14f871baf7066@18.168.182.86:30303",
			"enode://ec66ddcf1a974950bd4c782789a7e04f8aa7110a72569b6e65fcd51e937e74eed303b1ea734e4d19cfaec9fbff9b6ee65bf31dcb50ba79acce9dd63a6aca61c7@52.14.151.177:30303",
		}...)
		return bootnodes, nil
	default:
		return nil, fmt.Errorf("no valid chain config params provided")
	}
}
