package btc

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/lightninglabs/neutrino"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
	"golang.org/x/sync/errgroup"
)

const syncIntervalGap int32 = 2000

type SyncData struct {
	mu sync.RWMutex

	startBlock    *wtxmgr.BlockMeta
	syncStartTime time.Time
	lastlogging   time.Time

	// showLogs bool
	syncing       bool
	synced        bool
	isRescan      bool
	restartedScan bool

	syncStage utils.SyncStage

	// Listeners
	syncProgressListeners           map[string]sharedW.SyncProgressListener
	txAndBlockNotificationListeners map[string]sharedW.TxAndBlockNotificationListener
	blocksRescanProgressListener    sharedW.BlocksRescanProgressListener

	// Progress report information
	cfiltersFetchProgress    sharedW.CFiltersFetchProgressReport
	headersFetchProgress     sharedW.HeadersFetchProgressReport
	addressDiscoveryProgress sharedW.AddressDiscoveryProgressReport
	headersRescanProgress    sharedW.HeadersRescanProgressReport
}

func (asset *BTCAsset) initSyncProgressData() {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	asset.syncInfo.cfiltersFetchProgress = sharedW.CFiltersFetchProgressReport{}
	asset.syncInfo.headersFetchProgress = sharedW.HeadersFetchProgressReport{}
	asset.syncInfo.addressDiscoveryProgress = sharedW.AddressDiscoveryProgressReport{}
	asset.syncInfo.headersRescanProgress = sharedW.HeadersRescanProgressReport{}

	asset.syncInfo.cfiltersFetchProgress.GeneralSyncProgress = &sharedW.GeneralSyncProgress{}
	asset.syncInfo.headersFetchProgress.GeneralSyncProgress = &sharedW.GeneralSyncProgress{}
	asset.syncInfo.addressDiscoveryProgress.GeneralSyncProgress = &sharedW.GeneralSyncProgress{}
	asset.syncInfo.headersRescanProgress.GeneralSyncProgress = &sharedW.GeneralSyncProgress{}
}

func (asset *BTCAsset) resetSyncProgressData() {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	asset.syncInfo.syncing = false
	asset.syncInfo.synced = false
	asset.syncInfo.isRescan = false
	asset.syncInfo.restartedScan = false
}

func (asset *BTCAsset) AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	if _, ok := asset.syncInfo.txAndBlockNotificationListeners[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	if async {
		asset.syncInfo.txAndBlockNotificationListeners[uniqueIdentifier] = &sharedW.AsyncTxAndBlockNotificationListener{
			TxAndBlockNotificationListener: txAndBlockNotificationListener,
		}
		return nil
	}
	asset.syncInfo.txAndBlockNotificationListeners[uniqueIdentifier] = txAndBlockNotificationListener
	return nil
}

func (asset *BTCAsset) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	delete(asset.syncInfo.txAndBlockNotificationListeners, uniqueIdentifier)
}

