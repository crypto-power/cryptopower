package eth

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

func (asset *Asset) PublishUnminedTransactions() error {
	return utils.ErrETHMethodNotImplemented("PublishUnminedTransactions")
}

func (asset *Asset) NewUnsignedTx(accountNumber int32, utxos []*sharedW.UnspentOutput) error {
	return utils.ErrETHMethodNotImplemented("NewUnsignedTx")
}

func (asset *Asset) AddSendDestination(address string, unitAmount int64, sendMax bool) error {
	return utils.ErrETHMethodNotImplemented("AddSendDestination")
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
