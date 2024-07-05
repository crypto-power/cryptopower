package dcr

import (
	"math"
	"time"

	"decred.org/dcrwallet/v4/spv"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"golang.org/x/sync/errgroup"
)

func (asset *Asset) spvSyncNotificationCallbacks() *spv.Notifications {
	return &spv.Notifications{
		PeerConnected: func(peerCount int32, _ string) {
			asset.handlePeerCountUpdate(peerCount)
		},
		PeerDisconnected: func(peerCount int32, _ string) {
			asset.handlePeerCountUpdate(peerCount)
		},
		Synced:                       asset.syncedWallet,
		FetchHeadersStarted:          asset.fetchHeadersStarted,
		FetchHeadersProgress:         asset.fetchHeadersProgress,
		FetchHeadersFinished:         asset.fetchHeadersFinished,
		FetchMissingCFiltersStarted:  asset.fetchCFiltersStarted,
		FetchMissingCFiltersProgress: asset.fetchCFiltersProgress,
		FetchMissingCFiltersFinished: asset.fetchCFiltersEnded,
		DiscoverAddressesStarted:     asset.discoverAddressesStarted,
		DiscoverAddressesFinished:    asset.discoverAddressesFinished,
		RescanStarted:                asset.rescanStarted,
		RescanProgress:               asset.rescanProgress,
		RescanFinished:               asset.rescanFinished,
	}
}

func (asset *Asset) handlePeerCountUpdate(peerCount int32) {
	asset.syncData.mu.Lock()
	asset.syncData.numOfConnectedPeers = peerCount
	shouldLog := asset.syncData.showLogs && asset.syncData.syncing
	asset.syncData.mu.Unlock()

	for _, syncProgressListener := range asset.syncProgressListeners() {
		if syncProgressListener.OnPeerConnectedOrDisconnected != nil {
			syncProgressListener.OnPeerConnectedOrDisconnected(peerCount)
		}
	}

	if shouldLog {
		if peerCount == 1 {
			log.Infof("Connected to %d peer on %s.", peerCount, asset.chainParams.Name)
		} else {
			log.Infof("Connected to %d peers on %s.", peerCount, asset.chainParams.Name)
		}
	}
}

// Fetch CFilters Callbacks

func (asset *Asset) fetchCFiltersStarted() {
	asset.syncData.mu.Lock()
	asset.syncData.syncStage = CFiltersFetchSyncStage
	asset.syncData.scanStartTime = time.Now()
	asset.syncData.scanStartHeight = -1
	showLogs := asset.syncData.showLogs
	asset.syncData.mu.Unlock()

	if showLogs {
		log.Infof("Step 1 of 3 - fetching %d block headers.")
	}
}

func (asset *Asset) fetchCFiltersProgress(startCFiltersHeight, endCFiltersHeight int32) {
	// lock the mutex before reading and writing to asset.syncData.*
	asset.syncData.mu.Lock()

	if asset.syncData.scanStartHeight == -1 {
		asset.syncData.scanStartHeight = startCFiltersHeight
	}

	var cfiltersFetchData = &sharedW.CFiltersFetchProgressReport{
		GeneralSyncProgress:       &sharedW.GeneralSyncProgress{},
		TotalFetchedCFiltersCount: endCFiltersHeight - startCFiltersHeight,
	}

	totalCFiltersToFetch := float64(asset.GetBestBlockHeight() - asset.syncData.scanStartHeight)
	cfiltersFetchProgress := float64(cfiltersFetchData.TotalFetchedCFiltersCount) / totalCFiltersToFetch

	// If there was some period of inactivity,
	// assume that this process started at some point in the future,
	// thereby accounting for the total reported time of inactivity.
	asset.syncData.scanStartTime = asset.syncData.scanStartTime.Add(asset.syncData.totalInactiveSeconds)
	asset.syncData.totalInactiveSeconds = 0

	timeDurationTaken := time.Since(asset.syncData.scanStartTime)
	timeTakenSoFar := timeDurationTaken.Seconds()
	if timeTakenSoFar < 1 {
		timeTakenSoFar = 1
	}

	asset.syncData.mu.Unlock()

	estimatedTotalCFiltersFetchTime := timeTakenSoFar / cfiltersFetchProgress

	// Use CFilters fetch rate to estimate headers fetch time.
	cfiltersFetchRate := float64(cfiltersFetchData.TotalFetchedCFiltersCount) / timeTakenSoFar
	estimatedHeadersLeftToFetch := asset.estimateBlockHeadersCountAfter(asset.GetBestBlockTimeStamp())
	estimatedTotalHeadersFetchTime := float64(estimatedHeadersLeftToFetch) / cfiltersFetchRate
	// increase estimated value by fetchPercentage
	estimatedTotalHeadersFetchTime /= fetchPercentage

	estimatedDiscoveryTime := estimatedTotalHeadersFetchTime * discoveryPercentage
	estimatedRescanTime := estimatedTotalHeadersFetchTime * rescanPercentage
	estimatedTotalSyncTime := estimatedTotalCFiltersFetchTime + estimatedTotalHeadersFetchTime + estimatedDiscoveryTime + estimatedRescanTime

	totalSyncProgress := timeTakenSoFar / estimatedTotalSyncTime
	totalTimeRemainingSeconds := secondsToDuration(estimatedTotalSyncTime - timeTakenSoFar)

	// update headers fetching progress report including total progress percentage and total time remaining
	cfiltersFetchData.TotalCFiltersToFetch = int32(totalCFiltersToFetch)
	cfiltersFetchData.CurrentCFilterHeight = startCFiltersHeight
	cfiltersFetchData.CFiltersFetchProgress = roundUp(cfiltersFetchProgress * 100.0)
	cfiltersFetchData.CfiltersFetchTimeSpent = timeDurationTaken
	cfiltersFetchData.GeneralSyncProgress.TotalSyncProgress = roundUp(totalSyncProgress * 100.0)
	cfiltersFetchData.GeneralSyncProgress.TotalTimeRemaining = totalTimeRemainingSeconds

	// notify progress listener of estimated progress report
	asset.publishFetchCFiltersProgress(cfiltersFetchData)
}

