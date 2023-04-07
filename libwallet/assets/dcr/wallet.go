package dcr

import (
	"context"
	"sync"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/dcr"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/vsp"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	dcrW "decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/txrules"
	"github.com/decred/dcrd/chaincfg/v3"
)

type DCRAsset struct {
	*sharedW.Wallet

	synced            bool
	syncing           bool
	waitingForHeaders bool

	chainParams *chaincfg.Params

	cancelAccountMixer      context.CancelFunc `json:"-"`
	cancelAutoTicketBuyer   context.CancelFunc `json:"-"`
	cancelAutoTicketBuyerMu sync.RWMutex

	TxAuthoredInfo *TxAuthor

	vspClientsMu sync.Mutex
	vspClients   map[string]*vsp.Client
	vspMu        sync.RWMutex
	vsps         []*VSP

	notificationListenersMu          sync.RWMutex
	syncData                         *SyncData
	accountMixerNotificationListener map[string]AccountMixerNotificationListener
	txAndBlockNotificationListeners  map[string]sharedW.TxAndBlockNotificationListener
	blocksRescanProgressListener     sharedW.BlocksRescanProgressListener
}

// Verify that DCR implements the shared assets interface.
var _ sharedW.Asset = (*DCRAsset)(nil)

// initWalletLoader setups the loader.
func initWalletLoader(chainParams *chaincfg.Params, rootdir, walletDbDriver string) loader.AssetLoader {
	// TODO: Allow users provide values to override these defaults.
	cfg := &sharedW.WalletConfig{
		GapLimit:                20,
		AllowHighFees:           false,
		RelayFee:                txrules.DefaultRelayFeePerKb,
		AccountGapLimit:         dcrW.DefaultAccountGapLimit,
		DisableCoinTypeUpgrades: false,
		ManualTickets:           false,
		MixSplitLimit:           10,
	}

	stakeOptions := &dcr.StakeOptions{
		VotingEnabled: false,
		AddressReuse:  false,
		VotingAddress: nil,
	}

	loaderCfg := &dcr.LoaderConf{
		ChainParams:             chainParams,
		DBDirPath:               rootdir,
		StakeOptions:            stakeOptions,
		GapLimit:                cfg.GapLimit,
		RelayFee:                cfg.RelayFee,
		AllowHighFees:           cfg.AllowHighFees,
		DisableCoinTypeUpgrades: cfg.DisableCoinTypeUpgrades,
		ManualTickets:           cfg.ManualTickets,
		AccountGapLimit:         cfg.AccountGapLimit,
		MixSplitLimit:           cfg.MixSplitLimit,
	}
	walletLoader := dcr.NewLoader(loaderCfg)

	if walletDbDriver != "" {
		walletLoader.SetDatabaseDriver(walletDbDriver)
	}

	return walletLoader
}

// CreateNewWallet accepts the wallet pass information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the DCR asset. It then generates the DCR loader interface
// that is passed to be used upstream while creating a new wallet in the
// shared wallet implemenation.
// Immediately a new wallet is created, the function to safely cancel network sync
// is set. There after returning the new wallet's interface.
func CreateNewWallet(pass *sharedW.WalletAuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.DCRChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir, params.DbDriver)

	w, err := sharedW.CreateNewWallet(pass, ldr, params, utils.DCRWalletAsset)
	if err != nil {
		return nil, err
	}

	dcrWallet := &DCRAsset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners:  make(map[string]sharedW.TxAndBlockNotificationListener),
		accountMixerNotificationListener: make(map[string]AccountMixerNotificationListener),
		vspClients:                       make(map[string]*vsp.Client),
	}

	dcrWallet.SetNetworkCancelCallback(dcrWallet.SafelyCancelSync)

	return dcrWallet, nil
}

// CreateWatchOnlyWallet accepts the wallet name, extended public key and the
// init parameters to create a watch only wallet for the DCR asset.
// It validates the network type passed by fetching the chain parameters
// associated with it for the DCR asset. It then generates the DCR loader interface
// that is passed to be used upstream while creating the watch only wallet in the
// shared wallet implemenation.
// Immediately a watch only wallet is created, the function to safely cancel network sync
// is set. There after returning the watch only wallet's interface.
func CreateWatchOnlyWallet(walletName, extendedPublicKey string, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.DCRChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir, params.DbDriver)
	w, err := sharedW.CreateWatchOnlyWallet(walletName, extendedPublicKey,
		ldr, params, utils.DCRWalletAsset)
	if err != nil {
		return nil, err
	}

	dcrWallet := &DCRAsset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners: make(map[string]sharedW.TxAndBlockNotificationListener),
	}

	dcrWallet.SetNetworkCancelCallback(dcrWallet.SafelyCancelSync)

	return dcrWallet, nil
}

