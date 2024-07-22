package dcr

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"decred.org/dcrwallet/v4/errors"
	"decred.org/dcrwallet/v4/p2p"
	"decred.org/dcrwallet/v4/spv"
	w "decred.org/dcrwallet/v4/wallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/decred/dcrd/addrmgr/v2"
)

// reading/writing of properties of this struct are protected by mutex.x
type SyncData struct {
	mu sync.RWMutex

	syncProgressListeners map[string]*sharedW.SyncProgressListener
	showLogs              bool

	synced       bool
	syncing      bool
	cancelSync   context.CancelFunc
	cancelRescan context.CancelFunc
	syncCanceled chan struct{}

	bestBlockheight int32 // Synced peers best block height.

	// Flag to notify syncCanceled callback if the sync was canceled so as to be restarted.
	restartSyncRequested bool

	rescanning          bool
	numOfConnectedPeers int32

	*activeSyncData
}

func (s *SyncData) isSynced() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.synced
}

func (s *SyncData) connectedPeers() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.numOfConnectedPeers
}

func (s *SyncData) generalSyncProgress() *sharedW.GeneralSyncProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.genSyncProgress
}

// reading/writing of properties of this struct are protected by syncData.mu.
type activeSyncData struct {
	syncer    *spv.Syncer
	syncStage utils.SyncStage

	addressDiscoveryCompletedOrCanceled chan bool

	// scanStartTime tracks the time when syncing or rescanning starts.
	scanStartTime time.Time
	// scanStartHeight tracks the height when syncing or rescanning starts.
	scanStartHeight int32

	headersScanTimeSpent   time.Duration // time spent during the headers scan.
	cfiltersScanTimeSpent  time.Duration // time spent during the Cfilters scan.
	addrDiscoveryTimeSpent time.Duration // time spent in address discovery.
	rescanTimeSpent        time.Duration // time spent during the rescan.

	// genSyncProgress tracks progress of the current sync running.
	genSyncProgress *sharedW.GeneralSyncProgress

	totalInactiveDuration time.Duration
	isRescanning          bool
	isAddressDiscovery    bool
}

const (
	InvalidSyncStage          = utils.InvalidSyncStage
	CFiltersFetchSyncStage    = utils.CFiltersFetchSyncStage
	HeadersFetchSyncStage     = utils.HeadersFetchSyncStage
	AddressDiscoverySyncStage = utils.AddressDiscoverySyncStage
	HeadersRescanSyncStage    = utils.HeadersRescanSyncStage
)

func (asset *Asset) initActiveSyncData() {
	asset.syncData.mu.Lock()
	asset.syncData.activeSyncData = &activeSyncData{
		syncStage: InvalidSyncStage,

		scanStartHeight: -1,
	}
	asset.syncData.mu.Unlock()
}

func (asset *Asset) IsSyncProgressListenerRegisteredFor(uniqueIdentifier string) bool {
	asset.syncData.mu.RLock()
	_, exists := asset.syncData.syncProgressListeners[uniqueIdentifier]
	asset.syncData.mu.RUnlock()
	return exists
}

