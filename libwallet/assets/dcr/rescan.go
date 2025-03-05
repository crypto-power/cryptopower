package dcr

import (
	"context"
	"math"
	"time"

	"decred.org/dcrwallet/v4/errors"
	w "decred.org/dcrwallet/v4/wallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/values"
)

func (asset *Asset) RescanBlocks() error {
	return asset.RescanBlocksFromHeight(0)
}

func (asset *Asset) RescanBlocksFromHeight(startHeight int32) error {
	netBackend, err := asset.Internal().DCR.NetworkBackend()
	if err != nil {
		return errors.E(values.String(values.StrConnectNetworkFailed))
	}

	if asset.IsRescanning() || !asset.IsSynced() {
		return errors.E(values.String(values.StrRescanInvalid))
	}

	go func() {
		defer func() {
			asset.syncData.mu.Lock()
			asset.syncData.rescanning = false
			asset.syncData.cancelRescan = nil
			asset.syncData.mu.Unlock()
		}()

		ctx, cancel := asset.ShutdownContextWithCancel()

		asset.syncData.mu.Lock()
		asset.syncData.rescanning = true
		asset.syncData.cancelRescan = cancel
		asset.syncData.mu.Unlock()

		if asset.blocksRescanProgressListener != nil {
			asset.blocksRescanProgressListener.OnBlocksRescanStarted(asset.ID)
		}

		progress := make(chan w.RescanProgress, 1)
		go asset.Internal().DCR.RescanProgressFromHeight(ctx, netBackend, startHeight, progress)

		rescanStartTime := time.Now()

		for p := range progress { // listen to the progress channel
			if p.Err != nil {
				log.Error(p.Err)
				if asset.blocksRescanProgressListener != nil {
					asset.blocksRescanProgressListener.OnBlocksRescanEnded(asset.ID, p.Err)
				}
				return
			}

			rescanProgressReport := &sharedW.HeadersRescanProgressReport{
				CurrentRescanHeight: p.ScannedThrough,
				TotalHeadersToScan:  asset.GetBestBlockHeight(),
				WalletID:            asset.ID,
			}

			elapsedRescanTime := time.Since(rescanStartTime).Seconds()
			rescanRate := float64(p.ScannedThrough) / float64(rescanProgressReport.TotalHeadersToScan)

			rescanProgressReport.RescanProgress = int32(math.Round(rescanRate * 100))
			estimatedTotalRescanTime := elapsedRescanTime / rescanRate
			rescanProgressReport.RescanTimeRemaining = secondsToDuration(estimatedTotalRescanTime - elapsedRescanTime)

			rescanProgressReport.GeneralSyncProgress = &sharedW.GeneralSyncProgress{
				TotalSyncProgress:  rescanProgressReport.RescanProgress,
				TotalTimeRemaining: rescanProgressReport.RescanTimeRemaining,
			}

			if asset.blocksRescanProgressListener != nil {
				asset.blocksRescanProgressListener.OnBlocksRescanProgress(rescanProgressReport)
			}

			select {
			case <-ctx.Done():
				log.Info("Rescan canceled through context")

				if asset.blocksRescanProgressListener != nil {
					if ctx.Err() != nil && ctx.Err() != context.Canceled {
						asset.blocksRescanProgressListener.OnBlocksRescanEnded(asset.ID, ctx.Err())
					} else {
						asset.blocksRescanProgressListener.OnBlocksRescanEnded(asset.ID, nil)
					}
				}

				return
			default:
				continue
			}
		}

		var err error
		if startHeight == 0 {
			err = asset.reindexTransactions()
		} else {
			err = asset.GetWalletDataDb().SaveLastIndexPoint(startHeight)
			if err != nil {
				if asset.blocksRescanProgressListener != nil {
					asset.blocksRescanProgressListener.OnBlocksRescanEnded(asset.ID, err)
				}
				return
			}

			err = asset.IndexTransactions()
		}
		if asset.blocksRescanProgressListener != nil {
			asset.blocksRescanProgressListener.OnBlocksRescanEnded(asset.ID, err)
		}
	}()

	return nil
}

func (asset *Asset) CancelRescan() {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()
	if asset.syncData.cancelRescan != nil {
		asset.syncData.cancelRescan()
		asset.syncData.cancelRescan = nil

		log.Info("Rescan canceled.")
	}
}

func (asset *Asset) IsRescanning() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()
	return asset.syncData.rescanning
}

func (asset *Asset) SetBlocksRescanProgressListener(blocksRescanProgressListener *sharedW.BlocksRescanProgressListener) {
	asset.blocksRescanProgressListener = blocksRescanProgressListener
}
