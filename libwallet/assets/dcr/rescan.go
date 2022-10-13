package dcr

import (
	"context"
	"math"
	"time"

	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	mainW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

func (wallet *Wallet) RescanBlocks() error {
	return wallet.RescanBlocksFromHeight(0)
}

func (wallet *Wallet) RescanBlocksFromHeight(startHeight int32) error {

	netBackend, err := wallet.Internal().NetworkBackend()
	if err != nil {
		return errors.E(utils.ErrNotConnected)
	}

	if wallet.IsRescanning() || !wallet.IsSynced() {
		return errors.E(utils.ErrInvalid)
	}

	go func() {
		defer func() {
			wallet.syncData.mu.Lock()
			wallet.syncData.rescanning = false
			wallet.syncData.cancelRescan = nil
			wallet.syncData.mu.Unlock()
		}()

		ctx, cancel := wallet.ShutdownContextWithCancel()

		wallet.syncData.mu.Lock()
		wallet.syncData.rescanning = true
		wallet.syncData.cancelRescan = cancel
		wallet.syncData.mu.Unlock()

		if wallet.blocksRescanProgressListener != nil {
			wallet.blocksRescanProgressListener.OnBlocksRescanStarted(wallet.ID)
		}

		progress := make(chan w.RescanProgress, 1)
		go wallet.Internal().RescanProgressFromHeight(ctx, netBackend, startHeight, progress)

		rescanStartTime := time.Now().Unix()

		for p := range progress {
			if p.Err != nil {
				log.Error(p.Err)
				if wallet.blocksRescanProgressListener != nil {
					wallet.blocksRescanProgressListener.OnBlocksRescanEnded(wallet.ID, p.Err)
				}
				return
			}

			rescanProgressReport := &mainW.HeadersRescanProgressReport{
				CurrentRescanHeight: p.ScannedThrough,
				TotalHeadersToScan:  wallet.GetBestBlockHeight(),
				WalletID:            wallet.ID,
			}

			elapsedRescanTime := time.Now().Unix() - rescanStartTime
			rescanRate := float64(p.ScannedThrough) / float64(rescanProgressReport.TotalHeadersToScan)

			rescanProgressReport.RescanProgress = int32(math.Round(rescanRate * 100))
			estimatedTotalRescanTime := int64(math.Round(float64(elapsedRescanTime) / rescanRate))
			rescanProgressReport.RescanTimeRemaining = estimatedTotalRescanTime - elapsedRescanTime

			rescanProgressReport.GeneralSyncProgress = &mainW.GeneralSyncProgress{
				TotalSyncProgress:         rescanProgressReport.RescanProgress,
				TotalTimeRemainingSeconds: rescanProgressReport.RescanTimeRemaining,
			}

			if wallet.blocksRescanProgressListener != nil {
				wallet.blocksRescanProgressListener.OnBlocksRescanProgress(rescanProgressReport)
			}

			select {
			case <-ctx.Done():
				log.Info("Rescan canceled through context")

				if wallet.blocksRescanProgressListener != nil {
					if ctx.Err() != nil && ctx.Err() != context.Canceled {
						wallet.blocksRescanProgressListener.OnBlocksRescanEnded(wallet.ID, ctx.Err())
					} else {
						wallet.blocksRescanProgressListener.OnBlocksRescanEnded(wallet.ID, nil)
					}
				}

				return
			default:
				continue
			}
		}

		var err error
		if startHeight == 0 {
			err = wallet.reindexTransactions()
		} else {
			err = wallet.walletDataDB.SaveLastIndexPoint(startHeight)
			if err != nil {
				if wallet.blocksRescanProgressListener != nil {
					wallet.blocksRescanProgressListener.OnBlocksRescanEnded(wallet.ID, err)
				}
				return
			}

			err = wallet.IndexTransactions()
		}
		if wallet.blocksRescanProgressListener != nil {
			wallet.blocksRescanProgressListener.OnBlocksRescanEnded(wallet.ID, err)
		}
	}()

	return nil
}

func (wallet *Wallet) CancelRescan() {
	wallet.syncData.mu.Lock()
	defer wallet.syncData.mu.Unlock()
	if wallet.syncData.cancelRescan != nil {
		wallet.syncData.cancelRescan()
		wallet.syncData.cancelRescan = nil

		log.Info("Rescan canceled.")
	}
}

func (wallet *Wallet) IsRescanning() bool {
	wallet.syncData.mu.RLock()
	defer wallet.syncData.mu.RUnlock()
	return wallet.syncData.rescanning
}

func (wallet *Wallet) SetBlocksRescanProgressListener(blocksRescanProgressListener mainW.BlocksRescanProgressListener) {
	wallet.blocksRescanProgressListener = blocksRescanProgressListener
}
