package utils

import (
	"sync"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
)

// File holds implementation needed to manage the UI sync progess.

type ProgressInfo struct {
	remainingSyncTime    string
	headersToFetchOrScan int32
	stepFetchProgress    int32
	syncProgress         int
}

type SyncInfo struct {
	progressInfo sync.Map //map[sharedW.Asset]*ProgressInfo
	rescanUpdate *sharedW.HeadersRescanProgressReport
	syncInfoMu   sync.RWMutex
}

// This methods prevent the direct access of the mutex protected syncProgressInfo
// SyncInfo instance.

func (si *SyncInfo) IsSyncProgressSet(wallet sharedW.Asset) bool {
	defer si.syncInfoMu.RUnlock()
	si.syncInfoMu.RLock()
	_, ok := si.progressInfo[wallet]
	return ok
}

func (si *SyncInfo) GetSyncProgress(wallet sharedW.Asset) ProgressInfo {
	defer si.syncInfoMu.RUnlock()
	si.syncInfoMu.RLock()
	data, _ := si.progressInfo[wallet]
	if data == nil {
		return ProgressInfo{}
	}
	return *data
}

// setSyncProgress creates a new sync progress instance and stores a copy of it.
func (si *SyncInfo) setSyncProgress(wallet sharedW.Asset, timeRemaining int64,
	headersRemaining, stepFetchProgress, totalSyncProgress int32) ProgressInfo {

	progress := ProgressInfo{
		remainingSyncTime:    TimeFormat(int(timeRemaining), true),
		headersToFetchOrScan: 0,
		stepFetchProgress:    0,
		syncProgress:         0,
	}

	si.syncInfoMu.Lock()
	si.progressInfo[wallet] = &progress
	si.syncInfoMu.Unlock()

	return progress
}
