package listeners

import (
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/wallet"
)

// BlocksRescanProgressListener satisfies libwallet
// BlocksRescanProgressListener interface.
type BlocksRescanProgressListener struct {
	BlockRescanChan chan wallet.RescanUpdate
}

func NewBlocksRescanProgressListener() *BlocksRescanProgressListener {
	return &BlocksRescanProgressListener{
		BlockRescanChan: make(chan wallet.RescanUpdate, 4),
	}
}

// OnBlocksRescanStarted is a callback func called when block rescan is started.
func (br *BlocksRescanProgressListener) OnBlocksRescanStarted(walletID int) {
	br.UpdateNotification(wallet.RescanUpdate{
		Stage:    wallet.RescanStarted,
		WalletID: walletID,
	})
}

// OnBlocksRescanProgress is a callback func for block rescan progress report.
func (br *BlocksRescanProgressListener) OnBlocksRescanProgress(progress *sharedW.HeadersRescanProgressReport) {
	br.UpdateNotification(wallet.RescanUpdate{
		Stage:          wallet.RescanProgress,
		ProgressReport: progress,
		WalletID:       progress.WalletID,
	})
}

// OnBlocksRescanEnded is a callback func to notify the end of block rescan.
func (br *BlocksRescanProgressListener) OnBlocksRescanEnded(walletID int, _ error) {
	br.UpdateNotification(wallet.RescanUpdate{
		Stage:    wallet.RescanEnded,
		WalletID: walletID,
	})
}

func (br *BlocksRescanProgressListener) UpdateNotification(signal wallet.RescanUpdate) {
	select {
	case br.BlockRescanChan <- signal:
	default:
	}
}
