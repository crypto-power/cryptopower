package btc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/lightninglabs/neutrino"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

type SyncData struct {
	mu sync.RWMutex

	starttime time.Time
	progress  chan interface{}

	showLogs bool
	syncing  bool
	synced   bool

	syncStage             utils.SyncStage
	syncProgressListeners map[string]sharedW.SyncProgressListener

	cfiltersFetchProgress    sharedW.CFiltersFetchProgressReport
	headersFetchProgress     sharedW.HeadersFetchProgressReport
	addressDiscoveryProgress sharedW.AddressDiscoveryProgressReport
	headersRescanProgress    sharedW.HeadersRescanProgressReport
}

func (asset *BTCAsset) IsSyncProgressListenerRegisteredFor(uniqueIdentifier string) bool {
	asset.syncInfo.mu.RLock()
	_, exists := asset.syncInfo.syncProgressListeners[uniqueIdentifier]
	asset.syncInfo.mu.RUnlock()
	return exists
}

func (asset *BTCAsset) AddSyncProgressListener(syncProgressListener sharedW.SyncProgressListener, uniqueIdentifier string) error {
	if asset.IsSyncProgressListenerRegisteredFor(uniqueIdentifier) {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.syncInfo.mu.Lock()
	asset.syncInfo.syncProgressListeners[uniqueIdentifier] = syncProgressListener
	asset.syncInfo.mu.Unlock()

	// If sync is already on, notify this newly added listener of the current progress report.
	return asset.PublishLastSyncProgress(uniqueIdentifier)
}

func (asset *BTCAsset) RemoveSyncProgressListener(uniqueIdentifier string) {
	asset.syncInfo.mu.Lock()
	delete(asset.syncInfo.syncProgressListeners, uniqueIdentifier)
	asset.syncInfo.mu.Unlock()
}

func (asset *BTCAsset) syncProgressListeners() []sharedW.SyncProgressListener {
	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()

	listeners := make([]sharedW.SyncProgressListener, 0, len(asset.syncInfo.syncProgressListeners))
	for _, listener := range asset.syncInfo.syncProgressListeners {
		listeners = append(listeners, listener)
	}

	return listeners
}

func (asset *BTCAsset) PublishLastSyncProgress(uniqueIdentifier string) error {
	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()

	syncProgressListener, exists := asset.syncInfo.syncProgressListeners[uniqueIdentifier]
	if !exists {
		return errors.New(utils.ErrInvalid)
	}

	if asset.syncInfo.syncing {
		switch asset.syncInfo.syncStage {
		case utils.HeadersFetchSyncStage:
			syncProgressListener.OnHeadersFetchProgress(&asset.syncInfo.headersFetchProgress)
		case utils.AddressDiscoverySyncStage:
			syncProgressListener.OnAddressDiscoveryProgress(&asset.syncInfo.addressDiscoveryProgress)
		case utils.HeadersRescanSyncStage:
			syncProgressListener.OnHeadersRescanProgress(&asset.syncInfo.headersRescanProgress)
		}
	}

	return nil
}

func (asset *BTCAsset) ConnectSPVWallet(wg *sync.WaitGroup) (err error) {
	ctx, _ := asset.ShutdownContextWithCancel()
	return asset.connect(ctx, wg)
}

// connect will start the wallet and begin syncing.
func (asset *BTCAsset) connect(ctx context.Context, wg *sync.WaitGroup) error {
	err := asset.startWallet()
	if err != nil {
		return err
	}

	// Nanny for the caches checkpoints and txBlocks caches.
	wg.Add(1)

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
func (asset *BTCAsset) startWallet() error {
	if err := asset.chainClient.Start(); err != nil { // lazily starts connmgr
		asset.CancelSync()
		return fmt.Errorf("couldn't start Neutrino client: %v", err)
	}

	log.Info("Synchronizing wallet with network...")
	asset.Internal().BTC.SynchronizeRPC(asset.chainClient)

	go asset.fetchNotifications()

	asset.chainClient.WaitForShutdown()

	return nil
}

func (asset *BTCAsset) CancelSync() {
	// if err := asset.chainService.Stop(); err != nil {
	// 	log.Warnf("Error closing neutrino chain service: %v", err)
	// }
}

func (asset *BTCAsset) IsConnectedToBitcoinNetwork() bool {
	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()
	return asset.syncInfo.syncing || asset.syncInfo.synced
}
