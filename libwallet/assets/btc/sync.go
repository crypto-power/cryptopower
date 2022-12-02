package btc

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/lightninglabs/neutrino"
	"golang.org/x/sync/errgroup"
)

const (
	// syncIntervalGap defines the interval at which to publish and log progress
	// without unnecessarily spamming the reciever.
	syncIntervalGap = time.Second * 3

	// start helps to synchronously execute compare-and-swap operation when
	// initiating the notifications handler.
	start uint32 = 1

	// stop helps to synchronously execute compare-and-swap operation when
	// terminating the notifications handler.
	stop uint32 = 0
)

type SyncData struct {
	mu sync.RWMutex

	startHeight   *int32
	syncStartTime time.Time
	syncstarted   uint32
	txlistening   uint32

	syncing            bool
	synced             bool
	isRescan           bool
	restartedScan      bool
	isSyncShuttingDown bool

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

const (
	InvalidSyncStage          = utils.InvalidSyncStage
	CFiltersFetchSyncStage    = utils.CFiltersFetchSyncStage
	HeadersFetchSyncStage     = utils.HeadersFetchSyncStage
	AddressDiscoverySyncStage = utils.AddressDiscoverySyncStage
	HeadersRescanSyncStage    = utils.HeadersRescanSyncStage
)

func (asset *BTCAsset) initSyncProgressData() {
	cfiltersFetchProgress := sharedW.CFiltersFetchProgressReport{
		GeneralSyncProgress:         &sharedW.GeneralSyncProgress{},
		BeginFetchCFiltersTimeStamp: 0,
		StartCFiltersHeight:         -1,
		CfiltersFetchTimeSpent:      0,
		TotalFetchedCFiltersCount:   0,
	}

	headersFetchProgress := sharedW.HeadersFetchProgressReport{
		GeneralSyncProgress:      &sharedW.GeneralSyncProgress{},
		BeginFetchTimeStamp:      -1,
		HeadersFetchTimeSpent:    -1,
		TotalFetchedHeadersCount: 0,
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

func (asset *BTCAsset) resetSyncProgressData() {
	asset.syncData.syncing = false
	asset.syncData.synced = false
	asset.syncData.isRescan = false
	asset.syncData.restartedScan = false
}

func (asset *BTCAsset) AddSyncProgressListener(syncProgressListener sharedW.SyncProgressListener, uniqueIdentifier string) error {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if _, ok := asset.syncData.syncProgressListeners[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.syncData.syncProgressListeners[uniqueIdentifier] = syncProgressListener
	return nil
}

func (asset *BTCAsset) RemoveSyncProgressListener(uniqueIdentifier string) {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	delete(asset.syncData.syncProgressListeners, uniqueIdentifier)
}

// bestServerPeerBlockHeight accesses the connected peers and requests for the
// last synced block height in each one of them.
func (asset *BTCAsset) bestServerPeerBlockHeight() (height int32) {
	serverPeers := asset.chainClient.CS.Peers()
	for _, p := range serverPeers {
		if p.LastBlock() > height {
			height = p.LastBlock()
		}
	}
	return
}

func (asset *BTCAsset) updateSyncProgress(rawBlockHeight int32) {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	// Best block synced in the connected peers
	bestBlockheight := asset.bestServerPeerBlockHeight()

	// initial set up when sync begins.
	if asset.syncData.startHeight == nil {
		asset.syncData.syncStage = utils.HeadersFetchSyncStage
		asset.syncData.syncStartTime = time.Now()
		asset.syncData.startHeight = &rawBlockHeight

		if bestBlockheight != rawBlockHeight {
			// A rescan progress update must have been sent. Allow it
			return
		}
	}

	log.Infof("Current sync progress update is on block %v, target sync block is %v", rawBlockHeight, bestBlockheight)

	timeSpentSoFar := time.Since(asset.syncData.syncStartTime).Seconds()
	if timeSpentSoFar < 1 {
		timeSpentSoFar = 1
	}

	headersFetchedSoFar := float64(rawBlockHeight - *asset.syncData.startHeight)
	if headersFetchedSoFar < 1 {
		headersFetchedSoFar = 1
	}

	remainingHeaders := float64(bestBlockheight - rawBlockHeight)
	if remainingHeaders < 1 {
		remainingHeaders = 1
	}

	allHeadersToFetch := headersFetchedSoFar + remainingHeaders

	asset.syncData.headersFetchProgress.TotalHeadersToFetch = bestBlockheight
	asset.syncData.headersFetchProgress.HeadersFetchProgress = int32((headersFetchedSoFar * 100) / allHeadersToFetch)
	asset.syncData.headersFetchProgress.GeneralSyncProgress.TotalSyncProgress = asset.syncData.headersFetchProgress.HeadersFetchProgress
	asset.syncData.headersFetchProgress.GeneralSyncProgress.TotalTimeRemainingSeconds = int64((timeSpentSoFar * remainingHeaders) / headersFetchedSoFar)

	// publish the sync progress results to all listeners.
	for _, listener := range asset.syncData.syncProgressListeners {
		listener.OnHeadersFetchProgress(&asset.syncData.headersFetchProgress)
	}
}

func (asset *BTCAsset) publishHeadersFetchComplete() {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	for _, listener := range asset.syncData.syncProgressListeners {
		listener.OnSyncCompleted()
	}

	asset.syncData.synced = true
	asset.syncData.syncing = false
}

func (asset *BTCAsset) handleNotifications() {
	t := time.NewTicker(syncIntervalGap)

notificationsLoop:
	for {
		select {
		case n, ok := <-asset.chainClient.Notifications():
			if !ok {
				continue notificationsLoop
			}

			switch n := n.(type) {
			case chain.ClientConnected:
			case chain.BlockConnected:
				// Notification type is sent when a new block connects to the longest chain.
				// Trigger the progress report only when the block to be reported
				// is the best chaintip.

				select {
				case <-t.C:
					if !asset.IsSynced() {
						// initial sync is inprogress.
						asset.updateSyncProgress(n.Block.Height)
					} else {
						// initial sync is complete
						asset.publishBlockAttached(n.Block.Height)
					}
				default:
				}

			case chain.BlockDisconnected:
				select {
				case <-t.C:
					if !asset.IsSynced() {
						// initial sync is inprogress.
						asset.updateSyncProgress(n.Height)
					} else {
						// initial sync is complete
						asset.publishBlockAttached(n.Height)
					}
				default:
				}

			case chain.FilteredBlockConnected:
				// if relevants txs were detected. Atempt to send them first
				for _, tx := range n.RelevantTxs {
					asset.publishTransactionConfirmed(tx.Hash.String(), n.Block.Height)
				}

				// Update the progress at the interval of syncIntervalGap.
				select {
				case <-t.C:
					asset.updateSyncProgress(n.Block.Height)
				default:
				}

			case *chain.RescanProgress:
				// Notifications sent at interval of 10k blocks
				asset.updateSyncProgress(n.Height)

			case *chain.RescanFinished:
				asset.syncData.mu.Lock()
				asset.syncData.isRescan = false
				asset.syncData.mu.Unlock()

				// Notification type is sent when the rescan is completed.
				asset.updateSyncProgress(n.Height)
				asset.publishHeadersFetchComplete()

				// once initial scan is complete reset the ticket to track every
				// new block or transaction detected.
				t.Reset(1 * time.Second)

				// Only run the listener once the chain is synced and ready to listen
				// for newly mined block. This prevents unnecessary CPU use spikes
				// on startup when a wallet is syncing from scratch.
				go asset.listenForTransactions()

				// update the birthday and birthday block so that on next startup,
				// the recovery if necessary takes lesser time.
				go asset.updateAssetBirthday()

				// Since the initial run on a restored wallet, address discovery
				// is complete, mark discovered accounts as true.
				if asset.IsRestored && !asset.ContainsDiscoveredAccounts() {
					asset.MarkWalletAsDiscoveredAccounts()
				}
			}
		case <-asset.syncCtx.Done():
			break notificationsLoop
		}
	}

	// stop the ticker timer.
	t.Stop()
	// Signal that handleNotifications can be safely started next time its needed.
	atomic.StoreUint32(&asset.syncData.syncstarted, stop)
}

// prepareChain sets up the chain service and the chain source
func (asset *BTCAsset) prepareChain() error {
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

	asset.chainService = chainService
	asset.chainClient = chain.NewNeutrinoClient(asset.chainParams, chainService)

	return nil
}

func (asset *BTCAsset) loadChainService() (chainService *neutrino.ChainService, err error) {
	// Read config for persistent peers, if set parse and set neutrino's ConnectedPeers
	// persistentPeers.
	var persistentPeers []string
	peerAddresses := asset.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")
	if peerAddresses != "" {
		addresses := strings.Split(peerAddresses, ";")
		for _, address := range addresses {
			peerAddress, err := utils.NormalizeAddress(address, asset.chainParams.DefaultPort)
			if err != nil {
				log.Errorf("SPV peer address(%s) is invalid: %v", peerAddress, err)
			} else {
				persistentPeers = append(persistentPeers, peerAddress)
			}
		}

		if len(persistentPeers) == 0 {
			return chainService, errors.New(utils.ErrInvalidPeers)
		}
	}

	chainService, err = neutrino.NewChainService(neutrino.Config{
		DataDir:       asset.DataDir(),
		Database:      asset.GetWalletDataDb(),
		ChainParams:   *asset.chainParams,
		PersistToDisk: true, // keep cfilter headers on disk for efficient rescanning
		ConnectPeers:  persistentPeers,
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

	return chainService, nil
}

func (asset *BTCAsset) CancelSync() {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	log.Info("Canceling sync. May take a while for sync to fully cancel.")

	// reset the sync data first.
	asset.resetSyncProgressData()

	asset.stopSync()

	log.Info("SPV wallet closed")
}

// stopSync initiates the full chain sync stopping protocols.
// It does not stop the chain service which is intentionally left out since once
// stopped it can't be restarted easily.
func (asset *BTCAsset) stopSync() {
	asset.syncData.isSyncShuttingDown = true
	loadedAsset := asset.Internal().BTC
	if loadedAsset != nil {
		if loadedAsset.ShuttingDown() {
			asset.syncData.isSyncShuttingDown = false
			return
		}

		loadedAsset.Stop() // Stops the wallet to stop listion notification handler when syncing.
		loadedAsset.WaitForShutdown()
		// Initializes goroutine responsible for creating txs preventing double spend.
		// Initializes goroutine responsible for managing locked/unlocked wallet state.
		//
		// This is being called at this point reason being that even though we need to stop the wallet sync,
		// the wallet needs to be started to handle non sync related tasks such as changing password, renaming wallet, etc.
		loadedAsset.Start()
	}

	if asset.chainClient != nil {
		log.Info("Stopping neutrino client service interface")

		asset.chainClient.Stop() // If active, attempt to shut it down.
		asset.chainClient.WaitForShutdown()

		// Neutrino performs explicit chain service start but never explicit
		// chain service stop thus the need to have it done here when stopping
		// a wallet sync.
		asset.chainClient.CS.Stop()
		asset.chainClient = nil
	}
	asset.cancelSync()
	asset.syncData.isSyncShuttingDown = false
}

// startSync initiates the full chain sync starting protocols. It attempts to
// restart the chain service if it hasn't been initialized.
func (asset *BTCAsset) startSync() error {
	g, _ := errgroup.WithContext(asset.syncCtx)

	if asset.chainClient == nil {
		var err error
		asset.chainClient, err = asset.newChainClient()
		if err != nil {
			return err
		}
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

	log.Infof("Synchronizing wallet (%s) with network...", asset.GetWalletName())
	// Initializes the goroutines handling chain notifications, rescan progress and handlers.
	asset.Internal().BTC.SynchronizeRPC(asset.chainClient)

	select {
	// Wait for 5 seconds so that all goroutines initialized in SynchronizeRPC()
	// can startup successfully. To be specific, btcwallet's handleChainNotifications()
	// should have completed setting up by the time asset.handleNotifications() starts up.
	// This 5 seconds delay is arbitrary chosen, and if found inadequate in future,
	// it could be increased.
	case <-time.After(time.Second * 5):
	case <-asset.syncCtx.Done():
	}

	// Listen and handle incoming notification events.
	if atomic.CompareAndSwapUint32(&asset.syncData.syncstarted, stop, start) {
		go asset.handleNotifications()
	}

	return nil
}

func (asset *BTCAsset) IsConnectedToBitcoinNetwork() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.syncing || asset.syncData.synced
}

// startWallet initializes the *btcwallet.Wallet and its supporting players and
// starts syncing.
func (asset *BTCAsset) startWallet() (err error) {
	if asset.isRecoveryRequired() {
		if !asset.AllowAutomaticRescan() {
			return errors.New("cannot set earlier birthday while there are active deals")
		}

		log.Infof("Atempting a Forced Rescan on wallet (%s)", asset.GetWalletName())
		asset.ForceRescan()
	}

	// Initiate the sync protocol and return an error incase of failure.
	return asset.startSync()
}

// waitForSyncCompletion polls if the chain considers if itself as the current
// view of the network as synced. This is most helpful for the cases where the
// current block on the wallet is already synced but the next notification
// showing this change in chain view might take close to 10 minutes to come.
func (asset *BTCAsset) waitForSyncCompletion() {
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if asset.chainClient.IsCurrent() {
				asset.syncData.mu.Lock()
				asset.syncData.synced = true
				asset.syncData.syncing = false
				asset.syncData.mu.Unlock()
				return
			}
		case <-asset.syncCtx.Done():
			return
		}
	}
}

func (asset *BTCAsset) SpvSync() (err error) {
	// prevent an attempt to sync when the previous syncing has not been canceled
	if asset.IsSyncing() || asset.IsSynced() {
		return errors.New(utils.ErrSyncAlreadyInProgress)
	}

	// Initialize all progress report data.
	asset.initSyncProgressData()

	ctx, cancel := asset.ShutdownContextWithCancel()
	asset.mu.Lock()
	asset.syncCtx = ctx
	asset.cancelSync = cancel
	asset.mu.Unlock()

	// Set wallet synced state to true when chainclient considers itself
	// as synced with the network.
	go asset.waitForSyncCompletion()

	var restartSyncRequested bool

	asset.syncData.mu.Lock()
	restartSyncRequested = asset.syncData.restartedScan
	asset.syncData.restartedScan = false
	asset.syncData.syncing = true
	asset.syncData.synced = false
	asset.syncData.mu.Unlock()

	for _, listener := range asset.syncData.syncProgressListeners {
		listener.OnSyncStarted(restartSyncRequested)
	}

	go func() {
		err = asset.startWallet()
		if err != nil {
			log.Warn("error occured when starting BTC sync: ", err)
		}
	}()

	return err
}

func (asset *BTCAsset) ResetChainService() error {
	chainService, err := asset.loadChainService()
	if err != nil {
		log.Error(err)
		return err
	}

	asset.SafelyCancelSync()
	asset.chainClient.CS, asset.chainService = chainService, chainService

	asset.syncData.mu.Lock()
	asset.syncData.restartedScan = true
	asset.syncData.mu.Unlock()

	return asset.SpvSync()
}