func (asset *Asset) publishFetchCFiltersProgress(cfilters *sharedW.CFiltersFetchProgressReport) {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		if syncProgressListener.OnCFiltersFetchProgress != nil {
			syncProgressListener.OnCFiltersFetchProgress(cfilters)
		}
	}

	// update the general sync progress
	asset.updateGeneralSyncProgress(cfilters.GeneralSyncProgress)
}

func (asset *Asset) fetchCFiltersEnded() {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	// Record the time spent when the filter scan.
	asset.syncData.cfiltersScanTimeSpent = time.Since(asset.syncData.scanStartTime)

	// Clean up the shared data fields
	asset.syncData.scanStartTime = time.Time{}
	asset.syncData.scanStartHeight = -1
	asset.syncData.genSyncProgress = nil // clear preset general progress
}

// Fetch Headers Callbacks
func (asset *Asset) fetchHeadersStarted() {
	if !asset.IsSyncing() {
		return
	}

	// fetch all the peers information currently available
	peers, err := asset.PeerInfoRaw()
	if err != nil {
		log.Errorf("fetchHeadersStarted failed: %v", err)
		return
	}

	// pick the highest height.
	var peerInitialHeight int32
	for _, p := range peers {
		if int32(p.StartingHeight) > peerInitialHeight {
			peerInitialHeight = int32(p.StartingHeight)
		}
	}

	asset.syncData.mu.RLock()
	headersFetchingStarted := asset.syncData.scanStartHeight != -1
	asset.syncData.mu.RUnlock()

	if headersFetchingStarted {
		// This function gets invoked once for each active sync session.
		return
	}

	asset.waitingForHeaders = true

	lowestBlockHeight := asset.GetBestBlock().Height

	asset.syncData.mu.Lock()
	asset.syncData.syncStage = HeadersFetchSyncStage
	asset.syncData.scanStartTime = time.Now()
	asset.syncData.scanStartHeight = lowestBlockHeight
	asset.syncData.bestBlockheight = peerInitialHeight // Best block synced in the connected peers
	asset.syncData.totalInactiveSeconds = 0
	showLogs := asset.syncData.showLogs
	asset.syncData.mu.Unlock()

	if showLogs {
		log.Infof("Step 1 of 3 - fetching %d block headers.", peerInitialHeight-lowestBlockHeight)
	}
}

