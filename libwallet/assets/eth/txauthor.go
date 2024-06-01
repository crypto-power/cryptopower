package eth

import (
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

func (asset *Asset) PublishUnminedTransactions() error {
	return utils.ErrETHMethodNotImplemented("PublishUnminedTransactions")
}

func (asset *Asset) NewUnsignedTx(accountNumber int32, utxos []*sharedW.UnspentOutput) error {
	return utils.ErrETHMethodNotImplemented("NewUnsignedTx")
}

func (asset *Asset) AddSendDestination(id int, address string, unitAmount int64, sendMax bool) error {
	return utils.ErrETHMethodNotImplemented("AddSendDestination")
}

// RemoveSendDestination removes a destination address from the transaction.
func (asset *Asset) RemoveSendDestination(id int) {
	log.Error(utils.ErrETHMethodNotImplemented("RemoveSendDestination"))
}

func (asset *Asset) ComputeTxSizeEstimation(dstAddress string, utxos []*sharedW.UnspentOutput) (int, error) {
	return -1, utils.ErrETHMethodNotImplemented("ComputeTxSizeEstimation")
}

func (asset *Asset) Broadcast(passphrase, label string) ([]byte, error) {
	return nil, utils.ErrETHMethodNotImplemented("Broadcast")
}

func (asset *Asset) EstimateFeeAndSize() (*sharedW.TxFeeAndSize, error) {
	return nil, utils.ErrETHMethodNotImplemented("EstimateFeeAndSize")
}

func (asset *Asset) IsUnsignedTxExist() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsUnsignedTxExist"))
	return false
}

// SendDestination returns a list of all destination addresses added to the transaction.
func (asset *Asset) SendDestination(id int) *sharedW.TransactionDestination {
	log.Error(utils.ErrETHMethodNotImplemented("SendDestination"))
	return nil
}

func (asset *Asset) UpdateSendDestination(id int, address string, atomAmount int64, sendMax bool) error {
	return utils.ErrETHMethodNotImplemented("UpdateSendDestination")
}
