package libwallet

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"decred.org/dcrdex/client/asset"
	"decred.org/dcrdex/dex"
	"github.com/crypto-power/cryptopower/dexc"
	"github.com/crypto-power/cryptopower/libwallet/assets/btc"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/libwallet/assets/ltc"

	dexbtc "decred.org/dcrdex/client/asset/btc"
	dexDcr "decred.org/dcrdex/client/asset/dcr"
	dexltc "decred.org/dcrdex/client/asset/ltc"
	btccfg "github.com/btcsuite/btcd/chaincfg"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	dcrcfg "github.com/decred/dcrd/chaincfg/v3"
)

var (
	dexWalletLoaderMtx sync.RWMutex
	dexWalletLoader    func(walletID int) sharedW.Asset
)

func init() {
	prepareDexSupportForDCRWallet()
	prepareDexSupportForBTCCloneWallets()
}

func setDEXWalletLoader(fn func(walletID int) sharedW.Asset) {
	dexWalletLoaderMtx.Lock()
	defer dexWalletLoaderMtx.Unlock()
	dexWalletLoader = fn
}

func loadWalletForDEX(walletID int) sharedW.Asset {
	dexWalletLoaderMtx.RLock()
	defer dexWalletLoaderMtx.RUnlock()
	if dexWalletLoader == nil {
		return nil
	}
	return dexWalletLoader(walletID)
}

// prepareDexSupportForDCRWallet sets up the DEX client to allow using a
// cyptopower dcr wallet with DEX core.
func prepareDexSupportForDCRWallet() {
	// Build a custom wallet definition with custom config options
	// for use by the dex dcr ExchangeWallet.
	customWalletConfigOpts := []*asset.ConfigOption{
		{
			Key:         dexc.WalletIDConfigKey,
			DisplayName: "Wallet ID",
			Description: "ID of existing wallet to use",
		},
		{
			Key:         dexc.WalletAccountNumberConfigKey,
			DisplayName: "Wallet Account Number",
			Description: "Account number of the selected wallet",
		},
	}

	def := &asset.WalletDefinition{
		Type:        dexc.CustomDexWalletType,
		Description: "Uses an existing cryptopower Wallet.",
		ConfigOpts:  customWalletConfigOpts,
	}

	// This function will be invoked when the DEX client needs to
	// setup a dcr ExchangeWallet; it allows us to use an existing
	// wallet instance for wallet operations instead of json-rpc.
	var walletMaker = func(settings map[string]string, chainParams *dcrcfg.Params, logger dex.Logger) (dexDcr.Wallet, error) {
		walletIDStr := settings[dexc.WalletIDConfigKey]
		walletID, err := strconv.Atoi(walletIDStr)
		if err != nil || walletID < 0 {
			return nil, fmt.Errorf("invalid wallet ID %q in settings", walletIDStr)
		}

		wallet := loadWalletForDEX(walletID)
		if wallet == nil {
			return nil, fmt.Errorf("no wallet exists with ID %q", walletIDStr)
		}

		dcrWallet := wallet.Internal().DCR
		walletParams := dcrWallet.ChainParams()
		if walletParams.Net != chainParams.Net {
			return nil, fmt.Errorf("selected wallet is for %s network, expected %s", walletParams.Name, chainParams.Name)
		}

		if wallet.IsWatchingOnlyWallet() {
			return nil, fmt.Errorf("cannot use watch only wallet for DEX trade")
		}

		// Ensure the account exists.
		accountNumberStr := settings[dexc.WalletAccountNumberConfigKey]
		acctNum, err := strconv.ParseInt(accountNumberStr, 10, 64)
		if err != nil {
			return nil, err
		}

		accountNumber := int32(acctNum)
		if _, err = wallet.AccountName(accountNumber); err != nil {
			return nil, fmt.Errorf("error checking selected DEX account: %w", err)
		}

		dcrAsset, ok := wallet.(*dcr.Asset)
		if !ok {
			return nil, fmt.Errorf("DEX wallet not supported for %s", walletParams.Name)
		}

		return dcr.NewDEXWallet(dcrWallet, dcrAsset, accountNumber, dcrAsset.SyncData()), nil
	}

	dexDcr.RegisterCustomWallet(walletMaker, def)
}

