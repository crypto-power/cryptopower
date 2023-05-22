package libwallet

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcwallet/waddrmgr"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/ltc"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

func initializeBTCWalletParameters(netType utils.NetworkType) (*chaincfg.Params, error) {
	chainParams, err := utils.BTCChainParams(netType)
	if err != nil {
		return chainParams, err
	}

	return chainParams, nil
}

// CreateNewBTCWallet creates a new BTC wallet and returns it.
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

// CreateNewBTCWatchOnlyWallet creates a new BTC watch only wallet and returns it.
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

// RestoreBTCWallet restores a BTC wallet from a seed and returns it.
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

// BTCWalletWithXPub returns the ID of the BTC wallet that has an account with the
// provided xpub. Returns -1 if there is no such wallet.
func (mgr *AssetsManager) BTCWalletWithXPub(xpub string) (int, error) {
	for _, wallet := range mgr.Assets.BTC.Wallets {
		if !wallet.WalletOpened() {
			return -1, errors.Errorf("wallet %d is not open and cannot be checked", wallet.GetWalletID())
		}

		wAccs, err := wallet.GetAccountsRaw()
		if err != nil {
			return -1, err
		}

		for _, accs := range wAccs.Accounts {
			if accs.AccountNumber == btc.ImportedAccountNumber {
				continue
			}
			acctXPubKey, err := wallet.Internal().BTC.AccountProperties(btc.GetScope(), uint32(accs.AccountNumber))
			if err != nil {
				return -1, err
			}

			if acctXPubKey.AccountPubKey.String() == xpub {
				return wallet.GetWalletID(), nil
			}
		}
	}
	return -1, nil
}

// BTCWalletWithSeed returns the ID of the BTC wallet that was created or restored
// using the same seed as the one provided. Returns -1 if no wallet uses the
// provided seed.
func (mgr *AssetsManager) BTCWalletWithSeed(seedMnemonic string) (int, error) {
	if len(seedMnemonic) == 0 {
		return -1, errors.New(utils.ErrEmptySeed)
	}

	for _, wallet := range mgr.Assets.BTC.Wallets {
		if !wallet.WalletOpened() {
			return -1, errors.Errorf("cannot check if seed matches unloaded wallet %d", wallet.GetWalletID())
		}

		asset, ok := wallet.(*btc.Asset)
		if !ok {
			return -1, fmt.Errorf("invalid asset type")
		}

		wAccs, err := wallet.GetAccountsRaw()
		if err != nil {
			return -1, err
		}

		for _, accs := range wAccs.Accounts {
			if accs.AccountNumber == waddrmgr.ImportedAddrAccount {
				continue
			}
			xpub, err := asset.DeriveAccountXpub(seedMnemonic,
				accs.AccountNumber, wallet.Internal().BTC.ChainParams())
			if err != nil {
				return -1, err
			}

			usesSameSeed, err := asset.AccountXPubMatches(accs.AccountNumber, xpub)
			if err != nil {
				return -1, err
			}
			if usesSameSeed {
				return wallet.GetWalletID(), nil
			}
		}
	}
	return -1, nil
}

// RestoreLTCWallet restores a LTC wallet from a seed and returns it.
func (mgr *AssetsManager) RestoreLTCWallet(walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (sharedW.Asset, error) {
	pass := &sharedW.WalletAuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	wallet, err := ltc.RestoreWallet(seedMnemonic, pass, mgr.params)
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
