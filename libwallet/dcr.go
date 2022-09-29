package libwallet

import (
	"os"
	"path/filepath"

	"decred.org/dcrwallet/v2/errors"

	"github.com/decred/dcrd/chaincfg/v3"

	"gitlab.com/raedah/cryptopower/libwallet/utils"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/dcr"
)

// initializeDCRWalletParameters initializes the fields each DCR wallet is going to need to be setup
// such as chainparams, root directory, network and database references
func initializeDCRWalletParameters(rootDir, dbDriver, netType string) (*chaincfg.Params, string, error) {
	rootDir = filepath.Join(rootDir, netType, "dcr")
	err := os.MkdirAll(rootDir, os.ModePerm)
	if err != nil {
		return nil, "", errors.Errorf("failed to create dcr rootDir: %v", err)
	}

	chainParams, err := utils.ChainParams(netType)
	if err != nil {
		return chainParams, "", err
	}

	return chainParams, rootDir, nil
}

func (mw *MultiWallet) CreateNewDCRWallet(walletName, privatePassphrase string, privatePassphraseType int32) (*dcr.Wallet, error) {
	wallet, err := dcr.CreateNewWallet(walletName, privatePassphrase, privatePassphraseType, mw.db, mw.Assets.DCR.RootDir, mw.Assets.DCR.DBDriver, mw.Assets.DCR.ChainParams)
	if err != nil {
		return nil, err
	}

	mw.Assets.DCR.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) CreateNewDCRWatchOnlyWallet(walletName, extendedPublicKey string) (*dcr.Wallet, error) {
	wallet, err := dcr.CreateWatchOnlyWallet(walletName, extendedPublicKey, mw.db, mw.Assets.DCR.RootDir, mw.Assets.DCR.DBDriver, mw.Assets.DCR.ChainParams)
	if err != nil {
		return nil, err
	}

	mw.Assets.DCR.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) RestoreDCRWallet(walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (*dcr.Wallet, error) {
	wallet, err := dcr.RestoreWallet(walletName, seedMnemonic, privatePassphrase, privatePassphraseType)
	if err != nil {
		return nil, err
	}

	mw.Assets.DCR.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) DeleteDCRWallet(walletID int, privPass []byte) error {
	wallet := mw.DCRWalletWithID(walletID)

	err := wallet.DeleteWallet(privPass)
	if err != nil {
		return err
	}

	delete(mw.Assets.DCR.Wallets, walletID)

	return nil
}

func (mw *MultiWallet) DeleteBadDCRWallet(walletID int) error {
	wallet := mw.Assets.DCR.BadWallets[walletID]
	if wallet == nil {
		return errors.New(ErrNotExist)
	}

	log.Info("Deleting bad wallet")

	err := mw.db.DeleteStruct(wallet)
	if err != nil {
		return translateError(err)
	}

	os.RemoveAll(wallet.DataDir())
	delete(mw.Assets.DCR.BadWallets, walletID)

	return nil
}

func (mw *MultiWallet) DCRWalletWithID(walletID int) *dcr.Wallet {
	if wallet, ok := mw.Assets.DCR.Wallets[walletID]; ok {
		return wallet
	}
	return nil
}
