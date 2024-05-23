package libwallet

import (
	"context"
	"fmt"

	"decred.org/dcrwallet/v3/errors"
	"decred.org/dcrwallet/v3/walletseed"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/hdkeychain/v3"
	"github.com/tyler-smith/go-bip39"

	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
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

// CreateNewDCRWallet creates a new DCR wallet and returns it.
func (mgr *AssetsManager) CreateNewDCRWallet(walletName, privatePassphrase string, privatePassphraseType int32, wordSeedType sharedW.WordSeedType) (sharedW.Asset, error) {
	pass := &sharedW.AuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
		WordSeedType:    wordSeedType,
	}
	wallet, err := dcr.CreateNewWallet(pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.DCR.Wallets[wallet.GetWalletID()] = wallet

	// Allow spending from the default account by default.
	wallet.SetBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, true)

	return wallet, nil
}

// CreateNewDCRWatchOnlyWallet creates a new DCR watch only wallet and returns it.
func (mgr *AssetsManager) CreateNewDCRWatchOnlyWallet(walletName, extendedPublicKey string) (sharedW.Asset, error) {
	wallet, err := dcr.CreateWatchOnlyWallet(walletName, extendedPublicKey, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.DCR.Wallets[wallet.GetWalletID()] = wallet

	// Allow spending from the default account by default.
	wallet.SetBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, true)

	return wallet, nil
}

// RestoreDCRWallet restores a DCR wallet from a seed and returns it.
func (mgr *AssetsManager) RestoreDCRWallet(walletName, seedMnemonic, privatePassphrase string, wordSeedType sharedW.WordSeedType, privatePassphraseType int32) (sharedW.Asset, error) {
	pass := &sharedW.AuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
		WordSeedType:    wordSeedType,
	}
	wallet, err := dcr.RestoreWallet(seedMnemonic, pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.DCR.Wallets[wallet.GetWalletID()] = wallet

	// Allow spending from the default account by default.
	wallet.SetBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, true)

	return wallet, nil
}

// DCRWalletWithXPub returns the ID of the DCR wallet that has an account with the
// provided xpub. Returns -1 if there is no such wallet.
func (mgr *AssetsManager) DCRWalletWithXPub(xpub string) (int, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, w := range mgr.Assets.DCR.Wallets {
		if !w.WalletOpened() {
			return -1, errors.Errorf("wallet %d is not open and cannot be checked", w.GetWalletID())
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
				return w.GetWalletID(), nil
			}
		}
	}
	return -1, nil
}

// DCRWalletWithSeed returns the ID of the DCR wallet that was created or restored
// using the same seed as the one provided. Returns -1 if no wallet uses the
// provided seed.
func (mgr *AssetsManager) DCRWalletWithSeed(seedMnemonic string, wordSeedType sharedW.WordSeedType) (int, error) {
	if len(seedMnemonic) == 0 {
		return -1, errors.New(utils.ErrEmptySeed)
	}

	newSeedLegacyXPUb, newSeedSLIP0044XPUb, err := deriveBIP44AccountXPubsForDCR(seedMnemonic, wordSeedType,
		dcr.DefaultAccountNum, mgr.chainsParams.DCR)
	if err != nil {
		return -1, err
	}

	for _, wallet := range mgr.Assets.DCR.Wallets {
		if !wallet.WalletOpened() {
			return -1, errors.Errorf("cannot check if seed matches unloaded wallet %d", wallet.GetWalletID())
		}
		// NOTE: Existing watch-only wallets may have been created using the
		// xpub of an account that is NOT the default account and may return
		// incorrect result from the check below. But this would return true
		// if the watch-only wallet was created using the xpub of the default
		// account of the provided seed.
		fn := wallet.(interface {
			AccountXPubMatches(account uint32, legacyXPub, slip044XPub string) (bool, error)
		})
		usesSameSeed, err := fn.AccountXPubMatches(dcr.DefaultAccountNum, newSeedLegacyXPUb, newSeedSLIP0044XPUb)
		if err != nil {
			return -1, err
		}
		if usesSameSeed {
			return wallet.GetWalletID(), nil
		}
	}

	return -1, nil
}

// deriveBIP44AccountXPubForDCR derives and returns the legacy and SLIP0044 account
// xpubs using the BIP44 HD path for accounts: m/44'/<coin type>'/<account>'.
func deriveBIP44AccountXPubsForDCR(seedMnemonic string, wordSeedType sharedW.WordSeedType, account uint32, params *chaincfg.Params) (string, string, error) {
	var seed []byte
	var err error
	fmt.Println("-------deriveBIP44AccountXPubsForDCR-----wordSeedType--->", wordSeedType)
	if wordSeedType == sharedW.WordSeed33 {
		seed, err = walletseed.DecodeUserInput(seedMnemonic)
	} else {
		fmt.Println("-------deriveBIP44AccountXPubsForDCR-----seed--->", seedMnemonic)
		seed, err = bip39.NewSeedWithErrorChecking(seedMnemonic, "")
		fmt.Println("-------deriveBIP44AccountXPubsForDCR-----err--->", err)
	}
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
