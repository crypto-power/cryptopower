package dcr

import (
	"math"
	"time"

	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/spv"
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

func (asset *DCRAsset) fetchCFiltersStarted(walletID int) {
	asset.syncData.mu.Lock()
	asset.syncData.activeSyncData.syncStage = CFiltersFetchSyncStage
	asset.syncData.activeSyncData.cfiltersFetchProgress.BeginFetchCFiltersTimeStamp = time.Now().Unix()
	asset.syncData.activeSyncData.cfiltersFetchProgress.TotalFetchedCFiltersCount = 0
	showLogs := asset.syncData.showLogs
	asset.syncData.mu.Unlock()

	if showLogs {
		log.Infof("Step 1 of 3 - fetching %d block headers.")
	}
}

func (asset *DCRAsset) fetchCFiltersProgress(walletID int, startCFiltersHeight, endCFiltersHeight int32) {

	// lock the mutex before reading and writing to asset.syncData.*
	asset.syncData.mu.Lock()

	if asset.syncData.activeSyncData.cfiltersFetchProgress.StartCFiltersHeight == -1 {
		asset.syncData.activeSyncData.cfiltersFetchProgress.StartCFiltersHeight = startCFiltersHeight
	}

	asset.syncData.activeSyncData.cfiltersFetchProgress.TotalFetchedCFiltersCount += endCFiltersHeight - startCFiltersHeight

	totalCFiltersToFetch := asset.GetBestBlockHeight() - asset.syncData.activeSyncData.cfiltersFetchProgress.StartCFiltersHeight

	cfiltersFetchProgress := float64(asset.syncData.activeSyncData.cfiltersFetchProgress.TotalFetchedCFiltersCount) / float64(totalCFiltersToFetch)

	// If there was some period of inactivity,
	// assume that this process started at some point in the future,
	// thereby accounting for the total reported time of inactivity.
	asset.syncData.activeSyncData.cfiltersFetchProgress.BeginFetchCFiltersTimeStamp += asset.syncData.activeSyncData.totalInactiveSeconds
	asset.syncData.activeSyncData.totalInactiveSeconds = 0

	timeTakenSoFar := time.Now().Unix() - asset.syncData.activeSyncData.cfiltersFetchProgress.BeginFetchCFiltersTimeStamp
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
	asset.syncData.activeSyncData.cfiltersFetchProgress.TotalCFiltersToFetch = totalCFiltersToFetch
	asset.syncData.activeSyncData.cfiltersFetchProgress.CurrentCFilterHeight = startCFiltersHeight
	asset.syncData.activeSyncData.cfiltersFetchProgress.CFiltersFetchProgress = roundUp(cfiltersFetchProgress * 100.0)
	asset.syncData.activeSyncData.cfiltersFetchProgress.TotalSyncProgress = roundUp(totalSyncProgress * 100.0)
	asset.syncData.activeSyncData.cfiltersFetchProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds

	asset.syncData.mu.Unlock()

	// notify progress listener of estimated progress report
	asset.publishFetchCFiltersProgress()

	cfiltersFetchTimeRemaining := estimatedTotalCFiltersFetchTime - float64(timeTakenSoFar)
	debugInfo := &sharedW.DebugInfo{
		timeTakenSoFar,
		totalTimeRemainingSeconds,
		timeTakenSoFar,
		int64(math.Round(cfiltersFetchTimeRemaining)),
	}
	asset.publishDebugInfo(debugInfo)
}

func (asset *DCRAsset) publishFetchCFiltersProgress() {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.OnCFiltersFetchProgress(&asset.syncData.cfiltersFetchProgress)
	}
}

func (asset *DCRAsset) fetchCFiltersEnded(walletID int) {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	asset.syncData.activeSyncData.cfiltersFetchProgress.CfiltersFetchTimeSpent = time.Now().Unix() - asset.syncData.cfiltersFetchProgress.BeginFetchCFiltersTimeStamp

	// If there is some period of inactivity reported at this stage,
	// subtract it from the total stage time.
	asset.syncData.activeSyncData.cfiltersFetchProgress.CfiltersFetchTimeSpent -= asset.syncData.totalInactiveSeconds
	asset.syncData.activeSyncData.totalInactiveSeconds = 0
}

