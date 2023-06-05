package btc

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"github.com/btcsuite/btcwallet/wallet"
	sharedW "gitlab.com/cryptopower/cryptopower/libwallet/assets/wallet"
	"gitlab.com/cryptopower/cryptopower/libwallet/txhelper"
	"gitlab.com/cryptopower/cryptopower/libwallet/utils"
)

// txCache helps to cache the transactions fetched.
type txCache struct {
	blockHeight int32

	unminedTxs []sharedW.Transaction
	minedTxs   []sharedW.Transaction

	mu sync.RWMutex
}

// PublishUnminedTransactions publishes all unmined transactions to the network.
func (asset *Asset) PublishUnminedTransactions() error {
	if !asset.WalletOpened() {
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
		if err := asset.Internal().BTC.PublishTransaction(decodeTx, tx.Label); err != nil {
			return err
		}
	}
	return nil
}

// CountTransactions returns the total number of transactions for the wallet.
func (asset *Asset) CountTransactions(txFilter int32) (int, error) {
	if !asset.WalletOpened() {
		return -1, utils.ErrBTCNotInitialized
	}

	transactions, err := asset.filterTxs(0, 0, txFilter, true)
	return len(transactions), err
}

// GetTransactionRaw returns the transaction details for the given transaction hash.
func (asset *Asset) GetTransactionRaw(txHash string) (*sharedW.Transaction, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrBTCNotInitialized
	}

	transactions, err := asset.getTransactionsRaw(0, 0, true)
	for _, tx := range transactions {
		if tx.Hash == txHash {
			return &tx, nil
		}
	}
	return nil, err
}

// TxMatchesFilter checks if the transaction matches the given filter.
func (asset *Asset) TxMatchesFilter(_ *sharedW.Transaction, txFilter int32) bool {
	return txhelper.TxDirectionInvalid != asset.btcSupportedTxFilter(txFilter)
}

// GetTransactions returns the transactions for the wallet.
func (asset *Asset) GetTransactions(offset, limit, txFilter int32, newestFirst bool) (string, error) {
	if !asset.WalletOpened() {
		return "", utils.ErrBTCNotInitialized
	}

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

// GetTransactionsRaw returns the transactions for the wallet.
// The offset is the height of start block and limit is number of blocks will take
// from offset to get transactions. it is not the start block and the end block, so we need to
// get all transactions then return transactions that match the input limit and offset.
// If offset and limit are 0, it will return all transactions
// If newestFirst is true, it will return transactions from newest to oldest
func (asset *Asset) GetTransactionsRaw(offset, limit, txFilter int32, newestFirst bool) ([]sharedW.Transaction, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrBTCNotInitialized
	}

	transactions, err := asset.filterTxs(0, 0, txFilter, newestFirst)
	if err != nil {
		return nil, err
	}
	if offset == 0 && limit == 0 {
		return transactions, nil
	}
	totalTxs := int32(len(transactions))
	start := offset
	end := offset + limit
	if start >= totalTxs {
		return []sharedW.Transaction{}, nil
	}

	if end >= totalTxs {
		end = totalTxs
	}
	txs := transactions[start:end]
	return txs, nil
}

func (asset *Asset) btcSupportedTxFilter(txFilter int32) int32 {
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

// the offset is the height of start block
// limit is number of blocks will take from offset to get transactions
func (asset *Asset) filterTxs(offset, limit, txFilter int32, newestFirst bool) ([]sharedW.Transaction, error) {
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
func (asset *Asset) getTransactionsRaw(offset, limit int32, newestFirst bool) ([]sharedW.Transaction, error) {
	asset.txs.mu.RLock()
	allTxs := append(asset.txs.unminedTxs, asset.txs.minedTxs...)
	txCacheHeight := asset.txs.blockHeight
	asset.txs.mu.RUnlock()

	// if empty results were previously cached, check for updates.
	if txCacheHeight == asset.GetBestBlockHeight() && len(allTxs) > 0 {
		// if the best block hasn't changed return the preset list of txs.
		return allTxs, nil
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

	ctx := context.Background()
	if asset.syncCtx != nil {
		ctx = asset.syncCtx
	}
	loadedAsset := asset.Internal().BTC
	txResult, err := loadedAsset.GetTransactions(startBlock, endBlock, "", ctx.Done())
	if err != nil {
		return nil, err
	}

	unminedTxs := make([]sharedW.Transaction, 0)
	for _, transaction := range txResult.UnminedTransactions {
		unminedTx := asset.decodeTransactionWithTxSummary(sharedW.UnminedTxHeight, transaction)
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

func (asset *Asset) extractTxs(blocks []wallet.Block) []sharedW.Transaction {
	txs := make([]sharedW.Transaction, 0)
	for _, block := range blocks {
		for _, transaction := range block.Transactions {
			decodedTx := asset.decodeTransactionWithTxSummary(block.Height, transaction)
			txs = append(txs, decodedTx)
		}
	}
	return txs
}
