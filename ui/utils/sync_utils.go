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
	progressInfo sync.Map //map[sharedW.Asset]ProgressInfo
	rescanInfo   sync.Map //map[sharedW.Asset]*sharedW.HeadersRescanProgressReport
}

// NewSyncProgressInfo returns an instance of the SyncInfo with the respective
// maps initialized.
func NewSyncProgressInfo() *SyncInfo {
	return &SyncInfo{
		progressInfo: sync.Map{},
		rescanInfo:   sync.Map{},
	}
}

// IsSyncProgressSet returns true if a sync progress instance of the provided wallet
// exists.
func (si *SyncInfo) IsSyncProgressSet(wallet sharedW.Asset) bool {
	_, ok := si.progressInfo.Load(wallet)

	return ok
}

// GetSyncProgress returns a copy of the progress info associated with the provided
// asset type.
func (si *SyncInfo) GetSyncProgress(wallet sharedW.Asset) ProgressInfo {
	data, _ := si.progressInfo.Load(wallet)

	if data == nil {
		return ProgressInfo{}
	}
	return data.(ProgressInfo)
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

	si.progressInfo.Store(wallet, progress)

	return progress
}

// DeleteSyncProgress deletes the sync progress associated with the provided
// asset type.
func (si *SyncInfo) DeleteSyncProgress(wallet sharedW.Asset) {
	si.progressInfo.Delete(wallet)
}

// IsRescanProgressSet confirms if a rescan progress info associated with the
// provided asset type exists.
func (si *SyncInfo) IsRescanProgressSet(wallet sharedW.Asset) bool {
	_, ok := si.rescanInfo.Load(wallet)

	return ok
}

// GetRescanProgress returns the progress report associated with the provided
// asset type.
func (si *SyncInfo) GetRescanProgress(wallet sharedW.Asset) *sharedW.HeadersRescanProgressReport {
	data, _ := si.rescanInfo.Load(wallet)

	return data.(*sharedW.HeadersRescanProgressReport)
}

// SetRescanProgress updates the Rescan progress for the provided asset type.
func (si *SyncInfo) SetRescanProgress(wallet sharedW.Asset, data *sharedW.HeadersRescanProgressReport) {
	si.rescanInfo.Store(wallet, data)
}

// DeleteRescanProgress deletes the rescan progress for the provided asset type.
func (si *SyncInfo) DeleteRescanProgress(wallet sharedW.Asset) {
	si.rescanInfo.Delete(wallet)
}