// Fetch Headers Callbacks

func (asset *DCRAsset) fetchHeadersStarted(peerInitialHeight int32) {
	if !asset.IsSyncing() {
		return
	}

	asset.syncData.mu.RLock()
	headersFetchingStarted := asset.syncData.headersFetchProgress.BeginFetchTimeStamp != -1
	showLogs := asset.syncData.showLogs
	asset.syncData.mu.RUnlock()

	if headersFetchingStarted {
		// This function gets called for each newly connected peer so
		// ignore if headers fetching was already started.
		return
	}

	asset.waitingForHeaders = true

	lowestBlockHeight := asset.GetLowestBlock().Height

	asset.syncData.mu.Lock()
	asset.syncData.activeSyncData.syncStage = HeadersFetchSyncStage
	asset.syncData.activeSyncData.headersFetchProgress.BeginFetchTimeStamp = time.Now().Unix()
	asset.syncData.activeSyncData.headersFetchProgress.StartHeaderHeight = lowestBlockHeight
	asset.syncData.headersFetchProgress.TotalFetchedHeadersCount = 0
	asset.syncData.activeSyncData.totalInactiveSeconds = 0
	asset.syncData.mu.Unlock()

	if showLogs {
		log.Infof("Step 1 of 3 - fetching %d block headers.", peerInitialHeight-lowestBlockHeight)
	}
}

