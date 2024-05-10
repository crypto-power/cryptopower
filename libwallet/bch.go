package libwallet

import (
	"fmt"

	"decred.org/dcrwallet/v3/errors"
	"github.com/gcash/bchd/chaincfg"
	"github.com/dcrlabs/bchwallet/waddrmgr"

	"github.com/crypto-power/cryptopower/libwallet/assets/bch"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

func initializeBCHWalletParameters(netType utils.NetworkType) (*chaincfg.Params, error) {
	chainParams, err := utils.BCHChainParams(netType)
	if err != nil {
		return chainParams, err
	}

	return chainParams, nil
}

// CreateNewBCHWallet creates a new BCH wallet and returns it.
func (mgr *AssetsManager) CreateNewBCHWallet(walletName, privatePassphrase string, privatePassphraseType int32) (sharedW.Asset, error) {
	pass := &sharedW.AuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	wallet, err := bch.CreateNewWallet(pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.BCH.Wallets[wallet.GetWalletID()] = wallet

	return wallet, nil
}

// CreateNewBCHWatchOnlyWallet creates a new BCH watch only wallet and returns it.
func (mgr *AssetsManager) CreateNewBCHWatchOnlyWallet(walletName, extendedPublicKey string) (sharedW.Asset, error) {
	wallet, err := bch.CreateWatchOnlyWallet(walletName, extendedPublicKey, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.BCH.Wallets[wallet.GetWalletID()] = wallet

	return wallet, nil
}

// RestoreBCHWallet restores a BCH wallet from a seed and returns it.
func (mgr *AssetsManager) RestoreBCHWallet(walletName, seedMnemonic, privatePassphrase string, privatePassphraseType int32) (sharedW.Asset, error) {
	pass := &sharedW.AuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
	}
	wallet, err := bch.RestoreWallet(seedMnemonic, pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.BCH.Wallets[wallet.GetWalletID()] = wallet

	return wallet, nil
}

// BCHWalletWithXPub returns the ID of the BCH wallet that has an account with the
// provided xpub. Returns -1 if there is no such wallet.
func (mgr *AssetsManager) BCHWalletWithXPub(xpub string) (int, error) {
	for _, wallet := range mgr.Assets.BCH.Wallets {
		if !wallet.WalletOpened() {
			return -1, errors.Errorf("wallet %d is not open and cannot be checked", wallet.GetWalletID())
		}

		wAccs, err := wallet.GetAccountsRaw()
		if err != nil {
			return -1, err
		}

		for _, accs := range wAccs.Accounts {
			if accs.AccountNumber == bch.ImportedAccountNumber {
				continue
			}
			// acctXPubKey, err := wallet.Internal().BCH.AccountProperties(bch.GetScope(), accs.AccountNumber)
			// if err != nil {
			// 	return -1, err
			// }

			if /*acctXPubKey.AccountPubKey.String()*/ "" == xpub {
				return wallet.GetWalletID(), nil
			}
		}
	}
	return -1, nil
}

// BCHWalletWithSeed returns the ID of the BCH wallet that was created or restored
// using the same seed as the one provided. Returns -1 if no wallet uses the
// provided seed.
func (mgr *AssetsManager) BCHWalletWithSeed(seedMnemonic string) (int, error) {
	if len(seedMnemonic) == 0 {
		return -1, errors.New(utils.ErrEmptySeed)
	}

	for _, wallet := range mgr.Assets.BCH.Wallets {
		if !wallet.WalletOpened() {
			return -1, errors.Errorf("cannot check if seed matches unloaded wallet %d", wallet.GetWalletID())
		}

		asset, ok := wallet.(*bch.Asset)
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
