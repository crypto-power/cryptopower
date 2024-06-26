package ltc

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"decred.org/dcrwallet/v3/errors"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/dcrlabs/ltcwallet/waddrmgr"
	ltcwallet "github.com/dcrlabs/ltcwallet/wallet"
	"github.com/dcrlabs/ltcwallet/walletdb"
	"github.com/ltcsuite/ltcd/ltcutil"
)

// SetBlocksRescanProgressListener sets the blocks rescan progress listener.
func (asset *Asset) SetBlocksRescanProgressListener(blocksRescanProgressListener *sharedW.BlocksRescanProgressListener) {
	asset.blocksRescanProgressListener = blocksRescanProgressListener
}

// RescanBlocks rescans the blockchain for all addresses in the wallet.
func (asset *Asset) RescanBlocks() error {
	return asset.RescanBlocksFromHeight(0)
}

// RescanBlocksFromHeight rescans the blockchain for all addresses in the wallet
// starting from the provided block height.
func (asset *Asset) RescanBlocksFromHeight(startHeight int32) error {
	return asset.rescanBlocks(startHeight, nil)
}

func (asset *Asset) rescanBlocks(startHeight int32, addrs []ltcutil.Address) error {
	if !asset.IsConnectedToBitcoinNetwork() {
		return errors.E(utils.ErrNotConnected)
	}

	if !asset.WalletOpened() {
		return utils.ErrLTCNotInitialized
	}

	if !asset.IsSynced() {
		return errors.E(utils.ErrNotSynced)
	}

	if asset.IsRescanning() {
		return errors.E(utils.ErrSyncAlreadyInProgress)
	}

	bs, err := asset.getblockStamp(startHeight)
	if err != nil {
		return err
	}

	if addrs == nil {
		addrs = []ltcutil.Address{}
	}

	// Force rescan, to enforce address discovery.
	asset.forceRescan()

	asset.syncData.mu.Lock()
	asset.syncData.isRescan = true
	asset.syncData.forcedRescanActive = true
	asset.syncData.rescanStartTime = time.Now()
	asset.syncData.mu.Unlock()

	job := &ltcwallet.RescanJob{
		Addrs:      addrs,
		OutPoints:  nil,
		BlockStamp: *bs,
	}

	// It submits a rescan job without blocking on finishing the rescan.
	// The rescan success or failure is logged elsewhere, and the channel
	// is not required to be read, so discard the return value.
	errChan := asset.Internal().LTC.SubmitRescan(job)

	// Listen for the rescan finish event and update it.
	go func() {
		for err := range errChan {
			if err != nil {
				log.Errorf("rescan job failed: %v", err)
			}
		}

		asset.syncData.mu.Lock()
		asset.syncData.isRescan = false
		asset.syncData.mu.Unlock()
	}()

	// Attempt to start up the notifications handler.
	if atomic.CompareAndSwapUint32(&asset.syncData.syncstarted, stop, start) {
		go asset.handleNotifications()
	}

	return nil
}

// IsRescanning returns true if the wallet is currently rescanning the blockchain.
func (asset *Asset) IsRescanning() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.isRescan
}

// CancelRescan cancels the current rescan.
func (asset *Asset) CancelRescan() {
	asset.syncData.mu.Lock()
	asset.syncData.isRescan = false
	asset.syncData.mu.Unlock()

	if asset.blocksRescanProgressListener != nil {
		asset.blocksRescanProgressListener.OnBlocksRescanEnded(asset.ID, nil)
	}
}

