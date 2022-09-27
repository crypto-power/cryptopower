package dcr

import (
	"math"
	"time"

	"gitlab.com/raedah/cryptopower/libwallet/spv"
	"golang.org/x/sync/errgroup"
)

func (w *Wallet) spvSyncNotificationCallbacks() *spv.Notifications {
	return &spv.Notifications{
		PeerConnected: func(peerCount int32, addr string) {
			w.handlePeerCountUpdate(peerCount)
		},
		PeerDisconnected: func(peerCount int32, addr string) {
			w.handlePeerCountUpdate(peerCount)
		},
		Synced:                       w.synced,
		FetchHeadersStarted:          w.fetchHeadersStarted,
		FetchHeadersProgress:         w.fetchHeadersProgress,
		FetchHeadersFinished:         w.fetchHeadersFinished,
		FetchMissingCFiltersStarted:  w.fetchCFiltersStarted,
		FetchMissingCFiltersProgress: w.fetchCFiltersProgress,
		FetchMissingCFiltersFinished: w.fetchCFiltersEnded,
		DiscoverAddressesStarted:     w.discoverAddressesStarted,
		DiscoverAddressesFinished:    w.discoverAddressesFinished,
		RescanStarted:                w.rescanStarted,
		RescanProgress:               w.rescanProgress,
		RescanFinished:               w.rescanFinished,
	}
}

func (w *Wallet) handlePeerCountUpdate(peerCount int32) {
	w.syncData.mu.Lock()
	w.syncData.connectedPeers = peerCount
	shouldLog := w.syncData.showLogs && w.syncData.syncing
	w.syncData.mu.Unlock()

	for _, syncProgressListener := range w.syncProgressListeners() {
		syncProgressListener.OnPeerConnectedOrDisconnected(peerCount)
	}

	if shouldLog {
		if peerCount == 1 {
			log.Infof("Connected to %d peer on %s.", peerCount, w.chainParams.Name)
		} else {
			log.Infof("Connected to %d peers on %s.", peerCount, w.chainParams.Name)
		}
	}
}

// Fetch CFilters Callbacks

func (w *Wallet) fetchCFiltersStarted(walletID int) {
	w.syncData.mu.Lock()
	w.syncData.activeSyncData.syncStage = CFiltersFetchSyncStage
	w.syncData.activeSyncData.cfiltersFetchProgress.beginFetchCFiltersTimeStamp = time.Now().Unix()
	w.syncData.activeSyncData.cfiltersFetchProgress.totalFetchedCFiltersCount = 0
	showLogs := w.syncData.showLogs
	w.syncData.mu.Unlock()

	if showLogs {
		log.Infof("Step 1 of 3 - fetching %d block headers.")
	}
}