// prepareDexSupportForBTCCloneWallets sets up the DEX client to allow using a
// Cyptopower btc or ltc wallet with DEX core.
func prepareDexSupportForBTCCloneWallets() {
	// Build a custom wallet definition with custom config options for use by
	// the dexbtc.ExchangeWalletSPV.
	customWalletConfigOpts := []*asset.ConfigOption{
		{
			Key:         dexc.WalletIDConfigKey,
			DisplayName: "Wallet ID",
			Description: "ID of existing wallet to use",
		},
		{
			Key:         dexc.WalletAccountNumberConfigKey,
			DisplayName: "Wallet Account Number",
			Description: "Account number of the selected wallet",
		},
	}

	def := &asset.WalletDefinition{
		Type:        dexc.CustomDexWalletType,
		Description: "Uses an existing cryptopower Wallet.",
		ConfigOpts:  customWalletConfigOpts,
	}

	// Register wallet constructors. The constructor function will be invoked
	// when the DEX client needs to setup a dexbtc.BTCWallet and this allows us
	// to use an existing wallet instance for wallet operations.

	btcWalletConstructor := func(settings map[string]string, chainParams *btccfg.Params) (dexbtc.BTCWallet, error) {
		return btcCloneWalletConstructor(false, settings, chainParams)
	}
	dexbtc.RegisterCustomSPVWallet(btcWalletConstructor, def)

	ltcWalletConstructor := func(settings map[string]string, chainParams *btccfg.Params) (dexbtc.BTCWallet, error) {
		return btcCloneWalletConstructor(true, settings, chainParams)
	}
	dexltc.RegisterCustomSPVWallet(ltcWalletConstructor, def)
}

// btcCloneWalletConstructor is a shared wallet constructor used by btc and ltc
// to create dex compatible wallets.
func btcCloneWalletConstructor(isLtc bool, settings map[string]string, chainParams *btccfg.Params) (dexbtc.BTCWallet, error) {
	walletIDStr := settings[dexc.WalletIDConfigKey]
	walletID, err := strconv.Atoi(walletIDStr)
	if err != nil || walletID < 0 {
		return nil, fmt.Errorf("invalid wallet ID %q in settings", walletIDStr)
	}

	wallet := loadWalletForDEX(walletID)
	if wallet == nil {
		return nil, fmt.Errorf("no wallet exists with ID %q", walletIDStr)
	}

	if isLtc {
		if walletParams := wallet.Internal().LTC.ChainParams(); !strings.EqualFold(walletParams.Name, chainParams.Name) {
			return nil, fmt.Errorf("selected wallet is for %s network, expected %s", walletParams.Name, chainParams.Name)
		}
	} else {
		if walletParams := wallet.Internal().BTC.ChainParams(); walletParams.Net != chainParams.Net {
			return nil, fmt.Errorf("selected wallet is for %s network, expected %s", walletParams.Name, chainParams.Name)
		}
	}

	if wallet.IsWatchingOnlyWallet() {
		return nil, fmt.Errorf("cannot use watch only wallet for DEX trade")
	}

	// Ensure the wallet account exists.
	accountNumberStr := settings[dexc.WalletAccountNumberConfigKey]
	acctNum, err := strconv.ParseInt(accountNumberStr, 10, 64)
	if err != nil {
		return nil, err
	}

	accountNumber := int32(acctNum)
	if _, err = wallet.AccountName(accountNumber); err != nil {
		return nil, fmt.Errorf("error checking selected DEX account name: %w", err)
	}

	if isLtc {
		return ltc.NewDEXWallet(wallet.Internal().LTC, accountNumber, wallet.(*ltc.Asset).NeutrinoClient(), chainParams), nil
	}
	return btc.NewDEXWallet(wallet.Internal().BTC, accountNumber, wallet.(*btc.Asset).NeutrinoClient()), nil
}