// forceRescan forces a full rescan with active address discovery on wallet
// restart by setting the "synced to" field to nil.
func (asset *Asset) forceRescan() {
	wdb := asset.Internal().LTC.Database()
	err := walletdb.Update(wdb, func(dbtx walletdb.ReadWriteTx) error {
		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)

		if asset.IsRestored && !asset.ContainsDiscoveredAccounts() {
			// Force restored wallets on initial run to restore from genesis block.
			block := asset.Internal().LTC.ChainParams().GenesisBlock

			bs := waddrmgr.BlockStamp{
				Height:    0,
				Hash:      block.BlockHash(),
				Timestamp: block.Header.Timestamp,
			}

			// Setting the verification to true, requests the upstream not to
			// attempt checking for a better birthday block. This check causes
			// a crash if the optimum value identified by the upstream doesn't
			// match what was previously set.
			// Once the initial sync is complete, the system automatically sets
			// the most optimum birthday block. On premature exit if the
			// optimum will be available by then, its also set automatically.
			err := asset.Internal().LTC.Manager.SetBirthdayBlock(ns, bs, true)
			if err != nil {
				log.Errorf("Failed to set birthblock: %v", err)
			}
		}

		// never synced, forcing recovery from birthday block.
		return asset.Internal().LTC.Manager.SetSyncedTo(ns, nil)
	})
	if err != nil {
		log.Errorf("Failed to reset wallet manager sync height: %v", err)
		return
	}

	asset.syncData.mu.Lock()
	// Address recovery is triggered immediately after the chain
	// considers itself sync is complete and synced.
	asset.syncData.isRescan = true
	asset.syncData.mu.Unlock()

	// Trigger UI update showing ltc address recovery is in progress.
	// Its helps most when the wallet is synced but wallet recovery is running.
	asset.handleSyncUIUpdate()
}

// updateAssetBirthday updates the appropriate birthday and birthday block
// immediately after initial rescan is completed.
func (asset *Asset) updateAssetBirthday() {
	const op errors.Op = "updateAssetBirthday"

	txs, err := asset.getTransactionsRaw(0, 0, true)
	if err != nil {
		log.Error(errors.E(op, "getTransactionsRaw failed %v", err))
		// try updating birthday block on next startup.
		return
	}

	var block *waddrmgr.BlockStamp
	var birthdayBlockHeight int32

	if len(txs) > 0 {
		// handle wallets that have received or sent tx(s) i.e. have historical data.

		// txs are sorted from the newest to the oldest then pick the last tx (oldest).
		blockHeight := txs[len(txs)-1].BlockHeight
		if blockHeight == sharedW.UnminedTxHeight {
			// tx selected must be in mempool, use current best block height instead.
			blockHeight = asset.GetBestBlockHeight()
		}

		// select the block that is 10 blocks down the current. This is done
		// to have the rescan start a few blocks before the height where
		// relevant txs might be discovered. Wallet restoration starts at the block
		// immediately after the birthday block. This implies that our birthday
		// block can never hold any relavant txs otherwise the rescan will ignore them.
		birthdayBlockHeight = blockHeight - 10

		block, err = asset.getblockStamp(birthdayBlockHeight)
		if err != nil {
			log.Error(errors.E(op, "getblockStamp ", err))
			return
		}

		previousBirthdayblock, isverified, err := asset.getBirthdayBlock()
		if err != nil {
			log.Error(errors.E(op, "getBirthdayBlock failed %v", err))
			// continue with new birthday block update
		}

		if previousBirthdayblock == birthdayBlockHeight && isverified {
			// No need to set the same verified birthday again.
			return
		}

		log.Debugf("(%v) Setting the new Birthday Block=%v previous Birthday Block=%v",
			asset.GetWalletName(), birthdayBlockHeight, previousBirthdayblock)
	} else {
		// Handles wallets with no history.
		// The last synced block is set as the verified birthday block. Since this
		// wallet has no previous history, history before this synced block is not
		// of any significance to us therefore it can be ignore incases of future
		// wallet recovery.

		// query the current birthday block set for it to be verified below if it not.
		blockHeight, isverified, err := asset.getBirthdayBlock()
		if err != nil {
			log.Error(errors.E(op, "querying birthday block failed %v", err))
			// exit the verification on error
			return
		}

		syncedTo := asset.Internal().LTC.Manager.SyncedTo()
		birthdayBlockHeight = syncedTo.Height

		if blockHeight == birthdayBlockHeight && isverified {
			// Do not attempt to verify it again
			return
		}

		block, err = asset.getblockStamp(birthdayBlockHeight)
		if err != nil {
			log.Error(errors.E(op, err))
			return
		}
	}

	// At the wallet level update the new birthday chosen.
	asset.SetBirthday(block.Timestamp)

	// At the address manager level update the new birthday and birthday block chosen.
	err = walletdb.Update(asset.Internal().LTC.Database(), func(dbtx walletdb.ReadWriteTx) error {
		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)
		err := asset.Internal().LTC.Manager.SetBirthday(ns, block.Timestamp)
		if err != nil {
			return err
		}

		// Setting the verification to true, requests the upstream not to
		// attempt checking for a better birthday block. This check causes
		// a crash if the optimum value identified by the upstream doesn't
		// match what was previously set.
		// Once the initial sync is complete, the system automatically sets
		// the most optimum birthday block. On premature exit if the
		// optimum will be available by then, its also set automatically.
		return asset.Internal().LTC.Manager.SetBirthdayBlock(ns, *block, true)
	})

	if err != nil {
		log.Error(errors.E(op, "Updating the birthday block after initial sync failed: %v", err))
	}
}

