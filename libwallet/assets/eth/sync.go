package eth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/ethereum/go-ethereum/core/types"
)

// SyncData holds the data required to sync the wallet.
type SyncData struct {
	mu sync.RWMutex

	bestBlockheight     int32 // Synced peers best block height.
	syncstarted         uint32
	txlistening         uint32
	chainServiceStopped bool

	syncing            bool
	synced             bool
	isRescan           bool
	rescanStartTime    time.Time
	rescanStartHeight  *int32
	isSyncShuttingDown bool

	wg sync.WaitGroup

	// Listeners
	syncProgressListeners map[string]sharedW.SyncProgressListener

	*activeSyncData
}

// reading/writing of properties of this struct are protected by syncData.mu.
type activeSyncData struct {
	syncStage utils.SyncStage

	cfiltersFetchProgress    sharedW.CFiltersFetchProgressReport
	headersFetchProgress     sharedW.HeadersFetchProgressReport
	addressDiscoveryProgress sharedW.AddressDiscoveryProgressReport
	headersRescanProgress    sharedW.HeadersRescanProgressReport
}

func (asset *Asset) initSyncProgressData() {
	cfiltersFetchProgress := sharedW.CFiltersFetchProgressReport{
		GeneralSyncProgress:         &sharedW.GeneralSyncProgress{},
		BeginFetchCFiltersTimeStamp: 0,
		StartCFiltersHeight:         -1,
		CfiltersFetchTimeSpent:      0,
		TotalFetchedCFiltersCount:   0,
	}

	headersFetchProgress := sharedW.HeadersFetchProgressReport{
		GeneralSyncProgress:   &sharedW.GeneralSyncProgress{},
		HeadersFetchTimeSpent: -1,
	}

	addressDiscoveryProgress := sharedW.AddressDiscoveryProgressReport{
		GeneralSyncProgress:       &sharedW.GeneralSyncProgress{},
		AddressDiscoveryStartTime: -1,
		TotalDiscoveryTimeSpent:   -1,
	}

	headersRescanProgress := sharedW.HeadersRescanProgressReport{}
	headersRescanProgress.GeneralSyncProgress = &sharedW.GeneralSyncProgress{}

	asset.syncData.mu.Lock()
	asset.syncData.activeSyncData = &activeSyncData{
		cfiltersFetchProgress:    cfiltersFetchProgress,
		headersFetchProgress:     headersFetchProgress,
		addressDiscoveryProgress: addressDiscoveryProgress,
		headersRescanProgress:    headersRescanProgress,
	}
	asset.syncData.mu.Unlock()
}

// IsSyncing returns true if the wallet is syncing.
func (asset *Asset) IsSyncing() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.syncing
}

// IsSynced returns true if the wallet is synced.
func (asset *Asset) IsSynced() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.synced
}

// IsSyncShuttingDown returns true if the wallet is shutting down.
func (asset *Asset) IsSyncShuttingDown() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.isSyncShuttingDown
}

// IsRescanning returns true if the wallet is currently rescanning the blockchain.
func (asset *Asset) IsRescanning() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.isRescan
}

func (asset *Asset) handleNotifications() {
}

func (asset *Asset) CancelRescan() {
	log.Error(utils.ErrETHMethodNotImplemented("CancelRescan"))
}

func (asset *Asset) CancelSync() {
	log.Error(utils.ErrETHMethodNotImplemented("CancelSync"))
}

func (asset *Asset) RescanBlocks() error {
	return utils.ErrETHMethodNotImplemented("RescanBlocks")
}

func (asset *Asset) ConnectedPeers() int32 {
	return int32(asset.stack.Server().PeerCount())
}

func (asset *Asset) RemovePeers() {
	log.Error(utils.ErrETHMethodNotImplemented("RemovePeers"))
}

func (asset *Asset) SetSpecificPeer(address string) {
	log.Error(utils.ErrETHMethodNotImplemented("SetSpecificPeer"))
}

func (asset *Asset) GetExtendedPubKey(account int32) (string, error) {
	return "", utils.ErrETHMethodNotImplemented("GetExtendedPubKey")
}

// IsConnectedToEthereumNetwork returns true if the wallet is connected to the
// Ethereum network.
func (asset *Asset) IsConnectedToEthereumNetwork() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	isSyncing := asset.syncData.syncing || asset.syncData.synced
	return isSyncing || asset.syncData.isRescan
}

// startSync initiates the full chain sync starting protocols. It attempts to
// restart the chain service if it hasn't been initialized.
func (asset *Asset) startSync() error {
	// Wait for chain events and push them to clients
	heads := make(chan *types.Header, 16)
	sub, err := asset.client.SubscribeNewHead(context.Background(), heads)
	if err != nil {
		return fmt.Errorf("failed to subscribe to head events: %v", err)
	}
	defer sub.Unsubscribe()

	return nil
}

// startWallet initializes the eth wallet and starts syncing.
func (asset *Asset) startWallet() (err error) {
	// If this is an imported wallet and address dicovery has not been performed,
	// We want to set the assets birtday to the genesis block.
	if asset.IsRestored && !asset.ContainsDiscoveredAccounts() {
		// asset.forceRescan()
	}
	// Initiate the sync protocol and return an error incase of failure.
	return asset.startSync()
}

// SpvSync initiates the full chain sync starting protocols. It attempts to
// restart the chain service if it hasn't been initialized.
func (asset *Asset) SpvSync() (err error) {
	if !asset.WalletOpened() {
		return utils.ErrETHNotInitialized
	}

	// prevent an attempt to sync when the previous syncing has not been canceled
	if asset.IsSyncing() || asset.IsSynced() {
		return errors.New(utils.ErrSyncAlreadyInProgress)
	}

	if err := asset.prepareChain(); err != nil {
		return fmt.Errorf("preparing chain failed: %v", err)
	}

	// Initialize all progress report data.
	asset.initSyncProgressData()

	ctx, cancel := asset.ShutdownContextWithCancel()
	asset.notificationListenersMu.Lock()
	asset.syncCtx = ctx
	asset.cancelSync = cancel
	asset.notificationListenersMu.Unlock()

	// Set wallet synced state to true when chainclient considers itself
	// as synced with the network.
	// go asset.waitForSyncCompletion()

	asset.syncData.mu.Lock()
	asset.syncData.syncing = true
	asset.syncData.synced = false
	asset.syncData.mu.Unlock()

	for _, listener := range asset.syncData.syncProgressListeners {
		listener.OnSyncStarted()
	}

	go func() {
		err = asset.startWallet()
		if err != nil {
			log.Warn("error occured when starting ETH sync: ", err)
		}
	}()

	return err
}
