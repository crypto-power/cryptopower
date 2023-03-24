package btc

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/waddrmgr"
	w "github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
)

// SetBlocksRescanProgressListener sets the blocks rescan progress listener.
func (asset *Asset) SetBlocksRescanProgressListener(blocksRescanProgressListener sharedW.BlocksRescanProgressListener) {
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

func (asset *Asset) rescanBlocks(startHeight int32, addrs []btcutil.Address) error {
	if !asset.IsConnectedToBitcoinNetwork() {
		return errors.E(utils.ErrNotConnected)
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
		addrs = []btcutil.Address{}
	}

	asset.syncData.mu.Lock()
	asset.syncData.isRescan = true
	asset.syncData.rescanStartTime = time.Now()
	asset.syncData.mu.Unlock()

	job := &w.RescanJob{
		Addrs:      addrs,
		OutPoints:  nil,
		BlockStamp: *bs,
	}

	// It submits a rescan job without blocking on finishing the rescan.
	// The rescan success or failure is logged elsewhere, and the channel
	// is not required to be read, so discard the return value.
	errChan := asset.Internal().BTC.SubmitRescan(job)

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

// RescanAsync initiates a full wallet recovery (used address discovery
// and transaction scanning) by stopping the btcwallet, dropping the transaction
// history from the wallet db, resetting the synced-to height of the wallet
// manager, restarting the wallet and its chain client, and finally commanding
// the wallet to resynchronize, which starts asynchronous wallet recovery.
// Progress of the rescan should be monitored with syncStatus. During the rescan
// wallet balances and known transactions may not be reported accurately or
// located. The SPVService is not stopped, so most spvWallet methods will
// continue to work without error, but methods using the btcWallet will likely
// return incorrect results or errors.
func (asset *Asset) RescanAsync() error {
	if !atomic.CompareAndSwapUint32(&asset.rescanStarting, 0, 1) {
		log.Error("rescan already in progress")
		return fmt.Errorf("rescan already in progress")
	}

	defer atomic.StoreUint32(&asset.rescanStarting, 0)

	log.Info("Stopping wallet and chain client...")

	asset.Internal().BTC.Stop() // stops Wallet and chainClient (not chainService)
	asset.Internal().BTC.WaitForShutdown()
	asset.chainClient.WaitForShutdown()

	// Attempt to drop the the tx history. See the btcwallet/cmd/dropwtxmgr app
	// for more information. Because of how often a forces rescan will be triggered,
	// dropping the transaction history in every one of those ocassions won't make
	// much difference. Its recommended that on the manually triggered rescan that
	// is when dropping transaction history can be done.
	log.Infof("(%v) Dropping transaction history to perform full rescan...", asset.GetWalletName())

	err := w.DropTransactionHistory(asset.Internal().BTC.Database(), false)
	if err != nil {
		log.Errorf("Failed to drop wallet transaction history: %v", err)
		// continue with the rescan despite the error occuring
	}

	asset.ForceRescan()

	log.Info("Starting wallet...")
	asset.Internal().BTC.Start()

	if err := asset.chainClient.Start(); err != nil {
		return fmt.Errorf("couldn't start Neutrino client: %v", err)
	}

	log.Infof("Synchronizing wallet (%s) with network...", asset.GetWalletName())
	asset.Internal().BTC.SynchronizeRPC(asset.chainClient)
	return nil
}

// ForceRescan forces a full rescan with active address discovery on wallet
// restart by setting the "synced to" field to nil.
func (asset *Asset) ForceRescan() {
	wdb := asset.Internal().BTC.Database()
	err := walletdb.Update(wdb, func(dbtx walletdb.ReadWriteTx) error {
		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)

		if asset.IsRestored && !asset.ContainsDiscoveredAccounts() {
			// Force restored wallets on initial run to restore from genesis block.
			block := asset.Internal().BTC.ChainParams().GenesisBlock

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
			err := asset.Internal().BTC.Manager.SetBirthdayBlock(ns, bs, true)
			if err != nil {
				log.Errorf("Failed to set birthblock: %v", err)
			}
		}

		// never synced, forcing recovery from birthday block.
		return asset.Internal().BTC.Manager.SetSyncedTo(ns, nil)
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

	// Trigger UI update showing btc address recovery is in progress.
	// Its helps most when the wallet is synced but wallet recovery is running.
	asset.handleSyncUIUpdate()
}

// isRecoveryRequired scans if the current wallet requires a recovery. Starting
// a rescan leads to the recovery of funds and utxos scanned from the birthday block.
// If the the address manager is not synced to the last two new blocks detected,
// Or birthday mismatch exists, a rescan is initiated.
func (asset *Asset) isRecoveryRequired() bool {
	// Last block synced to the address manager.
	syncedTo := asset.Internal().BTC.Manager.SyncedTo()
	// Address manager should be synced to one of the blocks in the last 10 blocks,
	// from the current best block synced. Otherwise recovery will be triggered.
	isAddrmngNotSynced := !(syncedTo.Height >= asset.GetBestBlockHeight()-10)

	walletBirthday := asset.Internal().BTC.Manager.Birthday()
	isBirthdayMismatch := !asset.GetBirthday().Equal(walletBirthday)

	return isAddrmngNotSynced || isBirthdayMismatch
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

		syncedTo := asset.Internal().BTC.Manager.SyncedTo()
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
	err = walletdb.Update(asset.Internal().BTC.Database(), func(dbtx walletdb.ReadWriteTx) error {
		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)
		err := asset.Internal().BTC.Manager.SetBirthday(ns, block.Timestamp)
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
		return asset.Internal().BTC.Manager.SetBirthdayBlock(ns, *block, true)
	})

	if err != nil {
		log.Error(errors.E(op, "Updating the birthday block after initial sync failed: %v", err))
	}
}

// getBirthdayBlock returns the currently set birthday block.
func (asset *Asset) getBirthdayBlock() (int32, bool, error) {
	var birthdayblock int32
	var isverified bool
	err := walletdb.View(asset.Internal().BTC.Database(), func(dbtx walletdb.ReadTx) error {
		ns := dbtx.ReadBucket(wAddrMgrBkt)
		b, ok, err := asset.Internal().BTC.Manager.BirthdayBlock(ns)
		birthdayblock = b.Height
		isverified = ok
		return err
	})
	return birthdayblock, isverified, err
}

func (asset *Asset) updateRescanProgress(progress *chain.RescanProgress) {
	if asset.syncData.rescanStartHeight == nil {
		asset.syncData.rescanStartHeight = &progress.Height
	}

	headersFetchedSoFar := progress.Height - *asset.syncData.rescanStartHeight
	if headersFetchedSoFar < 1 {
		headersFetchedSoFar = 1
	}

	remainingHeaders := asset.GetBestBlockHeight() - progress.Height
	if remainingHeaders < 1 {
		remainingHeaders = 1
	}

	allHeadersToFetch := headersFetchedSoFar + remainingHeaders

	rescanProgressReport := &sharedW.HeadersRescanProgressReport{
		CurrentRescanHeight: progress.Height,
		TotalHeadersToScan:  allHeadersToFetch,
		WalletID:            asset.ID,
	}

	elapsedRescanTime := time.Now().Unix() - asset.syncData.rescanStartTime.Unix()
	rescanRate := float64(headersFetchedSoFar) / float64(rescanProgressReport.TotalHeadersToScan)

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

// updateSyncedToBlock is used to update syncedTo block. Sometimes btcwallet might
// miss the trigger event to update syncedTo block so the update is done here
// regardless thus avoid handling the possible scenario where btcwallet might miss
// the syncedto store trigger event.
func (asset *Asset) updateSyncedToBlock(height int32) {
	// Ignore blocks notifications recieved during the wallet recovery phase.
	if !asset.IsSynced() || asset.IsRescanning() {
		return
	}

	err := walletdb.Update(asset.Internal().BTC.Database(), func(dbtx walletdb.ReadWriteTx) error {
		addrmgrNs := dbtx.ReadWriteBucket(wAddrMgrBkt)

		bs, err := asset.getblockStamp(height)
		if err != nil {
			return err
		}

		return asset.Internal().BTC.Manager.SetSyncedTo(addrmgrNs, bs)
	})
	if err != nil {
		log.Errorf("updating syncedTo block failed: Error: %v", err)
	}
}
