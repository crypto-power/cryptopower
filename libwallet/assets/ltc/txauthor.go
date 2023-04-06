package ltc

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

// NewUnsignedTx creates a new unsigned transaction.
func (asset *Asset) NewUnsignedTx(sourceAccountNumber int32, utxos []*sharedW.UnspentOutput) error {
	return utils.ErrLTCMethodNotImplemented("NewUnsignedTx")
}

// IsUnsignedTxExist returns true if an unsigned transaction exists.
func (asset *Asset) IsUnsignedTxExist() bool {
	return false
}

// AddSendDestination adds a destination address to the transaction.
// The amount to be sent to the address is specified in satoshi.
// If sendMax is true, the amount is ignored and the maximum amount is sent.
func (asset *Asset) AddSendDestination(address string, satoshiAmount int64, sendMax bool) error {
	return utils.ErrLTCMethodNotImplemented("AddSendDestination")
}

// Broadcast broadcasts the transaction to the network.
func (asset *Asset) Broadcast(privatePassphrase, transactionLabel string) ([]byte, error) {
	return nil, utils.ErrLTCMethodNotImplemented("Broadcast")
}

// ComputeTxSizeEstimation computes the estimated size of the final raw transaction.
func (asset *Asset) ComputeTxSizeEstimation(dstAddress string, utxos []*sharedW.UnspentOutput) (int, error) {
	return 0, utils.ErrLTCMethodNotImplemented("ComputeTxSizeEstimation")
}

// EstimateFeeAndSize estimates the fee and size of the transaction.
func (asset *Asset) EstimateFeeAndSize() (*sharedW.TxFeeAndSize, error) {
	return nil, utils.ErrLTCMethodNotImplemented("EstimateFeeAndSize")
}
