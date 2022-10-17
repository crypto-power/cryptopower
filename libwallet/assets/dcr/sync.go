package dcr

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	"decred.org/dcrwallet/v2/errors"
	"decred.org/dcrwallet/v2/p2p"
	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/addrmgr/v2"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/spv"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

// reading/writing of properties of this struct are protected by mutex.x
type SyncData struct {
	mu sync.RWMutex

	syncProgressListeners map[string]sharedW.SyncProgressListener
	showLogs              bool

	synced       bool
	syncing      bool
	cancelSync   context.CancelFunc
	cancelRescan context.CancelFunc
	syncCanceled chan struct{}

	// Flag to notify syncCanceled callback if the sync was canceled so as to be restarted.
	restartSyncRequested bool

	rescanning     bool
	connectedPeers int32

	*activeSyncData
}

// reading/writing of properties of this struct are protected by syncData.mu.
type activeSyncData struct {
	syncer *spv.Syncer

	syncStage int32

	cfiltersFetchProgress    sharedW.CFiltersFetchProgressReport
	headersFetchProgress     sharedW.HeadersFetchProgressReport
	addressDiscoveryProgress sharedW.AddressDiscoveryProgressReport
	headersRescanProgress    sharedW.HeadersRescanProgressReport

	addressDiscoveryCompletedOrCanceled chan bool

	rescanStartTime int64

	totalInactiveSeconds int64
}

const (
	InvalidSyncStage          = -1
	CFiltersFetchSyncStage    = 0
	HeadersFetchSyncStage     = 1
	AddressDiscoverySyncStage = 2
	HeadersRescanSyncStage    = 3
)

func (asset *DCRAsset) initActiveSyncData() {

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
		syncStage: InvalidSyncStage,

		cfiltersFetchProgress:    cfiltersFetchProgress,
		headersFetchProgress:     headersFetchProgress,
		addressDiscoveryProgress: addressDiscoveryProgress,
		headersRescanProgress:    headersRescanProgress,
	}
	asset.syncData.mu.Unlock()
}

func (asset *DCRAsset) IsSyncProgressListenerRegisteredFor(uniqueIdentifier string) bool {
	asset.syncData.mu.RLock()
	_, exists := asset.syncData.syncProgressListeners[uniqueIdentifier]
	asset.syncData.mu.RUnlock()
	return exists
}

