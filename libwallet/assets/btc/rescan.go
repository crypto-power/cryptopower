package btc

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/chain"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

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

	asset.chainClient.SetStartTime(block.MsgBlock().Header.Timestamp)

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

	asset.mu.Lock()
	asset.isRescan = true
	asset.mu.Unlock()

	err := asset.chainClient.Rescan(startHash, addrs, outPoints)

	go asset.fetchNotifications()

	asset.mu.Lock()
	asset.isRescan = false
	asset.mu.Unlock()

	return err
}

func (asset *BTCAsset) fetchNotifications() {

	for {
		select {
		case n, ok := <-asset.chainClient.Notifications():
			if !ok {
				return
			}
			// var notificationName string
			// var err error
			switch n.(type) {
			case chain.ClientConnected:
				fmt.Println(" >>>>>>>>>>>>>>>>>>>>>>>>>> client connected")
			case chain.BlockConnected:
				fmt.Println(" >>>>>>>>>>>>>>>>>>>>>>>>>> block connected")
			case chain.BlockDisconnected:
				fmt.Println(" >>>>>>>>>>>>>>>>>>>>>>>>>> block disconnected")
			case chain.RelevantTx:
				fmt.Println(" >>>>>>>>>>>>>>>>>>>>>>>>>> relevant tx")
			case chain.FilteredBlockConnected:
				fmt.Println(" >>>>>>>>>>>>>>>>>>>>>>>>>> filtered block connected")
			case *chain.RescanProgress:
				fmt.Println(" >>>>>>>>>>>>>>>>>>>>>>>>>> rescan progress")
			case *chain.RescanFinished:
				fmt.Println(" >>>>>>>>>>>>>>>>>>>>>>>>>> rescan finished")
			}
		case <-asset.quit:
			return
		}
	}
	// for notifications := range  {
	// 	switch t := notifications.(type) {
	// 	default:
	// 		d, _ := json.MarshalIndent(notifications, " ", " ")
	// 		fmt.Println(" >>>>>>>>>>>>>>>>>>>>>>>>>> ", string(d), t)
	// 	}
	// }
}

func (asset *BTCAsset) IsRescanning() bool {
	return asset.isRescan
}

func (asset *BTCAsset) CancelRescan() {
	asset.chainClient.Stop()
}

func (asset *BTCAsset) SetBlocksRescanProgressListener(blocksRescanProgressListener sharedW.BlocksRescanProgressListener) {
	asset.blocksRescanProgressListener = blocksRescanProgressListener
}
