package libwallet

import (
	"os"
	"path/filepath"

	"decred.org/dcrwallet/v2/errors"

	"github.com/decred/dcrd/chaincfg/v3"

	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

// initializeDCRWalletParameters initializes the fields each DCR wallet is going to need to be setup
// such as chainparams, root directory, network and database references
func initializeDCRWalletParameters(rootDir, dbDriver string, netType utils.NetworkType) (*chaincfg.Params, string, error) {
	rootDir = filepath.Join(rootDir, string(netType)) // dcr now added in the dcr loader pkg
	err := os.MkdirAll(rootDir, os.ModePerm)
	if err != nil {
		return nil, "", errors.Errorf("failed to create dcr rootDir: %v", err)
	}

	chainParams, err := utils.DCRChainParams(netType)
	if err != nil {
		return chainParams, "", err
	}

	return chainParams, rootDir, nil
}

func (mw *MultiWallet) CreateNewDCRWallet(walletName, privatePassphrase string, privatePassphraseType int32) (*dcr.Wallet, error) {
	wallet, err := dcr.CreateNewWallet(walletName, privatePassphrase, privatePassphraseType, mw.db, mw.rootDir, mw.dbDriver, mw.net)
	if err != nil {
		return nil, err
	}

	mw.Assets.DCR.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) CreateNewDCRWatchOnlyWallet(walletName, extendedPublicKey string) (*dcr.Wallet, error) {
	wallet, err := dcr.CreateWatchOnlyWallet(mw.db, walletName, extendedPublicKey, mw.rootDir, mw.dbDriver, mw.net)
	if err != nil {
		return nil, err
	}

	mw.Assets.DCR.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) RestoreDCRWallet(walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (*dcr.Wallet, error) {
	wallet, err := dcr.RestoreWallet(privatePassphrase, privatePassphraseType, walletName, seedMnemonic, mw.rootDir, mw.dbDriver, mw.db, mw.net)
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
