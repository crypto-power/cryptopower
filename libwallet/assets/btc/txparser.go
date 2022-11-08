package btc

import (
	"fmt"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/wallet"
)

func (asset *BTCAsset) decodeTransactionWithTxSummary(blockheight int32, txsummary []wallet.TransactionSummary) []sharedW.Transaction {
	txs := make([]sharedW.Transaction, 0, len(txsummary))
	for _, rawtx := range txsummary {
		txHex := fmt.Sprintf("%x", rawtx.Transaction)
		decodedTx, _ := asset.decodeTxHex(txHex)
		txSize := decodedTx.SerializeSize()

		//TODO: Check why tx fee returned is zero despite int not being zero on the explorer
		feeRate := rawtx.Fee * 1000 / btcutil.Amount(txSize)

		// BTC transactions are either coinbase or regular txs.
		txType := txhelper.TxTypeRegular
		if blockchain.IsCoinBaseTx(decodedTx) {
			txType = txhelper.TxTypeCoinBase
		}

		inputs, totalInputsAmount := asset.decodeTxInputs(decodedTx, rawtx.MyInputs)
		outputs, totalOutputsAmount := asset.decodeTxOutputs(decodedTx, rawtx.MyOutputs)
		amount, direction := txhelper.TransactionAmountAndDirection(totalInputsAmount, totalOutputsAmount, int64(rawtx.Fee))

		txs = append(txs, sharedW.Transaction{
			WalletID:    asset.GetWalletID(),
			Hash:        rawtx.Hash.String(),
			Type:        txType,
			Hex:         txHex,
			Timestamp:   rawtx.Timestamp,
			BlockHeight: blockheight,

			Version:  decodedTx.Version,
			LockTime: int32(decodedTx.LockTime),
			Fee:      int64(rawtx.Fee),
			FeeRate:  int64(feeRate),
			Size:     txSize,
			Label:    rawtx.Label,

			Direction: direction,
			Amount:    amount,
			Inputs:    inputs,
			Outputs:   outputs,
		})
	}
	return txs
}
