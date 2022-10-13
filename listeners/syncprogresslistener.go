package listeners

import (
	mainW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/wallet"
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

func (sp *SyncProgressListener) OnSyncStarted(wasRestarted bool) {
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

func (sp *SyncProgressListener) OnCFiltersFetchProgress(cfiltersFetchProgress *mainW.CFiltersFetchProgressReport) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage:          wallet.CfiltersFetchProgress,
		ProgressReport: cfiltersFetchProgress,
	})
}

func (sp *SyncProgressListener) OnHeadersFetchProgress(headersFetchProgress *mainW.HeadersFetchProgressReport) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage:          wallet.HeadersFetchProgress,
		ProgressReport: headersFetchProgress,
	})
}

func (sp *SyncProgressListener) OnAddressDiscoveryProgress(addressDiscoveryProgress *mainW.AddressDiscoveryProgressReport) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage:          wallet.AddressDiscoveryProgress,
		ProgressReport: addressDiscoveryProgress,
	})
}

func (sp *SyncProgressListener) OnHeadersRescanProgress(headersRescanProgress *mainW.HeadersRescanProgressReport) {
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

func (sp *SyncProgressListener) OnSyncCanceled(willRestart bool) {
	sp.sendNotification(wallet.SyncStatusUpdate{
		Stage: wallet.SyncCanceled,
	})
}
func (sp *SyncProgressListener) OnSyncEndedWithError(err error)   {}
func (sp *SyncProgressListener) Debug(debugInfo *mainW.DebugInfo) {}

func (sp *SyncProgressListener) sendNotification(signal wallet.SyncStatusUpdate) {
	sp.SyncStatusChan <- signal
}