func (w *Wallet) fetchCFiltersProgress(walletID int, startCFiltersHeight, endCFiltersHeight int32) {

	// lock the mutex before reading and writing to w.syncData.*
	w.syncData.mu.Lock()

	if w.syncData.activeSyncData.cfiltersFetchProgress.startCFiltersHeight == -1 {
		w.syncData.activeSyncData.cfiltersFetchProgress.startCFiltersHeight = startCFiltersHeight
	}

	// wallet := w.DCRWalletWithID(walletID)
	w.syncData.activeSyncData.cfiltersFetchProgress.totalFetchedCFiltersCount += endCFiltersHeight - startCFiltersHeight

	totalCFiltersToFetch := w.GetBestBlockInt() - w.syncData.activeSyncData.cfiltersFetchProgress.startCFiltersHeight
	// cfiltersLeftToFetch := totalCFiltersToFetch - w.syncData.activeSyncData.cfiltersFetchProgress.totalFetchedCFiltersCount

	cfiltersFetchProgress := float64(w.syncData.activeSyncData.cfiltersFetchProgress.totalFetchedCFiltersCount) / float64(totalCFiltersToFetch)

	// If there was some period of inactivity,
	// assume that this process started at some point in the future,
	// thereby accounting for the total reported time of inactivity.
	w.syncData.activeSyncData.cfiltersFetchProgress.beginFetchCFiltersTimeStamp += w.syncData.activeSyncData.totalInactiveSeconds
	w.syncData.activeSyncData.totalInactiveSeconds = 0

	timeTakenSoFar := time.Now().Unix() - w.syncData.activeSyncData.cfiltersFetchProgress.beginFetchCFiltersTimeStamp
	if timeTakenSoFar < 1 {
		timeTakenSoFar = 1
	}
	estimatedTotalCFiltersFetchTime := float64(timeTakenSoFar) / cfiltersFetchProgress

	// Use CFilters fetch rate to estimate headers fetch time.
	cfiltersFetchRate := float64(w.syncData.activeSyncData.cfiltersFetchProgress.totalFetchedCFiltersCount) / float64(timeTakenSoFar)
	estimatedHeadersLeftToFetch := w.estimateBlockHeadersCountAfter(w.GetBestBlockTimeStamp())
	estimatedTotalHeadersFetchTime := float64(estimatedHeadersLeftToFetch) / cfiltersFetchRate
	// increase estimated value by FetchPercentage
	estimatedTotalHeadersFetchTime /= FetchPercentage

	estimatedDiscoveryTime := estimatedTotalHeadersFetchTime * DiscoveryPercentage
	estimatedRescanTime := estimatedTotalHeadersFetchTime * RescanPercentage
	estimatedTotalSyncTime := estimatedTotalCFiltersFetchTime + estimatedTotalHeadersFetchTime + estimatedDiscoveryTime + estimatedRescanTime

	totalSyncProgress := float64(timeTakenSoFar) / estimatedTotalSyncTime
	totalTimeRemainingSeconds := int64(math.Round(estimatedTotalSyncTime)) - timeTakenSoFar

	// update headers fetching progress report including total progress percentage and total time remaining
	w.syncData.activeSyncData.cfiltersFetchProgress.TotalCFiltersToFetch = totalCFiltersToFetch
	w.syncData.activeSyncData.cfiltersFetchProgress.CurrentCFilterHeight = startCFiltersHeight
	w.syncData.activeSyncData.cfiltersFetchProgress.CFiltersFetchProgress = roundUp(cfiltersFetchProgress * 100.0)
	w.syncData.activeSyncData.cfiltersFetchProgress.TotalSyncProgress = roundUp(totalSyncProgress * 100.0)
	w.syncData.activeSyncData.cfiltersFetchProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds

	w.syncData.mu.Unlock()

	// notify progress listener of estimated progress report
	w.publishFetchCFiltersProgress()

	cfiltersFetchTimeRemaining := estimatedTotalCFiltersFetchTime - float64(timeTakenSoFar)
	debugInfo := &DebugInfo{
		timeTakenSoFar,
		totalTimeRemainingSeconds,
		timeTakenSoFar,
		int64(math.Round(cfiltersFetchTimeRemaining)),
	}
	w.publishDebugInfo(debugInfo)
}

func (w *Wallet) publishFetchCFiltersProgress() {
	for _, syncProgressListener := range w.syncProgressListeners() {
		syncProgressListener.OnCFiltersFetchProgress(&w.syncData.cfiltersFetchProgress)
	}
}

func (w *Wallet) fetchCFiltersEnded(walletID int) {
	w.syncData.mu.Lock()
	defer w.syncData.mu.Unlock()

	w.syncData.activeSyncData.cfiltersFetchProgress.cfiltersFetchTimeSpent = time.Now().Unix() - w.syncData.cfiltersFetchProgress.beginFetchCFiltersTimeStamp

	// If there is some period of inactivity reported at this stage,
	// subtract it from the total stage time.
	w.syncData.activeSyncData.cfiltersFetchProgress.cfiltersFetchTimeSpent -= w.syncData.totalInactiveSeconds
	w.syncData.activeSyncData.totalInactiveSeconds = 0
}

// Fetch Headers Callbacks

