package libwallet

import (
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/chaincfg"

	"gitlab.com/raedah/cryptopower/libwallet/assets/btc"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

func initializeBTCWalletParameters(netType utils.NetworkType) (*chaincfg.Params, error) {
	chainParams, err := utils.BTCChainParams(netType)
	if err != nil {
		return chainParams, err
	}

	return chainParams, nil
}

func (mgr *AssetsManager) CreateNewBTCWallet(walletName, privatePassphrase string, privatePassphraseType int32) (sharedW.Asset, error) {
	pass := &sharedW.WalletAuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	wallet, err := btc.CreateNewWallet(pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.BTC.Wallets[wallet.GetWalletID()] = wallet

	// extract the db interface if it hasn't been set already.
	if mgr.db == nil && wallet != nil {
		mgr.setDBInterface(wallet.(sharedW.AssetsManagerDB))
	}

	return wallet, nil
}

func (mgr *AssetsManager) CreateNewBTCWatchOnlyWallet(walletName, extendedPublicKey string) (sharedW.Asset, error) {
	wallet, err := btc.CreateWatchOnlyWallet(walletName, extendedPublicKey, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.BTC.Wallets[wallet.GetWalletID()] = wallet

	// extract the db interface if it hasn't been set already.
	if mgr.db == nil && wallet != nil {
		mgr.setDBInterface(wallet.(sharedW.AssetsManagerDB))
	}

	return wallet, nil
}

func (mgr *AssetsManager) RestoreBTCWallet(walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (sharedW.Asset, error) {
	pass := &sharedW.WalletAuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	wallet, err := btc.RestoreWallet(seedMnemonic, pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.BTC.Wallets[wallet.GetWalletID()] = wallet

	// extract the db interface if it hasn't been set already.
	if mgr.db == nil && wallet != nil {
		mgr.setDBInterface(wallet.(sharedW.AssetsManagerDB))
	}

	return wallet, nil
}

func (mgr *AssetsManager) DeleteBTCWallet(walletID int, privPass string) error {
	wallet := mgr.BTCWalletWithID(walletID)

	// SetNetworkCancelCallback(wallet.SafelyCancelSyncOnly) called before the
	// asset interface is loaded guarantees that sync shutdown will happen
	// before upstream wallet deletion happens.
	err := wallet.DeleteWallet(privPass)
	if err != nil {
		return err
	}

	delete(mgr.Assets.BTC.Wallets, walletID)

	return nil
}

func (mgr *AssetsManager) BTCWalletWithID(walletID int) sharedW.Asset {
	if wallet, ok := mgr.Assets.BTC.Wallets[walletID]; ok {
		return wallet
	}
	return nil
}

// BTCWalletWithXPub returns the ID of the BTC wallet that has an account with the
// provided xpub. Returns -1 if there is no such wallet.
func (mgr *AssetsManager) BTCWalletWithXPub(xpub string) (int, error) {
	return -1, errors.New("Not implemented")
}

// BTCWalletWithSeed returns the ID of the BTC wallet that was created or restored
// using the same seed as the one provided. Returns -1 if no wallet uses the
// provided seed.
func (mgr *AssetsManager) BTCWalletWithSeed(seedMnemonic string) (int, error) {
	return -1, errors.New("Not implemented")
}