func (asset *Asset) fetchHeadersProgress(lastFetchedHeaderHeight int32, _ int64) {
	if !asset.IsSyncing() {
		return
	}

	asset.syncData.mu.RLock()
	startHeight := asset.syncData.scanStartHeight
	startTime := asset.syncData.scanStartTime
	peersBestBlock := asset.syncData.bestBlockheight
	headerSpentTime := asset.syncData.headersScanTimeSpent
	asset.syncData.mu.RUnlock()

	if startHeight == -1 {
		// Required preset data is missing. Invoke fetchHeadersStarted() first
		// before proceeding.
		return
	}

	if headerSpentTime.Milliseconds() > 0 {
		// This function gets called for each newly connected peer so ignore
		// this call if the headers fetching phase was previously completed.
		return
	}

	if asset.waitingForHeaders {
		asset.waitingForHeaders = asset.GetBestBlockHeight() > lastFetchedHeaderHeight
	}

	headersFetchedSoFar := float64(lastFetchedHeaderHeight - startHeight)
	if headersFetchedSoFar < 1 {
		headersFetchedSoFar = 1
	}

	fetchTimeTakenSoFar := time.Since(startTime).Seconds()
	if fetchTimeTakenSoFar < 1 {
		fetchTimeTakenSoFar = 1
	}

	remainingHeaders := float64(peersBestBlock - lastFetchedHeaderHeight)
	if remainingHeaders < 1 {
		remainingHeaders = 1
	}

	allHeadersToFetch := headersFetchedSoFar + remainingHeaders
	timeRemaining := (fetchTimeTakenSoFar * remainingHeaders) / headersFetchedSoFar
	syncProgress := int32((headersFetchedSoFar * 100) / allHeadersToFetch)

	headersFetchedData := &sharedW.HeadersFetchProgressReport{
		GeneralSyncProgress: &sharedW.GeneralSyncProgress{
			TotalSyncProgress:  syncProgress,
			TotalTimeRemaining: secondsToDuration(timeRemaining),
		},
	}
	headersFetchedData.TotalHeadersToFetch = asset.syncData.bestBlockheight
	headersFetchedData.HeadersFetchProgress = syncProgress
	headersFetchedData.HeadersFetchTimeSpent = time.Since(startTime)

	// notify progress listener of estimated progress report
	asset.publishFetchHeadersProgress(headersFetchedData)
}

func (asset *Asset) publishFetchHeadersProgress(headerFetch *sharedW.HeadersFetchProgressReport) {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		if syncProgressListener.OnHeadersFetchProgress != nil {
			syncProgressListener.OnHeadersFetchProgress(headerFetch)
		}
	}

	// update the general sync progress
	asset.updateGeneralSyncProgress(headerFetch.GeneralSyncProgress)
}

func (asset *Asset) fetchHeadersFinished() {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if !asset.syncData.syncing {
		// ignore if sync is not in progress
		return
	}

	// Record the time spent when the headers scan.
	asset.syncData.headersScanTimeSpent = time.Since(asset.syncData.scanStartTime)

	// Clean up the shared data fields
	asset.syncData.scanStartTime = time.Time{}
	asset.syncData.scanStartHeight = -1
	asset.syncData.genSyncProgress = nil // clear preset general progress

	if asset.syncData.showLogs && asset.syncData.syncing {
		log.Info("Fetch headers completed.")
	}
}

// Address/Account Discovery Callbacks

func (asset *Asset) discoverAddressesStarted() {
	if !asset.IsSyncing() {
		return
	}

	asset.syncData.mu.RLock()
	addressDiscoveryAlreadyStarted := asset.syncData.scanStartTime.IsZero()
	asset.syncData.mu.RUnlock()

	if addressDiscoveryAlreadyStarted {
		return
	}

	asset.syncData.mu.Lock()
	asset.syncData.isAddressDiscovery = true
	asset.syncData.syncStage = AddressDiscoverySyncStage
	asset.syncData.scanStartTime = time.Now()
	asset.syncData.addressDiscoveryCompletedOrCanceled = make(chan bool)
	asset.syncData.mu.Unlock()

	go asset.updateAddressDiscoveryProgress()

	if asset.syncData.showLogs {
		log.Info("Step 2 of 3 - discovering used addresses.")
	}
}

