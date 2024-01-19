package btc

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"decred.org/dcrwallet/v3/errors"
	"github.com/btcsuite/btcwallet/chain"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/lightninglabs/neutrino"
	"golang.org/x/sync/errgroup"
)

const (
	// start helps to synchronously execute compare-and-swap operation when
	// initiating the notifications handler.
	start uint32 = 1

	// stop helps to synchronously execute compare-and-swap operation when
	// terminating the notifications handler.
	stop uint32 = 0
)

// SyncData holds the data required to sync the wallet.
type SyncData struct {
	mu sync.RWMutex

	bestBlockheight     int32 // Synced peers best block height.
	syncstarted         uint32
	chainServiceStopped bool

	syncing            bool
	synced             bool
	isRescan           bool
	rescanStartTime    time.Time
	rescanStartHeight  *int32
	isSyncShuttingDown bool

	wg sync.WaitGroup

	// Listeners
	syncProgressListeners map[string]*sharedW.SyncProgressListener

	*activeSyncData
}

func (sd *SyncData) isSyncing() bool {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return sd.syncing
}

// reading/writing of properties of this struct are protected by syncData.mu.
type activeSyncData struct {
	syncStage utils.SyncStage

	cfiltersFetchProgress    sharedW.CFiltersFetchProgressReport
	headersFetchProgress     sharedW.HeadersFetchProgressReport
	addressDiscoveryProgress sharedW.AddressDiscoveryProgressReport
	headersRescanProgress    sharedW.HeadersRescanProgressReport
}

const (
	// InvalidSyncStage is the default sync stage.
	InvalidSyncStage = utils.InvalidSyncStage
	// CFiltersFetchSyncStage is the sync stage for fetching cfilters.
	CFiltersFetchSyncStage = utils.CFiltersFetchSyncStage
	// HeadersFetchSyncStage is the sync stage for fetching headers.
	HeadersFetchSyncStage = utils.HeadersFetchSyncStage
	// AddressDiscoverySyncStage is the sync stage for address discovery.
	AddressDiscoverySyncStage = utils.AddressDiscoverySyncStage
	// HeadersRescanSyncStage is the sync stage for headers rescan.
	HeadersRescanSyncStage = utils.HeadersRescanSyncStage
)

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

func (asset *Asset) resetSyncProgressData() {
	asset.syncData.syncing = false
	asset.syncData.synced = false
	asset.syncData.isRescan = false
}

