package txhelper

import (
	"math"

	"decred.org/dcrwallet/v3/wallet"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrdata/v7/txhelpers"
)

func MsgTxFeeSizeRate(transactionHex string) (msgTx *wire.MsgTx, fee dcrutil.Amount, size int, feeRate dcrutil.Amount, err error) {
	msgTx, err = txhelpers.MsgTxFromHex(transactionHex)
	if err != nil {
		return
	}

	size = msgTx.SerializeSize()
	fee, feeRate = txhelpers.TxFeeRate(msgTx)
	return
}

func TransactionAmountAndDirection(inputTotal, outputTotal, fee int64) (amount int64, direction int32) {
	amountDifference := outputTotal - inputTotal

	if amountDifference < 0 && float64(fee) == math.Abs(float64(amountDifference)) {
		// transferred internally, the only real amount spent was transaction fee
		direction = TxDirectionTransferred
		amount = fee
	} else if amountDifference > 0 {
		// received
		direction = TxDirectionReceived
		amount = outputTotal
	} else {
		// sent
		direction = TxDirectionSent
		amount = inputTotal - outputTotal - fee
	}

	return
}

func FormatTransactionType(txType wallet.TransactionType) string {
	switch txType {
	case wallet.TransactionTypeCoinbase:
		return TxTypeCoinBase
	case wallet.TransactionTypeTicketPurchase:
		return TxTypeTicketPurchase
	case wallet.TransactionTypeVote:
		return TxTypeVote
	case wallet.TransactionTypeRevocation:
		return TxTypeRevocation
	default:
		return TxTypeRegular
	}
}
