package utils

import (
	"strings"

	"decred.org/dcrwallet/v2/errors"
	"github.com/decred/dcrd/chaincfg/v3"
)

var (
	mainnetParams = chaincfg.MainNetParams()
	testnetParams = chaincfg.TestNet3Params()
)

func ChainParams(netType string) (*chaincfg.Params, error) {
	switch strings.ToLower(netType) {
	case strings.ToLower(mainnetParams.Name):
		return mainnetParams, nil
	case strings.ToLower(testnetParams.Name):
		return testnetParams, nil
	default:
		return nil, errors.New("invalid net type")
	}
}
