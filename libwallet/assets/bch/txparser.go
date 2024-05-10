package bch

import (
	"fmt"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/txhelper"
	"github.com/gcash/bchutil"
	// w "github.com/gcash/bchwallet/wallet"
	w "github.com/dcrlabs/bchwallet/wallet"
	"github.com/gcash/bchd/blockchain"
)

func (asset *Asset) decodeTransactionWithTxSummary(blockheight int32, txsummary w.TransactionSummary) *sharedW.Transaction {
	txHex := fmt.Sprintf("%x", txsummary.Transaction)
	decodedTx, _ := asset.decodeTxHex(txHex)
	txSize := decodedTx.SerializeSize()

	// TODO: Check why tx fee returned is zero despite int not being zero on the explorer
	feeRate := txsummary.Fee * 1000 / bchutil.Amount(txSize)

	// BCH transactions are either coinbase or regular txs.
	txType := txhelper.TxTypeRegular
	if blockchain.IsCoinBaseTx(decodedTx) {
		txType = txhelper.TxTypeCoinBase
	}

	inputs, totalInputsAmount := asset.decodeTxInputs(decodedTx, txsummary.MyInputs)
	outputs, totalOutputsAmount := asset.decodeTxOutputs(decodedTx, txsummary.MyOutputs)
	amount, direction := txhelper.TransactionAmountAndDirection(totalInputsAmount, totalOutputsAmount, int64(txsummary.Fee))

	return &sharedW.Transaction{
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

		Direction: direction,
		Amount:    amount,
		Inputs:    inputs,
		Outputs:   outputs,
	}
}
