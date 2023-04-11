package ltc

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

// PublishUnminedTransactions publishes all unmined transactions to the network.
func (asset *Asset) PublishUnminedTransactions() error {
	return utils.ErrLTCMethodNotImplemented("PublishUnminedTransactions")
}

// CountTransactions returns the total number of transactions for the wallet.
func (asset *Asset) CountTransactions(txFilter int32) (int, error) {
	return 0, utils.ErrLTCMethodNotImplemented("CountTransactions")
}

// TxMatchesFilter checks if the transaction matches the given filter.
func (asset *Asset) TxMatchesFilter(_ *sharedW.Transaction, txFilter int32) bool {
	log.Error(utils.ErrLTCMethodNotImplemented("TxMatchesFilter"))
	return false
}

// GetTransactionRaw returns the transaction details for the given transaction hash.
func (asset *Asset) GetTransactionRaw(txHash string) (*sharedW.Transaction, error) {
	return nil, utils.ErrLTCMethodNotImplemented("GetTransactionRaw")
}

// GetTransactionsRaw returns the transactions for the wallet.
// The offset is the height of start block and limit is number of blocks will take
// from offset to get transactions. it is not the start block and the end block, so we need to
// get all transactions then return transactions that match the input limit and offset.
// If offset and limit are 0, it will return all transactions
// If newestFirst is true, it will return transactions from newest to oldest
func (asset *Asset) GetTransactionsRaw(offset, limit, txFilter int32,
	newestFirst bool) ([]sharedW.Transaction, error) {
	return nil, utils.ErrLTCMethodNotImplemented("GetTransactionsRaw")
}
