package libwallet

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
	"gitlab.com/raedah/cryptopower/libwallet/spv"
)

// reading/writing of properties of this struct are protected by mutex.x
type syncData struct {
	mu sync.RWMutex

	syncProgressListeners map[string]SyncProgressListener
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

	cfiltersFetchProgress    CFiltersFetchProgressReport
	headersFetchProgress     HeadersFetchProgressReport
	addressDiscoveryProgress AddressDiscoveryProgressReport
	headersRescanProgress    HeadersRescanProgressReport

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

func (mw *MultiWallet) initActiveSyncData() {

	cfiltersFetchProgress := CFiltersFetchProgressReport{
		GeneralSyncProgress:         &GeneralSyncProgress{},
		beginFetchCFiltersTimeStamp: 0,
		startCFiltersHeight:         -1,
		cfiltersFetchTimeSpent:      0,
		totalFetchedCFiltersCount:   0,
	}

	headersFetchProgress := HeadersFetchProgressReport{
		GeneralSyncProgress:      &GeneralSyncProgress{},
		beginFetchTimeStamp:      -1,
		headersFetchTimeSpent:    -1,
		totalFetchedHeadersCount: 0,
	}

	addressDiscoveryProgress := AddressDiscoveryProgressReport{
		GeneralSyncProgress:       &GeneralSyncProgress{},
		addressDiscoveryStartTime: -1,
		totalDiscoveryTimeSpent:   -1,
	}

	headersRescanProgress := HeadersRescanProgressReport{}
	headersRescanProgress.GeneralSyncProgress = &GeneralSyncProgress{}

	mw.syncData.mu.Lock()
	mw.syncData.activeSyncData = &activeSyncData{
		syncStage: InvalidSyncStage,

		cfiltersFetchProgress:    cfiltersFetchProgress,
		headersFetchProgress:     headersFetchProgress,
		addressDiscoveryProgress: addressDiscoveryProgress,
		headersRescanProgress:    headersRescanProgress,
	}
	mw.syncData.mu.Unlock()
}

func (mw *MultiWallet) IsSyncProgressListenerRegisteredFor(uniqueIdentifier string) bool {
	mw.syncData.mu.RLock()
	_, exists := mw.syncData.syncProgressListeners[uniqueIdentifier]
	mw.syncData.mu.RUnlock()
	return exists
}

func (mw *MultiWallet) AddSyncProgressListener(syncProgressListener SyncProgressListener, uniqueIdentifier string) error {
	if mw.IsSyncProgressListenerRegisteredFor(uniqueIdentifier) {
		return errors.New(ErrListenerAlreadyExist)
	}

	mw.syncData.mu.Lock()
	mw.syncData.syncProgressListeners[uniqueIdentifier] = syncProgressListener
	mw.syncData.mu.Unlock()

	// If sync is already on, notify this newly added listener of the current progress report.
	return mw.PublishLastSyncProgress(uniqueIdentifier)
}

func (mw *MultiWallet) RemoveSyncProgressListener(uniqueIdentifier string) {
	mw.syncData.mu.Lock()
	delete(mw.syncData.syncProgressListeners, uniqueIdentifier)
	mw.syncData.mu.Unlock()
}

func (mw *MultiWallet) syncProgressListeners() []SyncProgressListener {
	mw.syncData.mu.RLock()
	defer mw.syncData.mu.RUnlock()

	listeners := make([]SyncProgressListener, 0, len(mw.syncData.syncProgressListeners))
	for _, listener := range mw.syncData.syncProgressListeners {
		listeners = append(listeners, listener)
	}

	return listeners
}

func (mw *MultiWallet) PublishLastSyncProgress(uniqueIdentifier string) error {
	mw.syncData.mu.RLock()
	defer mw.syncData.mu.RUnlock()

	syncProgressListener, exists := mw.syncData.syncProgressListeners[uniqueIdentifier]
	if !exists {
		return errors.New(ErrInvalid)
	}

	if mw.syncData.syncing && mw.syncData.activeSyncData != nil {
		switch mw.syncData.activeSyncData.syncStage {
		case HeadersFetchSyncStage:
			syncProgressListener.OnHeadersFetchProgress(&mw.syncData.headersFetchProgress)
		case AddressDiscoverySyncStage:
			syncProgressListener.OnAddressDiscoveryProgress(&mw.syncData.addressDiscoveryProgress)
		case HeadersRescanSyncStage:
			syncProgressListener.OnHeadersRescanProgress(&mw.syncData.headersRescanProgress)
		}
	}

	return nil
}

func (mw *MultiWallet) EnableSyncLogs() {
	mw.syncData.mu.Lock()
	mw.syncData.showLogs = true
	mw.syncData.mu.Unlock()
}

func (mw *MultiWallet) SyncInactiveForPeriod(totalInactiveSeconds int64) {
	mw.syncData.mu.Lock()
	defer mw.syncData.mu.Unlock()

	if !mw.syncData.syncing || mw.syncData.activeSyncData == nil {
		log.Debug("Not accounting for inactive time, wallet is not syncing.")
		return
	}

	mw.syncData.totalInactiveSeconds += totalInactiveSeconds
	if mw.syncData.connectedPeers == 0 {
		// assume it would take another 60 seconds to reconnect to peers
		mw.syncData.totalInactiveSeconds += 60
	}
}

func (mw *MultiWallet) SpvSync() error {
	// prevent an attempt to sync when the previous syncing has not been canceled
	if mw.IsSyncing() || mw.IsSynced() {
		return errors.New(ErrSyncAlreadyInProgress)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("::1"), Port: 0}
	addrManager := addrmgr.New(mw.rootDir, net.LookupIP) // TODO: be mindful of tor
	lp := p2p.NewLocalPeer(mw.chainParams, addr, addrManager)

	var validPeerAddresses []string
	peerAddresses := mw.ReadStringConfigValueForKey(SpvPersistentPeerAddressesConfigKey)
	if peerAddresses != "" {
		addresses := strings.Split(peerAddresses, ";")
		for _, address := range addresses {
			peerAddress, err := NormalizeAddress(address, mw.chainParams.DefaultPort)
			if err != nil {
				log.Errorf("SPV peer address(%s) is invalid: %v", peerAddress, err)
			} else {
				validPeerAddresses = append(validPeerAddresses, peerAddress)
			}
		}

		if len(validPeerAddresses) == 0 {
			return errors.New(ErrInvalidPeers)
		}
	}

	// init activeSyncData to be used to hold data used
	// to calculate sync estimates only during sync
	mw.initActiveSyncData()

	wallets := make(map[int]*w.Wallet)
	for id, wallet := range mw.wallets {
		wallets[id] = wallet.Internal()
		wallet.waitingForHeaders = true
		wallet.syncing = true
	}

	syncer := spv.NewSyncer(wallets, lp)
	syncer.SetNotifications(mw.spvSyncNotificationCallbacks())
	if len(validPeerAddresses) > 0 {
		syncer.SetPersistentPeers(validPeerAddresses)
	}

	ctx, cancel := mw.contextWithShutdownCancel()

	var restartSyncRequested bool

	mw.syncData.mu.Lock()
	restartSyncRequested = mw.syncData.restartSyncRequested
	mw.syncData.restartSyncRequested = false
	mw.syncData.syncing = true
	mw.syncData.cancelSync = cancel
	mw.syncData.syncCanceled = make(chan struct{})
	mw.syncData.syncer = syncer
	mw.syncData.mu.Unlock()

	for _, listener := range mw.syncProgressListeners() {
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
				mw.notifySyncError(errors.Errorf("SPV synchronization deadline exceeded: %v", syncError))
			} else if syncError == context.Canceled {
				close(mw.syncData.syncCanceled)
				mw.notifySyncCanceled()
			} else {
				mw.notifySyncError(syncError)
			}
		}

		//reset sync variables
		mw.resetSyncData()
	}()
	return nil
}

