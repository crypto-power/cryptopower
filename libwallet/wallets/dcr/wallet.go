package dcr

import (
	"context"
	"sync"

	"github.com/asdine/storm"
	"github.com/decred/dcrd/chaincfg/v3"
	"gitlab.com/raedah/cryptopower/libwallet/internal/vsp"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/wallet"
	mainW "gitlab.com/raedah/cryptopower/libwallet/wallets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/wallet/walletdata"
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
	vsps         []*mainW.VSP

	notificationListenersMu          sync.RWMutex
	syncData                         *SyncData
	accountMixerNotificationListener map[string]mainW.AccountMixerNotificationListener
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
		accountMixerNotificationListener: make(map[string]mainW.AccountMixerNotificationListener),
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
	wallet.accountMixerNotificationListener = make(map[string]mainW.AccountMixerNotificationListener)

	wallet.SetNetworkCancelCallback(wallet.SafelyCancelSync)

	return wallet.Prepare(rootDir, db, chainParams, wallet.ID)
}

func (wallet *Wallet) Synced() bool {
	return wallet.synced
}

func (wallet *Wallet) LockWallet() {
	if wallet.IsAccountMixerActive() {
		log.Error("LockWallet ignored due to active account mixer")
		return
	}

	if !wallet.Internal().Locked() {
		wallet.Internal().Lock()
	}
}

func (wallet *Wallet) SafelyCancelSync() {
	if wallet.IsConnectedToDecredNetwork() {
		wallet.CancelSync()
		defer func() {
			wallet.SpvSync()
		}()
	}
}