// AddSyncProgressListener registers a sync progress listener to the asset.
func (asset *Asset) AddSyncProgressListener(syncProgressListener *sharedW.SyncProgressListener, uniqueIdentifier string) error {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if _, ok := asset.syncData.syncProgressListeners[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.syncData.syncProgressListeners[uniqueIdentifier] = syncProgressListener
	return nil
}

// RemoveSyncProgressListener unregisters a sync progress listener from the asset.
func (asset *Asset) RemoveSyncProgressListener(uniqueIdentifier string) {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	delete(asset.syncData.syncProgressListeners, uniqueIdentifier)
}

// bestServerPeerBlockHeight accesses the connected peers and requests for the
// last synced block height.
func (asset *Asset) bestServerPeerBlockHeight() {
	serverPeers := asset.chainClient.CS.Peers()
	for _, p := range serverPeers {
		if p.LastBlock() > asset.syncData.bestBlockheight {
			asset.syncData.bestBlockheight = p.LastBlock()
			// If a dormant peer is picked, on the next iteration it will be dropped
			// because it will be behind.
			return
		}
	}
}

func (asset *Asset) updateSyncProgress(rawBlockHeight int32) {
	asset.syncData.mu.Lock()

	// Update the best block synced in the connected peers if need be
	asset.bestServerPeerBlockHeight()

	// initial set up when sync begins.
	if asset.syncData.headersFetchProgress.StartHeaderHeight == nil {
		asset.syncData.syncStage = utils.HeadersFetchSyncStage
		asset.syncData.headersFetchProgress.BeginFetchTimeStamp = time.Now()
		asset.syncData.headersFetchProgress.StartHeaderHeight = &rawBlockHeight

		if asset.syncData.bestBlockheight != rawBlockHeight {
			asset.syncData.mu.Unlock()
			// A rescan progress update must have been sent. Allow it
			return
		}
	}
	log.Infof("Current sync progress update is on block %v, target sync block is %v", rawBlockHeight, asset.syncData.bestBlockheight)

	timeSpentSoFar := time.Since(asset.syncData.headersFetchProgress.BeginFetchTimeStamp).Seconds()
	if timeSpentSoFar < 1 {
		timeSpentSoFar = 1
	}

	headersFetchedSoFar := float64(rawBlockHeight - *asset.syncData.headersFetchProgress.StartHeaderHeight)
	if headersFetchedSoFar < 1 {
		headersFetchedSoFar = 1
	}

	remainingHeaders := float64(asset.syncData.bestBlockheight - rawBlockHeight)
	if remainingHeaders < 1 {
		remainingHeaders = 1
	}

	allHeadersToFetch := headersFetchedSoFar + remainingHeaders

	asset.syncData.headersFetchProgress.TotalHeadersToFetch = asset.syncData.bestBlockheight
	asset.syncData.headersFetchProgress.HeadersFetchProgress = int32((headersFetchedSoFar * 100) / allHeadersToFetch)
	asset.syncData.headersFetchProgress.GeneralSyncProgress.TotalSyncProgress = asset.syncData.headersFetchProgress.HeadersFetchProgress
	asset.syncData.headersFetchProgress.GeneralSyncProgress.TotalTimeRemainingSeconds = int64((timeSpentSoFar * remainingHeaders) / headersFetchedSoFar)
	asset.syncData.mu.Unlock()

	// publish the sync progress results to all listeners.
	asset.syncData.mu.RLock()
	for _, listener := range asset.syncData.syncProgressListeners {
		if listener.OnHeadersFetchProgress != nil {
			listener.OnHeadersFetchProgress(&asset.syncData.headersFetchProgress)
		}
	}
	asset.syncData.mu.RUnlock()
}

func (asset *Asset) publishHeadersFetchComplete() {
	asset.syncData.mu.Lock()
	asset.syncData.synced = true
	asset.syncData.syncing = false
	asset.syncData.mu.Unlock()

	asset.handleSyncUIUpdate()
}

func (asset *Asset) handleSyncUIUpdate() {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()
	for _, listener := range asset.syncData.syncProgressListeners {
		if listener.OnSyncCompleted != nil {
			listener.OnSyncCompleted()
		}
	}
}

func (asset *Asset) handleNotifications() {
	notes := asset.Internal().BTC.NtfnServer.TransactionNotifications()
	defer notes.Done()

notificationsLoop:
	for {
		select {
		case n := <-notes.C:
			for _, block := range n.AttachedBlocks {
				// When syncing historical data no tx are available.
				// Txs are reported only when chain is synced and newly mined tx
				// we discovered in the latest block.
				for _, tx := range block.Transactions {
					log.Debugf("(%v) Incoming mined tx with hash=%v block=%v",
						asset.GetWalletName(), tx.Hash, block.Height)

					// Publish the confirmed tx notification.
					asset.publishTransactionConfirmed(tx.Hash.String(), block.Height)
				}

				asset.publishBlockAttached(block.Height)
			}

			txToCache := make([]*sharedW.Transaction, len(n.UnminedTransactions))

			// handle txs hitting the mempool.
			for i, tx := range n.UnminedTransactions {
				log.Debugf("(%v) Incoming unmined tx with hash (%v)",
					asset.GetWalletName(), tx.Hash.String())

				// decodeTxs
				txToCache[i] = asset.decodeTransactionWithTxSummary(sharedW.UnminedTxHeight, tx)

				// publish mempool tx.
				asset.mempoolTransactionNotification(txToCache[i])
			}

			if len(n.UnminedTransactions) > 0 {
				// Since the tx cache receives a fresh update only when a new
				// block is detected, update cache with the newly received mempool tx(s).
				asset.txs.mu.Lock()
				asset.txs.unminedTxs = append(txToCache, asset.txs.unminedTxs...)
				asset.txs.mu.Unlock()
			}

		case <-asset.syncCtx.Done():
			break notificationsLoop
		}
	}
	// Signal that handleNotifications can be safely started next time its needed.
	atomic.StoreUint32(&asset.syncData.syncstarted, stop)
}

func (asset *Asset) rescanFinished(height int32) {
	// Notification type is sent when the rescan is completed.
	asset.updateSyncProgress(height)
	asset.publishHeadersFetchComplete()

	// Since the initial run on a restored wallet, address discovery
	// is complete, mark discovered accounts as true.
	if asset.IsRestored && !asset.ContainsDiscoveredAccounts() {
		// Update the assets birthday from genesis block to a date closer
		// to when the privatekey was first used.
		asset.updateAssetBirthday()
		_ = asset.MarkWalletAsDiscoveredAccounts()
	}

	asset.syncData.mu.Lock()
	asset.syncData.isRescan = false
	asset.syncData.mu.Unlock()

	if asset.blocksRescanProgressListener != nil {
		asset.blocksRescanProgressListener.OnBlocksRescanEnded(asset.ID, nil)
	}
}

// prepareChain sets up the chain service and the chain source
func (asset *Asset) prepareChain() error {
	exists, err := asset.WalletExists()
	if err != nil {
		return fmt.Errorf("error verifying wallet existence: %v", err)
	}
	if !exists {
		return errors.New("wallet not found")
	}

	log.Debug("Starting native BTC wallet sync...")
	chainService, err := asset.loadChainService()
	if err != nil {
		return err
	}

	asset.chainClient = chain.NewNeutrinoClient(asset.chainParams, chainService)

	return nil
}

func (asset *Asset) loadChainService() (chainService *neutrino.ChainService, err error) {
	peerAddresses := asset.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")
	validPeerAddresses, errs := sharedW.ParseWalletPeers(peerAddresses, asset.chainParams.DefaultPort)
	for _, err := range errs { // Log errors if any
		log.Error(err)
	}

	if len(validPeerAddresses) == 0 && len(errs) > 0 {
		return chainService, errors.New(utils.ErrInvalidPeers)
	}

	asset.dailerCtx, asset.dailerCancel = asset.ShutdownContextWithCancel()
	chainService, err = neutrino.NewChainService(neutrino.Config{
		DataDir:       asset.DataDir(),
		Database:      asset.GetWalletDataDb().BTC,
		ChainParams:   *asset.chainParams,
		PersistToDisk: true, // keep cfilter headers on disk for efficient rescanning
		ConnectPeers:  validPeerAddresses,
		// Dialer function helps to better control the dialer functionality.
		Dialer: utils.DialerFunc(asset.dailerCtx),
		// WARNING: PublishTransaction currently uses the entire duration
		// because if an external bug, but even if the resolved, a typical
		// inv/getdata round trip is ~4 seconds, so we set this so neutrino does
		// not cancel queries too readily.
		BroadcastTimeout: 6 * time.Second,
	})
	if err != nil {
		log.Error(err)
		return nil, fmt.Errorf("couldn't create Neutrino ChainService: %v", err)
	}
	asset.syncData.mu.Lock()
	asset.syncData.chainServiceStopped = false
	asset.syncData.mu.Unlock()

	return chainService, nil
}

// CancelSync stops the sync process.
func (asset *Asset) CancelSync() {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	log.Info("Canceling sync. May take a while for sync to fully cancel.")

	// Cancel all the pending tcp connection at the node level.
	asset.dailerCancel()

	// reset the sync data first.
	asset.resetSyncProgressData()

	// Call stopSync in a goroutine, stopSync's shutdown waits, block
	// the calling thread - the UI thread - if stopsync is not called in a different routine.
	asset.syncData.wg.Add(1)
	go asset.stopSync()

	log.Infof("(%v) SPV wallet closed", asset.GetWalletName())
}

// stopSync initiates the full chain sync stopping protocols.
// It does not stop the chain service which is intentionally left out since once
// stopped it can't be restarted easily.
func (asset *Asset) stopSync() {
	asset.syncData.isSyncShuttingDown = true
	loadedAsset := asset.Internal().BTC
	if asset.WalletOpened() {
		// If wallet shutdown is in progress ignore the current request to shutdown.
		if loadedAsset.ShuttingDown() {
			asset.syncData.isSyncShuttingDown = false
			asset.syncData.wg.Done()
			return
		}

		// Procedure to safely stop a wallet from syncing.
		// 1. Shutdown the upstream wallet.
		loadedAsset.Stop() // Stops the chainclient too.
	}

	// 2. shutdown the chain client.
	asset.chainClient.Stop() // If active, attempt to shut it down.

	if asset.WalletOpened() {
		// Neutrino performs explicit chain service start but never explicit
		// chain service stop thus the need to have it done here when stopping
		// a wallet sync.
		// 3. Disabling the peers connectivity allows the upstream handleChainNotification
		// goroutine to return.
		if err := asset.chainClient.CS.Stop(); err != nil {
			// ignore the error and proceed with shutdown.
			log.Errorf("Stopping chain client failed: %v", err)
		}

		asset.syncData.chainServiceStopped = true
		// 4. Wait for the upstream wallet to shutdown completely.
		loadedAsset.WaitForShutdown()
	}

	// 5. Wait for the chain client to shutdown
	asset.chainClient.WaitForShutdown()

	// Declares that the sync context is done and goroutines listening to it
	// should exit. The shutdown protocol will eventually attempt to end this
	// context but we do it early to avoid panics that happen after db has been
	// closed but some goroutines still interact with the db.
	asset.cancelSync()
	asset.syncData.isSyncShuttingDown = false

	log.Infof("Stopping (%s) wallet and its neutrino interface", asset.GetWalletName())

	if asset.WalletOpened() {
		// Initializes goroutine responsible for creating txs preventing double spend.
		// Initializes goroutine responsible for managing locked/unlocked wallet state.
		//
		// 6. This is being called at this point reason being that even though we
		// need to stop the wallet sync, the wallet needs to be started to handle
		// non sync related tasks such as changing password, renaming wallet, etc.
		// when syncing is disabled.
		loadedAsset.Start()
	}
	asset.syncData.wg.Done()
}

// startSync initiates the full chain sync starting protocols. It attempts to
// restart the chain service if it hasn't been initialized.
func (asset *Asset) startSync() error {
	g, _ := errgroup.WithContext(asset.syncCtx)

	if asset.syncData.chainServiceStopped {
		chainService, err := asset.loadChainService()
		if err != nil {
			return err
		}
		asset.chainClient.CS = chainService
	}

	// Chain client performs explicit chain service start up thus no need
	// to re-initialize it.
	g.Go(asset.chainClient.Start)

	if err := g.Wait(); err != nil {
		asset.CancelSync()
		log.Errorf("couldn't start Neutrino client: %v", err)
		return err
	}

	// Subscribe to chainclient notifications.
	if err := asset.chainClient.NotifyBlocks(); err != nil {
		log.Errorf("subscribing to notifications failed: %v", err)
		return err
	}

	// Listen and handle incoming notification events.
	if atomic.CompareAndSwapUint32(&asset.syncData.syncstarted, stop, start) {
		go asset.handleNotifications()
	}

	log.Infof("Synchronizing wallet (%s) with network...", asset.GetWalletName())
	// Initializes the goroutines handling chain notifications, rescan progress and handlers.
	asset.Internal().BTC.SynchronizeRPC(asset.chainClient)

	return nil
}

// IsConnectedToBitcoinNetwork returns true if the wallet is connected to the
// bitcoin network.
func (asset *Asset) IsConnectedToBitcoinNetwork() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	isSyncing := asset.syncData.syncing || asset.syncData.synced
	return isSyncing || asset.syncData.isRescan
}