func (asset *Asset) updateAddressDiscoveryProgress() {
	// use ticker to calculate and broadcast address discovery progress every second
	everySecondTicker := time.NewTicker(1 * time.Second)

	asset.syncData.mu.Lock()
	totalHeadersFetchTime := asset.syncData.headersScanTimeSpent.Seconds()
	totalCfiltersFetchTime := asset.syncData.cfiltersScanTimeSpent.Seconds()
	asset.syncData.mu.Unlock()

	// these values will be used every second to calculate the total sync progress
	estimatedDiscoveryTime := totalHeadersFetchTime * discoveryPercentage
	estimatedRescanTime := totalHeadersFetchTime * rescanPercentage

	// track last logged time remaining and total percent to avoid re-logging same message
	var lastTimeRemaining time.Duration
	var lastTotalPercent int32 = -1

	for {
		if !asset.IsSyncing() {
			return
		}

		// If there was some period of inactivity,
		// assume that this process started at some point in the future,
		// thereby accounting for the total reported time of inactivity.
		asset.syncData.mu.Lock()
		asset.syncData.scanStartTime = asset.syncData.scanStartTime.Add(asset.syncData.totalInactiveSeconds)
		asset.syncData.totalInactiveSeconds = 0
		addressDiscoveryStartTime := asset.syncData.scanStartTime
		showLogs := asset.syncData.showLogs
		asset.syncData.mu.Unlock()

		select {
		case <-asset.syncData.addressDiscoveryCompletedOrCanceled:
			// stop calculating and broadcasting address discovery progress
			everySecondTicker.Stop()
			if showLogs {
				log.Info("Address discovery complete.")
			}
			return

		case <-everySecondTicker.C:
			// calculate address discovery progress
			elapsedDiscoveryTime := time.Since(addressDiscoveryStartTime).Seconds()
			discoveryProgress := (elapsedDiscoveryTime / estimatedDiscoveryTime) * 100

			totalSyncTime := totalCfiltersFetchTime + totalHeadersFetchTime
			if elapsedDiscoveryTime > estimatedDiscoveryTime {
				totalSyncTime += elapsedDiscoveryTime + estimatedRescanTime
			} else {
				totalSyncTime += estimatedDiscoveryTime + estimatedRescanTime
			}

			totalElapsedTime := totalCfiltersFetchTime + totalHeadersFetchTime + elapsedDiscoveryTime
			totalProgress := (totalElapsedTime / totalSyncTime) * 100

			remainingAccountDiscoveryTime := estimatedDiscoveryTime - elapsedDiscoveryTime
			if remainingAccountDiscoveryTime < 0 {
				remainingAccountDiscoveryTime = 0
			}

			totalProgressPercent := int32(totalProgress)
			totalTimeRemainingSeconds := secondsToDuration(remainingAccountDiscoveryTime + estimatedRescanTime)

			// update address discovery progress, total progress and total time remaining
			addressDiscoveryData := &sharedW.AddressDiscoveryProgressReport{
				GeneralSyncProgress: &sharedW.GeneralSyncProgress{
					TotalSyncProgress:  totalProgressPercent,
					TotalTimeRemaining: totalTimeRemainingSeconds,
				},
			}
			addressDiscoveryData.AddressDiscoveryProgress = int32(discoveryProgress)

			asset.publishAddressDiscoveryProgress(addressDiscoveryData)

			if showLogs {
				// avoid logging same message multiple times
				if totalProgressPercent != lastTotalPercent || totalTimeRemainingSeconds != lastTimeRemaining {
					log.Infof("Syncing %d%%, %s remaining, discovering used addresses.",
						totalProgressPercent, calculateTotalTimeRemaining(totalTimeRemainingSeconds))

					lastTotalPercent = totalProgressPercent
					lastTimeRemaining = totalTimeRemainingSeconds
				}
			}
		}
	}
}

func (asset *Asset) publishAddressDiscoveryProgress(addrDiscovery *sharedW.AddressDiscoveryProgressReport) {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		if syncProgressListener.OnAddressDiscoveryProgress != nil {
			syncProgressListener.OnAddressDiscoveryProgress(addrDiscovery)
		}
	}

	// update the general sync progress
	asset.updateGeneralSyncProgress(addrDiscovery.GeneralSyncProgress)
}

func (asset *Asset) discoverAddressesFinished() {
	if !asset.IsSyncing() {
		return
	}

	asset.syncData.mu.Lock()
	asset.syncData.isAddressDiscovery = false
	asset.syncData.genSyncProgress = nil // clear preset general progress

	// Record the time spent when the headers scan.
	asset.syncData.addrDiscoveryTimeSpent = time.Since(asset.syncData.scanStartTime)

	// Clean up the shared data fields
	asset.syncData.scanStartTime = time.Time{}
	asset.syncData.scanStartHeight = -1
	asset.syncData.genSyncProgress = nil // clear preset general progress
	asset.syncData.mu.Unlock()

	err := asset.MarkWalletAsDiscoveredAccounts() // Mark address discovery as completed.
	if err != nil {
		log.Error(err)
	}

	asset.stopUpdatingAddressDiscoveryProgress()
}

