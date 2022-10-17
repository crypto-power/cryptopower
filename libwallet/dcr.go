package libwallet

import (
	"os"
	"path/filepath"

	"decred.org/dcrwallet/v2/errors"

	"github.com/decred/dcrd/chaincfg/v3"

	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	"gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
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

func (mw *MultiWallet) CreateNewDCRWallet(walletName, privatePassphrase string, privatePassphraseType int32) (*dcr.DCRAsset, error) {
	pass := &wallet.WalletAuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	wallet, err := dcr.CreateNewWallet(pass, mw.params)
	if err != nil {
		return nil, err
	}

	mw.Assets.DCR.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) CreateNewDCRWatchOnlyWallet(walletName, extendedPublicKey string) (*dcr.DCRAsset, error) {
	wallet, err := dcr.CreateWatchOnlyWallet(walletName, extendedPublicKey, mw.params)
	if err != nil {
		return nil, err
	}

	mw.Assets.DCR.Wallets[wallet.ID] = wallet

	return wallet, nil
}

func (mw *MultiWallet) RestoreDCRWallet(walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (*dcr.DCRAsset, error) {
	pass := &wallet.WalletAuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	wallet, err := dcr.RestoreWallet(seedMnemonic, pass, mw.params)
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

	err := mw.params.DB.DeleteStruct(wallet)
	if err != nil {
		return translateError(err)
	}

	os.RemoveAll(wallet.DataDir())
	delete(mw.Assets.DCR.BadWallets, walletID)

	return nil
}

func (mw *MultiWallet) DCRWalletWithID(walletID int) *dcr.DCRAsset {
	if wallet, ok := mw.Assets.DCR.Wallets[walletID]; ok {
		return wallet
	}
	return nil
}
