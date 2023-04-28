package eth

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
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

func (asset *Asset) GetTransactionsRaw(offset, limit, txFilter int32, newestFirst bool) ([]sharedW.Transaction, error) {
	return nil, utils.ErrETHMethodNotImplemented("GetTransactionsRaw")
}