func (asset *Asset) AddSyncProgressListener(syncProgressListener *sharedW.SyncProgressListener, uniqueIdentifier string) error {
	if asset.IsSyncProgressListenerRegisteredFor(uniqueIdentifier) {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.syncData.mu.Lock()
	asset.syncData.syncProgressListeners[uniqueIdentifier] = syncProgressListener
	asset.syncData.mu.Unlock()

	return nil
}

func (asset *Asset) RemoveSyncProgressListener(uniqueIdentifier string) {
	asset.syncData.mu.Lock()
	delete(asset.syncData.syncProgressListeners, uniqueIdentifier)
	asset.syncData.mu.Unlock()
}

func (asset *Asset) syncProgressListeners() []*sharedW.SyncProgressListener {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	listeners := make([]*sharedW.SyncProgressListener, 0, len(asset.syncData.syncProgressListeners))
	for _, listener := range asset.syncData.syncProgressListeners {
		listeners = append(listeners, listener)
	}

	return listeners
}

func (asset *Asset) EnableSyncLogs() {
	asset.syncData.mu.Lock()
	asset.syncData.showLogs = true
	asset.syncData.mu.Unlock()
}

func (asset *Asset) SyncInactiveForPeriod(totalInactiveDuration time.Duration) {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if !asset.syncData.syncing || asset.syncData.activeSyncData == nil {
		log.Debug("Not accounting for inactive time, wallet is not syncing.")
		return
	}

	asset.syncData.totalInactiveDuration += totalInactiveDuration
	if asset.syncData.numOfConnectedPeers == 0 {
		// assume it would take another 60 seconds to reconnect to peers
		asset.syncData.totalInactiveDuration += secondsToDuration(60.0)
	}
}

func (asset *Asset) SetSpecificPeer(addresses string) {
	asset.SaveUserConfigValue(sharedW.SpvPersistentPeerAddressesConfigKey, addresses)
	_ = asset.RestartSpvSync()
}

func (asset *Asset) RemovePeers() {
	asset.SaveUserConfigValue(sharedW.SpvPersistentPeerAddressesConfigKey, "")
	_ = asset.RestartSpvSync()
}

func (asset *Asset) SpvSync() error {
	// prevent an attempt to sync when the previous syncing has not been canceled
	if asset.IsSyncing() || asset.IsSynced() {
		return errors.New(utils.ErrSyncAlreadyInProgress)
	}

	peerAddresses := asset.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")
	validPeerAddresses, errs := sharedW.ParseWalletPeers(peerAddresses, asset.chainParams.DefaultPort)
	for _, err := range errs { // Log errors if any
		log.Error(err)
	}

	if len(validPeerAddresses) == 0 && len(errs) > 0 {
		return errors.New(utils.ErrInvalidPeers)
	}

	// init activeSyncData to be used to hold data used
	// to calculate sync estimates only during sync
	asset.initActiveSyncData()

	asset.waitingForHeaders = true
	asset.syncing = true

	addr := &net.TCPAddr{IP: net.ParseIP("::1"), Port: 0}
	addrManager := addrmgr.New(asset.DataDir(), net.LookupIP) // TODO: be mindful of tor
	lp := p2p.NewLocalPeer(asset.chainParams, addr, addrManager)

	// Set the node to only connect to remote peers whose advertised best block
	// height is greater than the currently synced.
	lp.RequirePeerHeight(asset.GetBestBlockHeight())

	syncer := spv.NewSyncer(asset.Internal().DCR, lp)
	syncer.SetNotifications(asset.spvSyncNotificationCallbacks())
	if len(validPeerAddresses) > 0 {
		syncer.SetPersistentPeers(validPeerAddresses)
	}

	ctx, cancel := asset.ShutdownContextWithCancel()

	asset.syncData.mu.Lock()
	asset.syncData.restartSyncRequested = false
	asset.syncData.syncing = true
	asset.syncData.cancelSync = cancel
	asset.syncData.syncCanceled = make(chan struct{})
	asset.syncData.syncer = syncer
	asset.syncData.mu.Unlock()

	for _, listener := range asset.syncProgressListeners() {
		if listener.OnSyncStarted != nil {
			listener.OnSyncStarted()
		}
	}

	// syncer.Run uses a wait group to block the thread until the sync context
	// expires or is canceled or some other error occurs such as
	// losing connection to all persistent peers.
	go func() {
		syncError := syncer.Run(ctx)
		// sync has ended or errored
		if syncError != nil {
			if syncError == context.DeadlineExceeded {
				asset.notifySyncError(errors.Errorf("SPV synchronization deadline exceeded: %v", syncError))
			} else if syncError == context.Canceled {
				asset.notifySyncCanceled()
			} else {
				asset.notifySyncError(syncError)
			}
		}

		// Close the syncer channel after the syncer.Run stops.
		close(asset.syncData.syncCanceled)
		// reset sync variables
		asset.resetSyncData()
	}()
	return nil
}

func (asset *Asset) RestartSpvSync() error {
	asset.syncData.mu.Lock()
	asset.syncData.restartSyncRequested = true
	asset.syncData.mu.Unlock()

	asset.CancelSync() // necessary to unset the network backend.
	return asset.SpvSync()
}

func (asset *Asset) CancelSync() {
	asset.syncData.mu.RLock()
	cancelSync := asset.syncData.cancelSync
	asset.syncData.mu.RUnlock()

	if cancelSync != nil {
		log.Info("Cancelling sync. May take a while for sync to fully cancel.")

		// Stop running cspp mixers
		if asset.IsAccountMixerActive() {
			log.Infof("[%d] Stopping cspp mixer", asset.ID)
			err := asset.StopAccountMixer()
			if err != nil {
				log.Errorf("[%d] Error stopping cspp mixer: %v", asset.ID, err)
			}
		}

		// Cancels the context used for syncer.Run in spvSync().
		// This may not immediately cause the sync process to terminate,
		// but when it eventually terminates, syncer.Run will return `err == context.Canceled`.
		cancelSync()

		// When sync terminates and syncer.Run returns, we will get notified on this channel.
		<-asset.syncData.syncCanceled
	}

	// Indicate that the sync shutdown process is fully complete.
	asset.EndSyncShuttingDown()

	log.Info("Sync fully cancelled.")
}

func (asset *Asset) IsWaiting() bool {
	return asset.waitingForHeaders
}

func (asset *Asset) IsSyncing() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()
	return asset.syncData.syncing
}

func (asset *Asset) IsConnectedToDecredNetwork() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()
	return asset.syncData.syncing || asset.syncData.synced
}

func (asset *Asset) IsSynced() bool {
	return asset.syncData.isSynced()
}

func (asset *Asset) CurrentSyncStage() utils.SyncStage {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	if asset.syncData != nil && asset.syncData.syncing {
		return asset.syncData.syncStage
	}
	return InvalidSyncStage
}

