package libwallet

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/ltc"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/ltcsuite/ltcd/chaincfg"
)

// initializeLTCWalletParameters initializes the fields each LTC wallet is going to need to be setup
func initializeLTCWalletParameters(netType utils.NetworkType) (*chaincfg.Params, error) {
	ltcNetType := netType
	if netType == utils.Testnet {
		ltcNetType = utils.Testnet4
	}
	chainParams, err := utils.LTCChainParams(ltcNetType)
	if err != nil {
		return chainParams, err
	}
	return chainParams, nil
}

// CreateNewLTCWallet creates a new LTC wallet and returns it.
func (mgr *AssetsManager) CreateNewLTCWallet(walletName, privatePassphrase string, privatePassphraseType int32) (sharedW.Asset, error) {
	pass := &sharedW.WalletAuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	ltcNetType := mgr.params.NetType
	if mgr.params.NetType == utils.Testnet {
		ltcNetType = utils.Testnet4
	}
	mgr.params.NetType = ltcNetType
	wallet, err := ltc.CreateNewWallet(pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.LTC.Wallets[wallet.GetWalletID()] = wallet

	// extract the db interface if it hasn't been set already.
	if mgr.db == nil && wallet != nil {
		mgr.setDBInterface(wallet.(sharedW.AssetsManagerDB))
	}

	return wallet, nil
}
