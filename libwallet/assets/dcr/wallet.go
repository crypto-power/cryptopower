package dcr

import (
	"context"
	"sync"

	dcrW "decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/txrules"
	"github.com/decred/dcrd/chaincfg/v3"
	mainW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader"
	"gitlab.com/raedah/cryptopower/libwallet/internal/loader/dcr"
	"gitlab.com/raedah/cryptopower/libwallet/internal/vsp"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

// To be renamed to DCRAsset when optimizing the code.
// type DCRAsset struct {
type Wallet struct {
	*mainW.Wallet

	synced            bool
	syncing           bool
	waitingForHeaders bool

	chainParams *chaincfg.Params

	cancelAccountMixer      context.CancelFunc `json:"-"`
	cancelAutoTicketBuyer   context.CancelFunc `json:"-"`
	cancelAutoTicketBuyerMu sync.RWMutex

	vspClientsMu sync.Mutex
	vspClients   map[string]*vsp.Client
	vspMu        sync.RWMutex
	vsps         []*VSP

	notificationListenersMu          sync.RWMutex
	syncData                         *SyncData
	accountMixerNotificationListener map[string]AccountMixerNotificationListener
	txAndBlockNotificationListeners  map[string]mainW.TxAndBlockNotificationListener
	blocksRescanProgressListener     mainW.BlocksRescanProgressListener
}

// initWalletLoader setups the loader.
func initWalletLoader(chainParams *chaincfg.Params, rootdir, walletDbDriver string) loader.AssetLoader {
	// TODO: Allow users provide values to override these defaults.
	cfg := &mainW.WalletConfig{
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
	walletLoader := dcr.NewLoader(chainParams, rootdir, stakeOptions,
		cfg.GapLimit, cfg.RelayFee, cfg.AllowHighFees, cfg.DisableCoinTypeUpgrades,
		cfg.ManualTickets, cfg.AccountGapLimit, cfg.MixSplitLimit)

	if walletDbDriver != "" {
		walletLoader.SetDatabaseDriver(walletDbDriver)
	}

	return walletLoader
}

func CreateNewWallet(pass *mainW.WalletPassInfo, params *mainW.InitParams) (*Wallet, error) {
	chainParams, err := utils.DCRChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir, params.DbDriver)

	w, err := mainW.CreateNewWallet(pass, utils.DCRWalletAsset, ldr, params)
	if err != nil {
		return nil, err
	}

	dcrWallet := &Wallet{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]mainW.SyncProgressListener),
		},
		txAndBlockNotificationListeners:  make(map[string]mainW.TxAndBlockNotificationListener),
		accountMixerNotificationListener: make(map[string]AccountMixerNotificationListener),
		vspClients:                       make(map[string]*vsp.Client),
	}

	dcrWallet.SetNetworkCancelCallback(dcrWallet.SafelyCancelSync)

	return dcrWallet, nil
}

func CreateWatchOnlyWallet(walletName, extendedPublicKey string, params *mainW.InitParams) (*Wallet, error) {
	chainParams, err := utils.DCRChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir, params.DbDriver)
	w, err := mainW.CreateWatchOnlyWallet(walletName, extendedPublicKey,
		utils.DCRWalletAsset, ldr, params)
	if err != nil {
		return nil, err
	}

	dcrWallet := &Wallet{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]mainW.SyncProgressListener),
		},
	}

	dcrWallet.SetNetworkCancelCallback(dcrWallet.SafelyCancelSync)

	return dcrWallet, nil
}

func RestoreWallet(seedMnemonic string, pass *mainW.WalletPassInfo, params *mainW.InitParams) (*Wallet, error) {
	chainParams, err := utils.DCRChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir, params.DbDriver)
	w, err := mainW.RestoreWallet(seedMnemonic, pass, utils.DCRWalletAsset, ldr, params)
	if err != nil {
		return nil, err
	}

	dcrWallet := &Wallet{
		Wallet:      w,
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]mainW.SyncProgressListener),
		},
		vspClients: make(map[string]*vsp.Client),
	}

	dcrWallet.SetNetworkCancelCallback(dcrWallet.SafelyCancelSync)

	return dcrWallet, nil
}

func LoadExisting(w *mainW.Wallet, params *mainW.InitParams) (*Wallet, error) {
	chainParams, err := utils.DCRChainParams(params.NetType)
	if err != nil {
		return nil, err
	}

	ldr := initWalletLoader(chainParams, params.RootDir, params.DbDriver)
	dcrWallet := &Wallet{
		Wallet:      w,
		vspClients:  make(map[string]*vsp.Client),
		chainParams: chainParams,
		syncData: &SyncData{
			syncProgressListeners: make(map[string]mainW.SyncProgressListener),
		},
		txAndBlockNotificationListeners:  make(map[string]mainW.TxAndBlockNotificationListener),
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
// xpub that matches the coin type key used by the wallet.
func (wallet *Wallet) AccountXPubMatches(account uint32, legacyXPub, slip044XPub string) (bool, error) {
	ctx, _ := wallet.ShutdownContextWithCancel()

	acctXPubKey, err := wallet.Internal().DCR.AccountXpub(ctx, account)
	if err != nil {
		return false, err
	}
	acctXPub := acctXPubKey.String()

	if wallet.IsWatchingOnlyWallet() {
		// Coin type info isn't saved for watch-only wallets, so check
		// against both legacy and SLIP0044 coin types.
		return acctXPub == legacyXPub || acctXPub == slip044XPub, nil
	}

	cointype, err := wallet.Internal().DCR.CoinType(ctx)
	if err != nil {
		return false, err
	}

	if cointype == wallet.chainParams.LegacyCoinType {
		return acctXPub == legacyXPub, nil
	} else {
		return acctXPub == slip044XPub, nil
	}
}

func (wallet *Wallet) Synced() bool {
	return wallet.synced
}

func (wallet *Wallet) SafelyCancelSync() {
	if wallet.IsConnectedToDecredNetwork() {
		wallet.CancelSync()
		defer func() {
			wallet.SpvSync()
		}()
	}
}
