package utils

import (
	"fmt"
	"strings"

	btccfg "github.com/btcsuite/btcd/chaincfg"
	dcrcfg "github.com/decred/dcrd/chaincfg/v3"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type NetworkType string

const (
	Mainnet    NetworkType = "mainnet"
	Testnet    NetworkType = "testnet3"
	Regression NetworkType = "regression"
	Simulation NetworkType = "simulation"
	Unknown    NetworkType = "unknown"
)

// Display returns the title case network name to be displayed on the app UI.
func (n NetworkType) Display() string {
	switch n {
	case Testnet:
		return "Testnet"
	default:
		caser := cases.Title(language.Und)
		return caser.String(string(n))
	}
}

// ToNetworkType maps the provided network string identifier to the available
// network type constants.
func ToNetworkType(str string) NetworkType {
	switch strings.ToLower(str) {
	case "mainnet":
		return Mainnet
	case "testnet", "testnet3", "test":
		return Testnet
	case "regression", "reg", "regnet":
		return Regression
	case "simulation", "sim", "simnet":
		return Simulation
	default:
		return Unknown
	}
}

// ChainsParams collectively defines the chain parameters of all assets supported.
type ChainsParams struct {
	DCR *dcrcfg.Params
	BTC *btccfg.Params
}

var (
	DCRmainnetParams = dcrcfg.MainNetParams()
	DCRtestnetParams = dcrcfg.TestNet3Params()
	DCRSimnetParams  = dcrcfg.SimNetParams()
	DCRRegnetParams  = dcrcfg.RegNetParams()
	BTCmainnetParams = &btccfg.MainNetParams
	BTCtestnetParams = &btccfg.TestNet3Params
	BTCSimnetParams  = &btccfg.SimNetParams
	BTCRegnetParams  = &btccfg.RegressionNetParams
)

// DCRChainParams returns the network parameters from the DCR chain provided
// a given network.
func DCRChainParams(netType NetworkType) (*dcrcfg.Params, error) {
	switch netType {
	case Mainnet:
		return DCRmainnetParams, nil
	case Testnet:
		return DCRtestnetParams, nil
	case Simulation:
		return DCRSimnetParams, nil
	case Regression:
		return DCRRegnetParams, nil
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
	case Simulation:
		return BTCSimnetParams, nil
	case Regression:
		return BTCRegnetParams, nil
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
