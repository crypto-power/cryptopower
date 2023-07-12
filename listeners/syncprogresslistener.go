package listeners

import (
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/wallet"
)

// SyncProgressListener satisfies libwallet SyncProgressListener interface
// contract.
type SyncProgressListener struct {
	SyncStatusChan chan wallet.SyncStatusUpdate
}

func NewSyncProgress() *SyncProgressListener {
	return &SyncProgressListener{
		SyncStatusChan: make(chan wallet.SyncStatusUpdate, 4),
	}
}

func (sp *SyncProgressListener) OnSyncStarted() {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage: wallet.SyncStarted,
	})
}

func (sp *SyncProgressListener) OnPeerConnectedOrDisconnected(numberOfConnectedPeers int32) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage:          wallet.PeersConnected,
		ConnectedPeers: numberOfConnectedPeers,
	})
}

func (sp *SyncProgressListener) OnCFiltersFetchProgress(cfiltersFetchProgress *sharedW.CFiltersFetchProgressReport) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage:          wallet.CfiltersFetchProgress,
		ProgressReport: cfiltersFetchProgress,
	})
}

func (sp *SyncProgressListener) OnHeadersFetchProgress(headersFetchProgress *sharedW.HeadersFetchProgressReport) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage:          wallet.HeadersFetchProgress,
		ProgressReport: headersFetchProgress,
	})
}

func (sp *SyncProgressListener) OnAddressDiscoveryProgress(addressDiscoveryProgress *sharedW.AddressDiscoveryProgressReport) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage:          wallet.AddressDiscoveryProgress,
		ProgressReport: addressDiscoveryProgress,
	})
}

func (sp *SyncProgressListener) OnHeadersRescanProgress(headersRescanProgress *sharedW.HeadersRescanProgressReport) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage:          wallet.HeadersRescanProgress,
		ProgressReport: headersRescanProgress,
	})
}

func (sp *SyncProgressListener) OnSyncCompleted() {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage: wallet.SyncCompleted,
	})
}

func (sp *SyncProgressListener) OnSyncCanceled(_ /*willRestart*/ bool) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage: wallet.SyncCanceled,
	})
}
func (sp *SyncProgressListener) OnSyncEndedWithError(_ error) {}
func (sp *SyncProgressListener) Debug(_ *sharedW.DebugInfo)   {}

func (sp *SyncProgressListener) sendNotification(signal wallet.SyncStatusUpdate) {
	select {
	case sp.SyncStatusChan <- signal:
	default:
	}
}
