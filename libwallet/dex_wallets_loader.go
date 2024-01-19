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
	"github.com/crypto-power/cryptopower/libwallet/utils"

	dexbtc "decred.org/dcrdex/client/asset/btc"
	dexdcr "decred.org/dcrdex/client/asset/dcr"
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
		ConfigOpts:  append(customWalletConfigOpts, dexdcr.WalletOpts...),
	}

	// This function will be invoked when the DEX client needs to
	// setup a dcr ExchangeWallet; it allows us to use an existing
	// wallet instance for wallet operations instead of json-rpc.
	var walletMaker = func(settings map[string]string, chainParams *dcrcfg.Params, logger dex.Logger) (dexdcr.Wallet, error) {
		walletIDStr := settings[dexc.WalletIDConfigKey]
		walletID, err := strconv.Atoi(walletIDStr)
		if err != nil || walletID < 0 {
			return nil, fmt.Errorf("invalid wallet ID %q in settings", walletIDStr)
		}

		wallet := loadWalletForDEX(walletID)
		if wallet == nil {
			return nil, fmt.Errorf("no wallet exists with ID %q", walletIDStr)
		}

		walletParams := wallet.Internal().DCR.ChainParams()
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

		return dcr.NewDEXWallet(dcrAsset, accountNumber, dcrAsset.SyncData()), nil
	}

	dexdcr.RegisterCustomWallet(walletMaker, def)
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

	// Register wallet constructors. The constructor function will be invoked
	// when the DEX client needs to setup a dexbtc.BTCWallet and this allows us
	// to use an existing wallet instance for wallet operations.

	btcDef := &asset.WalletDefinition{
		Type:        dexc.CustomDexWalletType,
		Description: "Uses an existing cryptopower Wallet.",
		ConfigOpts:  append(customWalletConfigOpts, dexbtc.CommonConfigOpts(utils.BTCWalletAsset.String(), false)...),
	}
	dexbtc.RegisterCustomWallet(btcCloneWalletConstructor(false), btcDef)

	ltcDef := &asset.WalletDefinition{
		Type:        dexc.CustomDexWalletType,
		Description: "Uses an existing cryptopower Wallet.",
		ConfigOpts:  append(customWalletConfigOpts, dexbtc.CommonConfigOpts(utils.LTCWalletAsset.String(), false)...),
	}
	dexltc.RegisterCustomWallet(btcCloneWalletConstructor(true), ltcDef)

}

// btcCloneWalletConstructor is a shared wallet constructor used by btc and ltc
// to create dex compatible wallets.
func btcCloneWalletConstructor(isLtc bool) func(settings map[string]string, chainParams *btccfg.Params) (dexbtc.CustomWallet, error) {
	return func(settings map[string]string, chainParams *btccfg.Params) (dexbtc.CustomWallet, error) {
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
			ltcAsset := wallet.(*ltc.Asset)
			return ltc.NewDEXWallet(wallet.Internal().LTC, accountNumber, ltcAsset.NeutrinoClient(), chainParams, ltcAsset.SyncData()), nil
		}

		btcAsset := wallet.(*btc.Asset)
		return btc.NewDEXWallet(wallet.Internal().BTC, accountNumber, btcAsset.NeutrinoClient(), btcAsset.SyncData()), nil
	}
}
