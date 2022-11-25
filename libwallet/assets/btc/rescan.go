package btc

import (
	"fmt"
	"sync/atomic"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
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

	block, err := asset.chainService.GetBlock(*hash)
	if err != nil {
		return err
	}

	return asset.rescanBlocks(block.Hash(), nil)
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

	go func() {
		// Attempt to start up the notifications handler.
		if atomic.CompareAndSwapInt32(&asset.syncData.syncstarted, stop, start) {
			asset.handleNotifications()
		}
	}()

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
// "synced to" field to nil. See the btcwallet/cmd/dropwtxmgr app for more
// information.
func (asset *BTCAsset) ForceRescan() {
	wdb := asset.Internal().BTC.Database()

	log.Info("Dropping transaction history to perform full rescan...")
	err := w.DropTransactionHistory(wdb, false)
	if err != nil {
		log.Errorf("Failed to drop wallet transaction history: %v", err)
		// Continue to attempt restarting the wallet anyway.
	}

	err = walletdb.Update(wdb, func(dbtx walletdb.ReadWriteTx) error {
		ns := dbtx.ReadWriteBucket(wAddrMgrBkt)                  // it'll be fine
		return asset.Internal().BTC.Manager.SetSyncedTo(ns, nil) // never synced, forcing recovery from birthday
	})
	if err != nil {
		log.Errorf("Failed to reset wallet manager sync height: %v", err)
	}
}
