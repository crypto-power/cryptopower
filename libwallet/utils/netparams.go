package utils

import (
	"fmt"

	btccfg "github.com/btcsuite/btcd/chaincfg"
	dcrcfg "github.com/decred/dcrd/chaincfg/v3"
)

type NetworkType string

const (
	Mainnet    NetworkType = "mainnet"
	Testnet    NetworkType = "testnet"
	Staging    NetworkType = "staging"
	Regression NetworkType = "regression"
	Simulation NetworkType = "simulation"
)

// ChainsParams collectively defines the chain parameters of all assets supported.
type ChainsParams struct {
	DCR *dcrcfg.Params
	BTC *btccfg.Params
}

var (
	DCRmainnetParams = dcrcfg.MainNetParams()
	DCRtestnetParams = dcrcfg.TestNet3Params()
	BTCmainnetParams = &btccfg.MainNetParams
	BTCtestnetParams = &btccfg.TestNet3Params
)

// DCRChainParams returns the network parameters from the DCR chain provided
// a given network.
func DCRChainParams(netType NetworkType) (*dcrcfg.Params, error) {
	switch netType {
	case Mainnet:
		return DCRmainnetParams, nil
	case Testnet:
		return DCRtestnetParams, nil
	default:
		return nil, fmt.Errorf("%v: (%v)", ErrInvalidNet, netType)
	}
}

// BTCChainParams returns the network parameters from the BTC chain provided
// a given network.
func BTCChainParams(netType NetworkType) (*btccfg.Params, error) {
	switch netType {
	case Mainnet:
		return BTCmainnetParams, nil
	case Testnet:
		return BTCtestnetParams, nil
	default:
		return nil, fmt.Errorf("%v: (%v)", ErrInvalidNet, netType)
	}
}

// GetChainParams returns the network parameters of a chain provided its
// asset type and network type.
func GetChainParams(assetType AssetType, netType NetworkType) (*ChainsParams, error) {
	switch assetType {
	case BTCWalletAsset:
		params, err := BTCChainParams(netType)
		if err != nil {
			return nil, err
		}
		return &ChainsParams{BTC: params}, nil
	case DCRWalletAsset:
		params, err := DCRChainParams(netType)
		if err != nil {
			return nil, err
		}
		return &ChainsParams{DCR: params}, nil
	default:
		return nil, fmt.Errorf("%v: (%v)", ErrAssetUnknown, assetType)
	}
}