func (mw *MultiWallet) RestartSpvSync() error {
	mw.syncData.mu.Lock()
	mw.syncData.restartSyncRequested = true
	mw.syncData.mu.Unlock()

	mw.CancelSync() // necessary to unset the network backend.
	return mw.SpvSync()
}

func (mw *MultiWallet) CancelSync() {
	mw.syncData.mu.RLock()
	cancelSync := mw.syncData.cancelSync
	mw.syncData.mu.RUnlock()

	if cancelSync != nil {
		log.Info("Canceling sync. May take a while for sync to fully cancel.")

		// Stop running cspp mixers
		for _, wallet := range mw.wallets {
			if wallet.IsAccountMixerActive() {
				log.Infof("[%d] Stopping cspp mixer", wallet.ID)
				err := mw.StopAccountMixer(wallet.ID)
				if err != nil {
					log.Errorf("[%d] Error stopping cspp mixer: %v", wallet.ID, err)
				}
			}
		}

		// Cancel the context used for syncer.Run in spvSync().
		// This may not immediately cause the sync process to terminate,
		// but when it eventually terminates, syncer.Run will return `err == context.Canceled`.
		cancelSync()

		// When sync terminates and syncer.Run returns `err == context.Canceled`,
		// we will get notified on this channel.
		<-mw.syncData.syncCanceled

		log.Info("Sync fully canceled.")
	}
}

func (wallet *Wallet) IsWaiting() bool {
	return wallet.waitingForHeaders
}

func (wallet *Wallet) IsSynced() bool {
	return wallet.synced
}

func (wallet *Wallet) IsSyncing() bool {
	return wallet.syncing
}

func (mw *MultiWallet) IsConnectedToDecredNetwork() bool {
	mw.syncData.mu.RLock()
	defer mw.syncData.mu.RUnlock()
	return mw.syncData.syncing || mw.syncData.synced
}