func (asset *DCRAsset) fetchHeadersProgress(lastFetchedHeaderHeight int32, lastFetchedHeaderTime int64) {
	if !asset.IsSyncing() {
		return
	}

	asset.syncData.mu.RLock()
	headersFetchingCompleted := asset.syncData.activeSyncData.headersFetchProgress.HeadersFetchTimeSpent != -1
	asset.syncData.mu.RUnlock()

	if headersFetchingCompleted {
		// This function gets called for each newly connected peer so ignore
		// this call if the headers fetching phase was previously completed.
		return
	}

	if asset.waitingForHeaders {
		asset.waitingForHeaders = asset.GetBestBlockHeight() > lastFetchedHeaderHeight
	}

	// lock the mutex before reading and writing to asset.syncData.*
	asset.syncData.mu.Lock()

	if lastFetchedHeaderHeight > asset.syncData.activeSyncData.headersFetchProgress.StartHeaderHeight {
		asset.syncData.activeSyncData.headersFetchProgress.TotalFetchedHeadersCount = lastFetchedHeaderHeight - asset.syncData.activeSyncData.headersFetchProgress.StartHeaderHeight
	}

	headersLeftToFetch := asset.estimateBlockHeadersCountAfter(lastFetchedHeaderTime)
	totalHeadersToFetch := lastFetchedHeaderHeight + headersLeftToFetch
	headersFetchProgress := float64(asset.syncData.activeSyncData.headersFetchProgress.TotalFetchedHeadersCount) / float64(totalHeadersToFetch)

	// If there was some period of inactivity,
	// assume that this process started at some point in the future,
	// thereby accounting for the total reported time of inactivity.
	asset.syncData.activeSyncData.headersFetchProgress.BeginFetchTimeStamp += asset.syncData.activeSyncData.totalInactiveSeconds
	asset.syncData.activeSyncData.totalInactiveSeconds = 0

	fetchTimeTakenSoFar := time.Now().Unix() - asset.syncData.activeSyncData.headersFetchProgress.BeginFetchTimeStamp
	if fetchTimeTakenSoFar < 1 {
		fetchTimeTakenSoFar = 1
	}
	estimatedTotalHeadersFetchTime := float64(fetchTimeTakenSoFar) / headersFetchProgress

	// For some reason, the actual total headers fetch time is more than the predicted/estimated time.
	// Account for this difference by multiplying the estimatedTotalHeadersFetchTime by an incrementing factor.
	// The incrementing factor is inversely proportional to the headers fetch progress,
	// ranging from 0.5 to 0 as headers fetching progress increases from 0 to 1.
	// todo, the above noted (mal)calculation may explain this difference.
	// TODO: is this adjustment still needed since the calculation has been corrected.
	adjustmentFactor := 0.5 * (1 - headersFetchProgress)
	estimatedTotalHeadersFetchTime += estimatedTotalHeadersFetchTime * adjustmentFactor

	estimatedDiscoveryTime := estimatedTotalHeadersFetchTime * discoveryPercentage
	estimatedRescanTime := estimatedTotalHeadersFetchTime * rescanPercentage
	estimatedTotalSyncTime := float64(asset.syncData.activeSyncData.cfiltersFetchProgress.CfiltersFetchTimeSpent) +
		estimatedTotalHeadersFetchTime + estimatedDiscoveryTime + estimatedRescanTime

	totalSyncProgress := float64(fetchTimeTakenSoFar) / estimatedTotalSyncTime
	totalTimeRemainingSeconds := int64(math.Round(estimatedTotalSyncTime)) - fetchTimeTakenSoFar

	// update headers fetching progress report including total progress percentage and total time remaining
	asset.syncData.activeSyncData.headersFetchProgress.TotalHeadersToFetch = totalHeadersToFetch
	asset.syncData.activeSyncData.headersFetchProgress.CurrentHeaderHeight = lastFetchedHeaderHeight
	asset.syncData.activeSyncData.headersFetchProgress.CurrentHeaderTimestamp = lastFetchedHeaderTime
	asset.syncData.activeSyncData.headersFetchProgress.HeadersFetchProgress = roundUp(headersFetchProgress * 100.0)
	asset.syncData.activeSyncData.headersFetchProgress.TotalSyncProgress = roundUp(totalSyncProgress * 100.0)
	asset.syncData.activeSyncData.headersFetchProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds

	// unlock the mutex before issuing notification callbacks to prevent potential deadlock
	// if any invoked callback takes a considerable amount of time to execute.
	asset.syncData.mu.Unlock()

	// notify progress listener of estimated progress report
	asset.publishFetchHeadersProgress()

	// todo: also log report if showLog == true
	timeTakenSoFar := asset.syncData.activeSyncData.cfiltersFetchProgress.CfiltersFetchTimeSpent + fetchTimeTakenSoFar
	headersFetchTimeRemaining := estimatedTotalHeadersFetchTime - float64(fetchTimeTakenSoFar)
	debugInfo := &sharedW.DebugInfo{
		timeTakenSoFar,
		totalTimeRemainingSeconds,
		fetchTimeTakenSoFar,
		int64(math.Round(headersFetchTimeRemaining)),
	}
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

	asset.syncData.activeSyncData.headersFetchProgress.StartHeaderHeight = -1
	asset.syncData.headersFetchProgress.TotalFetchedHeadersCount = 0
	asset.syncData.activeSyncData.headersFetchProgress.HeadersFetchTimeSpent = time.Now().Unix() - asset.syncData.headersFetchProgress.BeginFetchTimeStamp

	// If there is some period of inactivity reported at this stage,
	// subtract it from the total stage time.
	asset.syncData.activeSyncData.headersFetchProgress.HeadersFetchTimeSpent -= asset.syncData.totalInactiveSeconds
	asset.syncData.activeSyncData.totalInactiveSeconds = 0

	if asset.syncData.activeSyncData.headersFetchProgress.HeadersFetchTimeSpent < 150 {
		// This ensures that minimum ETA used for stage 2 (address discovery) is 120 seconds (80% of 150 seconds).
		asset.syncData.activeSyncData.headersFetchProgress.HeadersFetchTimeSpent = 150
	}

	if asset.syncData.showLogs && asset.syncData.syncing {
		log.Info("Fetch headers completed.")
	}
}

// Address/Account Discovery Callbacks

