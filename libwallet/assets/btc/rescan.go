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
	if !asset.IsConnectedToBitcoinNetwork() {
		return errors.E(utils.ErrNotConnected)
	}

	if asset.IsRescanning() {
		return errors.E(utils.ErrSyncAlreadyInProgress)
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

	go func() {
		err := asset.chainClient.NotifyReceived(addrs)
		if err != nil {
			log.Error(err)
		}
	}()

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
func (asset *BTCAsset) ForceRescan() {
	// Forcing rescan on a wallet that is not sync will lead to crash and potentially unusable wallet.
	if !asset.IsSynced() {
		return
	}

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
	}
}

// isRecoveryRequired scans if the current wallet requires a recovery. Starting
// a rescan leads to the recovery of funds and utxos scanned from the birthday block.
// If the the address manager is not synced to the last two new blocks detected,
// Or birthday mismatch exists, a rescan is initiated.
func (asset *BTCAsset) isRecoveryRequired() bool {
	// Last block synced to the address manager.
	syncedTo := asset.Internal().BTC.Manager.SyncedTo()
	// Address manager should be synced to the previous block, otherwise rescan
	// maybe triggered. Previous block => (one block behind the current best block)
	isAddrmngNotSynced := !(syncedTo.Height >= asset.GetBestBlockHeight()-1)

	walletBirthday := asset.Internal().BTC.Manager.Birthday()
	isBirthdayMismatch := !asset.GetBirthday().Equal(walletBirthday)

	return isAddrmngNotSynced || isBirthdayMismatch
}

// updateAssetBirthday updates the appropriate birthday and birthday block
// immediately after initial rescan is completed.
func (asset *BTCAsset) updateAssetBirthday() {
	const op errors.Op = "updateAssetBirthday"

	txs, err := asset.getTransactionsRaw(0, 0, true)
	if err != nil {
		log.Error(errors.E(op, "getTransactionsRaw failed %v", err))
		// try updating birthday block on next startup.
		return
	}

	// Only update the wallet birthday and birthdayblock for the wallet that
	// have received or sent tx(s).
	if len(txs) > 0 {
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
		birthdayBlockHeight := blockHeight - 10

		hash, err := asset.chainClient.GetBlockHash(int64(birthdayBlockHeight))
		if err != nil {
			log.Error(errors.E(op, "GetBlockHash failed %v", err))
			return
		}

		block, err := asset.chainClient.GetBlock(hash)
		if err != nil {
			log.Error(errors.E(op, "GetBlock failed %v", err))
			return
		}

		previousBirthdayblock, _, err := asset.getBirthdayBlock()
		if err != nil {
			log.Error(errors.E(op, "getBirthdayBlock failed %v", err))
			// continue with new birthday block update
		}

		if previousBirthdayblock == birthdayBlockHeight {
			// No need to set the same birthday again.
			return
		}

		log.Debugf("(%v) Setting the new Birthday Block=%v previous Birthday Block=%v",
			asset.GetWalletName(), birthdayBlockHeight, previousBirthdayblock)

		// At the wallet level update the new birthday chosen.
		asset.SetBirthday(block.Header.Timestamp)

		// At the address manager level update the new birthday and birthday block chosen.
		err = walletdb.Update(asset.Internal().BTC.Database(), func(dbtx walletdb.ReadWriteTx) error {
			ns := dbtx.ReadWriteBucket(wAddrMgrBkt)
			err := asset.Internal().BTC.Manager.SetBirthday(ns, block.Header.Timestamp)
			if err != nil {
				return err
			}

			birthdayBlock := waddrmgr.BlockStamp{
				Hash:      *hash,
				Height:    birthdayBlockHeight,
				Timestamp: block.Header.Timestamp,
			}

			// Setting the verification to true, requests the upstream not to
			// attempt checking for a better birthday block. This check causes
			// a crash if the optimum value identified by the upstream doesn't
			// match what was previously set.
			// Once the initial sync is complete, the system automatically sets
			// the most optimum birthday block. On premature exit if the
			// optimum will be available by then, its also set automatically.
			return asset.Internal().BTC.Manager.SetBirthdayBlock(ns, birthdayBlock, true)
		})

		if err != nil {
			log.Error(errors.E(op, "Updating the birthday block after initial sync failed: %v", err))
		}
	}
}

// getBirthdayBlock returns the currently set birthday block.
func (asset *BTCAsset) getBirthdayBlock() (int32, bool, error) {
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
