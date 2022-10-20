package btc

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	// w "github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	// "github.com/btcsuite/btcwallet/wallet/txhelper"

	"github.com/btcsuite/btcwallet/wallet/txrules"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/txhelper"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

type TxAuthor struct {
	sourceWallet        *BTCAsset
	sourceAccountNumber uint32
	destinations        []sharedW.TransactionDestination
	changeAddress       string
	inputs              []*wire.TxIn
	changeDestination   *sharedW.TransactionDestination

	unsignedTx     *txauthor.AuthoredTx
	needsConstruct bool
}

func (asset *BTCAsset) NewUnsignedTx(sourceAccountNumber int32) (*TxAuthor, error) {
	sourceWallet := asset
	if sourceWallet == nil {
		return nil, fmt.Errorf(utils.ErrWalletNotFound)
	}

	_, err := sourceWallet.GetAccount(sourceAccountNumber)
	if err != nil {
		return nil, err
	}

	return &TxAuthor{
		sourceWallet:        sourceWallet,
		sourceAccountNumber: uint32(sourceAccountNumber),
		destinations:        make([]sharedW.TransactionDestination, 0),
		needsConstruct:      true,
	}, nil
}

func (tx *TxAuthor) AddSendDestination(address string, satoshiAmount int64, sendMax bool) error {
	_, err := btcutil.DecodeAddress(address, tx.sourceWallet.chainParams)
	if err != nil {
		return utils.TranslateError(err)
	}

	if err := tx.validateSendAmount(sendMax, satoshiAmount); err != nil {
		return err
	}

	tx.destinations = append(tx.destinations, sharedW.TransactionDestination{
		Address:       address,
		SatoshiAmount: satoshiAmount,
		SendMax:       sendMax,
	})
	tx.needsConstruct = true

	return nil
}

func (tx *TxAuthor) UpdateSendDestination(index int, address string, satoshiAmount int64, sendMax bool) error {
	if err := tx.validateSendAmount(sendMax, satoshiAmount); err != nil {
		return err
	}

	if len(tx.destinations) < index {
		return errors.New(utils.ErrIndexOutOfRange)
	}

	tx.destinations[index] = sharedW.TransactionDestination{
		Address:       address,
		SatoshiAmount: satoshiAmount,
		SendMax:       sendMax,
	}
	tx.needsConstruct = true
	return nil
}

func (tx *TxAuthor) RemoveSendDestination(index int) {
	if len(tx.destinations) > index {
		tx.destinations = append(tx.destinations[:index], tx.destinations[index+1:]...)
		tx.needsConstruct = true
	}
}

func (tx *TxAuthor) SendDestination(atIndex int) *sharedW.TransactionDestination {
	return &tx.destinations[atIndex]
}

func (tx *TxAuthor) SetChangeDestination(address string) {
	tx.changeDestination = &sharedW.TransactionDestination{
		Address: address,
	}
	tx.needsConstruct = true
}

func (tx *TxAuthor) RemoveChangeDestination() {
	tx.changeDestination = nil
	tx.needsConstruct = true
}

func (tx *TxAuthor) TotalSendAmount() *sharedW.Amount {
	var totalSendAmountSatoshi int64 = 0
	for _, destination := range tx.destinations {
		totalSendAmountSatoshi += destination.SatoshiAmount
	}

	return &sharedW.Amount{
		SatoshiValue: totalSendAmountSatoshi,
		BtcValue:     btcutil.Amount(totalSendAmountSatoshi).ToBTC(),
	}
}

func (tx *TxAuthor) EstimateFeeAndSize() (*sharedW.TxFeeAndSize, error) {
	unsignedTx, err := tx.unsignedTransaction()
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	feeToSendTx := txrules.FeeForSerializeSize(txrules.DefaultRelayFeePerKb, 0 /*unsignedTx.EstimatedSignedSerializeSize*/)
	feeAmount := &sharedW.Amount{
		SatoshiValue: int64(feeToSendTx),
		BtcValue:     feeToSendTx.ToBTC(),
	}

	var change *sharedW.Amount
	if unsignedTx.ChangeIndex >= 0 {
		txOut := unsignedTx.Tx.TxOut[unsignedTx.ChangeIndex]
		change = &sharedW.Amount{
			SatoshiValue: txOut.Value,
			BtcValue:     AmountBTC(txOut.Value),
		}
	}

	return &sharedW.TxFeeAndSize{
		// EstimatedSignedSize: unsignedTx.EstimatedSignedSerializeSize,
		Fee:    feeAmount,
		Change: change,
	}, nil
}

