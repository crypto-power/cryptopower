package btc

import (
	"encoding/json"
	"sort"
	"sync"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/btcsuite/btcwallet/wallet"
)

// UnminedTxHeight defines the block height of the txs in the mempool
const UnminedTxHeight int32 = -1

// txCache helps to cache the transactions fetched.
type txCache struct {
	blockHeight int32

	unminedTxs []sharedW.Transaction
	minedTxs   []sharedW.Transaction

	mu sync.RWMutex
}

func (asset *BTCAsset) PublishUnminedTransactions() error {
	loadedAsset := asset.Internal().BTC
	if loadedAsset == nil {
		return utils.ErrBTCNotInitialized
	}

	// Triggers the update of txs in the mempool if they are outdated
	if _, err := asset.getTransactionsRaw(0, 0, true); err != nil {
		return err
	}

	asset.txs.mu.RLock()
	mempoolTxs := asset.txs.unminedTxs
	asset.txs.mu.RUnlock()

	for _, tx := range mempoolTxs {
		decodeTx, err := asset.decodeTxHex(tx.Hex)
		if err != err {
			return err
		}
		if err := loadedAsset.PublishTransaction(decodeTx, tx.Label); err != nil {
			return err
		}
	}
	return nil
}

func (asset *BTCAsset) CountTransactions(txFilter int32) (int, error) {
	transactions, err := asset.filterTxs(0, 0, txFilter, true)
	return len(transactions), err
}

func (asset *BTCAsset) GetTransactionRaw(txHash string) (*sharedW.Transaction, error) {
	transactions, err := asset.getTransactionsRaw(0, 0, true)
	for _, tx := range transactions {
		if tx.Hash == txHash {
			return &tx, nil
		}
	}
	return nil, err
}

func (asset *BTCAsset) TxMatchesFilter(_ *sharedW.Transaction, txFilter int32) bool {
	return txhelper.TxDirectionInvalid != asset.btcSupportedTxFilter(txFilter)
}

func (asset *BTCAsset) GetTransactions(offset, limit, txFilter int32, newestFirst bool) (string, error) {
	transactions, err := asset.filterTxs(offset, limit, txFilter, newestFirst)
	if err != nil {
		return "", err
	}

	jsonEncodedTransactions, err := json.Marshal(&transactions)
	if err != nil {
		return "", err
	}

	return string(jsonEncodedTransactions), nil
}

func (asset *BTCAsset) GetTransactionsRaw(offset, limit, txFilter int32,
	newestFirst bool) (transactions []sharedW.Transaction, err error) {
	transactions, err = asset.filterTxs(offset, limit, txFilter, newestFirst)
	return
}

func (asset *BTCAsset) btcSupportedTxFilter(txFilter int32) int32 {
	switch txFilter {
	case utils.TxFilterSent:
		return txhelper.TxDirectionSent
	case utils.TxFilterReceived:
		return txhelper.TxDirectionReceived
	case utils.TxFilterAll:
		return txhelper.TxDirectionAll
	default:
		return txhelper.TxDirectionInvalid
	}
}

func (asset *BTCAsset) filterTxs(offset, limit, txFilter int32, newestFirst bool) ([]sharedW.Transaction, error) {
	txType := asset.btcSupportedTxFilter(txFilter)
	transactions, err := asset.getTransactionsRaw(offset, limit, newestFirst)
	if err != nil {
		return []sharedW.Transaction{}, nil
	}

	if txType == txhelper.TxDirectionAll {
		return transactions, err
	}

	txsCopy := make([]sharedW.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		if tx.Direction == txType {
			txsCopy = append(txsCopy, tx)
		}
	}
	return txsCopy, nil
}

// getTransactionsRaw returns the transactions between the start block and the endblock.
// start block height is equal to the offset and endblock is equal to the summation
// of the offset and the limit values.
// If startblock is less that the endblock the list return is in ascending order
// (starts with the oldest) otherwise its in descending (starts with the newest) order.
func (asset *BTCAsset) getTransactionsRaw(offset, limit int32, newestFirst bool) ([]sharedW.Transaction, error) {
	asset.txs.mu.RLock()
	allTxs := append(asset.txs.unminedTxs, asset.txs.minedTxs...)
	txCacheHeight := asset.txs.blockHeight
	asset.txs.mu.RUnlock()

	if txCacheHeight == asset.GetBestBlockHeight() {
		// if the best block hasn't changed return the preset list of txs.
		return allTxs, nil
	}

	loadedAsset := asset.Internal().BTC
	if loadedAsset == nil {
		return nil, utils.ErrBTCNotInitialized
	}

	// if both offset and limit are each equal to zero, the transactions returned
	// include mempool contents and the mined txs.
	var startBlock, endBlock *wallet.BlockIdentifier
	if offset > 0 {
		if newestFirst { // Ascending order
			startBlock = wallet.NewBlockIdentifierFromHeight(offset)
		} else { // Descending Order
			endBlock = wallet.NewBlockIdentifierFromHeight(offset)
		}
	}

	// refHeight can be used as the start or endblock height depending on the order
	// required.
	refHeight := offset + limit
	if refHeight > 0 {
		if newestFirst { // Ascending order
			endBlock = wallet.NewBlockIdentifierFromHeight(refHeight)
		} else { // Descending Order
			startBlock = wallet.NewBlockIdentifierFromHeight(refHeight)
		}
	}

	txResult, err := loadedAsset.GetTransactions(startBlock, endBlock, "", asset.syncCtx.Done())
	if err != nil {
		return nil, err
	}

	unminedTxs := make([]sharedW.Transaction, 0)
	for _, transaction := range txResult.UnminedTransactions {
		unminedTx := asset.decodeTransactionWithTxSummary(UnminedTxHeight, transaction)
		unminedTxs = append(unminedTxs, unminedTx)
	}

	minedTxs := asset.extractTxs(txResult.MinedTransactions)

	if newestFirst {
		sort.Slice(unminedTxs, func(i, j int) bool {
			return unminedTxs[i].Timestamp > unminedTxs[j].Timestamp
		})
		sort.Slice(minedTxs, func(i, j int) bool {
			return minedTxs[i].Timestamp > minedTxs[j].Timestamp
		})
	}

	// Cache the recent data.
	asset.txs.mu.Lock()
	asset.txs.unminedTxs = unminedTxs
	asset.txs.minedTxs = minedTxs
	asset.txs.blockHeight = asset.GetBestBlockHeight()
	asset.txs.mu.Unlock()

	// Return the summation of unmined and the mined txs.
	return append(unminedTxs, minedTxs...), nil
}

func (asset *BTCAsset) extractTxs(blocks []wallet.Block) []sharedW.Transaction {
	txs := make([]sharedW.Transaction, 0)
	for _, block := range blocks {
		for _, transaction := range block.Transactions {
			decodedTx := asset.decodeTransactionWithTxSummary(block.Height, transaction)
			txs = append(txs, decodedTx)
		}
	}
	return txs
}
