package ltc

import (
	"fmt"

	"github.com/ltcsuite/ltcd/blockchain"
	"github.com/ltcsuite/ltcd/ltcutil"
	w "github.com/ltcsuite/ltcwallet/wallet"
	sharedW "gitlab.com/cryptopower/cryptopower/libwallet/assets/wallet"
	"gitlab.com/cryptopower/cryptopower/libwallet/txhelper"
)

func (asset *Asset) decodeTransactionWithTxSummary(blockheight int32, txsummary w.TransactionSummary) sharedW.Transaction {
	txHex := fmt.Sprintf("%x", txsummary.Transaction)
	decodedTx, _ := asset.decodeTxHex(txHex)
	txSize := decodedTx.SerializeSize()

	// TODO: Check why tx fee returned is zero despite int not being zero on the explorer
	feeRate := txsummary.Fee * 1000 / ltcutil.Amount(txSize)

	// LTC transactions are either coinbase or regular txs.
	txType := txhelper.TxTypeRegular
	if blockchain.IsCoinBaseTx(decodedTx) {
		txType = txhelper.TxTypeCoinBase
	}

	inputs, totalInputsAmount := asset.decodeTxInputs(decodedTx, txsummary.MyInputs)
	outputs, totalOutputsAmount := asset.decodeTxOutputs(decodedTx, txsummary.MyOutputs)
	amount, direction := txhelper.TransactionAmountAndDirection(totalInputsAmount, totalOutputsAmount, int64(txsummary.Fee))

	tx := sharedW.Transaction{
		WalletID:    asset.GetWalletID(),
		Hash:        txsummary.Hash.String(),
		Type:        txType,
		Hex:         txHex,
		Timestamp:   txsummary.Timestamp,
		BlockHeight: blockheight,

		Version:  decodedTx.Version,
		LockTime: int32(decodedTx.LockTime),
		Fee:      int64(txsummary.Fee),
		FeeRate:  int64(feeRate),
		Size:     txSize,
		Label:    txsummary.Label,

		Direction: direction,
		Amount:    amount,
		Inputs:    inputs,
		Outputs:   outputs,
	}
	return tx
}
