package btc

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

func (asset *BTCAsset) SetBlocksRescanProgressListener(blocksRescanProgressListener sharedW.BlocksRescanProgressListener) {
	asset.syncInfo.blocksRescanProgressListener = blocksRescanProgressListener
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

	return asset.rescanBlocks(block.Hash(), nil, nil)
}

func (asset *BTCAsset) rescanBlocks(startHash *chainhash.Hash, addrs []btcutil.Address,
	outPoints map[wire.OutPoint]btcutil.Address) error {
	if asset.IsRescanning() || !asset.IsSynced() {
		return errors.E(utils.ErrInvalid)
	}

	if startHash == nil {
		return errors.New("block hash from where to start rescanning must be provided")
	}

	if addrs == nil {
		addrs = make([]btcutil.Address, 0)
	}

	if outPoints == nil {
		outPoints = make(map[wire.OutPoint]btcutil.Address)
	}

	asset.syncInfo.mu.Lock()
	asset.syncInfo.isRescan = false
	asset.syncInfo.mu.Unlock()

	go asset.chainClient.Rescan(startHash, addrs, outPoints)

	err := asset.chainClient.NotifyBlocks()
	if err != nil {
		return err
	}

	go asset.handleNotifications()
	return nil
}

func (asset *BTCAsset) IsRescanning() bool {
	asset.syncInfo.mu.RLock()
	defer asset.syncInfo.mu.RUnlock()

	return asset.syncInfo.isRescan
}

func (asset *BTCAsset) CancelRescan() {
	asset.syncInfo.mu.Lock()
	asset.syncInfo.isRescan = false
	asset.syncInfo.mu.Unlock()

	asset.chainClient.Stop()
}