// getBirthdayBlock returns the currently set birthday block.
func (asset *Asset) getBirthdayBlock() (int32, bool, error) {
	var birthdayblock int32
	var isverified bool
	err := walletdb.View(asset.Internal().LTC.Database(), func(dbtx walletdb.ReadTx) error {
		ns := dbtx.ReadBucket(wAddrMgrBkt)
		b, ok, err := asset.Internal().LTC.Manager.BirthdayBlock(ns)
		birthdayblock = b.Height
		isverified = ok
		return err
	})
	return birthdayblock, isverified, err
}

func (asset *Asset) updateRescanProgress(height int32) {
	if asset.syncData.rescanStartHeight == nil {
		asset.syncData.rescanStartHeight = &height
	}

	headersFetchedSoFar := float64(height - *asset.syncData.rescanStartHeight)
	if headersFetchedSoFar < 1 {
		headersFetchedSoFar = 1
	}

	remainingHeaders := float64(asset.GetBestBlockHeight() - height)
	if remainingHeaders < 1 {
		remainingHeaders = 1
	}

	allHeadersToFetch := headersFetchedSoFar + remainingHeaders

	rescanProgressReport := &sharedW.HeadersRescanProgressReport{
		CurrentRescanHeight: height,
		TotalHeadersToScan:  int32(allHeadersToFetch),
		WalletID:            asset.ID,
	}

	elapsedRescanTime := time.Now().Unix() - asset.syncData.rescanStartTime.Unix()
	rescanRate := headersFetchedSoFar / float64(rescanProgressReport.TotalHeadersToScan)

	rescanProgressReport.RescanProgress = int32((headersFetchedSoFar * 100) / allHeadersToFetch)
	estimatedTotalRescanTime := int64(math.Round(float64(elapsedRescanTime) / rescanRate))
	rescanProgressReport.RescanTimeRemaining = estimatedTotalRescanTime - elapsedRescanTime

	rescanProgressReport.GeneralSyncProgress = &sharedW.GeneralSyncProgress{
		TotalSyncProgress:         rescanProgressReport.RescanProgress,
		TotalTimeRemainingSeconds: rescanProgressReport.RescanTimeRemaining,
	}

	if asset.blocksRescanProgressListener != nil {
		asset.blocksRescanProgressListener.OnBlocksRescanProgress(rescanProgressReport)
	}
}

func (asset *Asset) getblockStamp(height int32) (*waddrmgr.BlockStamp, error) {
	startHash, err := asset.GetBlockHash(int64(height))
	if err != nil {
		return nil, fmt.Errorf("invalid block height provided: Error: %v", err)
	}

	block, err := asset.chainClient.GetBlock(startHash)
	if err != nil {
		return nil, fmt.Errorf("invalid block hash provided: Error: %v", err)
	}

	return &waddrmgr.BlockStamp{
		Hash:      block.BlockHash(),
		Height:    height,
		Timestamp: block.Header.Timestamp,
	}, nil
}