func (asset *DCRAsset) discoverAddressesStarted(walletID int) {
	if !asset.IsSyncing() {
		return
	}

	asset.syncData.mu.RLock()
	addressDiscoveryAlreadyStarted := asset.syncData.activeSyncData.addressDiscoveryProgress.AddressDiscoveryStartTime != -1
	totalHeadersFetchTime := float64(asset.syncData.activeSyncData.headersFetchProgress.HeadersFetchTimeSpent)
	asset.syncData.mu.RUnlock()

	if addressDiscoveryAlreadyStarted {
		return
	}

	asset.syncData.mu.Lock()
	asset.syncData.activeSyncData.syncStage = AddressDiscoverySyncStage
	asset.syncData.activeSyncData.addressDiscoveryProgress.AddressDiscoveryStartTime = time.Now().Unix()
	asset.syncData.activeSyncData.addressDiscoveryProgress.WalletID = walletID
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
				int64(math.Round(totalElapsedTime)),
				totalTimeRemainingSeconds,
				int64(math.Round(elapsedDiscoveryTime)),
				int64(math.Round(remainingAccountDiscoveryTime)),
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
		syncProgressListener.OnAddressDiscoveryProgress(&asset.syncData.activeSyncData.addressDiscoveryProgress)
	}
}

func (asset *DCRAsset) discoverAddressesFinished(walletID int) {
	if !asset.IsSyncing() {
		return
	}

	asset.stopUpdatingAddressDiscoveryProgress()
}

func (asset *DCRAsset) stopUpdatingAddressDiscoveryProgress() {
	asset.syncData.mu.Lock()
	if asset.syncData.activeSyncData != nil && asset.syncData.activeSyncData.addressDiscoveryCompletedOrCanceled != nil {
		close(asset.syncData.activeSyncData.addressDiscoveryCompletedOrCanceled)
		asset.syncData.activeSyncData.addressDiscoveryCompletedOrCanceled = nil
		asset.syncData.activeSyncData.addressDiscoveryProgress.TotalDiscoveryTimeSpent = time.Now().Unix() - asset.syncData.addressDiscoveryProgress.AddressDiscoveryStartTime
	}
	asset.syncData.mu.Unlock()
}

// Blocks Scan Callbacks

func (asset *DCRAsset) rescanStarted(walletID int) {
	asset.stopUpdatingAddressDiscoveryProgress()

	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if !asset.syncData.syncing {
		// ignore if sync is not in progress
		return
	}

	asset.syncData.activeSyncData.syncStage = HeadersRescanSyncStage
	asset.syncData.activeSyncData.rescanStartTime = time.Now().Unix()

	// retain last total progress report from address discovery phase
	asset.syncData.activeSyncData.headersRescanProgress.TotalTimeRemainingSeconds = asset.syncData.activeSyncData.addressDiscoveryProgress.TotalTimeRemainingSeconds
	asset.syncData.activeSyncData.headersRescanProgress.TotalSyncProgress = asset.syncData.activeSyncData.addressDiscoveryProgress.TotalSyncProgress
	asset.syncData.activeSyncData.headersRescanProgress.WalletID = walletID

	if asset.syncData.showLogs && asset.syncData.syncing {
		log.Info("Step 3 of 3 - Scanning block headers.")
	}
}

