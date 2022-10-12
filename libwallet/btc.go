package libwallet

import (
	"os"
	"path/filepath"

	"decred.org/dcrwallet/v2/errors"

	"github.com/btcsuite/btcd/chaincfg"

	"gitlab.com/raedah/cryptopower/libwallet/utils"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/btc"
)

func initializeBTCWalletParameters(rootDir, dbDriver, netType string) (*chaincfg.Params, string, error) {
	rootDir = filepath.Join(rootDir, netType) // btc now added in the btc pkg
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
	wallet, err := btc.CreateNewWallet(walletName, privatePassphrase, privatePassphraseType, mw.db, mw.Assets.BTC.RootDir, mw.Assets.BTC.DBDriver, mw.Assets.BTC.ChainParams)
	if err != nil {
		return nil, err
	}

	mw.Assets.BTC.Wallets[wallet.ID] = wallet

	return wallet, nil
}
