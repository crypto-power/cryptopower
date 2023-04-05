package libwallet

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/ltcsuite/ltcd/chaincfg"
)

// initializeLTCWalletParameters initializes the fields each LTC wallet is going to need to be setup
func initializeLTCWalletParameters(netType utils.NetworkType) (*chaincfg.Params, error) {
	chainParams, err := utils.LTCChainParams(netType)
	if err != nil {
		return chainParams, err
	}
	return chainParams, nil
}