func (asset *DCRAsset) AddSyncProgressListener(syncProgressListener sharedW.SyncProgressListener, uniqueIdentifier string) error {
	if asset.IsSyncProgressListenerRegisteredFor(uniqueIdentifier) {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.syncData.mu.Lock()
	asset.syncData.syncProgressListeners[uniqueIdentifier] = syncProgressListener
	asset.syncData.mu.Unlock()

	// If sync is already on, notify this newly added listener of the current progress report.
	return asset.PublishLastSyncProgress(uniqueIdentifier)
}

func (asset *DCRAsset) RemoveSyncProgressListener(uniqueIdentifier string) {
	asset.syncData.mu.Lock()
	delete(asset.syncData.syncProgressListeners, uniqueIdentifier)
	asset.syncData.mu.Unlock()
}

func (asset *DCRAsset) syncProgressListeners() []sharedW.SyncProgressListener {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	listeners := make([]sharedW.SyncProgressListener, 0, len(asset.syncData.syncProgressListeners))
	for _, listener := range asset.syncData.syncProgressListeners {
		listeners = append(listeners, listener)
	}

	return listeners
}

func (asset *DCRAsset) PublishLastSyncProgress(uniqueIdentifier string) error {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	syncProgressListener, exists := asset.syncData.syncProgressListeners[uniqueIdentifier]
	if !exists {
		return errors.New(utils.ErrInvalid)
	}

	if asset.syncData.syncing && asset.syncData.activeSyncData != nil {
		switch asset.syncData.activeSyncData.syncStage {
		case HeadersFetchSyncStage:
			syncProgressListener.OnHeadersFetchProgress(&asset.syncData.headersFetchProgress)
		case AddressDiscoverySyncStage:
			syncProgressListener.OnAddressDiscoveryProgress(&asset.syncData.addressDiscoveryProgress)
		case HeadersRescanSyncStage:
			syncProgressListener.OnHeadersRescanProgress(&asset.syncData.headersRescanProgress)
		}
	}

	return nil
}

func (asset *DCRAsset) EnableSyncLogs() {
	asset.syncData.mu.Lock()
	asset.syncData.showLogs = true
	asset.syncData.mu.Unlock()
}

func (asset *DCRAsset) SyncInactiveForPeriod(totalInactiveSeconds int64) {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if !asset.syncData.syncing || asset.syncData.activeSyncData == nil {
		log.Debug("Not accounting for inactive time, wallet is not syncing.")
		return
	}

	asset.syncData.totalInactiveSeconds += totalInactiveSeconds
	if asset.syncData.connectedPeers == 0 {
		// assume it would take another 60 seconds to reconnect to peers
		asset.syncData.totalInactiveSeconds += 60
	}
}

func (asset *DCRAsset) SetSpecificPeer(address string) {
	asset.SaveUserConfigValue(sharedW.SpvPersistentPeerAddressesConfigKey, address)
	asset.RestartSpvSync()
}

func (asset *DCRAsset) RemoveSpecificPeer() {
	asset.SaveUserConfigValue(sharedW.SpvPersistentPeerAddressesConfigKey, "")
	asset.RestartSpvSync()
}

func (asset *DCRAsset) SpvSync() error {
	// prevent an attempt to sync when the previous syncing has not been canceled
	if asset.IsSyncing() || asset.IsSynced() {
		return errors.New(utils.ErrSyncAlreadyInProgress)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("::1"), Port: 0}
	addrManager := addrmgr.New(asset.RootDir(), net.LookupIP) // TODO: be mindful of tor
	lp := p2p.NewLocalPeer(asset.chainParams, addr, addrManager)

	var validPeerAddresses []string
	peerAddresses := asset.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")
	if peerAddresses != "" {
		addresses := strings.Split(peerAddresses, ";")
		for _, address := range addresses {
			peerAddress, err := normalizeAddress(address, asset.chainParams.DefaultPort)
			if err != nil {
				log.Errorf("SPV peer address(%s) is invalid: %v", peerAddress, err)
			} else {
				validPeerAddresses = append(validPeerAddresses, peerAddress)
			}
		}

		if len(validPeerAddresses) == 0 {
			return errors.New(utils.ErrInvalidPeers)
		}
	}

	// init activeSyncData to be used to hold data used
	// to calculate sync estimates only during sync
	asset.initActiveSyncData()

	wallets := make(map[int]*w.Wallet)
	wallets[0] = asset.Internal().DCR
	asset.waitingForHeaders = true
	asset.syncing = true

	syncer := spv.NewSyncer(wallets, lp)
	syncer.SetNotifications(asset.spvSyncNotificationCallbacks())
	if len(validPeerAddresses) > 0 {
		syncer.SetPersistentPeers(validPeerAddresses)
	}

	ctx, cancel := asset.ShutdownContextWithCancel()

	var restartSyncRequested bool

	asset.syncData.mu.Lock()
	restartSyncRequested = asset.syncData.restartSyncRequested
	asset.syncData.restartSyncRequested = false
	asset.syncData.syncing = true
	asset.syncData.cancelSync = cancel
	asset.syncData.syncCanceled = make(chan struct{})
	asset.syncData.syncer = syncer
	asset.syncData.mu.Unlock()

	for _, listener := range asset.syncProgressListeners() {
		listener.OnSyncStarted(restartSyncRequested)
	}

	// syncer.Run uses a wait group to block the thread until the sync context
	// expires or is canceled or some other error occurs such as
	// losing connection to all persistent peers.
	go func() {
		syncError := syncer.Run(ctx)
		//sync has ended or errored
		if syncError != nil {
			if syncError == context.DeadlineExceeded {
				asset.notifySyncError(errors.Errorf("SPV synchronization deadline exceeded: %v", syncError))
			} else if syncError == context.Canceled {
				close(asset.syncData.syncCanceled)
				asset.notifySyncCanceled()
			} else {
				asset.notifySyncError(syncError)
			}
		}

		//reset sync variables
		asset.resetSyncData()
	}()
	return nil
}

func (asset *DCRAsset) RestartSpvSync() error {
	asset.syncData.mu.Lock()
	asset.syncData.restartSyncRequested = true
	asset.syncData.mu.Unlock()

	asset.CancelSync() // necessary to unset the network backend.
	return asset.SpvSync()
}

func (asset *DCRAsset) CancelSync() {
	asset.syncData.mu.RLock()
	cancelSync := asset.syncData.cancelSync
	asset.syncData.mu.RUnlock()

	if cancelSync != nil {
		log.Info("Canceling sync. May take a while for sync to fully cancel.")

		// Stop running cspp mixers
		if asset.IsAccountMixerActive() {
			log.Infof("[%d] Stopping cspp mixer", asset.ID)
			err := asset.StopAccountMixer()
			if err != nil {
				log.Errorf("[%d] Error stopping cspp mixer: %v", asset.ID, err)
			}
		}

		// Cancel the context used for syncer.Run in spvSync().
		// This may not immediately cause the sync process to terminate,
		// but when it eventually terminates, syncer.Run will return `err == context.Canceled`.
		cancelSync()

		// When sync terminates and syncer.Run returns `err == context.Canceled`,
		// we will get notified on this channel.
		<-asset.syncData.syncCanceled

		log.Info("Sync fully canceled.")
	}
}

func (asset *DCRAsset) IsWaiting() bool {
	return asset.waitingForHeaders
}

func (asset *DCRAsset) IsSyncing() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()
	return asset.syncData.syncing
}