func (mw *MultiWallet) IsSynced() bool {
	mw.syncData.mu.RLock()
	defer mw.syncData.mu.RUnlock()
	return mw.syncData.synced
}

func (mw *MultiWallet) IsSyncing() bool {
	mw.syncData.mu.RLock()
	defer mw.syncData.mu.RUnlock()
	return mw.syncData.syncing
}

func (mw *MultiWallet) CurrentSyncStage() int32 {
	mw.syncData.mu.RLock()
	defer mw.syncData.mu.RUnlock()

	if mw.syncData != nil && mw.syncData.syncing {
		return mw.syncData.syncStage
	}
	return InvalidSyncStage
}

func (mw *MultiWallet) GeneralSyncProgress() *GeneralSyncProgress {
	mw.syncData.mu.RLock()
	defer mw.syncData.mu.RUnlock()

	if mw.syncData != nil && mw.syncData.syncing {
		switch mw.syncData.syncStage {
		case HeadersFetchSyncStage:
			return mw.syncData.headersFetchProgress.GeneralSyncProgress
		case AddressDiscoverySyncStage:
			return mw.syncData.addressDiscoveryProgress.GeneralSyncProgress
		case HeadersRescanSyncStage:
			return mw.syncData.headersRescanProgress.GeneralSyncProgress
		case CFiltersFetchSyncStage:
			return mw.syncData.cfiltersFetchProgress.GeneralSyncProgress
		}
	}

	return nil
}

func (mw *MultiWallet) ConnectedPeers() int32 {
	mw.syncData.mu.RLock()
	defer mw.syncData.mu.RUnlock()
	return mw.syncData.connectedPeers
}

func (mw *MultiWallet) PeerInfoRaw() ([]PeerInfo, error) {
	if !mw.IsConnectedToDecredNetwork() {
		return nil, errors.New(ErrNotConnected)
	}

	syncer := mw.syncData.syncer

	infos := make([]PeerInfo, 0, len(syncer.GetRemotePeers()))
	for _, rp := range syncer.GetRemotePeers() {
		info := PeerInfo{
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

func (mw *MultiWallet) PeerInfo() (string, error) {
	infos, err := mw.PeerInfoRaw()
	if err != nil {
		return "", err
	}

	result, _ := json.Marshal(infos)
	return string(result), nil
}

func (mw *MultiWallet) GetBestBlock() *BlockInfo {
	var bestBlock int32 = -1
	var blockInfo *BlockInfo
	for _, wallet := range mw.wallets {
		if !wallet.WalletOpened() {
			continue
		}

		walletBestBLock := wallet.GetBestBlock()
		if walletBestBLock > bestBlock || bestBlock == -1 {
			bestBlock = walletBestBLock
			blockInfo = &BlockInfo{Height: bestBlock, Timestamp: wallet.GetBestBlockTimeStamp()}
		}
	}

	return blockInfo
}

func (mw *MultiWallet) GetLowestBlock() *BlockInfo {
	var lowestBlock int32 = -1
	var blockInfo *BlockInfo
	for _, wallet := range mw.wallets {
		if !wallet.WalletOpened() {
			continue
		}
		walletBestBLock := wallet.GetBestBlock()
		if walletBestBLock < lowestBlock || lowestBlock == -1 {
			lowestBlock = walletBestBLock
			blockInfo = &BlockInfo{Height: lowestBlock, Timestamp: wallet.GetBestBlockTimeStamp()}
		}
	}

	return blockInfo
}

func (wallet *Wallet) GetBestBlock() int32 {
	if wallet.Internal() == nil {
		// This method is sometimes called after a wallet is deleted and causes crash.
		log.Error("Attempting to read best block height without a loaded wallet.")
		return 0
	}

	_, height := wallet.Internal().MainChainTip(wallet.shutdownContext())
	return height
}

func (wallet *Wallet) GetBestBlockTimeStamp() int64 {
	if wallet.Internal() == nil {
		// This method is sometimes called after a wallet is deleted and causes crash.
		log.Error("Attempting to read best block timestamp without a loaded wallet.")
		return 0
	}

	ctx := wallet.shutdownContext()
	_, height := wallet.Internal().MainChainTip(ctx)
	identifier := w.NewBlockIdentifierFromHeight(height)
	info, err := wallet.Internal().BlockInfo(ctx, identifier)
	if err != nil {
		log.Error(err)
		return 0
	}
	return info.Timestamp
}

func (mw *MultiWallet) GetLowestBlockTimestamp() int64 {
	var timestamp int64 = -1
	for _, wallet := range mw.wallets {
		bestBlockTimestamp := wallet.GetBestBlockTimeStamp()
		if bestBlockTimestamp < timestamp || timestamp == -1 {
			timestamp = bestBlockTimestamp
		}
	}
	return timestamp
}
