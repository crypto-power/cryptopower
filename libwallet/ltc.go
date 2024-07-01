package libwallet

import (
	"fmt"

	"decred.org/dcrwallet/v3/errors"
	"github.com/crypto-power/cryptopower/libwallet/assets/ltc"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/dcrlabs/ltcwallet/waddrmgr"
	"github.com/ltcsuite/ltcd/chaincfg"
)

// initializeLTCWalletParameters initializes the fields each LTC wallet is going to need to be setup
func initializeLTCWalletParameters(netType utils.NetworkType) (*chaincfg.Params, error) {
	chainParams, err := utils.LTCChainParams(netType)
	if err != nil {
		return chainParams, err
	}
	return chainParams, nil
}

// CreateNewLTCWallet creates a new LTC wallet and returns it.
func (mgr *AssetsManager) CreateNewLTCWallet(walletName, privatePassphrase string, privatePassphraseType int32, wordSeedType sharedW.WordSeedType) (sharedW.Asset, error) {
	pass := &sharedW.AuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
		WordSeedType:    wordSeedType,
	}

	wallet, err := ltc.CreateNewWallet(pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.LTC.Wallets[wallet.GetWalletID()] = wallet

	return wallet, nil
}

// CreateNewBTCWatchOnlyWallet creates a new BTC watch only wallet and returns it.
func (mgr *AssetsManager) CreateNewLTCWatchOnlyWallet(walletName, extendedPublicKey string) (sharedW.Asset, error) {
	wallet, err := ltc.CreateWatchOnlyWallet(walletName, extendedPublicKey, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.LTC.Wallets[wallet.GetWalletID()] = wallet

	return wallet, nil
}

// RestoreLTCWallet restores a LTC wallet from a seed and returns it.
func (mgr *AssetsManager) RestoreLTCWallet(walletName, seedMnemonic, privatePassphrase string, wordSeedType sharedW.WordSeedType, privatePassphraseType int32) (sharedW.Asset, error) {
	pass := &sharedW.AuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
		WordSeedType:    wordSeedType,
	}
	wallet, err := ltc.RestoreWallet(seedMnemonic, pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.LTC.Wallets[wallet.GetWalletID()] = wallet

	return wallet, nil
}

// LTCWalletWithSeed returns the ID of the LTC wallet that was created or restored
// using the same seed as the one provided. Returns -1 if no wallet uses the
// provided seed.
func (mgr *AssetsManager) LTCWalletWithSeed(seedMnemonic string, wordSeedType sharedW.WordSeedType) (int, error) {
	if len(seedMnemonic) == 0 {
		return -1, errors.New(utils.ErrEmptySeed)
	}

	for _, wallet := range mgr.Assets.LTC.Wallets {
		if !wallet.WalletOpened() {
			return -1, errors.Errorf("cannot check if seed matches unloaded wallet %d", wallet.GetWalletID())
		}

		asset, ok := wallet.(*ltc.Asset)
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
			xpub, err := asset.DeriveAccountXpub(seedMnemonic, wordSeedType,
				accs.AccountNumber, wallet.Internal().LTC.ChainParams())
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

// LTCWalletWithXPub returns the ID of the LTC wallet that has an account with the
// provided xpub. Returns -1 if there is no such wallet.
func (mgr *AssetsManager) LTCWalletWithXPub(xpub string) (int, error) {
	for _, wallet := range mgr.Assets.LTC.Wallets {
		if !wallet.WalletOpened() {
			return -1, errors.Errorf("wallet %d is not open and cannot be checked", wallet.GetWalletID())
		}

		wAccs, err := wallet.GetAccountsRaw()
		if err != nil {
			return -1, err
		}

		for _, accs := range wAccs.Accounts {
			if accs.AccountNumber == ltc.ImportedAccountNumber {
				continue
			}
			acctXPubKey, err := wallet.Internal().LTC.AccountProperties(ltc.GetScope(), accs.AccountNumber)
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
