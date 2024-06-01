package eth

import (
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

func (asset *Asset) CountTransactions(txFilter int32) (int, error) {
	return -1, utils.ErrETHMethodNotImplemented("CountTransactions")
}

func (asset *Asset) GetTransactionRaw(txHash string) (*sharedW.Transaction, error) {
	return nil, utils.ErrETHMethodNotImplemented("GetTransactionRaw")
}

func (asset *Asset) TxMatchesFilter(tx *sharedW.Transaction, txFilter int32) bool {
	log.Error(utils.ErrETHMethodNotImplemented("TxMatchesFilter"))
	return false
}

// GetTransactionsRaw returns the transactions for the wallet.
// The offset is the height of start block and limit is number of blocks will take
// from offset to get transactions. it is not the start block and the end block, so we need to
// get all transactions then return transactions that match the input limit and offset.
// If offset and limit are 0, it will return all transactions
// If newestFirst is true, it will return transactions from newest to oldest
func (asset *Asset) GetTransactionsRaw(offset, limit, txFilter int32, newestFirst bool, txHashSearch string) ([]*sharedW.Transaction, error) {
	return nil, utils.ErrETHMethodNotImplemented("GetTransactionsRaw")
}

// func (asset *Asset) getTransactionsRaw(offset, limit int32, newestFirst bool) ([]*sharedW.Transaction, error) {
// 	return nil, utils.ErrETHMethodNotImplemented("GetTransactionsRaw")
// }