func (asset *Asset) IsAddressDiscovering() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	if asset.syncData != nil && asset.syncData.syncing {
		return asset.syncData.isAddressDiscovery
	}

	return false
}

func (asset *Asset) IsSycnRescanning() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	if asset.syncData != nil && asset.syncData.syncing {
		return asset.syncData.isRescanning
	}

	return false
}

func (asset *Asset) ConnectedPeers() int32 {
	return asset.syncData.connectedPeers()
}

func (asset *Asset) GeneralSyncProgress() *sharedW.GeneralSyncProgress {
	if asset.syncData != nil {
		return asset.syncData.generalSyncProgress()
	}
	return nil
}

func (asset *Asset) SyncData() *SyncData {
	return asset.syncData
}

func (asset *Asset) PeerInfoRaw() ([]sharedW.PeerInfo, error) {
	if !asset.IsConnectedToDecredNetwork() {
		return nil, errors.New(utils.ErrNotConnected)
	}

	syncer := asset.syncData.syncer

	infos := make([]sharedW.PeerInfo, 0, len(syncer.GetRemotePeers()))
	for _, rp := range syncer.GetRemotePeers() {
		info := sharedW.PeerInfo{
			ID:             int32(rp.ID()),
			Addr:           rp.RemoteAddr().String(),
			AddrLocal:      rp.LocalAddr().String(),
			Services:       fmt.Sprintf("%08d", uint64(rp.Services())),
			Version:        rp.Pver(),
			SubVer:         rp.UA(),
			StartingHeight: int64(rp.InitialHeight()),
			BanScore:       int32(rp.BanScore()),
		}

		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ID < infos[j].ID
	})

	return infos, nil
}

func (asset *Asset) PeerInfo() (string, error) {
	infos, err := asset.PeerInfoRaw()
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(infos)
	return string(result), nil
}

func (asset *Asset) GetBestBlock() *sharedW.BlockInfo {
	blockInfo := sharedW.InvalidBlock
	if !asset.WalletOpened() {
		return blockInfo
	}

	walletBestBLock := asset.GetBestBlockHeight()
	if walletBestBLock > sharedW.InvalidBlock.Height {
		blockInfo = &sharedW.BlockInfo{Height: walletBestBLock, Timestamp: asset.GetBestBlockTimeStamp()}
	}

	return blockInfo
}

// GetBestBlockHeight returns the height of the best block already synced.
func (asset *Asset) GetBestBlockHeight() int32 {
	if !asset.WalletOpened() {
		// This method is sometimes called after a wallet is deleted and causes crash.
		log.Error("Attempting to read best block height without a loaded asset.")
		return sharedW.InvalidBlock.Height
	}
	ctx, _ := asset.ShutdownContextWithCancel()
	_, height := asset.Internal().DCR.MainChainTip(ctx)
	return height
}

func (asset *Asset) GetBestBlockTimeStamp() int64 {
	if !asset.WalletOpened() {
		// This method is sometimes called after a wallet is deleted and causes crash.
		log.Error("Attempting to read best block timestamp without a loaded asset.")
		return sharedW.InvalidBlock.Timestamp
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	_, height := asset.Internal().DCR.MainChainTip(ctx)
	identifier := w.NewBlockIdentifierFromHeight(height)
	info, err := asset.Internal().DCR.BlockInfo(ctx, identifier)
	if err != nil {
		log.Error(err)
		return sharedW.InvalidBlock.Timestamp
	}
	return info.Timestamp
}

func (asset *Asset) DiscoverUsage(gapLimit uint32) error {
	if !asset.WalletOpened() {
		return utils.ErrDCRNotInitialized
	}

	netBackend, err := asset.Internal().DCR.NetworkBackend()
	if err != nil {
		return errors.E(utils.ErrNotConnected)
	}

	// prevent usage discovery if the wallet is syncing.
	if asset.IsSyncing() {
		return errors.New(utils.ErrSyncAlreadyInProgress)
	}

	// Prevent usage discovery if wallet isn't synced.
	if !asset.IsSynced() {
		return errors.New(utils.ErrNotSynced)
	}

	// rescan from genesis block. Todo: Allow users to supply rescanpoint.
	startBlock := asset.Internal().DCR.ChainParams().GenesisHash

	go func() {
		defer func() {
			asset.syncData.mu.Lock()
			asset.syncData.syncing = false
			asset.syncData.cancelSync = nil
			asset.syncData.mu.Unlock()
			asset.discoverAddressesFinished()
		}()

		ctx, cancel := asset.ShutdownContextWithCancel()

		asset.syncData.mu.Lock()
		asset.syncData.syncing = true
		asset.syncData.cancelSync = cancel
		asset.syncData.mu.Unlock()

		asset.discoverAddressesStarted()

		err := asset.Internal().DCR.DiscoverActiveAddresses(ctx, netBackend, &startBlock, !asset.Internal().DCR.Locked(), gapLimit)
		if err != nil {
			log.Error(err)
		}
	}()

	return nil
}
