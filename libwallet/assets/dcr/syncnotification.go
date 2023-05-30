package dcr

import (
	"math"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"decred.org/dcrwallet/v2/spv"
	"golang.org/x/sync/errgroup"
)

func (asset *DCRAsset) spvSyncNotificationCallbacks() *spv.Notifications {
	return &spv.Notifications{
		PeerConnected: func(peerCount int32, addr string) {
			asset.handlePeerCountUpdate(peerCount)
		},
		PeerDisconnected: func(peerCount int32, addr string) {
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

func (asset *DCRAsset) handlePeerCountUpdate(peerCount int32) {
	asset.syncData.mu.Lock()
	asset.syncData.connectedPeers = peerCount
	shouldLog := asset.syncData.showLogs && asset.syncData.syncing
	asset.syncData.mu.Unlock()

	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.OnPeerConnectedOrDisconnected(peerCount)
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

func (asset *DCRAsset) fetchCFiltersStarted() {
	asset.syncData.mu.Lock()
	asset.syncData.syncStage = CFiltersFetchSyncStage
	asset.syncData.cfiltersFetchProgress.BeginFetchCFiltersTimeStamp = time.Now().Unix()
	asset.syncData.cfiltersFetchProgress.TotalFetchedCFiltersCount = 0
	showLogs := asset.syncData.showLogs
	asset.syncData.mu.Unlock()

	if showLogs {
		log.Infof("Step 1 of 3 - fetching %d block headers.")
	}
}

func (asset *DCRAsset) fetchCFiltersProgress(startCFiltersHeight, endCFiltersHeight int32) {
	// lock the mutex before reading and writing to asset.syncData.*
	asset.syncData.mu.Lock()

	if asset.syncData.cfiltersFetchProgress.StartCFiltersHeight == -1 {
		asset.syncData.cfiltersFetchProgress.StartCFiltersHeight = startCFiltersHeight
	}

	asset.syncData.cfiltersFetchProgress.TotalFetchedCFiltersCount += endCFiltersHeight - startCFiltersHeight

	totalCFiltersToFetch := asset.GetBestBlockHeight() - asset.syncData.cfiltersFetchProgress.StartCFiltersHeight

	cfiltersFetchProgress := float64(asset.syncData.cfiltersFetchProgress.TotalFetchedCFiltersCount) / float64(totalCFiltersToFetch)

	// If there was some period of inactivity,
	// assume that this process started at some point in the future,
	// thereby accounting for the total reported time of inactivity.
	asset.syncData.cfiltersFetchProgress.BeginFetchCFiltersTimeStamp += asset.syncData.totalInactiveSeconds
	asset.syncData.totalInactiveSeconds = 0

	timeTakenSoFar := time.Now().Unix() - asset.syncData.cfiltersFetchProgress.BeginFetchCFiltersTimeStamp
	if timeTakenSoFar < 1 {
		timeTakenSoFar = 1
	}
	estimatedTotalCFiltersFetchTime := float64(timeTakenSoFar) / cfiltersFetchProgress

	// Use CFilters fetch rate to estimate headers fetch time.
	cfiltersFetchRate := float64(asset.syncData.activeSyncData.cfiltersFetchProgress.TotalFetchedCFiltersCount) / float64(timeTakenSoFar)
	estimatedHeadersLeftToFetch := asset.estimateBlockHeadersCountAfter(asset.GetBestBlockTimeStamp())
	estimatedTotalHeadersFetchTime := float64(estimatedHeadersLeftToFetch) / cfiltersFetchRate
	// increase estimated value by fetchPercentage
	estimatedTotalHeadersFetchTime /= fetchPercentage

	estimatedDiscoveryTime := estimatedTotalHeadersFetchTime * discoveryPercentage
	estimatedRescanTime := estimatedTotalHeadersFetchTime * rescanPercentage
	estimatedTotalSyncTime := estimatedTotalCFiltersFetchTime + estimatedTotalHeadersFetchTime + estimatedDiscoveryTime + estimatedRescanTime

	totalSyncProgress := float64(timeTakenSoFar) / estimatedTotalSyncTime
	totalTimeRemainingSeconds := int64(math.Round(estimatedTotalSyncTime)) - timeTakenSoFar

	// update headers fetching progress report including total progress percentage and total time remaining
	asset.syncData.cfiltersFetchProgress.TotalCFiltersToFetch = totalCFiltersToFetch
	asset.syncData.cfiltersFetchProgress.CurrentCFilterHeight = startCFiltersHeight
	asset.syncData.cfiltersFetchProgress.CFiltersFetchProgress = roundUp(cfiltersFetchProgress * 100.0)
	asset.syncData.cfiltersFetchProgress.TotalSyncProgress = roundUp(totalSyncProgress * 100.0)
	asset.syncData.cfiltersFetchProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds

	asset.syncData.mu.Unlock()

	// notify progress listener of estimated progress report
	asset.publishFetchCFiltersProgress()

	cfiltersFetchTimeRemaining := estimatedTotalCFiltersFetchTime - float64(timeTakenSoFar)
	debugInfo := &sharedW.DebugInfo{
		TotalTimeElapsed:          timeTakenSoFar,
		TotalTimeRemaining:        totalTimeRemainingSeconds,
		CurrentStageTimeElapsed:   timeTakenSoFar,
		CurrentStageTimeRemaining: int64(math.Round(cfiltersFetchTimeRemaining)),
	}
	asset.publishDebugInfo(debugInfo)
}

func (asset *DCRAsset) publishFetchCFiltersProgress() {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.OnCFiltersFetchProgress(&asset.syncData.cfiltersFetchProgress)
	}
}

func (asset *DCRAsset) fetchCFiltersEnded() {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	asset.syncData.cfiltersFetchProgress.CfiltersFetchTimeSpent = time.Now().Unix() - asset.syncData.cfiltersFetchProgress.BeginFetchCFiltersTimeStamp

	// If there is some period of inactivity reported at this stage,
	// subtract it from the total stage time.
	asset.syncData.cfiltersFetchProgress.CfiltersFetchTimeSpent -= asset.syncData.totalInactiveSeconds
	asset.syncData.totalInactiveSeconds = 0
}

// Fetch Headers Callbacks

func (asset *DCRAsset) fetchHeadersStarted() {
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
	headersFetchingStarted := asset.syncData.headersFetchProgress.StartHeaderHeight != nil
	asset.syncData.mu.RUnlock()

	if headersFetchingStarted {
		// This function gets called for each newly connected peer so
		// ignore if headers fetching was already started.
		return
	}

	asset.waitingForHeaders = true

	lowestBlockHeight := asset.GetBestBlock().Height

	asset.syncData.mu.Lock()
	asset.syncData.syncStage = HeadersFetchSyncStage
	asset.syncData.headersFetchProgress.BeginFetchTimeStamp = time.Now()
	asset.syncData.headersFetchProgress.StartHeaderHeight = &lowestBlockHeight
	asset.syncData.bestBlockheight = peerInitialHeight // Best block synced in the connected peers
	asset.syncData.totalInactiveSeconds = 0
	showLogs := asset.syncData.showLogs
	asset.syncData.mu.Unlock()

	if showLogs {
		log.Infof("Step 1 of 3 - fetching %d block headers.", peerInitialHeight-lowestBlockHeight)
	}
}

func (asset *DCRAsset) fetchHeadersProgress(lastFetchedHeaderHeight int32, lastFetchedHeaderTime int64) {
	if !asset.IsSyncing() {
		return
	}

	asset.syncData.mu.Lock()

	if asset.syncData.headersFetchProgress.HeadersFetchTimeSpent != -1 {
		// This function gets called for each newly connected peer so ignore
		// this call if the headers fetching phase was previously completed.
		return
	}

	if asset.waitingForHeaders {
		asset.waitingForHeaders = asset.GetBestBlockHeight() > lastFetchedHeaderHeight
	}

	headersFetchedSoFar := float64(lastFetchedHeaderHeight - *asset.syncData.headersFetchProgress.StartHeaderHeight)
	if headersFetchedSoFar < 1 {
		headersFetchedSoFar = 1
	}

	fetchTimeTakenSoFar := time.Since(asset.syncData.headersFetchProgress.BeginFetchTimeStamp).Seconds()
	if fetchTimeTakenSoFar < 1 {
		fetchTimeTakenSoFar = 1
	}

	remainingHeaders := float64(asset.syncData.bestBlockheight - lastFetchedHeaderHeight)
	if remainingHeaders < 1 {
		remainingHeaders = 1
	}

	allHeadersToFetch := headersFetchedSoFar + remainingHeaders

	asset.syncData.headersFetchProgress.TotalHeadersToFetch = asset.syncData.bestBlockheight
	asset.syncData.headersFetchProgress.HeadersFetchProgress = int32((headersFetchedSoFar * 100) / allHeadersToFetch)
	asset.syncData.headersFetchProgress.GeneralSyncProgress.TotalSyncProgress = asset.syncData.headersFetchProgress.HeadersFetchProgress
	asset.syncData.headersFetchProgress.GeneralSyncProgress.TotalTimeRemainingSeconds = int64((fetchTimeTakenSoFar * remainingHeaders) / headersFetchedSoFar)

	timeTakenSoFar := asset.syncData.cfiltersFetchProgress.CfiltersFetchTimeSpent + int64(fetchTimeTakenSoFar)
	debugInfo := &sharedW.DebugInfo{
		TotalTimeElapsed:          timeTakenSoFar,
		TotalTimeRemaining:        asset.syncData.headersFetchProgress.GeneralSyncProgress.TotalTimeRemainingSeconds,
		CurrentStageTimeElapsed:   int64(fetchTimeTakenSoFar),
		CurrentStageTimeRemaining: int64(fetchTimeTakenSoFar),
	}

	asset.syncData.mu.Unlock()

	// notify progress listener of estimated progress report
	asset.publishFetchHeadersProgress()
	asset.publishDebugInfo(debugInfo)
}

func (asset *DCRAsset) publishFetchHeadersProgress() {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.OnHeadersFetchProgress(&asset.syncData.headersFetchProgress)
	}
}

func (asset *DCRAsset) fetchHeadersFinished() {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if !asset.syncData.syncing {
		// ignore if sync is not in progress
		return
	}

	asset.syncData.headersFetchProgress.StartHeaderHeight = nil
	asset.syncData.headersFetchProgress.HeadersFetchTimeSpent = int64(time.Since(asset.syncData.headersFetchProgress.BeginFetchTimeStamp).Seconds())

	// If there is some period of inactivity reported at this stage,
	// subtract it from the total stage time.
	asset.syncData.headersFetchProgress.HeadersFetchTimeSpent -= asset.syncData.totalInactiveSeconds
	asset.syncData.totalInactiveSeconds = 0

	if asset.syncData.headersFetchProgress.HeadersFetchTimeSpent < 150 {
		// This ensures that minimum ETA used for stage 2 (address discovery) is 120 seconds (80% of 150 seconds).
		asset.syncData.headersFetchProgress.HeadersFetchTimeSpent = 150
	}

	if asset.syncData.showLogs && asset.syncData.syncing {
		log.Info("Fetch headers completed.")
	}
}

// Address/Account Discovery Callbacks

func (asset *DCRAsset) discoverAddressesStarted() {
	if !asset.IsSyncing() {
		return
	}

	asset.syncData.mu.RLock()
	addressDiscoveryAlreadyStarted := asset.syncData.addressDiscoveryProgress.AddressDiscoveryStartTime != -1
	totalHeadersFetchTime := float64(asset.syncData.headersFetchProgress.HeadersFetchTimeSpent)
	asset.syncData.mu.RUnlock()

	if addressDiscoveryAlreadyStarted {
		return
	}

	asset.syncData.mu.Lock()
	asset.syncData.syncStage = AddressDiscoverySyncStage
	asset.syncData.addressDiscoveryProgress.AddressDiscoveryStartTime = time.Now().Unix()
	asset.syncData.addressDiscoveryCompletedOrCanceled = make(chan bool)
	asset.syncData.mu.Unlock()

	go asset.updateAddressDiscoveryProgress(totalHeadersFetchTime)

	if asset.syncData.showLogs {
		log.Info("Step 2 of 3 - discovering used addresses.")
	}
}

func (asset *DCRAsset) updateAddressDiscoveryProgress(totalHeadersFetchTime float64) {
	// use ticker to calculate and broadcast address discovery progress every second
	everySecondTicker := time.NewTicker(1 * time.Second)

	// these values will be used every second to calculate the total sync progress
	estimatedDiscoveryTime := totalHeadersFetchTime * discoveryPercentage
	estimatedRescanTime := totalHeadersFetchTime * rescanPercentage

	// track last logged time remaining and total percent to avoid re-logging same message
	var lastTimeRemaining int64
	var lastTotalPercent int32 = -1

	for {
		if !asset.IsSyncing() {
			return
		}

		// If there was some period of inactivity,
		// assume that this process started at some point in the future,
		// thereby accounting for the total reported time of inactivity.
		asset.syncData.mu.Lock()
		asset.syncData.addressDiscoveryProgress.AddressDiscoveryStartTime += asset.syncData.totalInactiveSeconds
		asset.syncData.totalInactiveSeconds = 0
		addressDiscoveryStartTime := asset.syncData.addressDiscoveryProgress.AddressDiscoveryStartTime
		totalCfiltersFetchTime := float64(asset.syncData.cfiltersFetchProgress.CfiltersFetchTimeSpent)
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
			elapsedDiscoveryTime := float64(time.Now().Unix() - addressDiscoveryStartTime)
			discoveryProgress := (elapsedDiscoveryTime / estimatedDiscoveryTime) * 100

			var totalSyncTime float64
			if elapsedDiscoveryTime > estimatedDiscoveryTime {
				totalSyncTime = totalCfiltersFetchTime + totalHeadersFetchTime + elapsedDiscoveryTime + estimatedRescanTime
			} else {
				totalSyncTime = totalCfiltersFetchTime + totalHeadersFetchTime + estimatedDiscoveryTime + estimatedRescanTime
			}

			totalElapsedTime := totalCfiltersFetchTime + totalHeadersFetchTime + elapsedDiscoveryTime
			totalProgress := (totalElapsedTime / totalSyncTime) * 100

			remainingAccountDiscoveryTime := math.Round(estimatedDiscoveryTime - elapsedDiscoveryTime)
			if remainingAccountDiscoveryTime < 0 {
				remainingAccountDiscoveryTime = 0
			}

			totalProgressPercent := int32(math.Round(totalProgress))
			totalTimeRemainingSeconds := int64(math.Round(remainingAccountDiscoveryTime + estimatedRescanTime))

			// update address discovery progress, total progress and total time remaining
			asset.syncData.mu.Lock()
			asset.syncData.addressDiscoveryProgress.AddressDiscoveryProgress = int32(math.Round(discoveryProgress))
			asset.syncData.addressDiscoveryProgress.TotalSyncProgress = totalProgressPercent
			asset.syncData.addressDiscoveryProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds
			asset.syncData.mu.Unlock()

			asset.publishAddressDiscoveryProgress()

			debugInfo := &sharedW.DebugInfo{
				TotalTimeElapsed:          int64(math.Round(totalElapsedTime)),
				TotalTimeRemaining:        totalTimeRemainingSeconds,
				CurrentStageTimeElapsed:   int64(math.Round(elapsedDiscoveryTime)),
				CurrentStageTimeRemaining: int64(math.Round(remainingAccountDiscoveryTime)),
			}
			asset.publishDebugInfo(debugInfo)

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

func (asset *DCRAsset) publishAddressDiscoveryProgress() {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.OnAddressDiscoveryProgress(&asset.syncData.addressDiscoveryProgress)
	}
}

func (asset *DCRAsset) discoverAddressesFinished() {
	if !asset.IsSyncing() {
		return
	}

	asset.stopUpdatingAddressDiscoveryProgress()
}

func (asset *DCRAsset) stopUpdatingAddressDiscoveryProgress() {
	asset.syncData.mu.Lock()
	if asset.syncData != nil && asset.syncData.addressDiscoveryCompletedOrCanceled != nil {
		close(asset.syncData.addressDiscoveryCompletedOrCanceled)
		asset.syncData.addressDiscoveryCompletedOrCanceled = nil
		asset.syncData.addressDiscoveryProgress.TotalDiscoveryTimeSpent = time.Now().Unix() - asset.syncData.addressDiscoveryProgress.AddressDiscoveryStartTime
	}
	asset.syncData.mu.Unlock()
}

// Blocks Scan Callbacks

func (asset *DCRAsset) rescanStarted() {
	asset.stopUpdatingAddressDiscoveryProgress()

	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if !asset.syncData.syncing {
		// ignore if sync is not in progress
		return
	}

	asset.syncData.syncStage = HeadersRescanSyncStage
	asset.syncData.rescanStartTime = time.Now().Unix()

	// retain last total progress report from address discovery phase
	asset.syncData.headersRescanProgress.TotalTimeRemainingSeconds = asset.syncData.addressDiscoveryProgress.TotalTimeRemainingSeconds
	asset.syncData.headersRescanProgress.TotalSyncProgress = asset.syncData.addressDiscoveryProgress.TotalSyncProgress

	if asset.syncData.showLogs && asset.syncData.syncing {
		log.Info("Step 3 of 3 - Scanning block headers.")
	}
}

func (asset *DCRAsset) rescanProgress(rescannedThrough int32) {
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
	asset.syncData.rescanStartTime += asset.syncData.totalInactiveSeconds
	asset.syncData.totalInactiveSeconds = 0

	elapsedRescanTime := time.Now().Unix() - asset.syncData.rescanStartTime
	estimatedTotalRescanTime := int64(math.Round(float64(elapsedRescanTime) / rescanRate))
	totalTimeRemainingSeconds := estimatedTotalRescanTime - elapsedRescanTime
	totalElapsedTime := asset.syncData.cfiltersFetchProgress.CfiltersFetchTimeSpent + asset.syncData.headersFetchProgress.HeadersFetchTimeSpent +
		asset.syncData.addressDiscoveryProgress.TotalDiscoveryTimeSpent + elapsedRescanTime

	asset.syncData.headersRescanProgress.TotalHeadersToScan = totalHeadersToScan
	asset.syncData.headersRescanProgress.RescanProgress = int32(math.Round(rescanRate * 100))
	asset.syncData.headersRescanProgress.CurrentRescanHeight = rescannedThrough
	asset.syncData.headersRescanProgress.RescanTimeRemaining = totalTimeRemainingSeconds

	// do not update total time taken and total progress percent if elapsedRescanTime is 0
	// because the estimatedTotalRescanTime will be inaccurate (also 0)
	// which will make the estimatedTotalSyncTime equal to totalElapsedTime
	// giving the wrong impression that the process is complete
	if elapsedRescanTime > 0 {
		estimatedTotalSyncTime := asset.syncData.cfiltersFetchProgress.CfiltersFetchTimeSpent + asset.syncData.headersFetchProgress.HeadersFetchTimeSpent +
			asset.syncData.addressDiscoveryProgress.TotalDiscoveryTimeSpent + estimatedTotalRescanTime
		totalProgress := (float64(totalElapsedTime) / float64(estimatedTotalSyncTime)) * 100

		asset.syncData.headersRescanProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds
		asset.syncData.headersRescanProgress.TotalSyncProgress = int32(math.Round(totalProgress))
	}

	asset.syncData.mu.Unlock()

	asset.publishHeadersRescanProgress()

	debugInfo := &sharedW.DebugInfo{
		TotalTimeElapsed:          totalElapsedTime,
		TotalTimeRemaining:        totalTimeRemainingSeconds,
		CurrentStageTimeElapsed:   elapsedRescanTime,
		CurrentStageTimeRemaining: totalTimeRemainingSeconds,
	}
	asset.publishDebugInfo(debugInfo)

	asset.syncData.mu.RLock()
	if asset.syncData.showLogs {
		log.Infof("Syncing %d%%, %s remaining, scanning %d of %d block headers.",
			asset.syncData.headersRescanProgress.TotalSyncProgress,
			calculateTotalTimeRemaining(asset.syncData.headersRescanProgress.TotalTimeRemainingSeconds),
			asset.syncData.headersRescanProgress.CurrentRescanHeight,
			asset.syncData.headersRescanProgress.TotalHeadersToScan,
		)
	}
	asset.syncData.mu.RUnlock()
}

func (asset *DCRAsset) publishHeadersRescanProgress() {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.OnHeadersRescanProgress(&asset.syncData.headersRescanProgress)
	}
}

func (asset *DCRAsset) rescanFinished() {
	if !asset.IsSyncing() {
		// ignore if sync is not in progress
		return
	}

	asset.syncData.mu.Lock()
	asset.syncData.headersRescanProgress.TotalTimeRemainingSeconds = 0
	asset.syncData.headersRescanProgress.TotalSyncProgress = 100

	// Reset these value so that address discovery would
	// not be skipped for the next sharedW.
	asset.syncData.addressDiscoveryProgress.AddressDiscoveryStartTime = -1
	asset.syncData.addressDiscoveryProgress.TotalDiscoveryTimeSpent = -1
	asset.syncData.mu.Unlock()

	asset.publishHeadersRescanProgress()
}

func (asset *DCRAsset) publishDebugInfo(debugInfo *sharedW.DebugInfo) {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.Debug(debugInfo)
	}
}

/** Helper functions start here */

func (asset *DCRAsset) estimateBlockHeadersCountAfter(lastHeaderTime int64) int32 {
	// Use the difference between current time (now) and last reported block time,
	// to estimate total headers to fetch.
	timeDifferenceInSeconds := float64(time.Now().Unix() - lastHeaderTime)
	targetTimePerBlockInSeconds := asset.chainParams.TargetTimePerBlock.Seconds()
	estimatedHeadersDifference := timeDifferenceInSeconds / targetTimePerBlockInSeconds

	// return next integer value (upper limit) if estimatedHeadersDifference is a fraction
	return int32(math.Ceil(estimatedHeadersDifference))
}

func (asset *DCRAsset) notifySyncError(err error) {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.OnSyncEndedWithError(err)
	}
}

func (asset *DCRAsset) notifySyncCanceled() {
	asset.syncData.mu.RLock()
	restartSyncRequested := asset.syncData.restartSyncRequested
	asset.syncData.mu.RUnlock()

	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.OnSyncCanceled(restartSyncRequested)
	}
}

func (asset *DCRAsset) resetSyncData() {
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

func (asset *DCRAsset) syncedWallet(synced bool) {
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
					syncProgressListener.OnSyncCompleted()
				} else {
					syncProgressListener.OnSyncCanceled(false)
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