// startWallet initializes the *btcwallet.Wallet and its supporting players and
// starts syncing.
func (asset *Asset) startWallet() (err error) {
	// If this is an imported wallet and address discovery has not been performed,
	// We want to set the assets birtday to the genesis block.
	if asset.IsRestored && !asset.ContainsDiscoveredAccounts() {
		asset.forceRescan()
	}
	// Initiate the sync protocol and return an error incase of failure.
	return asset.startSync()
}

// waitForSyncCompletion polls if the chain considers if itself as the current
// view of the network as synced. This is most helpful for the cases where the
// current block on the wallet is already synced but the next notification
// showing this change in chain view might take close to 10 minutes to come.
func (asset *Asset) waitForSyncCompletion() {
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			block, err := asset.chainClient.CS.BestBlock()
			if err != nil {
				log.Error("GetBestBlock hash for BTC failed, Err: ", err)
			}
			asset.updateSyncProgress(block.Height)
			asset.updateRescanProgress(block.Height)

			if asset.chainClient.IsCurrent() {
				asset.rescanFinished(block.Height)

				asset.syncData.mu.Lock()
				asset.syncData.synced = true
				asset.syncData.syncing = false
				asset.syncData.mu.Unlock()

				// Trigger UI update showing btc address recovery is in progress.
				asset.handleSyncUIUpdate()
				return
			}
		case <-asset.syncCtx.Done():
			return
		}
	}
}

