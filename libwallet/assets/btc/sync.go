package btc

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/lightninglabs/neutrino"
	"golang.org/x/sync/errgroup"
)

const (
	// syncIntervalGap defines the interval at which to publish and log progress
	// without unnecessarily spamming the reciever.
	syncIntervalGap = time.Second * 3

	// start helps to synchronously execute compare-and-swap operation when
	// initiating the notifications handler.
	start int32 = 1

	// stop helps to synchronously execute compare-and-swap operation when
	// terminating the notifications handler.
	stop int32 = 0
)

type SyncData struct {
	mu sync.RWMutex

	startBlock    *wtxmgr.BlockMeta
	syncStartTime time.Time
	syncstarted   int32

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
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	// Best block synced in the connected peers
	bestBlockheight := asset.bestServerPeerBlockHeight()

	// initial set up when sync begins.
	if asset.syncInfo.startBlock == nil {
		asset.syncInfo.syncStage = utils.HeadersFetchSyncStage
		asset.syncInfo.syncStartTime = time.Now()
		asset.syncInfo.startBlock = rawBlock

		if bestBlockheight != rawBlock.Height {
			// A rescan progress update must have been sent. Allow it
			return
		}
	}

	log.Infof("Current sync progress update is on block %v, target sync block is %v", rawBlock.Height, bestBlockheight)

	timeSpentSoFar := time.Since(asset.syncInfo.syncStartTime).Seconds()
	if timeSpentSoFar < 1 {
		timeSpentSoFar = 1
	}

	headersFetchedSoFar := float64(rawBlock.Height - asset.syncInfo.startBlock.Height)
	if headersFetchedSoFar < 1 {
		headersFetchedSoFar = 1
	}

	remainingHeaders := float64(bestBlockheight - rawBlock.Height)
	if remainingHeaders < 1 {
		remainingHeaders = 1
	}

	allHeadersToFetch := headersFetchedSoFar + remainingHeaders

	asset.syncInfo.headersFetchProgress.TotalHeadersToFetch = bestBlockheight
	asset.syncInfo.headersFetchProgress.HeadersFetchProgress = int32((headersFetchedSoFar * 100) / allHeadersToFetch)
	asset.syncInfo.headersFetchProgress.GeneralSyncProgress.TotalSyncProgress = asset.syncInfo.headersFetchProgress.HeadersFetchProgress
	asset.syncInfo.headersFetchProgress.GeneralSyncProgress.TotalTimeRemainingSeconds = int64((timeSpentSoFar * remainingHeaders) / headersFetchedSoFar)

	// publish the sync progress results to all listeners.
	for _, listener := range asset.syncInfo.syncProgressListeners {
		listener.OnHeadersFetchProgress(&asset.syncInfo.headersFetchProgress)
	}
}

func (asset *BTCAsset) publishHeadersFetchComplete() {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	for _, listener := range asset.syncInfo.syncProgressListeners {
		listener.OnSyncCompleted()
	}

	asset.syncInfo.synced = true
	asset.syncInfo.syncing = false
}

// publishRelevantTxs publishes all the relevant tx identified in a filtered
// block. A valid list of addresses associated with the current block need to
// be provided.
func (asset *BTCAsset) publishRelevantTxs(txs []*wtxmgr.TxRecord) {
	if txs == nil {
		return
	}

	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()

	for _, tx := range txs {
		//TODO: Properly decode a btc TxRecord into the sharedW.Transaction
		// Issue referenced here: https://code.cryptopower.dev/group/cryptopower/-/issues/1160
		tempTransaction := sharedW.Transaction{
			WalletID:  asset.GetWalletID(),
			Hash:      tx.Hash.String(),
			Type:      txhelper.TxTypeRegular,
			Direction: txhelper.TxDirectionReceived,
		}
		for _, listener := range asset.syncInfo.txAndBlockNotificationListeners {
			result, err := json.Marshal(tempTransaction)
			if err != nil {
				log.Error(err)
			}
			listener.OnTransaction(string(result))
		}
	}
}