func (asset *DCRAsset) IsConnectedToDecredNetwork() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()
	return asset.syncData.syncing || asset.syncData.synced
}

func (asset *DCRAsset) IsSynced() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()
	return asset.syncData.synced
}

func (asset *DCRAsset) CurrentSyncStage() int32 {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	if asset.syncData != nil && asset.syncData.syncing {
		return asset.syncData.syncStage
	}
	return InvalidSyncStage
}

func (asset *DCRAsset) GeneralSyncProgress() *sharedW.GeneralSyncProgress {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	if asset.syncData != nil && asset.syncData.syncing {
		switch asset.syncData.syncStage {
		case HeadersFetchSyncStage:
			return asset.syncData.headersFetchProgress.GeneralSyncProgress
		case AddressDiscoverySyncStage:
			return asset.syncData.addressDiscoveryProgress.GeneralSyncProgress
		case HeadersRescanSyncStage:
			return asset.syncData.headersRescanProgress.GeneralSyncProgress
		case CFiltersFetchSyncStage:
			return asset.syncData.cfiltersFetchProgress.GeneralSyncProgress
		}
	}

	return nil
}

func (asset *DCRAsset) ConnectedPeers() int32 {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()
	return asset.syncData.connectedPeers
}

func (asset *DCRAsset) PeerInfoRaw() ([]sharedW.PeerInfo, error) {
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

func (asset *DCRAsset) PeerInfo() (string, error) {
	infos, err := asset.PeerInfoRaw()
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(infos)
	return string(result), nil
}

func (asset *DCRAsset) GetBestBlock() *sharedW.BlockInfo {
	var bestBlock int32 = -1
	var blockInfo *sharedW.BlockInfo
	if !asset.WalletOpened() {
		return nil
	}

	walletBestBLock := asset.GetBestBlockHeight()
	if walletBestBLock > bestBlock || bestBlock == -1 {
		bestBlock = walletBestBLock
		blockInfo = &sharedW.BlockInfo{Height: bestBlock, Timestamp: asset.GetBestBlockTimeStamp()}
	}

	return blockInfo
}

func (asset *DCRAsset) GetLowestBlock() *sharedW.BlockInfo {
	var lowestBlock int32 = -1
	var blockInfo *sharedW.BlockInfo
	if !asset.WalletOpened() {
		return nil
	}
	walletBestBLock := asset.GetBestBlockHeight()
	if walletBestBLock < lowestBlock || lowestBlock == -1 {
		lowestBlock = walletBestBLock
		blockInfo = &sharedW.BlockInfo{Height: lowestBlock, Timestamp: asset.GetBestBlockTimeStamp()}
	}

	return blockInfo
}

func (asset *DCRAsset) GetBestBlockHeight() int32 {
	if asset.Internal() == nil {
		// This method is sometimes called after a wallet is deleted and causes crash.
		log.Error("Attempting to read best block height without a loaded asset.")
		return 0
	}
	ctx, _ := asset.ShutdownContextWithCancel()
	_, height := asset.Internal().DCR.MainChainTip(ctx)
	return height
}

func (asset *DCRAsset) GetBestBlockTimeStamp() int64 {
	if asset.Internal() == nil {
		// This method is sometimes called after a wallet is deleted and causes crash.
		log.Error("Attempting to read best block timestamp without a loaded asset.")
		return 0
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	_, height := asset.Internal().DCR.MainChainTip(ctx)
	identifier := w.NewBlockIdentifierFromHeight(height)
	info, err := asset.Internal().DCR.BlockInfo(ctx, identifier)
	if err != nil {
		log.Error(err)
		return 0
	}
	return info.Timestamp
}

func (asset *DCRAsset) GetLowestBlockTimestamp() int64 {
	var timestamp int64 = -1
	bestBlockTimestamp := asset.GetBestBlockTimeStamp()
	if bestBlockTimestamp < timestamp || timestamp == -1 {
		timestamp = bestBlockTimestamp
	}

	return timestamp
}

func (asset *DCRAsset) DiscoverUsage(gapLimit uint32) error {
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
			asset.discoverAddressesFinished(asset.ID)
		}()

		ctx, cancel := asset.ShutdownContextWithCancel()

		asset.syncData.mu.Lock()
		asset.syncData.syncing = true
		asset.syncData.cancelSync = cancel
		asset.syncData.mu.Unlock()

		asset.discoverAddressesStarted(asset.ID)

		err := asset.Internal().DCR.DiscoverActiveAddresses(ctx, netBackend, &startBlock, !asset.Internal().DCR.Locked(), gapLimit)
		log.Error(err)
	}()

	return nil
}
