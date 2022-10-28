package btc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/lightninglabs/neutrino"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

const syncIntervalGap int32 = 2000

type SyncData struct {
	mu sync.RWMutex

	startBlock    *wtxmgr.BlockMeta
	syncStartTime time.Time

	// showLogs bool
	syncing       bool
	synced        bool
	isRescan      bool
	restartedScan bool

	syncStage             utils.SyncStage
	syncProgressListeners map[string]sharedW.SyncProgressListener

	cfiltersFetchProgress    sharedW.CFiltersFetchProgressReport
	headersFetchProgress     sharedW.HeadersFetchProgressReport
	addressDiscoveryProgress sharedW.AddressDiscoveryProgressReport
	headersRescanProgress    sharedW.HeadersRescanProgressReport
}

func (asset *BTCAsset) AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	_, ok := asset.txAndBlockNotificationListeners[uniqueIdentifier]
	if ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	if async {
		asset.txAndBlockNotificationListeners[uniqueIdentifier] = &sharedW.AsyncTxAndBlockNotificationListener{
			TxAndBlockNotificationListener: txAndBlockNotificationListener,
		}
	} else {
		asset.txAndBlockNotificationListeners[uniqueIdentifier] = txAndBlockNotificationListener
	}

	return nil
}

func (asset *BTCAsset) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	delete(asset.txAndBlockNotificationListeners, uniqueIdentifier)
}

func (asset *BTCAsset) AddSyncProgressListener(syncProgressListener sharedW.SyncProgressListener, uniqueIdentifier string) error {
	asset.syncInfo.mu.Lock()
	defer asset.syncInfo.mu.Unlock()

	_, exists := asset.syncInfo.syncProgressListeners[uniqueIdentifier]
	if exists {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.syncInfo.syncProgressListeners[uniqueIdentifier] = syncProgressListener

	// If sync is already on, notify this newly added listener of the current progress report.
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

	// If the rawBlock is nil the network sync must have happenned. Resets the
	// sync info for the next sync use.
	if rawBlock == nil {
		asset.syncInfo.startBlock = nil
		return
	}

	// sync must be running for further execution to proceed
	if !asset.IsSyncing() {
		return
	}

	// Best block synced in the connected peers
	bestBlockheight := asset.bestServerPeerBlockHeight()

	// initial set up when sync begins.
	if asset.syncInfo.startBlock == nil {
		asset.syncInfo.headersFetchProgress.GeneralSyncProgress = &sharedW.GeneralSyncProgress{}
		asset.syncInfo.syncStage = utils.HeadersFetchSyncStage
		asset.syncInfo.syncStartTime = time.Now()
		asset.syncInfo.startBlock = rawBlock
		return
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
		if asset.IsSynced() {
			listener.OnSyncCompleted()
		}
	}
}

func (asset *BTCAsset) fetchNotifications() {
	for {
		select {
		case n, ok := <-asset.chainClient.Notifications():
			if !ok {
				return
			}
			switch n := n.(type) {
			case chain.ClientConnected:
				// TODO: Implementation to be added
			case chain.BlockConnected:
				b := wtxmgr.BlockMeta(n)
				asset.updateSyncProgress(&b)
			case chain.BlockDisconnected:
				// TODO: Implementation to be added
			case chain.RelevantTx:
				// TODO: Implementation to be added
			case chain.FilteredBlockConnected:
				if (n.Block.Height % syncIntervalGap) < syncIntervalGap {
					asset.updateSyncProgress(n.Block)
				}
			case *chain.RescanProgress:
				// TODO: Implementation to be added
			case *chain.RescanFinished:
				// TODO: Implementation to be added
			}
		case <-asset.syncCtx.Done():
			return
		}
	}
}

func (asset *BTCAsset) ConnectSPVWallet(wg *sync.WaitGroup) (err error) {
	ctx, _ := asset.ShutdownContextWithCancel()
	return asset.connect(ctx, wg)
}

// connect will start the wallet and begin syncing.
func (asset *BTCAsset) connect(ctx context.Context, wg *sync.WaitGroup) error {
	go func() {
		if err := asset.startWallet(ctx); err != nil {
			log.Error(err)
		}
	}()

	return nil
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

// startWallet initializes the *btcwallet.Wallet and its supporting players and
// starts syncing.
func (asset *BTCAsset) startWallet(ctx context.Context) error {
	if err := asset.chainClient.Start(); err != nil { // lazily starts connmgr
		asset.CancelSync()
		log.Errorf("couldn't start Neutrino client: %v", err)
		return err
	}

	err := asset.chainClient.NotifyBlocks()
	if err != nil {
		log.Error(err)
		return err
	}

	go asset.fetchNotifications()

	log.Info("Synchronizing wallet with network...")
	asset.Internal().BTC.SynchronizeRPC(asset.chainClient)

	asset.chainClient.WaitForShutdown()

	return nil
}

func (asset *BTCAsset) CancelSync() {
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
