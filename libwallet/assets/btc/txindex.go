package btc

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	w "github.com/btcsuite/btcwallet/wallet"
)

func (asset *BTCAsset) IndexTransactions() error {
	beginHeight, err := asset.GetWalletDataDb().ReadIndexingStartBlock()
	if err != nil {
		log.Errorf("[%d] Get tx indexing start point error: %v", asset.ID, err)
		return err
	}

	endHeight := asset.GetBestBlockHeight()

	startBlock := w.NewBlockIdentifierFromHeight(beginHeight)
	endBlock := w.NewBlockIdentifierFromHeight(endHeight)

	defer func() {
		count, err := asset.GetWalletDataDb().Count(utils.TxFilterAll, asset.RequiredConfirmations(), endHeight, &sharedW.Transaction{})
		if err != nil {
			log.Errorf("[%d] Post-indexing tx count error :%v", asset.ID, err)
		} else if count > 0 {
			log.Infof("[%d] Transaction index finished at %d, %d transaction(s) indexed in total", asset.ID, endHeight, count)
		}

		err = asset.GetWalletDataDb().SaveLastIndexPoint(endHeight)
		if err != nil {
			log.Errorf("[%d] Set tx index end block height error: ", asset.ID, err)
		}
	}()

	log.Infof("[%d] Indexing transactions start height: %d, end height: %d", asset.ID, beginHeight, endHeight)
	_, err = asset.Internal().BTC.GetTransactions(startBlock, endBlock, "", asset.syncCtx.Done())
	return err
}

func (asset *BTCAsset) reindexTransactions() error {
	err := asset.GetWalletDataDb().ClearSavedTransactions(&sharedW.Transaction{})
	if err != nil {
		return err
	}

	return asset.IndexTransactions()
}