func (w *Wallet) fetchHeadersStarted(peerInitialHeight int32) {
	if !w.IsSyncing() {
		return
	}

	w.syncData.mu.RLock()
	headersFetchingStarted := w.syncData.headersFetchProgress.beginFetchTimeStamp != -1
	showLogs := w.syncData.showLogs
	w.syncData.mu.RUnlock()

	if headersFetchingStarted {
		// This function gets called for each newly connected peer so
		// ignore if headers fetching was already started.
		return
	}

	w.WaitingForHeaders = true

	lowestBlockHeight := w.GetLowestBlock().Height

	w.syncData.mu.Lock()
	w.syncData.activeSyncData.syncStage = HeadersFetchSyncStage
	w.syncData.activeSyncData.headersFetchProgress.beginFetchTimeStamp = time.Now().Unix()
	w.syncData.activeSyncData.headersFetchProgress.startHeaderHeight = lowestBlockHeight
	w.syncData.headersFetchProgress.totalFetchedHeadersCount = 0
	w.syncData.activeSyncData.totalInactiveSeconds = 0
	w.syncData.mu.Unlock()

	if showLogs {
		log.Infof("Step 1 of 3 - fetching %d block headers.", peerInitialHeight-lowestBlockHeight)
	}
}

func (w *Wallet) fetchHeadersProgress(lastFetchedHeaderHeight int32, lastFetchedHeaderTime int64) {
	if !w.IsSyncing() {
		return
	}

	w.syncData.mu.RLock()
	headersFetchingCompleted := w.syncData.activeSyncData.headersFetchProgress.headersFetchTimeSpent != -1
	w.syncData.mu.RUnlock()

	if headersFetchingCompleted {
		// This function gets called for each newly connected peer so ignore
		// this call if the headers fetching phase was previously completed.
		return
	}

	// for _, wallet := range w.wallets {
	if w.WaitingForHeaders {
		w.WaitingForHeaders = w.GetBestBlockInt() > lastFetchedHeaderHeight
	}
	// }

	// lock the mutex before reading and writing to w.syncData.*
	w.syncData.mu.Lock()

	if lastFetchedHeaderHeight > w.syncData.activeSyncData.headersFetchProgress.startHeaderHeight {
		w.syncData.activeSyncData.headersFetchProgress.totalFetchedHeadersCount = lastFetchedHeaderHeight - w.syncData.activeSyncData.headersFetchProgress.startHeaderHeight
	}

	headersLeftToFetch := w.estimateBlockHeadersCountAfter(lastFetchedHeaderTime)
	totalHeadersToFetch := lastFetchedHeaderHeight + headersLeftToFetch
	headersFetchProgress := float64(w.syncData.activeSyncData.headersFetchProgress.totalFetchedHeadersCount) / float64(totalHeadersToFetch)

	// If there was some period of inactivity,
	// assume that this process started at some point in the future,
	// thereby accounting for the total reported time of inactivity.
	w.syncData.activeSyncData.headersFetchProgress.beginFetchTimeStamp += w.syncData.activeSyncData.totalInactiveSeconds
	w.syncData.activeSyncData.totalInactiveSeconds = 0

	fetchTimeTakenSoFar := time.Now().Unix() - w.syncData.activeSyncData.headersFetchProgress.beginFetchTimeStamp
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

	estimatedDiscoveryTime := estimatedTotalHeadersFetchTime * DiscoveryPercentage
	estimatedRescanTime := estimatedTotalHeadersFetchTime * RescanPercentage
	estimatedTotalSyncTime := float64(w.syncData.activeSyncData.cfiltersFetchProgress.cfiltersFetchTimeSpent) +
		estimatedTotalHeadersFetchTime + estimatedDiscoveryTime + estimatedRescanTime

	totalSyncProgress := float64(fetchTimeTakenSoFar) / estimatedTotalSyncTime
	totalTimeRemainingSeconds := int64(math.Round(estimatedTotalSyncTime)) - fetchTimeTakenSoFar

	// update headers fetching progress report including total progress percentage and total time remaining
	w.syncData.activeSyncData.headersFetchProgress.TotalHeadersToFetch = totalHeadersToFetch
	w.syncData.activeSyncData.headersFetchProgress.CurrentHeaderHeight = lastFetchedHeaderHeight
	w.syncData.activeSyncData.headersFetchProgress.CurrentHeaderTimestamp = lastFetchedHeaderTime
	w.syncData.activeSyncData.headersFetchProgress.HeadersFetchProgress = roundUp(headersFetchProgress * 100.0)
	w.syncData.activeSyncData.headersFetchProgress.TotalSyncProgress = roundUp(totalSyncProgress * 100.0)
	w.syncData.activeSyncData.headersFetchProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds

	// unlock the mutex before issuing notification callbacks to prevent potential deadlock
	// if any invoked callback takes a considerable amount of time to execute.
	w.syncData.mu.Unlock()

	// notify progress listener of estimated progress report
	w.publishFetchHeadersProgress()

	// todo: also log report if showLog == true
	timeTakenSoFar := w.syncData.activeSyncData.cfiltersFetchProgress.cfiltersFetchTimeSpent + fetchTimeTakenSoFar
	headersFetchTimeRemaining := estimatedTotalHeadersFetchTime - float64(fetchTimeTakenSoFar)
	debugInfo := &DebugInfo{
		timeTakenSoFar,
		totalTimeRemainingSeconds,
		fetchTimeTakenSoFar,
		int64(math.Round(headersFetchTimeRemaining)),
	}
	w.publishDebugInfo(debugInfo)
}

