package dcr

import (
	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/chaincfg/chainhash"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/assets/wallet/walletdata"
)

func (asset *DCRAsset) IndexTransactions() error {
	ctx, _ := asset.ShutdownContextWithCancel()

	var totalIndex int32
	var txEndHeight uint32
	rangeFn := func(block *w.Block) (bool, error) {
		for _, transaction := range block.Transactions {

			var blockHash *chainhash.Hash
			if block.Header != nil {
				hash := block.Header.BlockHash()
				blockHash = &hash
			} else {
				blockHash = nil
			}

			tx, err := asset.decodeTransactionWithTxSummary(&transaction, blockHash)
			if err != nil {
				return false, err
			}

			_, err = asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, tx)
			if err != nil {
				log.Errorf("[%d] Index tx replace tx err : %v", asset.ID, err)
				return false, err
			}

			totalIndex++
		}

		if block.Header != nil {
			txEndHeight = block.Header.Height
			err := asset.GetWalletDataDb().SaveLastIndexPoint(int32(txEndHeight))
			if err != nil {
				log.Errorf("[%d] Set tx index end block height error: ", asset.ID, err)
				return false, err
			}

			log.Debugf("[%d] Index saved for transactions in block %d", asset.ID, txEndHeight)
		}

		select {
		case <-ctx.Done():
			return true, ctx.Err()
		default:
			return false, nil
		}
	}

	beginHeight, err := asset.GetWalletDataDb().ReadIndexingStartBlock()
	if err != nil {
		log.Errorf("[%d] Get tx indexing start point error: %v", asset.ID, err)
		return err
	}

	endHeight := asset.GetBestBlockHeight()

	startBlock := w.NewBlockIdentifierFromHeight(beginHeight)
	endBlock := w.NewBlockIdentifierFromHeight(endHeight)

	defer func() {
		count, err := asset.GetWalletDataDb().Count(walletdata.TxFilterAll, asset.RequiredConfirmations(), endHeight, &sharedW.Transaction{})
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
	return asset.Internal().DCR.GetTransactions(ctx, rangeFn, startBlock, endBlock)
}

func (asset *DCRAsset) reindexTransactions() error {
	err := asset.GetWalletDataDb().ClearSavedTransactions(&sharedW.Transaction{})
	if err != nil {
		return err
	}

	return asset.IndexTransactions()
}
