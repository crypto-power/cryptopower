package btc

import (
// "github.com/btcsuite/btcwallet/wallet/txauthor"
)

// func (tx *TxAuthor) constructCustomTransaction() (*txauthor.AuthoredTx, error) {
// 	// Used to generate an internal address for change,
// 	// if no change destination is provided and
// 	// no recipient is set to receive max amount.
// 	nextInternalAddress := func() (string, error) {
// 		ctx, _ := tx.sourceWallet.ShutdownContextWithCancel()
// 		addr, err := tx.sourceWallet.Internal().BTC.NewChangeAddress(ctx, tx.sourceAccountNumber)
// 		if err != nil {
// 			return "", err
// 		}
// 		return addr.String(), nil
// 	}

// 	return tx.newUnsignedTxUTXO(tx.inputs, tx.destinations, tx.changeDestination, nextInternalAddress)
// }
