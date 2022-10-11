package utils

import (
	"strings"

	"decred.org/dcrwallet/v2/errors"
	btccfg "github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/chaincfg/v3"
)

var (
	mainnetParams    = chaincfg.MainNetParams()
	testnetParams    = chaincfg.TestNet3Params()
	BTCmainnetParams = &btccfg.MainNetParams
	BTCtestnetParams = &btccfg.TestNet3Params
)

func DCRChainParams(netType string) (*chaincfg.Params, error) {
	switch strings.ToLower(netType) {
	case strings.ToLower(mainnetParams.Name):
		return mainnetParams, nil
	case strings.ToLower(testnetParams.Name):
		return testnetParams, nil
	default:
		return nil, errors.New("invalid net type")
	}
}

func BTCChainParams(netType string) (*btccfg.Params, error) {
	switch strings.ToLower(netType) {
	case strings.ToLower(BTCmainnetParams.Name):
		return BTCmainnetParams, nil
	case strings.ToLower(BTCtestnetParams.Name):
		return BTCtestnetParams, nil
	default:
		return nil, errors.New("invalid net type")
	}
}