func (w *Wallet) publishFetchHeadersProgress() {
	for _, syncProgressListener := range w.syncProgressListeners() {
		syncProgressListener.OnHeadersFetchProgress(&w.syncData.headersFetchProgress)
	}
}

func (w *Wallet) fetchHeadersFinished() {
	w.syncData.mu.Lock()
	defer w.syncData.mu.Unlock()

	if !w.syncData.syncing {
		// ignore if sync is not in progress
		return
	}

	w.syncData.activeSyncData.headersFetchProgress.startHeaderHeight = -1
	w.syncData.headersFetchProgress.totalFetchedHeadersCount = 0
	w.syncData.activeSyncData.headersFetchProgress.headersFetchTimeSpent = time.Now().Unix() - w.syncData.headersFetchProgress.beginFetchTimeStamp

	// If there is some period of inactivity reported at this stage,
	// subtract it from the total stage time.
	w.syncData.activeSyncData.headersFetchProgress.headersFetchTimeSpent -= w.syncData.totalInactiveSeconds
	w.syncData.activeSyncData.totalInactiveSeconds = 0

	if w.syncData.activeSyncData.headersFetchProgress.headersFetchTimeSpent < 150 {
		// This ensures that minimum ETA used for stage 2 (address discovery) is 120 seconds (80% of 150 seconds).
		w.syncData.activeSyncData.headersFetchProgress.headersFetchTimeSpent = 150
	}

	if w.syncData.showLogs && w.syncData.syncing {
		log.Info("Fetch headers completed.")
	}
}

// Address/Account Discovery Callbacks

func (w *Wallet) discoverAddressesStarted(walletID int) {
	if !w.IsSyncing() {
		return
	}

	w.syncData.mu.RLock()
	addressDiscoveryAlreadyStarted := w.syncData.activeSyncData.addressDiscoveryProgress.addressDiscoveryStartTime != -1
	totalHeadersFetchTime := float64(w.syncData.activeSyncData.headersFetchProgress.headersFetchTimeSpent)
	w.syncData.mu.RUnlock()

	if addressDiscoveryAlreadyStarted {
		return
	}

	w.syncData.mu.Lock()
	w.syncData.activeSyncData.syncStage = AddressDiscoverySyncStage
	w.syncData.activeSyncData.addressDiscoveryProgress.addressDiscoveryStartTime = time.Now().Unix()
	w.syncData.activeSyncData.addressDiscoveryProgress.WalletID = walletID
	w.syncData.addressDiscoveryCompletedOrCanceled = make(chan bool)
	w.syncData.mu.Unlock()

	go w.updateAddressDiscoveryProgress(totalHeadersFetchTime)

	if w.syncData.showLogs {
		log.Info("Step 2 of 3 - discovering used addresses.")
	}
}

