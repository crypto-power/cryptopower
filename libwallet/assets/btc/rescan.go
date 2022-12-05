package btc

import (
	"fmt"
	"sync/atomic"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcwallet/waddrmgr"
	w "github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
)

func (asset *BTCAsset) SetBlocksRescanProgressListener(blocksRescanProgressListener sharedW.BlocksRescanProgressListener) {
	asset.blocksRescanProgressListener = blocksRescanProgressListener
}

func (asset *BTCAsset) RescanBlocks() error {
	return asset.RescanBlocksFromHeight(0)
}

func (asset *BTCAsset) RescanBlocksFromHeight(startHeight int32) error {
	hash, err := asset.GetBlockHash(int64(startHeight))
	if err != nil {
		return err
	}

	return asset.rescanBlocks(hash, nil)
}

func (asset *BTCAsset) rescanBlocks(startHash *chainhash.Hash, addrs []btcutil.Address) error {
	if asset.IsRescanning() || !asset.IsSynced() {
		return errors.E(utils.ErrInvalid)
	}

	if startHash == nil {
		return errors.New("block hash from where to start rescanning must be provided")
	}

	if addrs == nil {
		addrs = []btcutil.Address{}
	}

	asset.syncData.mu.Lock()
	asset.syncData.isRescan = true
	asset.syncData.mu.Unlock()

	err := asset.chainClient.NotifyReceived(addrs)
	if err != nil {
		return err
	}

	// Attempt to start up the notifications handler.
	if atomic.CompareAndSwapUint32(&asset.syncData.syncstarted, stop, start) {
		go asset.handleNotifications()
	}

	return nil
}

func (asset *BTCAsset) IsRescanning() bool {
	asset.syncData.mu.RLock()
	defer asset.syncData.mu.RUnlock()

	return asset.syncData.isRescan
}

