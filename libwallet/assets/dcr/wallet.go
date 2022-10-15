package dcr

import (
	"context"
	"sync"

	"github.com/asdine/storm"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	mainW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/assets/wallet/walletdata"
	"gitlab.com/raedah/cryptopower/libwallet/internal/vsp"
)

// To be renamed to DCRAsset when optimizing the code.
// type DCRAsset struct {
type Wallet struct {
	*mainW.Wallet

	/* needed to load existing wallets at the multiwallet level */

	// Order by ID at the multiwallet level fails without this field declared
	// Here. It introduces a bug where the ID cannot be passed upstream.
	// It will be fixed once the code at the multiwallet level is optimized.
	ID int `storm:"id,increment"`

	/* needed to load existing wallets at the multiwallet level */

	rootDir string // to be moved upstream as it same for all assets

	synced            bool
	syncing           bool
	waitingForHeaders bool

	chainParams  *chaincfg.Params
	walletDataDB *walletdata.DB // field to be replaced in the code with GetWalletDataDb()

	cancelAccountMixer context.CancelFunc `json:"-"`

	cancelAutoTicketBuyerMu sync.Mutex
	cancelAutoTicketBuyer   context.CancelFunc `json:"-"`

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

func CreateNewWallet(walletName, privatePassphrase string, privatePassphraseType int32, db *storm.DB, rootDir, dbDriver string, chainParams *chaincfg.Params) (*Wallet, error) {
	w, err := mainW.CreateNewWallet(walletName, privatePassphrase, privatePassphraseType, db, rootDir, dbDriver, chainParams)
	if err != nil {
		return nil, err
	}

	dcrWallet := &Wallet{
		Wallet: w,

		rootDir:      rootDir, // To moved to the upstream wallet
		chainParams:  chainParams,
		walletDataDB: w.GetWalletDataDb(),

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

func CreateWatchOnlyWallet(walletName, extendedPublicKey string, db *storm.DB, rootDir, dbDriver string, chainParams *chaincfg.Params) (*Wallet, error) {
	w, err := wallet.CreateWatchOnlyWallet(walletName, extendedPublicKey, db, rootDir, dbDriver, chainParams)
	if err != nil {
		return nil, err
	}

	dcrWallet := &Wallet{
		Wallet: w,

		rootDir:     rootDir, // To moved to the upstream wallet
		chainParams: chainParams,

		walletDataDB: w.GetWalletDataDb(),

		syncData: &SyncData{
			syncProgressListeners: make(map[string]mainW.SyncProgressListener),
		},
	}

	dcrWallet.SetNetworkCancelCallback(dcrWallet.SafelyCancelSync)

	return dcrWallet, nil
}

func RestoreWallet(walletName, seedMnemonic, rootDir, dbDriver string, db *storm.DB, chainParams *chaincfg.Params, privatePassphrase string, privatePassphraseType int32) (*Wallet, error) {
	w, err := wallet.RestoreWallet(walletName, seedMnemonic, rootDir, dbDriver, db, chainParams, privatePassphrase, privatePassphraseType)
	if err != nil {
		return nil, err
	}

	dcrWallet := &Wallet{
		Wallet: w,

		rootDir:     rootDir, // To moved to the upstream wallet
		chainParams: chainParams,

		walletDataDB: w.GetWalletDataDb(),

		syncData: &SyncData{
			syncProgressListeners: make(map[string]mainW.SyncProgressListener),
		},
		vspClients: make(map[string]*vsp.Client),
	}

	dcrWallet.SetNetworkCancelCallback(dcrWallet.SafelyCancelSync)

	return dcrWallet, nil
}

func (wallet *Wallet) LoadExisting(rootDir string, db *storm.DB, chainParams *chaincfg.Params) error {
	wallet.vspClients = make(map[string]*vsp.Client)
	wallet.rootDir = rootDir
	wallet.chainParams = chainParams

	wallet.syncData = &SyncData{
		syncProgressListeners: make(map[string]mainW.SyncProgressListener),
	}
	wallet.txAndBlockNotificationListeners = make(map[string]mainW.TxAndBlockNotificationListener)
	wallet.accountMixerNotificationListener = make(map[string]AccountMixerNotificationListener)

	err := wallet.Prepare(rootDir, db, chainParams, wallet.ID)
	if err != nil {
		return err
	}

	wallet.SetNetworkCancelCallback(wallet.SafelyCancelSync)
	wallet.walletDataDB = wallet.GetWalletDataDb()

	return nil
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

func (wallet *Wallet) AmountCoin(amount int64) float64 {
	return dcrutil.Amount(amount).ToCoin()
}

func (wallet *Wallet) SafelyCancelSync() {
	if wallet.IsConnectedToDecredNetwork() {
		wallet.CancelSync()
		defer func() {
			wallet.SpvSync()
		}()
	}
}