func (w *Wallet) updateAddressDiscoveryProgress(totalHeadersFetchTime float64) {
	// use ticker to calculate and broadcast address discovery progress every second
	everySecondTicker := time.NewTicker(1 * time.Second)

	// these values will be used every second to calculate the total sync progress
	estimatedDiscoveryTime := totalHeadersFetchTime * DiscoveryPercentage
	estimatedRescanTime := totalHeadersFetchTime * RescanPercentage

	// track last logged time remaining and total percent to avoid re-logging same message
	var lastTimeRemaining int64
	var lastTotalPercent int32 = -1

	for {
		if !w.IsSyncing() {
			return
		}

		// If there was some period of inactivity,
		// assume that this process started at some point in the future,
		// thereby accounting for the total reported time of inactivity.
		w.syncData.mu.Lock()
		w.syncData.addressDiscoveryProgress.addressDiscoveryStartTime += w.syncData.totalInactiveSeconds
		w.syncData.totalInactiveSeconds = 0
		addressDiscoveryStartTime := w.syncData.addressDiscoveryProgress.addressDiscoveryStartTime
		totalCfiltersFetchTime := float64(w.syncData.cfiltersFetchProgress.cfiltersFetchTimeSpent)
		showLogs := w.syncData.showLogs
		w.syncData.mu.Unlock()

		select {
		case <-w.syncData.addressDiscoveryCompletedOrCanceled:
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
			w.syncData.mu.Lock()
			w.syncData.addressDiscoveryProgress.AddressDiscoveryProgress = int32(math.Round(discoveryProgress))
			w.syncData.addressDiscoveryProgress.TotalSyncProgress = totalProgressPercent
			w.syncData.addressDiscoveryProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds
			w.syncData.mu.Unlock()

			w.publishAddressDiscoveryProgress()

			debugInfo := &DebugInfo{
				int64(math.Round(totalElapsedTime)),
				totalTimeRemainingSeconds,
				int64(math.Round(elapsedDiscoveryTime)),
				int64(math.Round(remainingAccountDiscoveryTime)),
			}
			w.publishDebugInfo(debugInfo)

			if showLogs {
				// avoid logging same message multiple times
				if totalProgressPercent != lastTotalPercent || totalTimeRemainingSeconds != lastTimeRemaining {
					log.Infof("Syncing %d%%, %s remaining, discovering used addresses.",
						totalProgressPercent, CalculateTotalTimeRemaining(totalTimeRemainingSeconds))

					lastTotalPercent = totalProgressPercent
					lastTimeRemaining = totalTimeRemainingSeconds
				}
			}
		}
	}
}

func (w *Wallet) publishAddressDiscoveryProgress() {
	for _, syncProgressListener := range w.syncProgressListeners() {
		syncProgressListener.OnAddressDiscoveryProgress(&w.syncData.activeSyncData.addressDiscoveryProgress)
	}
}

func (w *Wallet) discoverAddressesFinished(walletID int) {
	if !w.IsSyncing() {
		return
	}

	w.stopUpdatingAddressDiscoveryProgress()
}

func (w *Wallet) stopUpdatingAddressDiscoveryProgress() {
	w.syncData.mu.Lock()
	if w.syncData.activeSyncData != nil && w.syncData.activeSyncData.addressDiscoveryCompletedOrCanceled != nil {
		close(w.syncData.activeSyncData.addressDiscoveryCompletedOrCanceled)
		w.syncData.activeSyncData.addressDiscoveryCompletedOrCanceled = nil
		w.syncData.activeSyncData.addressDiscoveryProgress.totalDiscoveryTimeSpent = time.Now().Unix() - w.syncData.addressDiscoveryProgress.addressDiscoveryStartTime
	}
	w.syncData.mu.Unlock()
}

// Blocks Scan Callbacks

func (w *Wallet) rescanStarted(walletID int) {
	w.stopUpdatingAddressDiscoveryProgress()

	w.syncData.mu.Lock()
	defer w.syncData.mu.Unlock()

	if !w.syncData.syncing {
		// ignore if sync is not in progress
		return
	}

	w.syncData.activeSyncData.syncStage = HeadersRescanSyncStage
	w.syncData.activeSyncData.rescanStartTime = time.Now().Unix()

	// retain last total progress report from address discovery phase
	w.syncData.activeSyncData.headersRescanProgress.TotalTimeRemainingSeconds = w.syncData.activeSyncData.addressDiscoveryProgress.TotalTimeRemainingSeconds
	w.syncData.activeSyncData.headersRescanProgress.TotalSyncProgress = w.syncData.activeSyncData.addressDiscoveryProgress.TotalSyncProgress
	w.syncData.activeSyncData.headersRescanProgress.WalletID = walletID

	if w.syncData.showLogs && w.syncData.syncing {
		log.Info("Step 3 of 3 - Scanning block headers.")
	}
}