func (tx *TxAuthor) unsignedTransaction() (*txauthor.AuthoredTx, error) {
	if tx.needsConstruct || tx.unsignedTx == nil {
		unsignedTx, err := tx.constructTransaction()
		if err != nil {
			return nil, err
		}

		tx.needsConstruct = false
		tx.unsignedTx = unsignedTx
	}

	return tx.unsignedTx, nil
}

func (tx *TxAuthor) constructTransaction() (*txauthor.AuthoredTx, error) {
	if len(tx.inputs) != 0 {
		return tx.constructCustomTransaction()
	}

	var err error
	var outputs = make([]*wire.TxOut, 0)
	// var outputSelectionAlgorithm w.OutputSelectionAlgorithm = w.OutputSelectionAlgorithmDefault
	var changeSource *txauthor.ChangeSource

	for _, destination := range tx.destinations {
		if err := tx.validateSendAmount(destination.SendMax, destination.AtomAmount); err != nil {
			return nil, err
		}

		// check if multiple destinations are set to receive max amount
		if destination.SendMax && changeSource != nil {
			return nil, fmt.Errorf("cannot send max amount to multiple recipients")
		}

		if destination.SendMax {
			// This is a send max destination, set output selection algo to all.
			// outputSelectionAlgorithm = w.OutputSelectionAlgorithmAll

			// Use this destination address to make a changeSource rather than a tx output.
			changeSource, err = txhelper.MakeBTCTxChangeSource(destination.Address, tx.sourceWallet.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing change source: %v", err)
				return nil, fmt.Errorf("max amount change source error: %v", err)
			}
		} else {
			output, err := txhelper.MakeBTCTxOutput(destination.Address, destination.AtomAmount, tx.sourceWallet.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing tx output: %v", err)
				return nil, fmt.Errorf("make tx output error: %v", err)
			}

			outputs = append(outputs, output)
		}
	}

	if changeSource == nil {
		// btcwallet should ordinarily handle cases where a nil changeSource
		// is passed to `sharedW.NewUnsignedTransaction` but the changeSource
		// generated there errors on internal gap address limit exhaustion
		// instead of wrapping around to a previously returned address.
		//
		// Generating a changeSource manually here, ensures that the gap address
		// limit exhaustion error is avoided.
		changeSource, err = tx.changeSource()
		if err != nil {
			return nil, err
		}
	}

	// requiredConfirmations := tx.sourceWallet.RequiredConfirmations()
	return txauthor.NewUnsignedTransaction(outputs, txrules.DefaultRelayFeePerKb, nil, changeSource)
}

// changeSource derives an internal address from the source wallet and account
// for this unsigned tx, if a change address had not been previously derived.
// The derived (or previously derived) address is used to prepare a
// change source for receiving change from this tx back into the sharedW.
func (tx *TxAuthor) changeSource() (*txauthor.ChangeSource, error) {
	if tx.changeAddress == "" {

		changeAccount := tx.sourceAccountNumber

		address, err := tx.sourceWallet.Internal().BTC.NewChangeAddress(changeAccount, tx.sourceWallet.GetScope())
		if err != nil {
			return nil, fmt.Errorf("change address error: %v", err)
		}
		tx.changeAddress = address.String()
	}

	changeSource, err := txhelper.MakeBTCTxChangeSource(tx.changeAddress, tx.sourceWallet.chainParams)
	if err != nil {
		log.Errorf("constructTransaction: error preparing change source: %v", err)
		return nil, fmt.Errorf("change source error: %v", err)
	}

	return changeSource, nil
}

// validateSendAmount validate the amount to send to a destination address
func (tx *TxAuthor) validateSendAmount(sendMax bool, satoshiAmount int64) error {
	if !sendMax && (satoshiAmount <= 0 || satoshiAmount > maxAmountSatoshi) {
		return errors.E(errors.Invalid, "invalid amount")
	}
	return nil
}
