package libwallet

import (
	"context"
	"os"

	"decred.org/dcrwallet/v2/errors"
	"decred.org/dcrwallet/walletseed"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/hdkeychain/v3"

	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

// initializeDCRWalletParameters initializes the fields each DCR wallet is going to need to be setup
// such as chainparams.
func initializeDCRWalletParameters(netType utils.NetworkType) (*chaincfg.Params, error) {
	chainParams, err := utils.DCRChainParams(netType)
	if err != nil {
		return chainParams, err
	}
	return chainParams, nil
}

func (mgr *AssetsManager) CreateNewDCRWallet(walletName, privatePassphrase string, privatePassphraseType int32) (*dcr.DCRAsset, error) {
	pass := &sharedW.WalletAuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	wallet, err := dcr.CreateNewWallet(pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.DCR.Wallets[wallet.ID] = wallet

	// extract the db interface if it hasn't been set already.
	if mgr.db == nil && wallet != nil {
		mgr.setDBInterface(wallet)
	}

	return wallet, nil
}

func (mgr *AssetsManager) CreateNewDCRWatchOnlyWallet(walletName, extendedPublicKey string) (*dcr.DCRAsset, error) {
	wallet, err := dcr.CreateWatchOnlyWallet(walletName, extendedPublicKey, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.DCR.Wallets[wallet.ID] = wallet

	// extract the db interface if it hasn't been set already.
	if mgr.db == nil && wallet != nil {
		mgr.setDBInterface(wallet)
	}

	return wallet, nil
}

func (mgr *AssetsManager) RestoreDCRWallet(walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (*dcr.DCRAsset, error) {
	pass := &sharedW.WalletAuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	wallet, err := dcr.RestoreWallet(seedMnemonic, pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.DCR.Wallets[wallet.ID] = wallet

	// extract the db interface if it hasn't been set already.
	if mgr.db == nil && wallet != nil {
		mgr.setDBInterface(wallet)
	}

	return wallet, nil
}

func (mgr *AssetsManager) DeleteDCRWallet(walletID int, privPass string) error {
	wallet := mgr.DCRWalletWithID(walletID)

	wallet.SetNetworkCancelCallback(wallet.SafelyCancelSyncOnly)
	err := wallet.DeleteWallet(privPass)
	if err != nil {
		return err
	}

	delete(mgr.Assets.DCR.Wallets, walletID)

	return nil
}

func (mgr *AssetsManager) DeleteBadDCRWallet(walletID int) error {
	wallet := mgr.Assets.DCR.BadWallets[walletID]
	if wallet == nil {
		return errors.New(utils.ErrNotExist)
	}

	log.Info("Deleting bad wallet")

	err := mgr.params.DB.DeleteStruct(wallet)
	if err != nil {
		return utils.TranslateError(err)
	}

	os.RemoveAll(wallet.DataDir())
	delete(mgr.Assets.DCR.BadWallets, walletID)

	return nil
}

func (mgr *AssetsManager) DCRWalletWithID(walletID int) *dcr.DCRAsset {
	if wallet, ok := mgr.Assets.DCR.Wallets[walletID]; ok {
		return wallet
	}
	return nil
}

// DCRWalletWithXPub returns the ID of the DCR wallet that has an account with the
// provided xpub. Returns -1 if there is no such wallet.
func (mgr *AssetsManager) DCRWalletWithXPub(xpub string) (int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, w := range mgr.Assets.DCR.Wallets {
		if !w.WalletOpened() {
			return -1, errors.Errorf("wallet %d is not open and cannot be checked", w.ID)
		}
		accounts, err := w.Internal().DCR.Accounts(ctx)
		if err != nil {
			return -1, err
		}
		for _, account := range accounts.Accounts {
			if account.AccountNumber == dcr.ImportedAccountNumber {
				continue
			}
			acctXPub, err := w.Internal().DCR.AccountXpub(ctx, account.AccountNumber)
			if err != nil {
				return -1, err
			}
			if acctXPub.String() == xpub {
				return w.ID, nil
			}
		}
	}
	return -1, nil
}

// DCRWalletWithSeed returns the ID of the DCR wallet that was created or restored
// using the same seed as the one provided. Returns -1 if no wallet uses the
// provided seed.
func (mgr *AssetsManager) DCRWalletWithSeed(seedMnemonic string) (int, error) {
	if len(seedMnemonic) == 0 {
		return -1, errors.New(utils.ErrEmptySeed)
	}

	newSeedLegacyXPUb, newSeedSLIP0044XPUb, err := deriveBIP44AccountXPubsForDCR(seedMnemonic,
		dcr.DefaultAccountNum, mgr.chainsParams.DCR)
	if err != nil {
		return -1, err
	}

	for _, wallet := range mgr.Assets.DCR.Wallets {
		if !wallet.WalletOpened() {
			return -1, errors.Errorf("cannot check if seed matches unloaded wallet %d", wallet.ID)
		}
		// NOTE: Existing watch-only wallets may have been created using the
		// xpub of an account that is NOT the default account and may return
		// incorrect result from the check below. But this would return true
		// if the watch-only wallet was created using the xpub of the default
		// account of the provided seed.
		usesSameSeed, err := wallet.AccountXPubMatches(dcr.DefaultAccountNum, newSeedLegacyXPUb, newSeedSLIP0044XPUb)
		if err != nil {
			return -1, err
		}
		if usesSameSeed {
			return wallet.ID, nil
		}
	}

	return -1, nil
}

// deriveBIP44AccountXPubForDCR derives and returns the legacy and SLIP0044 account
// xpubs using the BIP44 HD path for accounts: m/44'/<coin type>'/<account>'.
func deriveBIP44AccountXPubsForDCR(seedMnemonic string, account uint32, params *chaincfg.Params) (string, string, error) {
	seed, err := walletseed.DecodeUserInput(seedMnemonic)
	if err != nil {
		return "", "", err
	}
	defer func() {
		for i := range seed {
			seed[i] = 0
		}
	}()

	// Derive the master extended key from the provided seed.
	masterNode, err := hdkeychain.NewMaster(seed, params)
	if err != nil {
		return "", "", err
	}
	defer masterNode.Zero()

	// Derive the purpose key as a child of the master node.
	purpose, err := masterNode.Child(44 + hdkeychain.HardenedKeyStart)
	if err != nil {
		return "", "", err
	}
	defer purpose.Zero()

	accountXPub := func(coinType uint32) (string, error) {
		coinTypePrivKey, err := purpose.Child(coinType + hdkeychain.HardenedKeyStart)
		if err != nil {
			return "", err
		}
		defer coinTypePrivKey.Zero()
		acctPrivKey, err := coinTypePrivKey.Child(account + hdkeychain.HardenedKeyStart)
		if err != nil {
			return "", err
		}
		defer acctPrivKey.Zero()
		return acctPrivKey.Neuter().String(), nil
	}

	legacyXPUb, err := accountXPub(params.LegacyCoinType)
	if err != nil {
		return "", "", err
	}
	slip0044XPUb, err := accountXPub(params.SLIP0044CoinType)
	if err != nil {
		return "", "", err
	}

	return legacyXPUb, slip0044XPUb, nil
}