// SpvSync initiates the full chain sync starting protocols. It attempts to
// restart the chain service if it hasn't been initialized.
func (asset *Asset) SpvSync() (err error) {
	if !asset.WalletOpened() {
		return utils.ErrBTCNotInitialized
	}

	// prevent an attempt to sync when the previous syncing has not been canceled
	if asset.IsSyncing() || asset.IsSynced() {
		return errors.New(utils.ErrSyncAlreadyInProgress)
	}

	// Initialize all progress report data.
	asset.initSyncProgressData()

	ctx, cancel := asset.ShutdownContextWithCancel()
	asset.notificationListenersMu.Lock()
	asset.syncCtx = ctx
	asset.cancelSync = cancel
	asset.notificationListenersMu.Unlock()

	asset.syncData.mu.Lock()
	asset.syncData.syncing = true
	asset.syncData.synced = false
	asset.syncData.mu.Unlock()

	// Set wallet synced state to true when chainclient considers itself
	// as synced with the network.
	go asset.waitForSyncCompletion()

	for _, listener := range asset.syncData.syncProgressListeners {
		if listener.OnSyncStarted != nil {
			listener.OnSyncStarted()
		}
	}

	go func() {
		err = asset.startWallet()
		if err != nil {
			log.Warn("error occurred when starting BTC sync: ", err)
		}
	}()

	return err
}

// reloadChainService loads a new instance of chain service to be used
// for sync. It restarts sync if the wallet was previously connected to the btc newtork
// before the function call.
func (asset *Asset) reloadChainService() error {
	if !asset.WalletOpened() {
		return utils.ErrBTCNotInitialized
	}

	isPrevConnected := asset.IsConnectedToNetwork()
	if isPrevConnected {
		asset.CancelSync()
	}

	_ = asset.chainClient.CS.Stop()
	chainService, err := asset.loadChainService()
	if err != nil {
		return err
	}
	asset.chainClient.CS = chainService

	// If the asset is previously connected to the network call SpvSync to
	// start sync using the new instance of chain service.
	if isPrevConnected {
		return asset.SpvSync()
	}
	return nil
}