func (asset *BTCAsset) CancelRescan() {
	asset.syncData.mu.Lock()
	asset.syncData.isRescan = false
	asset.syncData.mu.Unlock()

	asset.chainClient.Stop()
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
func (asset *BTCAsset) RescanAsync() error {
	if !atomic.CompareAndSwapUint32(&asset.rescanStarting, 0, 1) {
		log.Error("rescan already in progress")
		return fmt.Errorf("rescan already in progress")
	}
	defer atomic.StoreUint32(&asset.rescanStarting, 0)
	log.Info("Stopping wallet and chain client...")
	asset.Internal().BTC.Stop() // stops Wallet and chainClient (not chainService)
	asset.Internal().BTC.WaitForShutdown()
	asset.chainClient.WaitForShutdown()

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
// restart by dropping the complete transaction history and setting the
// "synced to" field to nil.
func (asset *BTCAsset) ForceRescan() {
	wdb := asset.Internal().BTC.Database()

	// Attempt tp drop the the tx history.
	// asset.dropTxHistory()

	err := walletdb.Update(wdb, func(dbtx walletdb.ReadWriteTx) error {
		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)                  // it'll be fine
		return asset.Internal().BTC.Manager.SetSyncedTo(ns, nil) // never synced, forcing recovery from birthday
	})
	if err != nil {
		log.Errorf("Failed to reset wallet manager sync height: %v", err)
	}
}

// dropTxHistory deletes the txs history. See the btcwallet/cmd/dropwtxmgr app
// for more information.
func (asset *BTCAsset) dropTxHistory() error {
	log.Infof("(%v) Dropping transaction history to perform full rescan...", asset.GetWalletName())

	err := w.DropTransactionHistory(asset.Internal().BTC.Database(), false)
	if err != nil {
		log.Errorf("Failed to drop wallet transaction history: %v", err)
	}
	return err
}

// performRescan scans if the current wallet requires a recovery. Starting a rescan
// leads to the recovery of funds and utxos scanned from the birthday block.
// If the the address manager is not synced to the last two new blocks detected,
// Or birthday mismatch exists, a rescan is initiated.
func (asset *BTCAsset) performRescan() bool {
	// Last block synced to the address manager.
	syncedTo := asset.Internal().BTC.Manager.SyncedTo()
	// Address manager should be synced to the previous block, otherwise rescan
	// maybe triggered. Previous block => (one block behind the current best block)
	isAddrmngNotSynced := !(syncedTo.Height >= asset.GetBestBlockHeight()-1)
	fmt.Printf("(%v) Synced To: %v Previous Block: %v \n", asset.GetWalletName(), syncedTo.Height, asset.GetBestBlockHeight()-1)

	walletBirthday := asset.Internal().BTC.Manager.Birthday()
	isBirthdayMismatch := !asset.GetBirthday().Equal(walletBirthday)
	fmt.Printf("(%v) Is birthday Match: %v \n", asset.GetWalletName(), asset.GetBirthday().Equal(walletBirthday))

	return isAddrmngNotSynced || isBirthdayMismatch
}

// updateAssetBirthday updates the appropriate birthday immediately after
// initial rescan is completed. The appropriate birthday value is comparison
// between the wallet creation date and its earliest use to send or receive a tx
// whichever happened first.
func (asset *BTCAsset) updateAssetBirthday() {
	txs, err := asset.Internal().BTC.ListAllTransactions()
	if err != nil {
		log.Debugf(" ListAllTransactions() failed %v", err)
	}

	// Only update the wallet birthday and birthdayblock from the wallets that
	// have received tx.
	if len(txs) > 0 {
		// Since txs returned are ordered from the newest to the oldest, Use the last tx.
		lastTx := txs[len(txs)-1]

		blockHeight := asset.GetBestBlockHeight()
		if lastTx.BlockHeight != nil {
			// tx selected must be is in mempool, use current best height instead.
			blockHeight = *lastTx.BlockHeight
		}

		// select the block that is 10 blocks down the current. Thi
		birthdayBlockHeight := blockHeight - 10

		hash, err := asset.chainClient.GetBlockHash(int64(birthdayBlockHeight))
		if err != nil {
			log.Errorf("Update birthdayBlock: GetBlockHash failed %v", err)
			return
		}

		block, err := asset.chainClient.GetBlock(hash)
		if err != nil {
			log.Errorf("Update birthdayBlock: GetBlock failed %v", err)
			return
		}

		previousBirthdayblock, _, err := asset.getBirthdayBlock()
		if err != nil {
			log.Errorf("Update birthdayBlock: getBirthdayBlock failed %v", err)
			// continue with new birthday block setting
		}

		currentBirthday := block.Header.Timestamp
		log.Infof("(%v) Setting the new Birthday Block=%v previous Birthday Block=%v",
			asset.GetWalletName(), birthdayBlockHeight, previousBirthdayblock)

		// At the wallet level update the new birthday choosen.
		asset.SetBirthday(currentBirthday)

		// At the address manager level update the new birthday anf birthday block choosen
		err = walletdb.Update(asset.Internal().BTC.Database(), func(dbtx walletdb.ReadWriteTx) error {
			ns := dbtx.ReadWriteBucket(wAddrMgrBkt)
			err := asset.Internal().BTC.Manager.SetBirthday(ns, currentBirthday)
			if err != nil {
				return err
			}

			birthdayBlock := waddrmgr.BlockStamp{
				Hash:      *hash,
				Height:    birthdayBlockHeight,
				Timestamp: currentBirthday,
			}

			return asset.Internal().BTC.Manager.SetBirthdayBlock(ns, birthdayBlock, false)
		})

		if err != nil {
			log.Errorf("Updating the birthday block after initial sync failed: %v", err)
		}
	}
}

// getBirthdayBlock returns the currently set birthday block.
func (asset *BTCAsset) getBirthdayBlock() (int32, bool, error) {
	var birthdayblock int32
	var isverified bool
	err := walletdb.Update(asset.Internal().BTC.Database(), func(dbtx walletdb.ReadWriteTx) error {
		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)
		b, ok, err := asset.Internal().BTC.Manager.BirthdayBlock(ns)
		birthdayblock = b.Height
		isverified = ok
		return err
	})
	return birthdayblock, isverified, err
}
