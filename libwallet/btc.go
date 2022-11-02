package libwallet

import (
	"decred.org/dcrwallet/v2/errors"
	"decred.org/dcrwallet/walletseed"
	"github.com/btcsuite/btcd/chaincfg"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
)

func initializeBTCWalletParameters(netType utils.NetworkType) (*chaincfg.Params, error) {
	chainParams, err := utils.BTCChainParams(netType)
	if err != nil {
		return chainParams, err
	}

	return chainParams, nil
}

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
	return -1, errors.New("Not implemented")
}

// BTCWalletWithSeed returns the ID of the BTC wallet that was created or restored
// using the same seed as the one provided. Returns -1 if no wallet uses the
// provided seed.
func (mgr *AssetsManager) BTCWalletWithSeed(seedMnemonic string) (int, error) {
	if len(seedMnemonic) == 0 {
		return -1, errors.New(utils.ErrEmptySeed)
	}

	newSeedHdXPUb, err := deriveBIP44AccountXPubsForBTC(seedMnemonic,
		btc.DefaultAccountNum, mgr.chainsParams.BTC)
	if err != nil {
		return -1, err
	}

	for _, wallet := range mgr.Assets.BTC.Wallets {
		if !wallet.WalletOpened() {
			return -1, errors.Errorf("cannot check if seed matches unloaded wallet %d", wallet.GetWalletID())
		}
		// NOTE: Existing watch-only wallets may have been created using the
		// xpub of an account that is NOT the default account and may return
		// incorrect result from the check below. But this would return true
		// if the watch-only wallet was created using the xpub of the default
		// account of the provided seed.
		fn := wallet.(interface {
			AccountXPubMatches(account uint32, hdXPub string) (bool, error)
		})
		usesSameSeed, err := fn.AccountXPubMatches(btc.DefaultAccountNum, newSeedHdXPUb)
		if err != nil {
			return -1, err
		}
		if usesSameSeed {
			return wallet.GetWalletID(), nil
		}
	}

	return -1, nil
}

func deriveBIP44AccountXPubsForBTC(seedMnemonic string, account uint32, params *chaincfg.Params) (string, error) {
	seed, err := walletseed.DecodeUserInput(seedMnemonic)
	if err != nil {
		return "", err
	}

	defer func() {
		for i := range seed {
			seed[i] = 0
		}
	}()

	// Derive the master extended key from the provided seed.
	masterNode, err := hdkeychain.NewMaster(seed, params)
	if err != nil {
		return "", err
	}
	defer masterNode.Zero()

	// Derive the purpose key as a child of the master node.
	purpose, err := masterNode.Derive(44 + hdkeychain.HardenedKeyStart)
	if err != nil {
		return "", err
	}
	defer purpose.Zero()

	accountXPub := func(coinType uint32) (string, error) {
		coinTypePrivKey, err := purpose.Derive(coinType + hdkeychain.HardenedKeyStart)
		if err != nil {
			return "", err
		}
		defer coinTypePrivKey.Zero()
		acctPrivKey, err := coinTypePrivKey.Derive(account + hdkeychain.HardenedKeyStart)
		if err != nil {
			return "", err
		}
		defer acctPrivKey.Zero()
		extendedKey, err := acctPrivKey.Neuter()
		if err != nil {
			return "", err
		}
		return extendedKey.String(), nil
	}

	hdXPUb, err := accountXPub(params.HDCoinType)
	if err != nil {
		return "", err
	}

	return hdXPUb, nil
}