func (asset *Asset) stopUpdatingAddressDiscoveryProgress() {
	asset.syncData.mu.Lock()
	if asset.syncData != nil && asset.syncData.addressDiscoveryCompletedOrCanceled != nil {
		close(asset.syncData.addressDiscoveryCompletedOrCanceled)
		asset.syncData.addressDiscoveryCompletedOrCanceled = nil
		asset.syncData.addrDiscoveryTimeSpent = time.Since(asset.syncData.scanStartTime)
	}
	asset.syncData.mu.Unlock()
}

// Blocks Scan Callbacks
func (asset *Asset) rescanStarted() {
	asset.stopUpdatingAddressDiscoveryProgress()

	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if !asset.syncData.syncing {
		// ignore if sync is not in progress
		return
	}

	asset.syncData.isRescanning = true
	asset.syncData.syncStage = HeadersRescanSyncStage
	asset.syncData.scanStartTime = time.Now()

	if asset.syncData.showLogs && asset.syncData.syncing {
		log.Info("Step 3 of 3 - Scanning block headers.")
	}
}

func (asset *Asset) rescanProgress(rescannedThrough int32) {
	if !asset.IsSyncing() {
		// ignore if sync is not in progress
		return
	}

	totalHeadersToScan := asset.GetBestBlockHeight()

	rescanRate := float64(rescannedThrough) / float64(totalHeadersToScan)

	asset.syncData.mu.Lock()

	// If there was some period of inactivity,
	// assume that this process started at some point in the future,
	// thereby accounting for the total reported time of inactivity.
	asset.syncData.scanStartTime = asset.syncData.scanStartTime.Add(asset.syncData.totalInactiveSeconds)
	asset.syncData.totalInactiveSeconds = 0

	elapsedRescanTime := time.Since(asset.syncData.scanStartTime)
	estimatedTotalRescanTime := elapsedRescanTime.Seconds() / rescanRate
	totalTimeRemainingSeconds := secondsToDuration(estimatedTotalRescanTime) - elapsedRescanTime
	totalElapsedTimePreRescans := asset.syncData.cfiltersScanTimeSpent +
		asset.syncData.headersScanTimeSpent + asset.syncData.addrDiscoveryTimeSpent
	asset.syncData.mu.Unlock()

	totalElapsedTime := totalElapsedTimePreRescans + elapsedRescanTime

	headersRescanData := &sharedW.HeadersRescanProgressReport{
		GeneralSyncProgress: &sharedW.GeneralSyncProgress{},
	}
	headersRescanData.TotalHeadersToScan = totalHeadersToScan
	headersRescanData.RescanProgress = int32(rescanRate * 100)
	headersRescanData.CurrentRescanHeight = rescannedThrough
	headersRescanData.RescanTimeRemaining = totalTimeRemainingSeconds

	// do not update total time taken and total progress percent if elapsedRescanTime is 0
	// because the estimatedTotalRescanTime will be inaccurate (also 0)
	// which will make the estimatedTotalSyncTime equal to totalElapsedTime
	// giving the wrong impression that the process is complete
	if elapsedRescanTime > 0 {
		estimatedTotalSyncTime := totalElapsedTimePreRescans + secondsToDuration(estimatedTotalRescanTime)
		totalProgress := (totalElapsedTime.Seconds() / estimatedTotalSyncTime.Seconds()) * 100

		headersRescanData.GeneralSyncProgress.TotalTimeRemaining = totalTimeRemainingSeconds
		headersRescanData.GeneralSyncProgress.TotalSyncProgress = int32(totalProgress)
	}

	asset.publishHeadersRescanProgress(headersRescanData)

	asset.syncData.mu.RLock()
	if asset.syncData.showLogs {
		log.Infof("Syncing %d%%, %s remaining, scanning %d of %d block headers.",
			headersRescanData.TotalSyncProgress,
			calculateTotalTimeRemaining(headersRescanData.TotalTimeRemaining),
			headersRescanData.CurrentRescanHeight, headersRescanData.TotalHeadersToScan,
		)
	}
	asset.syncData.mu.RUnlock()
}

func (asset *Asset) publishHeadersRescanProgress(headersRescanData *sharedW.HeadersRescanProgressReport) {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		if syncProgressListener.OnHeadersRescanProgress != nil {
			syncProgressListener.OnHeadersRescanProgress(headersRescanData)
		}
	}

	// update the general sync progress
	asset.updateGeneralSyncProgress(headersRescanData.GeneralSyncProgress)
}