func (w *Wallet) rescanProgress(walletID int, rescannedThrough int32) {
	if !w.IsSyncing() {
		// ignore if sync is not in progress
		return
	}

	totalHeadersToScan := w.GetBestBlockInt()

	rescanRate := float64(rescannedThrough) / float64(totalHeadersToScan)

	w.syncData.mu.Lock()

	// If there was some period of inactivity,
	// assume that this process started at some point in the future,
	// thereby accounting for the total reported time of inactivity.
	w.syncData.activeSyncData.rescanStartTime += w.syncData.activeSyncData.totalInactiveSeconds
	w.syncData.activeSyncData.totalInactiveSeconds = 0

	elapsedRescanTime := time.Now().Unix() - w.syncData.activeSyncData.rescanStartTime
	estimatedTotalRescanTime := int64(math.Round(float64(elapsedRescanTime) / rescanRate))
	totalTimeRemainingSeconds := estimatedTotalRescanTime - elapsedRescanTime
	totalElapsedTime := w.syncData.activeSyncData.cfiltersFetchProgress.cfiltersFetchTimeSpent + w.syncData.activeSyncData.headersFetchProgress.headersFetchTimeSpent +
		w.syncData.activeSyncData.addressDiscoveryProgress.totalDiscoveryTimeSpent + elapsedRescanTime

	w.syncData.activeSyncData.headersRescanProgress.WalletID = walletID
	w.syncData.activeSyncData.headersRescanProgress.TotalHeadersToScan = totalHeadersToScan
	w.syncData.activeSyncData.headersRescanProgress.RescanProgress = int32(math.Round(rescanRate * 100))
	w.syncData.activeSyncData.headersRescanProgress.CurrentRescanHeight = rescannedThrough
	w.syncData.activeSyncData.headersRescanProgress.RescanTimeRemaining = totalTimeRemainingSeconds

	// do not update total time taken and total progress percent if elapsedRescanTime is 0
	// because the estimatedTotalRescanTime will be inaccurate (also 0)
	// which will make the estimatedTotalSyncTime equal to totalElapsedTime
	// giving the wrong impression that the process is complete
	if elapsedRescanTime > 0 {
		estimatedTotalSyncTime := w.syncData.activeSyncData.cfiltersFetchProgress.cfiltersFetchTimeSpent + w.syncData.activeSyncData.headersFetchProgress.headersFetchTimeSpent +
			w.syncData.activeSyncData.addressDiscoveryProgress.totalDiscoveryTimeSpent + estimatedTotalRescanTime
		totalProgress := (float64(totalElapsedTime) / float64(estimatedTotalSyncTime)) * 100

		w.syncData.activeSyncData.headersRescanProgress.TotalTimeRemainingSeconds = totalTimeRemainingSeconds
		w.syncData.activeSyncData.headersRescanProgress.TotalSyncProgress = int32(math.Round(totalProgress))
	}

	w.syncData.mu.Unlock()

	w.publishHeadersRescanProgress()

	debugInfo := &DebugInfo{
		totalElapsedTime,
		totalTimeRemainingSeconds,
		elapsedRescanTime,
		totalTimeRemainingSeconds,
	}
	w.publishDebugInfo(debugInfo)

	w.syncData.mu.RLock()
	if w.syncData.showLogs {
		log.Infof("Syncing %d%%, %s remaining, scanning %d of %d block headers.",
			w.syncData.activeSyncData.headersRescanProgress.TotalSyncProgress,
			CalculateTotalTimeRemaining(w.syncData.activeSyncData.headersRescanProgress.TotalTimeRemainingSeconds),
			w.syncData.activeSyncData.headersRescanProgress.CurrentRescanHeight,
			w.syncData.activeSyncData.headersRescanProgress.TotalHeadersToScan,
		)
	}
	w.syncData.mu.RUnlock()
}

func (w *Wallet) publishHeadersRescanProgress() {
	for _, syncProgressListener := range w.syncProgressListeners() {
		syncProgressListener.OnHeadersRescanProgress(&w.syncData.activeSyncData.headersRescanProgress)
	}
}