func (asset *DCRAsset) rescanProgress(walletID int, rescannedThrough int32) {
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
	asset.syncData.activeSyncData.rescanStartTime += asset.syncData.activeSyncData.totalInactiveSeconds
	asset.syncData.activeSyncData.totalInactiveSeconds = 0

	elapsedRescanTime := time.Now().Unix() - asset.syncData.activeSyncData.rescanStartTime
	estimatedTotalRescanTime := int64(math.Round(float64(elapsedRescanTime) / rescanRate))
	totalTimeRemainingSeconds := estimatedTotalRescanTime - elapsedRescanTime
	totalElapsedTime := asset.syncData.activeSyncData.cfiltersFetchProgress.CfiltersFetchTimeSpent + asset.syncData.activeSyncData.headersFetchProgress.HeadersFetchTimeSpent +
		asset.syncData.activeSyncData.addressDiscoveryProgress.TotalDiscoveryTimeSpent + elapsedRescanTime

	asset.syncData.activeSyncData.headersRescanProgress.WalletID = walletID
	asset.syncData.activeSyncData.headersRescanProgress.TotalHeadersToScan = totalHeadersToScan
	asset.syncData.activeSyncData.headersRescanProgress.RescanProgress = int32(math.Round(rescanRate * 100))
	asset.syncData.activeSyncData.headersRescanProgress.CurrentRescanHeight = rescannedThrough
	asset.syncData.activeSyncData.headersRescanProgress.RescanTimeRemaining = totalTimeRemainingSeconds

	// do not update total time taken and total progress percent if elapsedRescanTime is 0
	// because the estimatedTotalRescanTime will be inaccurate (also 0)
	// which will make the estimatedTotalSyncTime equal to totalElapsedTime
	// giving the wrong impression that the process is complete
	if elapsedRescanTime > 0 {
		estimatedTotalSyncTime := asset.syncData.activeSyncData.cfiltersFetchProgress.CfiltersFetchTimeSpent + asset.syncData.activeSyncData.headersFetchProgress.HeadersFetchTimeSpent +
			asset.syncData.activeSyncData.addressDiscoveryProgress.TotalDiscoveryTimeSpent + estimatedTotalRescanTime
		totalProgress := (float64(totalElapsedTime) / float64(estimatedTotalSyncTime)) * 100

		asset.syncData.activeSyncData.headersRescanProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds
		asset.syncData.activeSyncData.headersRescanProgress.TotalSyncProgress = int32(math.Round(totalProgress))
	}

	asset.syncData.mu.Unlock()

	asset.publishHeadersRescanProgress()

	debugInfo := &sharedW.DebugInfo{
		totalElapsedTime,
		totalTimeRemainingSeconds,
		elapsedRescanTime,
		totalTimeRemainingSeconds,
	}
	asset.publishDebugInfo(debugInfo)

	asset.syncData.mu.RLock()
	if asset.syncData.showLogs {
		log.Infof("Syncing %d%%, %s remaining, scanning %d of %d block headers.",
			asset.syncData.activeSyncData.headersRescanProgress.TotalSyncProgress,
			calculateTotalTimeRemaining(asset.syncData.activeSyncData.headersRescanProgress.TotalTimeRemainingSeconds),
			asset.syncData.activeSyncData.headersRescanProgress.CurrentRescanHeight,
			asset.syncData.activeSyncData.headersRescanProgress.TotalHeadersToScan,
		)
	}
	asset.syncData.mu.RUnlock()
}

func (asset *DCRAsset) publishHeadersRescanProgress() {
	for _, syncProgressListener := range asset.syncProgressListeners() {
		syncProgressListener.OnHeadersRescanProgress(&asset.syncData.activeSyncData.headersRescanProgress)
	}
}

func (asset *DCRAsset) rescanFinished(walletID int) {
	if !asset.IsSyncing() {
		// ignore if sync is not in progress
		return
	}

	asset.syncData.mu.Lock()
	asset.syncData.activeSyncData.headersRescanProgress.WalletID = walletID
	asset.syncData.activeSyncData.headersRescanProgress.TotalTimeRemainingSeconds = 0
	asset.syncData.activeSyncData.headersRescanProgress.TotalSyncProgress = 100

	// Reset these value so that address discovery would
	// not be skipped for the next sharedW.
	asset.syncData.activeSyncData.addressDiscoveryProgress.AddressDiscoveryStartTime = -1
	asset.syncData.activeSyncData.addressDiscoveryProgress.TotalDiscoveryTimeSpent = -1
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

func (asset *DCRAsset) syncedWallet(walletID int, synced bool) {

	indexTransactions := func() {
		// begin indexing transactions after sync is completed,
		// syncProgressListeners.OnSynced() will be invoked after transactions are indexed
		var txIndexing errgroup.Group
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
