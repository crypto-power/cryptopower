package libwallet

import (
	"os"
	"path/filepath"

	"decred.org/dcrwallet/v2/errors"

	"github.com/btcsuite/btcd/chaincfg"

	"gitlab.com/raedah/cryptopower/libwallet/assets/btc"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

func initializeBTCWalletParameters(rootDir, dbDriver string, netType utils.NetworkType) (*chaincfg.Params, string, error) {
	rootDir = filepath.Join(rootDir, string(netType)) // btc now added in the btc loader pkg
	err := os.MkdirAll(rootDir, os.ModePerm)
	if err != nil {
		return nil, "", errors.Errorf("failed to create btc rootDir: %v", err)
	}

	chainParams, err := utils.BTCChainParams(netType)
	if err != nil {
		return chainParams, "", err
	}

	return chainParams, rootDir, nil
}

func (mw *MultiWallet) CreateNewBTCWallet(walletName, privatePassphrase string, privatePassphraseType int32) (*btc.Wallet, error) {
	wallet, err := btc.CreateNewWallet(walletName, privatePassphrase, privatePassphraseType, mw.db, mw.rootDir, mw.dbDriver, mw.net)
	if err != nil {
		return nil, err
	}

	mw.Assets.BTC.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) CreateNewBTCWatchOnlyWallet(walletName, extendedPublicKey string) (*btc.Wallet, error) {
	wallet, err := btc.CreateWatchOnlyWallet(mw.db, walletName, extendedPublicKey, mw.rootDir, mw.dbDriver, mw.net)
	if err != nil {
		return nil, err
	}

	mw.Assets.BTC.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) RestoreBTCWallet(walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (*btc.Wallet, error) {
	wallet, err := btc.RestoreWallet(privatePassphrase, privatePassphraseType, walletName, seedMnemonic, mw.rootDir, mw.dbDriver, mw.db, mw.net)
	if err != nil {
		return nil, err
	}

	mw.Assets.BTC.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) DeleteBTCWallet(walletID int, privPass []byte) error {
	wallet := mw.BTCWalletWithID(walletID)

	err := wallet.DeleteWallet(privPass)
	if err != nil {
		return err
	}

	delete(mw.Assets.BTC.Wallets, walletID)

	return nil
}

func (mw *MultiWallet) BTCWalletWithID(walletID int) *btc.Wallet {
	if wallet, ok := mw.Assets.BTC.Wallets[walletID]; ok {
		return wallet
	}
	return nil
}
