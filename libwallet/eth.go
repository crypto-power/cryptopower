package libwallet

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/eth"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/ethereum/go-ethereum/params"
)

// initializeETHWalletParameters initializes the fields each ETH wallet is going to need to be setup
func initializeETHWalletParameters(netType utils.NetworkType) (*params.ChainConfig, error) {
	chainParams, err := utils.ETHChainParams(netType)
	if err != nil {
		return chainParams, err
	}
	return chainParams, nil
}

// CreateNewETHWallet creates a new ETH wallet and returns it.
func (mgr *AssetsManager) CreateNewETHWallet(walletName, privatePassphrase string, privatePassphraseType int32) (sharedW.Asset, error) {
	pass := &sharedW.WalletAuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}

	wallet, err := eth.CreateNewWallet(pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.ETH.Wallets[wallet.GetWalletID()] = wallet

	// extract the db interface if it hasn't been set already.
	if mgr.db == nil && wallet != nil {
		mgr.setDBInterface(wallet.(sharedW.AssetsManagerDB))
	}

	return wallet, nil
}
