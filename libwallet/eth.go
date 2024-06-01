package libwallet

import (
	"github.com/crypto-power/cryptopower/libwallet/assets/eth"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/ethereum/go-ethereum/params"
)

// initializeETHWalletParameters initializes the fields each ETH wallet is going to need to be setup
func initializeETHWalletParameters(netType utils.NetworkType) (*params.ChainConfig, error) {
	chainParams, err := utils.ETHChainParams(netType)
	if err != nil {
		return chainParams, err
	}
	return chainParams, nil
}

// CreateNewETHWallet creates a new ETH wallet and returns it.
func (mgr *AssetsManager) CreateNewETHWallet(walletName, privatePassphrase string, privatePassphraseType int32, wordSeedType sharedW.WordSeedType) (sharedW.Asset, error) {
	pass := &sharedW.AuthInfo{
		Name:            walletName,
		PrivatePass:     privatePassphrase,
		PrivatePassType: privatePassphraseType,
		WordSeedType:    wordSeedType,
	}

	wallet, err := eth.CreateNewWallet(pass, mgr.params)
	if err != nil {
		return nil, err
	}

	mgr.Assets.ETH.Wallets[wallet.GetWalletID()] = wallet

	return wallet, nil
}

// RestoreETHWallet restores a ETH wallet from a seed and returns it.
func (mgr *AssetsManager) RestoreETHWallet(walletName, seedMnemonic, privatePassphrase string, wordSeedType sharedW.WordSeedType, privatePassphraseType int32) (sharedW.Asset, error) {
	return nil, utils.ErrETHMethodNotImplemented("RestoreETHWallet")
}

// ETHWalletWithXPub returns the ID of the ETH wallet that has an account with the
// provided xpub. Returns -1 if there is no such wallet.func (mgr *AssetsManager) ETHWalletWithXpub(walletName, extendedPublicKey string) (sharedW.Asset, error) {
func (mgr *AssetsManager) ETHWalletWithXPub(extendedPublicKey string) (int, error) {
	return -1, utils.ErrETHMethodNotImplemented("ETHWalletWithXpub")
}

// DCRWalletWithSeed returns the ID of the DCR wallet that was created or restored
// using the same seed as the one provided. Returns -1 if no wallet uses the
// provided seed.

// ETHWalletWithSeed returns the ID of the ETH wallet that was created or restored
// using the same seed as the one provided. Returns -1 if no wallet uses the
// provided seed.
func (mgr *AssetsManager) ETHWalletWithSeed(seedMnemonic string, wordSeedType sharedW.WordSeedType) (int, error) {
	return -1, utils.ErrETHMethodNotImplemented("ETHWalletWithSeed")
}
