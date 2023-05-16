package eth

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	gethutils "github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/les"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
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

// SyncData holds the data required to sync the wallet.
type SyncData struct {
	mu sync.RWMutex

	syncstarted         uint32
	syncEnded           uint32
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

func (asset *Asset) resetSyncProgressData() {
	asset.syncData.syncing = false
	asset.syncData.synced = false
	asset.syncData.isRescan = false
}

func (asset *Asset) updateSyncProgress() {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	syncingProgress := asset.client.ApiBackend.SyncProgress()
	startHeaderHeight := int32(syncingProgress.StartingBlock)
	rawBlockHeight := int32(syncingProgress.CurrentBlock)
	bestBlockheight := int32(syncingProgress.HighestBlock)

	if asset.syncData.headersFetchProgress.StartHeaderHeight == nil {
		asset.syncData.syncStage = utils.HeadersFetchSyncStage
		asset.syncData.headersFetchProgress.BeginFetchTimeStamp = time.Now()
		asset.syncData.headersFetchProgress.StartHeaderHeight = &rawBlockHeight
	}

	log.Infof("Current sync progress update is on block %v, target sync block is %v", rawBlockHeight, bestBlockheight)

	timeSpentSoFar := time.Since(asset.syncData.headersFetchProgress.BeginFetchTimeStamp).Seconds()
	if timeSpentSoFar < 1 {
		timeSpentSoFar = 1
	}

	headersFetchedSoFar := float64(rawBlockHeight - startHeaderHeight)
	if headersFetchedSoFar < 1 {
		headersFetchedSoFar = 1
	}

	remainingHeaders := float64(bestBlockheight - rawBlockHeight)
	if remainingHeaders < 1 {
		remainingHeaders = 1
	}

	allHeadersToFetch := headersFetchedSoFar + remainingHeaders

	asset.syncData.headersFetchProgress.TotalHeadersToFetch = int32(bestBlockheight)
	asset.syncData.headersFetchProgress.HeadersFetchProgress = int32((headersFetchedSoFar * 100) / allHeadersToFetch)
	asset.syncData.headersFetchProgress.GeneralSyncProgress.TotalSyncProgress = asset.syncData.headersFetchProgress.HeadersFetchProgress
	asset.syncData.headersFetchProgress.GeneralSyncProgress.TotalTimeRemainingSeconds = int64((timeSpentSoFar * remainingHeaders) / headersFetchedSoFar)

	// publish the sync progress results to all listeners.
	for _, listener := range asset.syncData.syncProgressListeners {
		listener.OnHeadersFetchProgress(&asset.syncData.headersFetchProgress)
	}
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
	t := time.NewTicker(syncIntervalGap)

	heads := make(chan core.ChainHeadEvent, 5)
	sub := asset.client.ApiBackend.SubscribeChainHeadEvent(heads)

	defer func() {
		t.Stop()
		sub.Unsubscribe()
	}()

notificationsLoop:
	for {
		select {
		case <-heads:
			select {
			case <-t.C:
				if !asset.IsSynced() {
					// initial sync is inprogress.
					asset.updateSyncProgress()
				} else {
					// initial sync is complete
					asset.publishBlockAttached()
				}
			default:
			}
		case err := <-sub.Err():
			log.Errorf(" error when handling chain header events: %v", err)

		case <-asset.syncCtx.Done():
			break notificationsLoop
		}
	}
}

func (asset *Asset) CancelRescan() {
	log.Error(utils.ErrETHMethodNotImplemented("CancelRescan"))
}

// shutdownWallet closes down the local node.
func (asset *Asset) shutdownWallet() {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	err := asset.stack.Close()
	if err != nil {
		log.Errorf("node shutdown error: %v", err)
	}

	asset.stack.Wait()

	log.Infof("(%v) Light Ethereum (LES) wallet closed", asset.GetWalletName())
}

// CancelSync disables the events sync events subscribed to.
func (asset *Asset) CancelSync() {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	log.Info("Canceling sync. May take a while for sync to fully cancel.")

	// reset the sync data first.
	asset.resetSyncProgressData()

	if asset.client != nil {
		// Cancel sync download operations running.
		asset.client.Downloader().Cancel()
	}

	// Sends a request to cancel the events subscription.
	asset.cancelSync()
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
	// Listen and handle incoming notification events.
	if atomic.CompareAndSwapUint32(&asset.syncData.syncstarted, stop, start) {
		go asset.handleNotifications()
	}

	return nil
}

// startWallet initializes the eth wallet and starts syncing.
func (asset *Asset) startWallet() (err error) {
	// Initiate the sync protocol and return an error incase of failure.
	return asset.startSync()
}

// prepareChain initialize the local node responsible for p2p connections.
func (asset *Asset) prepareChain() error {
	if asset.client != nil && asset.stack != nil {
		// an active instance already exists, not need to re-initialize the same.
		return nil
	}

	if !asset.WalletOpened() {
		return errors.New("wallet account not loaded")
	}

	ks := asset.Internal().ETH.Keystore
	if ks == nil || len(ks.Accounts()) == 0 {
		return errors.New("no existing wallet account found")
	}

	// generates a private key using the provided hashed seed. asset.EncryptedSeed has
	// a length of 64 bytes but only 32 are required to generate an ECDSA private
	// key.
	privatekey, err := crypto.ToECDSA(asset.EncryptedSeed[:32])
	if err != nil {
		return err
	}

	bootnodes, err := utils.GetBootstrapNodes(asset.chainParams)
	if err != nil {
		return fmt.Errorf("invalid bootstrap nodes: %v", err)
	}

	// Convert the bootnodes to internal enode representations
	var enodes []*enode.Node
	for _, boot := range bootnodes {
		url, err := enode.Parse(enode.ValidSchemes, boot)
		if err == nil {
			enodes = append(enodes, url)
		} else {
			log.Error("Failed to parse bootnode URL", "url", boot, " err", err)
		}
	}

	cfg := node.DefaultConfig
	cfg.DBEngine = "leveldb" // leveldb is used instead of pebble db.
	cfg.Name = executionClient
	cfg.WSModules = append(cfg.WSModules, "eth")
	cfg.DataDir = asset.DataDir()
	cfg.P2P.PrivateKey = privatekey
	cfg.P2P.BootstrapNodesV5 = enodes
	cfg.P2P.NoDiscovery = true
	cfg.P2P.DiscoveryV5 = true
	cfg.P2P.MaxPeers = 25

	stack, err := node.New(&cfg)
	if err != nil {
		return err
	}
	asset.stack = stack

	genesis, err := utils.GetGenesis(asset.chainParams)
	if err != nil {
		return fmt.Errorf("invalid genesis block: %v", err)
	}

	// Assemble the Ethereum light client protocol
	ethcfg := ethconfig.Defaults
	ethcfg.SyncMode = downloader.LightSync
	ethcfg.GPO = ethconfig.LightClientGPO
	ethcfg.NetworkId = asset.chainParams.ChainID.Uint64()
	ethcfg.Genesis = genesis
	gethutils.SetDNSDiscoveryDefaults(&ethcfg, genesis.ToBlock().Hash())

	asset.client, err = les.New(stack, &ethcfg)
	if err != nil {
		return fmt.Errorf("failed to register the Ethereum service: %w", err)
	}
	return nil
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
			progress := asset.client.ApiBackend.SyncProgress()
			if progress.CurrentBlock >= progress.HighestBlock && progress.HighestBlock > 0 {
				asset.syncData.mu.Lock()
				asset.syncData.synced = true
				asset.syncData.syncing = false
				asset.syncData.mu.Unlock()

				// Trigger UI update showing ltc address recovery is in progress.
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
		return utils.ErrETHNotInitialized
	}

	// prevent an attempt to sync when the previous syncing has not been canceled
	if asset.IsSyncing() || asset.IsSynced() {
		return errors.New(utils.ErrSyncAlreadyInProgress)
	}

	// Initialize all progress report data.
	asset.initSyncProgressData()

	if err := asset.prepareChain(); err != nil {
		return fmt.Errorf("preparing chain failed: %v", err)
	}

	// Boot up the client and ensure it connects to bootnodes
	if err := asset.stack.Start(); err != nil {
		return err
	}

	ctx, cancel := asset.ShutdownContextWithCancel()
	asset.notificationListenersMu.Lock()
	asset.syncCtx = ctx
	asset.cancelSync = cancel
	asset.notificationListenersMu.Unlock()

	// Set wallet synced state to true when chainclient considers itself
	// as synced with the network.
	go asset.waitForSyncCompletion()

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