func (asset *BTCAsset) AddSyncProgressListener(syncProgressListener sharedW.SyncProgressListener, uniqueIdentifier string) error {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	if _, ok := asset.syncInfo.syncProgressListeners[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.syncInfo.syncProgressListeners[uniqueIdentifier] = syncProgressListener
	return nil
}

func (asset *BTCAsset) RemoveSyncProgressListener(uniqueIdentifier string) {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	delete(asset.syncInfo.syncProgressListeners, uniqueIdentifier)
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

func (asset *BTCAsset) updateSyncProgress(rawBlock *wtxmgr.BlockMeta) {
	// sync must be running for further execution to proceed
	if !asset.IsSyncing() {
		log.Warn("Sync not running")
		return
	}

	asset.syncInfo.mu.Lock()
	// asset.syncInfo.isfetchingData = true

	defer func() {
		// asset.syncInfo.isfetchingData = false
		asset.syncInfo.mu.Unlock()
	}()

	// If the rawBlock is nil the network sync must have happenned. Resets the
	// sync info for the next sync use.
	if rawBlock == nil {
		asset.syncInfo.startBlock = nil
		return
	}

	// Best block synced in the connected peers
	bestBlockheight := asset.bestServerPeerBlockHeight()
	isSyncComplete := bestBlockheight == rawBlock.Height

	// initial set up when sync begins.
	if asset.syncInfo.startBlock == nil {
		asset.syncInfo.syncStage = utils.HeadersFetchSyncStage
		asset.syncInfo.syncStartTime = time.Now()
		asset.syncInfo.lastlogging = time.Now()
		asset.syncInfo.startBlock = rawBlock

		// Handle initial set up that also sync ups the local chain.
		for _, listener := range asset.syncInfo.syncProgressListeners {
			if isSyncComplete {
				listener.OnSyncCompleted()
			}
		}
		return
	}

	// log at intervals of 10 seconds or more to reduce excessive logging when faster
	// updates are streaming in.
	if time.Since(asset.syncInfo.lastlogging) >= 10*time.Second {
		log.Infof("Current sync progress update on block is %v, target sync block is %v", rawBlock.Height, bestBlockheight)
		asset.syncInfo.lastlogging = time.Now()
	}

	timeSpentSoFar := time.Since(asset.syncInfo.syncStartTime).Seconds()
	headersFetchedSoFar := rawBlock.Height - asset.syncInfo.startBlock.Height
	remainingHeaders := bestBlockheight - rawBlock.Height
	allHeadersToFetch := headersFetchedSoFar + remainingHeaders

	if timeSpentSoFar < 1 {
		timeSpentSoFar = 1
	}

	asset.syncInfo.headersFetchProgress.TotalHeadersToFetch = bestBlockheight
	asset.syncInfo.headersFetchProgress.HeadersFetchProgress = int32((float64(headersFetchedSoFar) * 100) / float64(allHeadersToFetch))
	asset.syncInfo.headersFetchProgress.GeneralSyncProgress.TotalSyncProgress = asset.syncInfo.headersFetchProgress.HeadersFetchProgress
	asset.syncInfo.headersFetchProgress.GeneralSyncProgress.TotalTimeRemainingSeconds = int64((float64(timeSpentSoFar) * float64(remainingHeaders)) / float64(headersFetchedSoFar))

	// publish the sync progress results to all listeners.
	for _, listener := range asset.syncInfo.syncProgressListeners {
		listener.OnHeadersFetchProgress(&asset.syncInfo.headersFetchProgress)

		// when synced send the sync completed status
		if isSyncComplete {
			listener.OnSyncCompleted()
		}
	}
}

func (asset *BTCAsset) fetchNotifications() {
	fmt.Println(" >>>>>> fetchNotifications() <<<<<<<")
	t := time.NewTicker(time.Second * 2)
	for {
		select {
		case n, ok := <-asset.chainClient.Notifications():
			if !ok {
				return
			}

			select {
			case <-t.C:
				d, _ := json.Marshal(n)
				log.Warn(" >>>>>> <-asset.chainClient.Notifications() <<<<<<<", string(d))
			case <-asset.syncCtx.Done():
				return
			}

			switch n := n.(type) {
			case chain.ClientConnected:
				// Notification type sent is sent when the client connects or reconnects
				// to the RPC server. It initialize the sync progress data report.
				asset.initSyncProgressData()

			case chain.BlockConnected:
				// Notification type is sent when a new block connects to the longest chain.
				// Trigger the progress report only when the block to be reported
				// is the best chaintip.
				if asset.bestServerPeerBlockHeight() == n.Block.Height {
					b := wtxmgr.BlockMeta(n)
					asset.updateSyncProgress(&b)
				}

			case chain.BlockDisconnected:
				// TODO: Implementation to be added
				fmt.Println(" >>>>>>>>>>>>>>>>>> BlockDisconnected <<<<<<<<<<<<<", n.Block.Height)
			case chain.RelevantTx:
				// TODO: Implementation to be added
				fmt.Println(" >>>>>>>>>>>>>>>>>> relevant tx <<<<<<<<<<<<<", n.Block.Height)
			case chain.FilteredBlockConnected:
				// Update the progress at the interval of syncIntervalGap or if current block is the last one.
				if (n.Block.Height%syncIntervalGap) == 0 || asset.bestServerPeerBlockHeight() == n.Block.Height {
					asset.updateSyncProgress(n.Block)
				}
			case *chain.RescanProgress:
				fmt.Println(" >>>>>>>>>>>>>>>>>> Rescan Progress <<<<<<<<<<<<<", n.Height)
				// TODO: Implementation to be added
			case *chain.RescanFinished:
				// TODO: Implementation to be added
				fmt.Println(" >>>>>>>>>>>>>>>>>> Rescan Finished <<<<<<<<<<<<<", n.Height)
			default:
				fmt.Println(" >>>>>>>>>>>>>>>>>> Default case <<<<<<<<<<<<<", n)
			}
		case <-asset.syncCtx.Done():
			return
		}
	}
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

	// Depending on the network, we add some addpeers or a connect peer. On
	// regtest, if the peers haven't been explicitly set, add the simnet harness
	// alpha node as an additional peer so we don't have to type it in. On
	// mainet and testnet3, add a known reliable persistent peer to be used in
	// addition to normal DNS seed-based peer discovery.
	var addPeers []string
	var connectPeers []string
	switch asset.chainParams.Net {
	case wire.MainNet:
		addPeers = []string{"cfilters.ssgen.io"}
	case wire.TestNet3:
		addPeers = []string{"dex-test.ssgen.io"}
	case wire.TestNet, wire.SimNet: // plain "wire.TestNet" is regnet!
		connectPeers = []string{"localhost:20575"}
	}

	log.Debug("Starting neutrino chain service...")
	chainService, err := neutrino.NewChainService(neutrino.Config{
		DataDir:       asset.DataDir(),
		Database:      asset.GetWalletDataDb(),
		ChainParams:   *asset.chainParams,
		PersistToDisk: true, // keep cfilter headers on disk for efficient rescanning
		AddPeers:      addPeers,
		ConnectPeers:  connectPeers,
		// WARNING: PublishTransaction currently uses the entire duration
		// because if an external bug, but even if the resolved, a typical
		// inv/getdata round trip is ~4 seconds, so we set this so neutrino does
		// not cancel queries too readily.
		BroadcastTimeout: 6 * time.Second,
	})
	if err != nil {
		log.Error(err)
		return fmt.Errorf("couldn't create Neutrino ChainService: %v", err)
	}

	asset.chainService = chainService
	asset.chainClient = chain.NewNeutrinoClient(asset.chainParams, chainService)

	return nil
}

func (asset *BTCAsset) CancelSync() {
	fmt.Println(" Canceling the sync for wallet: ", asset.GetWalletName())
	// stop the local sync notifications
	asset.cancelSync()

	if err := asset.chainService.Stop(); err != nil {
		log.Warnf("Error closing neutrino chain service: %v", err)
	}
}

func (asset *BTCAsset) IsConnectedToBitcoinNetwork() bool {
	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()

	return asset.syncInfo.syncing || asset.syncInfo.synced
}

// startWallet initializes the *btcwallet.Wallet and its supporting players and
// starts syncing.
func (asset *BTCAsset) startWallet() (err error) {
	if asset.chainClient.NotifyReceived([]btcutil.Address{}); err != nil {
		log.Error(err)
		return err
	}

	g, _ := errgroup.WithContext(asset.syncCtx)
	g.Go(asset.chainClient.Start)

	go asset.fetchNotifications()

	log.Info("Synchronizing BTC wallet with network...")

	go asset.Internal().BTC.SynchronizeRPC(asset.chainClient)

	asset.chainClient.WaitForShutdown()

	if err = g.Wait(); err != nil { // lazily starts connmgr
		asset.CancelSync()
		log.Errorf("couldn't start Neutrino client: %v", err)
		return err
	}

	return nil
}

func (asset *BTCAsset) ConnectSPVWallet() (err error) {
	// start the wallet and begin syncing.
	return asset.startWallet()
}

func (asset *BTCAsset) waitUntilBackendIsSynced() {
	// poll at intervals of 5 seconds if the backend is synced.
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if asset.chainClient.IsCurrent() {
				asset.syncInfo.mu.Lock()
				asset.syncInfo.synced = true
				asset.syncInfo.mu.Unlock()
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

	ctx, cancel := asset.ShutdownContextWithCancel()
	asset.mu.Lock()
	asset.syncCtx = ctx
	asset.cancelSync = cancel
	asset.mu.Unlock()

	var restartSyncRequested bool

	asset.syncInfo.mu.Lock()
	restartSyncRequested = asset.syncInfo.restartedScan
	asset.syncInfo.restartedScan = false
	asset.syncInfo.syncing = true
	asset.syncInfo.synced = false
	asset.syncInfo.mu.Unlock()

	for _, listener := range asset.syncInfo.syncProgressListeners {
		listener.OnSyncStarted(restartSyncRequested)
	}

	go asset.waitUntilBackendIsSynced()

	go func() {
		fmt.Println(" >>>>>> ConnectSPVWallet() <<<<<<<")
		err = asset.ConnectSPVWallet()
		if err != nil {
			log.Warn("error occured when starting BTC sync: ", err)
		}

		asset.resetSyncProgressData()
	}()

	return err
}
