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
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/walletdb"
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
	syncCanceled  chan struct{}

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

func (asset *BTCAsset) updateSyncProgress(rawBlock *wtxmgr.BlockMeta) {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	asset.setSyncedTo(rawBlock)

	// Best block synced in the connected peers
	bestBlockheight := asset.bestServerPeerBlockHeight()

	// initial set up when sync begins.
	if asset.syncData.startBlock == nil {
		asset.syncData.syncStage = utils.HeadersFetchSyncStage
		asset.syncData.syncStartTime = time.Now()
		asset.syncData.startBlock = rawBlock

		if bestBlockheight != rawBlock.Height {
			// A rescan progress update must have been sent. Allow it
			return
		}
	}

	log.Infof("Current sync progress update is on block %v, target sync block is %v", rawBlock.Height, bestBlockheight)

	timeSpentSoFar := time.Since(asset.syncData.syncStartTime).Seconds()
	if timeSpentSoFar < 1 {
		timeSpentSoFar = 1
	}

	headersFetchedSoFar := float64(rawBlock.Height - asset.syncData.startBlock.Height)
	if headersFetchedSoFar < 1 {
		headersFetchedSoFar = 1
	}

	remainingHeaders := float64(bestBlockheight - rawBlock.Height)
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

// setSyncedTo marks the wallet manager to be in sync with the recently-seen
// block as reported by the rawBlock parameter.
func (asset *BTCAsset) setSyncedTo(rawBlock *wtxmgr.BlockMeta) {
	wdb := asset.Internal().BTC.Database()
	bs := waddrmgr.BlockStamp{
		Height:    rawBlock.Height,
		Hash:      rawBlock.Hash,
		Timestamp: rawBlock.Time,
	}
	err := walletdb.Update(wdb, func(dbtx walletdb.ReadWriteTx) error {
		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)
		return asset.Internal().BTC.Manager.SetSyncedTo(ns, &bs)
	})
	if err != nil {
		log.Error(err)
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

// publishRelevantTxs publishes all the relevant tx identified in a filtered
// block. A valid list of addresses associated with the current block need to
// be provided.
func (asset *BTCAsset) publishRelevantTxs(txs []*wtxmgr.TxRecord) {
	if txs == nil {
		return
	}

	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	for _, tx := range txs {
		//TODO: Properly decode a btc TxRecord into the sharedW.Transaction
		// Issue referenced here: https://code.cryptopower.dev/group/cryptopower/-/issues/1160
		tempTransaction := sharedW.Transaction{
			WalletID:  asset.GetWalletID(),
			Hash:      tx.Hash.String(),
			Type:      txhelper.TxTypeRegular,
			Direction: txhelper.TxDirectionReceived,
		}
		for _, listener := range asset.txAndBlockNotificationListeners {
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
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	asset.setSyncedTo(rawBlock)

	for _, listener := range asset.txAndBlockNotificationListeners {
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
				select {
				case <-t.C:
					b := wtxmgr.BlockMeta{
						Block: wtxmgr.Block{
							Hash:   *n.Hash,
							Height: n.Height,
						},
						Time: n.Time,
					}
					asset.updateSyncProgress(&b)
				default:
				}
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
	atomic.StoreInt32(&asset.syncData.syncstarted, stop)
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

func (asset *BTCAsset) Birthday() time.Time {
	return asset.Wallet.CreatedAt
}

func (asset *BTCAsset) updateDBBirthday(bday time.Time) error {
	return walletdb.Update(asset.Internal().BTC.Database(), func(dbtx walletdb.ReadWriteTx) error {
		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)
		return asset.Internal().BTC.Manager.SetBirthday(ns, bday)
	})
}

func (asset *BTCAsset) CancelSync() {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	log.Info("Canceling sync. May take a while for sync to fully cancel.")

	// reset the sync data first.
	asset.resetSyncProgressData()

	if asset.chainClient != nil {
		log.Info("Stopping neutrino client chain interface")
		asset.chainClient.Stop()
		asset.chainClient.WaitForShutdown()
	}

	// stop the local sync notifications
	// asset.cancelSync()  //TODO: Update cancel logic
	log.Info("SPV wallet closed")
}

func (asset *BTCAsset) IsConnectedToBitcoinNetwork() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.syncing || asset.syncData.synced
}

// startWallet initializes the *btcwallet.Wallet and its supporting players and
// starts syncing.
func (asset *BTCAsset) startWallet() (err error) {

	oldBday := asset.Internal().BTC.Manager.Birthday()

	performRescan := asset.Birthday().Before(oldBday)
	if performRescan && !asset.AllowAutomaticRescan() {
		return errors.New("cannot set earlier birthday while there are active deals")
	}

	if !oldBday.Equal(asset.Birthday()) {
		if err := asset.updateDBBirthday(asset.Birthday()); err != nil {
			log.Errorf("Failed to reset wallet manager birthday: %v", err)
			performRescan = false
		}
	}

	if performRescan {
		log.Infof("ForceRescan for wallet (%s)", asset.GetWalletName())
		asset.ForceRescan()
	}

	g, _ := errgroup.WithContext(asset.syncCtx)
	g.Go(asset.chainClient.Start)

	if err = g.Wait(); err != nil {
		asset.CancelSync()
		log.Errorf("couldn't start Neutrino client: %v", err)
		return err
	}

	go func() {
		if atomic.CompareAndSwapInt32(&asset.syncData.syncstarted, stop, start) {
			asset.handleNotifications()
		}
	}()

	log.Infof("Synchronizing BTC wallet (%s) with network...", asset.GetWalletName())
	go asset.Internal().BTC.SynchronizeRPC(asset.chainClient)

	go func() {
		if atomic.CompareAndSwapInt32(&asset.syncData.syncstarted, stop, start) {
			asset.listenForTransactions()
		}
	}()

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
