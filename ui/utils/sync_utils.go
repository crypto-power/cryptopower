package utils

import (
	"sync"
	"time"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
)

// File holds implementation needed to manage the UI sync progess.

type ProgressInfo struct {
	remainingSyncTime    string
	headersToFetchOrScan int32
	stepFetchProgress    int32
	syncProgress         int
}

func (pi ProgressInfo) RemainingSyncTime() string {
	return pi.remainingSyncTime
}

func (pi ProgressInfo) HeadersToFetchOrScan() int32 {
	return pi.headersToFetchOrScan
}

func (pi ProgressInfo) StepFetchProgress() int32 {
	return pi.stepFetchProgress
}

func (pi ProgressInfo) SyncProgress() int {
	return pi.syncProgress
}

type SyncInfo struct {
	progressInfo map[sharedW.Asset]*ProgressInfo
	rescanInfo   map[sharedW.Asset]*sharedW.HeadersRescanProgressReport
	syncInfoMu   sync.RWMutex
}

// NewSyncProgressInfo returns an instance of the SyncInfo with the respective
// maps initialized.
func NewSyncProgressInfo() *SyncInfo {
	return &SyncInfo{
		progressInfo: make(map[sharedW.Asset]*ProgressInfo),
		rescanInfo:   make(map[sharedW.Asset]*sharedW.HeadersRescanProgressReport),
	}
}

// IsSyncProgressSet returns true if a sync progress instance of the provided wallet
// exists.
func (si *SyncInfo) IsSyncProgressSet(wallet sharedW.Asset) bool {
	si.syncInfoMu.RLock()
	_, ok := si.progressInfo[wallet]
	si.syncInfoMu.RUnlock()

	return ok
}

// GetSyncProgress returns a copy of the progress info associated with the provided
// asset type.
func (si *SyncInfo) GetSyncProgress(wallet sharedW.Asset) ProgressInfo {
	si.syncInfoMu.RLock()
	data := si.progressInfo[wallet]
	si.syncInfoMu.RUnlock()

	if data == nil {
		return ProgressInfo{}
	}
	return *data
}

// SetSyncProgress creates a new sync progress instance and stores a copy of it.
func (si *SyncInfo) SetSyncProgress(wallet sharedW.Asset, timeRemaining time.Duration,
	headersFetched, stepFetchProgress, totalSyncProgress int32) ProgressInfo {

	progress := ProgressInfo{
		remainingSyncTime:    TimeFormat(int(timeRemaining.Seconds()), true),
		headersToFetchOrScan: headersFetched,
		stepFetchProgress:    stepFetchProgress,
		syncProgress:         int(totalSyncProgress),
	}

	si.syncInfoMu.Lock()
	si.progressInfo[wallet] = &progress
	si.syncInfoMu.Unlock()

	return progress
}

// DeleteSyncProgress deletes the sync progress associated with the provided
// asset type.
func (si *SyncInfo) DeleteSyncProgress(wallet sharedW.Asset) {
	si.syncInfoMu.Lock()
	delete(si.progressInfo, wallet)
	si.syncInfoMu.Unlock()
}

// IsRescanProgressSet confirms if a rescan progress info associated with the
// provided asset type exists.
func (si *SyncInfo) IsRescanProgressSet(wallet sharedW.Asset) bool {
	si.syncInfoMu.RLock()
	_, ok := si.rescanInfo[wallet]
	si.syncInfoMu.RUnlock()

	return ok
}

// GetRescanProgress returns the progress report associated with the provided
// asset type.
func (si *SyncInfo) GetRescanProgress(wallet sharedW.Asset) *sharedW.HeadersRescanProgressReport {
	si.syncInfoMu.RLock()
	data := si.rescanInfo[wallet]
	si.syncInfoMu.RUnlock()

	return data
}

// SetRescanProgress updates the Rescan progress for the provided asset type.
func (si *SyncInfo) SetRescanProgress(wallet sharedW.Asset, data *sharedW.HeadersRescanProgressReport) {
	si.syncInfoMu.Lock()
	si.rescanInfo[wallet] = data
	si.syncInfoMu.Unlock()
}

// DeleteRescanProgress deletes the rescan progress for the provided asset type.
func (si *SyncInfo) DeleteRescanProgress(wallet sharedW.Asset) {
	si.syncInfoMu.Lock()
	delete(si.rescanInfo, wallet)
	si.syncInfoMu.Unlock()
}