func (asset *Asset) rescanFinished() {
	if !asset.IsSyncing() {
		// ignore if sync is not in progress
		return
	}

	asset.syncData.mu.Lock()
	asset.syncData.isRescanning = false

	// Record the time spent when the headers scan.
	asset.syncData.rescanTimeSpent = time.Since(asset.syncData.scanStartTime)

	// Clean up the shared data fields
	asset.syncData.scanStartTime = time.Time{}
	asset.syncData.scanStartHeight = -1
	asset.syncData.genSyncProgress = nil // clear preset general progress
	asset.syncData.mu.Unlock()
}

/** Helper functions start here */

func (asset *Asset) estimateBlockHeadersCountAfter(lastHeaderTime int64) int32 {
	// Use the difference between current time (now) and last reported block time,
	// to estimate total headers to fetch.
	timeDifferenceInSeconds := float64(time.Now().Unix() - lastHeaderTime)
	targetTimePerBlockInSeconds := asset.chainParams.TargetTimePerBlock.Seconds()
	estimatedHeadersDifference := timeDifferenceInSeconds / targetTimePerBlockInSeconds

	// return next integer value (upper limit) if estimatedHeadersDifference is a fraction
	return int32(math.Ceil(estimatedHeadersDifference))
}

func (asset *Asset) notifySyncError(err error) {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		if syncProgressListener.OnSyncEndedWithError != nil {
			syncProgressListener.OnSyncEndedWithError(err)
		}
	}
}

func (asset *Asset) notifySyncCanceled() {
	asset.syncData.mu.RLock()
	restartSyncRequested := asset.syncData.restartSyncRequested
	asset.syncData.mu.RUnlock()

	for _, syncProgressListener := range asset.syncProgressListeners() {
		if syncProgressListener.OnSyncCanceled != nil {
			syncProgressListener.OnSyncCanceled(restartSyncRequested)
		}
	}
}

func (asset *Asset) resetSyncData() {
	// It's possible that sync ends or errors while address discovery is ongoing.
	// If this happens, it's important to stop the address discovery process before
	// resetting sync data.
	asset.stopUpdatingAddressDiscoveryProgress()

	asset.syncData.mu.Lock()
	asset.syncData.syncing = false
	asset.syncData.synced = false
	asset.syncData.cancelSync = nil
	asset.syncData.syncCanceled = nil
	asset.syncData.activeSyncData = nil
	asset.syncData.mu.Unlock()

	asset.waitingForHeaders = true
	asset.LockWallet() // lock wallet if previously unlocked to perform account discovery.
}

func (asset *Asset) syncedWallet(synced bool) {
	ctx, _ := asset.ShutdownContextWithCancel()

	indexTransactions := func() {
		// begin indexing transactions after sync is completed,
		// syncProgressListeners.OnSynced() will be invoked after transactions are indexed
		txIndexing, _ := errgroup.WithContext(ctx)
		txIndexing.Go(asset.IndexTransactions)

		go func() {
			err := txIndexing.Wait()
			if err != nil {
				log.Errorf("Tx Index Error: %v", err)
			}

			for _, syncProgressListener := range asset.syncProgressListeners() {
				if synced {
					if syncProgressListener.OnSyncCompleted != nil {
						syncProgressListener.OnSyncCompleted()
					}
				} else {
					if syncProgressListener.OnSyncCanceled != nil {
						syncProgressListener.OnSyncCanceled(false)
					}
				}
			}
		}()
	}

	asset.syncData.mu.RLock()
	allWalletsSynced := asset.syncData.synced
	asset.syncData.mu.RUnlock()

	if allWalletsSynced && synced {
		indexTransactions()
		return
	}

	asset.synced = synced
	asset.syncing = false
	asset.listenForTransactions()

	if !asset.Internal().DCR.Locked() {
		asset.LockWallet() // lock wallet if previously unlocked to perform account discovery.
		err := asset.MarkWalletAsDiscoveredAccounts()
		if err != nil {
			log.Error(err)
		}
	}

	asset.syncData.mu.Lock()
	asset.syncData.syncing = false
	asset.syncData.synced = true
	asset.syncData.mu.Unlock()

	indexTransactions()
}

// updateGeneralSyncProgress tracks the general sync progress of the actively
// running sync.
func (asset *Asset) updateGeneralSyncProgress(progress *sharedW.GeneralSyncProgress) {
	asset.syncData.mu.Lock()
	asset.syncData.genSyncProgress = progress
	asset.syncData.mu.Unlock()
}