// publishNewBlock once the initial sync is complete all the new blocks recieved
// are published through this method.
func (asset *BTCAsset) publishNewBlock(rawBlock *wtxmgr.BlockMeta) {
	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()

	for _, listener := range asset.syncInfo.txAndBlockNotificationListeners {
		listener.OnBlockAttached(asset.ID, rawBlock.Height)
	}
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
				// Notification type sent is sent when the client connects or reconnects
				// to the RPC server. It initialize the sync progress data report.

			case chain.BlockConnected:
				// Notification type is sent when a new block connects to the longest chain.
				// Trigger the progress report only when the block to be reported
				// is the best chaintip.
				select {
				case <-t.C:
					b := wtxmgr.BlockMeta(n)
					if !asset.IsSynced() {
						// initial sync is inprogress.
						asset.updateSyncProgress(&b)
					} else {
						// initial sync is complete
						asset.publishNewBlock(&b)
					}
				default:
				}

			case chain.BlockDisconnected:
				// TODO: Implementation to be added
			case chain.FilteredBlockConnected:
				// if relevants txs were detected. Atempt to send them first
				asset.publishRelevantTxs(n.RelevantTxs)

				// Update the progress at the interval of syncIntervalGap.
				select {
				case <-t.C:
					asset.updateSyncProgress(n.Block)
				default:
				}
			case *chain.RescanProgress:
				// TODO: Implementation to be added
			case *chain.RescanFinished:
				// Notification type is sent when the rescan is completed.
				b := wtxmgr.BlockMeta{
					Block: wtxmgr.Block{
						Hash:   *n.Hash,
						Height: n.Height,
					},
					Time: n.Time,
				}
				asset.updateSyncProgress(&b)
				asset.publishHeadersFetchComplete()

				// once initial scan is complete reset the ticket to track every
				// new block or transaction detected.
				t = time.NewTicker(1 * time.Second)
			}
		case <-asset.syncCtx.Done():
			break notificationsLoop
		}
	}

	// stop the ticker timer.
	t.Stop()
	// Signal that handleNotifications can be safely started next time its needed.
	atomic.StoreInt32(&asset.syncInfo.syncstarted, stop)
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
		//TODO: Add more servers to connect peers from.
		addPeers = []string{"cfilters.ssgen.io"}
	case wire.TestNet3:
		//TODO: Add more servers to connect peers from.
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
	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()

	// reset the sync data first.
	asset.resetSyncProgressData()

	g, _ := errgroup.WithContext(asset.syncCtx)
	g.Go(asset.chainService.Stop)

	// stop the local sync notifications
	asset.cancelSync()

	if err := g.Wait(); err != nil {
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
	g, _ := errgroup.WithContext(asset.syncCtx)
	g.Go(asset.chainClient.Start)

	asset.chainClient.WaitForShutdown()

	if err = g.Wait(); err != nil {
		asset.CancelSync()
		log.Errorf("couldn't start Neutrino client: %v", err)
		return err
	}

	//TODO: set a valid list of addresses to track. This helps to received all
	// the relevant tx identified in a block.
	if asset.chainClient.NotifyReceived([]btcutil.Address{}); err != nil {
		log.Error(err)
		return err
	}

	go func() {
		if atomic.CompareAndSwapInt32(&asset.syncInfo.syncstarted, stop, start) {
			asset.handleNotifications()
		}
	}()

	log.Info("Synchronizing BTC wallet with network...")

	go asset.Internal().BTC.SynchronizeRPC(asset.chainClient)

	return nil
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
				asset.syncInfo.mu.Lock()
				asset.syncInfo.synced = true
				asset.syncInfo.syncing = false
				asset.syncInfo.mu.Unlock()
				return
			}
		case <-asset.syncCtx.Done():
			return
		}
	}
}

func (asset *BTCAsset) SpvSync() (err error) {
	if !asset.IsSynced() {
		// instead of waiting until the next block's notification comes run this.
		go asset.waitForSyncCompletion()
	}

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

	go func() {
		err = asset.startWallet()
		if err != nil {
			log.Warn("error occured when starting BTC sync: ", err)
		}
	}()

	return err
}
