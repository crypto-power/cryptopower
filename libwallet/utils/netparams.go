package utils

import (
	"fmt"
	"strings"

	btccfg "github.com/btcsuite/btcd/chaincfg"
	dcrcfg "github.com/decred/dcrd/chaincfg/v3"
	ltccfg "github.com/ltcsuite/ltcd/chaincfg"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type NetworkType string

const (
	Mainnet    NetworkType = "mainnet"
	Testnet    NetworkType = "testnet"
	Regression NetworkType = "regression"
	Simulation NetworkType = "simulation"
	DEXTest    NetworkType = "dextest"
	Unknown    NetworkType = "unknown"
)

// Display returns the title case network name to be displayed on the app UI.
func (n NetworkType) Display() string {
	caser := cases.Title(language.Und)
	return caser.String(string(n))
}

// ToNetworkType maps the provided network string identifier to the available
// network type constants.
func ToNetworkType(str string) NetworkType {
	switch strings.ToLower(str) {
	case "mainnet":
		return Mainnet
	case "testnet", "testnet3", "test", "testnet4":
		return Testnet
	case "regression", "reg", "regnet":
		return Regression
	case "simulation", "sim", "simnet":
		return Simulation
	case "dextest":
		return DEXTest
	default:
		return Unknown
	}
}

// ChainsParams collectively defines the chain parameters of all assets supported.
type ChainsParams struct {
	DCR *dcrcfg.Params
	BTC *btccfg.Params
	LTC *ltccfg.Params
}

var (
	DCRmainnetParams      = dcrcfg.MainNetParams()
	DCRtestnetParams      = dcrcfg.TestNet3Params()
	DCRSimnetParams       = dcrcfg.SimNetParams()
	DCRRegnetParams       = dcrcfg.RegNetParams()
	BTCmainnetParams      = &btccfg.MainNetParams
	BTCtestnetParams      = &btccfg.TestNet3Params
	BTCSimnetParams       = &btccfg.SimNetParams
	BTCRegnetParamsVal    = btccfg.RegressionNetParams
	LTCmainnetParams      = &ltccfg.MainNetParams
	LTCtestnetParams      = &ltccfg.TestNet4Params
	LTCSimnetParams       = &ltccfg.SimNetParams
	LTCRegnetParamsVal    = ltccfg.RegressionNetParams
	DCRDEXSimnetParams    = dcrcfg.SimNetParams()
	BTCDEXRegnetParamsVal = btccfg.RegressionNetParams
	LTCDEXRegnetParamsVal = ltccfg.RegressionNetParams
)

func init() {
	DCRDEXSimnetParams.DefaultPort = "19560"
	BTCDEXRegnetParamsVal.DefaultPort = "20575"
	LTCDEXRegnetParamsVal.DefaultPort = "20585"
}

// NetDir returns data directory name for a given asset's type and network connected.
// If "unknown" is returned, unsupported asset type or network was detected.
func NetDir(assetType AssetType, netType NetworkType) string {
	dirName := "unknown"
	params, err := GetChainParams(assetType, netType)
	if err != nil {
		return dirName
	}

	switch assetType {
	case BTCWalletAsset:
		dirName = params.BTC.Name
	case DCRWalletAsset:
		dirName = params.DCR.Name
	case LTCWalletAsset:
		dirName = params.LTC.Name
	}

	return strings.ToLower(dirName)
}

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
	case DEXTest:
		return DCRDEXSimnetParams, nil
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
		return &BTCRegnetParamsVal, nil
	case DEXTest:
		return &BTCDEXRegnetParamsVal, nil
	default:
		return nil, fmt.Errorf("%v: (%v)", ErrInvalidNet, netType)
	}
}

// LTCChainParams returns the network parameters from the LTC chain provided
// a network type is given.
func LTCChainParams(netType NetworkType) (*ltccfg.Params, error) {
	switch netType {
	case Mainnet:
		return LTCmainnetParams, nil
	case Testnet:
		return LTCtestnetParams, nil
	case Simulation:
		return LTCSimnetParams, nil
	case Regression:
		return &LTCRegnetParamsVal, nil
	case DEXTest:
		return &LTCDEXRegnetParamsVal, nil
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
	case LTCWalletAsset:
		params, err := LTCChainParams(netType)
		if err != nil {
			return nil, err
		}
		return &ChainsParams{LTC: params}, nil
	default:
		return nil, fmt.Errorf("%v: (%v)", ErrAssetUnknown, assetType)
	}
}