func (w *Wallet) rescanFinished(walletID int) {
	if !w.IsSyncing() {
		// ignore if sync is not in progress
		return
	}

	w.syncData.mu.Lock()
	w.syncData.activeSyncData.headersRescanProgress.WalletID = walletID
	w.syncData.activeSyncData.headersRescanProgress.TotalTimeRemainingSeconds = 0
	w.syncData.activeSyncData.headersRescanProgress.TotalSyncProgress = 100

	// Reset these value so that address discovery would
	// not be skipped for the next wallet.
	w.syncData.activeSyncData.addressDiscoveryProgress.addressDiscoveryStartTime = -1
	w.syncData.activeSyncData.addressDiscoveryProgress.totalDiscoveryTimeSpent = -1
	w.syncData.mu.Unlock()

	w.publishHeadersRescanProgress()
}

func (w *Wallet) publishDebugInfo(debugInfo *DebugInfo) {
	for _, syncProgressListener := range w.syncProgressListeners() {
		syncProgressListener.Debug(debugInfo)
	}
}

/** Helper functions start here */

func (w *Wallet) estimateBlockHeadersCountAfter(lastHeaderTime int64) int32 {
	// Use the difference between current time (now) and last reported block time,
	// to estimate total headers to fetch.
	timeDifferenceInSeconds := float64(time.Now().Unix() - lastHeaderTime)
	targetTimePerBlockInSeconds := w.chainParams.TargetTimePerBlock.Seconds()
	estimatedHeadersDifference := timeDifferenceInSeconds / targetTimePerBlockInSeconds

	// return next integer value (upper limit) if estimatedHeadersDifference is a fraction
	return int32(math.Ceil(estimatedHeadersDifference))
}

func (w *Wallet) notifySyncError(err error) {
	for _, syncProgressListener := range w.syncProgressListeners() {
		syncProgressListener.OnSyncEndedWithError(err)
	}
}

func (w *Wallet) notifySyncCanceled() {
	w.syncData.mu.RLock()
	restartSyncRequested := w.syncData.restartSyncRequested
	w.syncData.mu.RUnlock()

	for _, syncProgressListener := range w.syncProgressListeners() {
		syncProgressListener.OnSyncCanceled(restartSyncRequested)
	}
}

func (w *Wallet) resetSyncData() {
	// It's possible that sync ends or errors while address discovery is ongoing.
	// If this happens, it's important to stop the address discovery process before
	// resetting sync data.
	w.stopUpdatingAddressDiscoveryProgress()

	w.syncData.mu.Lock()
	w.syncData.syncing = false
	w.syncData.synced = false
	w.syncData.cancelSync = nil
	w.syncData.syncCanceled = nil
	w.syncData.activeSyncData = nil
	w.syncData.mu.Unlock()

	w.WaitingForHeaders = true
	w.LockWallet() // lock wallet if previously unlocked to perform account discovery.
}

func (w *Wallet) synced(walletID int, synced bool) {

	indexTransactions := func() {
		// begin indexing transactions after sync is completed,
		// syncProgressListeners.OnSynced() will be invoked after transactions are indexed
		var txIndexing errgroup.Group
		txIndexing.Go(w.IndexTransactions)

		go func() {
			err := txIndexing.Wait()
			if err != nil {
				log.Errorf("Tx Index Error: %v", err)
			}

			for _, syncProgressListener := range w.syncProgressListeners() {
				if synced {
					syncProgressListener.OnSyncCompleted()
				} else {
					syncProgressListener.OnSyncCanceled(false)
				}
			}
		}()
	}

	w.syncData.mu.RLock()
	allWalletsSynced := w.syncData.synced
	w.syncData.mu.RUnlock()

	if allWalletsSynced && synced {
		indexTransactions()
		return
	}

	w.Synced = synced
	w.Syncing = false
	w.listenForTransactions()

	if !w.Internal().Locked() {
		w.LockWallet() // lock wallet if previously unlocked to perform account discovery.
		err := w.markWalletAsDiscoveredAccounts()
		if err != nil {
			log.Error(err)
		}
	}

	w.syncData.mu.Lock()
	w.syncData.syncing = false
	w.syncData.synced = true
	w.syncData.mu.Unlock()

	indexTransactions()
}