// RestoreWallet accepts the seed, wallet pass information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the DCR asset. It then generates the DCR loader interface
// that is passed to be used upstream while restoring the wallet in the
// shared wallet implemenation.
// Immediately wallet restore is complete, the function to safely cancel network sync
// is set. There after returning the restored wallet's interface.
func RestoreWallet(seedMnemonic string, pass *sharedW.WalletAuthInfo, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.DCRChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir, params.DbDriver)
	w, err := sharedW.RestoreWallet(seedMnemonic, pass, ldr, params, utils.DCRWalletAsset)
	if err != nil {
		return nil, err
	}

	dcrWallet := &DCRAsset{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		vspClients:                       make(map[string]*vsp.Client),
		txAndBlockNotificationListeners:  make(map[string]sharedW.TxAndBlockNotificationListener),
		accountMixerNotificationListener: make(map[string]AccountMixerNotificationListener),
	}

	dcrWallet.SetNetworkCancelCallback(dcrWallet.SafelyCancelSync)

	return dcrWallet, nil
}

// LoadExisting accepts the stored shared wallet information and the init parameters.
// It validates the network type passed by fetching the chain parameters
// associated with it for the DCR asset. It then generates the DCR loader interface
// that is passed to be used upstream while loading the existing the wallet in the
// shared wallet implemenation.
// Immediately loading the existing wallet is complete, the function to safely
// cancel network sync is set. There after returning the loaded wallet's interface.
func LoadExisting(w *sharedW.Wallet, params *sharedW.InitParams) (sharedW.Asset, error) {
	chainParams, err := utils.DCRChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir, params.DbDriver)
	dcrWallet := &DCRAsset{
		Wallet:      w,
		vspClients:  make(map[string]*vsp.Client),
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]sharedW.SyncProgressListener),
		},
		txAndBlockNotificationListeners:  make(map[string]sharedW.TxAndBlockNotificationListener),
		accountMixerNotificationListener: make(map[string]AccountMixerNotificationListener),
	}

	err = dcrWallet.Prepare(ldr, params)
	if err != nil {
		return nil, err
	}

	dcrWallet.SetNetworkCancelCallback(dcrWallet.SafelyCancelSync)

	return dcrWallet, nil
}

// AccountXPubMatches checks if the xpub of the provided account matches the
// provided legacy or SLIP0044 xpub. While both the legacy and SLIP0044 xpubs
// will be checked for watch-only wallets, other wallets will only check the
// xpub that matches the coin type key used by the asset.
func (asset *DCRAsset) AccountXPubMatches(account uint32, legacyXPub, slip044XPub string) (bool, error) {
	if !asset.WalletOpened() {
		return false, utils.ErrDCRNotInitialized
	}

	ctx, _ := asset.ShutdownContextWithCancel()

	acctXPubKey, err := asset.Internal().DCR.AccountXpub(ctx, account)
	if err != nil {
		return false, err
	}
	acctXPub := acctXPubKey.String()

	if asset.IsWatchingOnlyWallet() {
		// Coin type info isn't saved for watch-only wallets, so check
		// against both legacy and SLIP0044 coin types.
		return acctXPub == legacyXPub || acctXPub == slip044XPub, nil
	}

	cointype, err := asset.Internal().DCR.CoinType(ctx)
	if err != nil {
		return false, err
	}

	if cointype == asset.chainParams.LegacyCoinType {
		return acctXPub == legacyXPub, nil
	} else {
		return acctXPub == slip044XPub, nil
	}
}

func (asset *DCRAsset) Synced() bool {
	return asset.synced
}

// SafelyCancelSync is used to controllably disable network activity.
func (asset *DCRAsset) SafelyCancelSync() {
	if asset.IsConnectedToDecredNetwork() {
		asset.CancelSync()
	}
}

func (asset *DCRAsset) IsConnectedToNetwork() bool {
	return asset.IsConnectedToDecredNetwork()
}
