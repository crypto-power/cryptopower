package dcr

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/txauthor"
	"decred.org/dcrwallet/v2/wallet/txrules"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
	mainW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/txhelper"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

type TxAuthor struct {
	sourceWallet        *Wallet
	sourceAccountNumber uint32
	destinations        []mainW.TransactionDestination
	changeAddress       string
	inputs              []*wire.TxIn
	changeDestination   *mainW.TransactionDestination

	unsignedTx     *txauthor.AuthoredTx
	needsConstruct bool
}

func (wallet *Wallet) NewUnsignedTx(sourceAccountNumber int32) (*TxAuthor, error) {
	sourceWallet := wallet
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
		destinations:        make([]mainW.TransactionDestination, 0),
		needsConstruct:      true,
	}, nil
}

func (tx *TxAuthor) AddSendDestination(address string, atomAmount int64, sendMax bool) error {
	_, err := stdaddr.DecodeAddress(address, tx.sourceWallet.chainParams)
	if err != nil {
		return utils.TranslateError(err)
	}

	if err := tx.validateSendAmount(sendMax, atomAmount); err != nil {
		return err
	}

	tx.destinations = append(tx.destinations, mainW.TransactionDestination{
		Address:    address,
		AtomAmount: atomAmount,
		SendMax:    sendMax,
	})
	tx.needsConstruct = true

	return nil
}

func (tx *TxAuthor) UpdateSendDestination(index int, address string, atomAmount int64, sendMax bool) error {
	if err := tx.validateSendAmount(sendMax, atomAmount); err != nil {
		return err
	}

	if len(tx.destinations) < index {
		return errors.New(utils.ErrIndexOutOfRange)
	}

	tx.destinations[index] = mainW.TransactionDestination{
		Address:    address,
		AtomAmount: atomAmount,
		SendMax:    sendMax,
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

func (tx *TxAuthor) SendDestination(atIndex int) *mainW.TransactionDestination {
	return &tx.destinations[atIndex]
}

func (tx *TxAuthor) SetChangeDestination(address string) {
	tx.changeDestination = &mainW.TransactionDestination{
		Address: address,
	}
	tx.needsConstruct = true
}

func (tx *TxAuthor) RemoveChangeDestination() {
	tx.changeDestination = nil
	tx.needsConstruct = true
}

func (tx *TxAuthor) TotalSendAmount() *mainW.Amount {
	var totalSendAmountAtom int64 = 0
	for _, destination := range tx.destinations {
		totalSendAmountAtom += destination.AtomAmount
	}

	return &mainW.Amount{
		AtomValue: totalSendAmountAtom,
		DcrValue:  dcrutil.Amount(totalSendAmountAtom).ToCoin(),
	}
}

func (tx *TxAuthor) EstimateFeeAndSize() (*mainW.TxFeeAndSize, error) {
	unsignedTx, err := tx.unsignedTransaction()
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	feeToSendTx := txrules.FeeForSerializeSize(txrules.DefaultRelayFeePerKb, unsignedTx.EstimatedSignedSerializeSize)
	feeAmount := &mainW.Amount{
		AtomValue: int64(feeToSendTx),
		DcrValue:  feeToSendTx.ToCoin(),
	}

	var change *mainW.Amount
	if unsignedTx.ChangeIndex >= 0 {
		txOut := unsignedTx.Tx.TxOut[unsignedTx.ChangeIndex]
		change = &mainW.Amount{
			AtomValue: txOut.Value,
			DcrValue:  AmountCoin(txOut.Value),
		}
	}

	return &mainW.TxFeeAndSize{
		EstimatedSignedSize: unsignedTx.EstimatedSignedSerializeSize,
		Fee:                 feeAmount,
		Change:              change,
	}, nil
}

func (tx *TxAuthor) EstimateMaxSendAmount() (*mainW.Amount, error) {
	txFeeAndSize, err := tx.EstimateFeeAndSize()
	if err != nil {
		return nil, err
	}

	spendableAccountBalance, err := tx.sourceWallet.SpendableForAccount(int32(tx.sourceAccountNumber))
	if err != nil {
		return nil, err
	}

	maxSendableAmount := spendableAccountBalance - txFeeAndSize.Fee.AtomValue

	return &mainW.Amount{
		AtomValue: maxSendableAmount,
		DcrValue:  dcrutil.Amount(maxSendableAmount).ToCoin(),
	}, nil
}

func (tx *TxAuthor) UseInputs(utxoKeys []string) error {
	// first clear any previously set inputs
	// so that an outdated set of inputs isn't used if an error occurs from this function
	tx.inputs = nil
	inputs := make([]*wire.TxIn, 0, len(utxoKeys))
	for _, utxoKey := range utxoKeys {
		idx := strings.Index(utxoKey, ":")
		hash := utxoKey[:idx]
		hashIndex := utxoKey[idx+1:]
		index, err := strconv.Atoi(hashIndex)
		if err != nil {
			return fmt.Errorf("no valid utxo found for '%s' in the source account at index %d", utxoKey, index)
		}

		txHash, err := chainhash.NewHashFromStr(hash)
		if err != nil {
			return err
		}

		op := &wire.OutPoint{
			Hash:  *txHash,
			Index: uint32(index),
		}
		ctx, _ := tx.sourceWallet.ShutdownContextWithCancel()
		outputInfo, err := tx.sourceWallet.Internal().DCR.OutputInfo(ctx, op)
		if err != nil {
			return fmt.Errorf("no valid utxo found for '%s' in the source account", utxoKey)
		}

		input := wire.NewTxIn(op, int64(outputInfo.Amount), nil)
		inputs = append(inputs, input)
	}

	tx.inputs = inputs
	tx.needsConstruct = true
	return nil
}

func (tx *TxAuthor) Broadcast(privatePassphrase []byte) ([]byte, error) {
	defer func() {
		for i := range privatePassphrase {
			privatePassphrase[i] = 0
		}
	}()

	n, err := tx.sourceWallet.Internal().DCR.NetworkBackend()
	if err != nil {
		log.Error(err)
		return nil, err
	}

	unsignedTx, err := tx.unsignedTransaction()
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	if unsignedTx.ChangeIndex >= 0 {
		unsignedTx.RandomizeChangePosition()
	}

	var txBuf bytes.Buffer
	txBuf.Grow(unsignedTx.Tx.SerializeSize())
	err = unsignedTx.Tx.Serialize(&txBuf)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(txBuf.Bytes()))
	if err != nil {
		log.Error(err)
		//Bytes do not represent a valid raw transaction
		return nil, err
	}

	lock := make(chan time.Time, 1)
	defer func() {
		lock <- time.Time{}
	}()

	ctx, _ := tx.sourceWallet.ShutdownContextWithCancel()
	err = tx.sourceWallet.Internal().DCR.Unlock(ctx, privatePassphrase, lock)
	if err != nil {
		log.Error(err)
		return nil, errors.New(utils.ErrInvalidPassphrase)
	}

	var additionalPkScripts map[wire.OutPoint][]byte

	invalidSigs, err := tx.sourceWallet.Internal().DCR.SignTransaction(ctx, &msgTx, txscript.SigHashAll, additionalPkScripts, nil, nil)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	invalidInputIndexes := make([]uint32, len(invalidSigs))
	for i, e := range invalidSigs {
		invalidInputIndexes[i] = e.InputIndex
	}

	var serializedTransaction bytes.Buffer
	serializedTransaction.Grow(msgTx.SerializeSize())
	err = msgTx.Serialize(&serializedTransaction)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	err = msgTx.Deserialize(bytes.NewReader(serializedTransaction.Bytes()))
	if err != nil {
		//Invalid tx
		log.Error(err)
		return nil, err
	}

	txHash, err := tx.sourceWallet.Internal().DCR.PublishTransaction(ctx, &msgTx, n)
	if err != nil {
		return nil, utils.TranslateError(err)
	}
	return txHash[:], nil
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
	var outputSelectionAlgorithm w.OutputSelectionAlgorithm = w.OutputSelectionAlgorithmDefault
	var changeSource txauthor.ChangeSource

	ctx, _ := tx.sourceWallet.ShutdownContextWithCancel()
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
			outputSelectionAlgorithm = w.OutputSelectionAlgorithmAll

			// Use this destination address to make a changeSource rather than a tx output.
			changeSource, err = txhelper.MakeTxChangeSource(destination.Address, tx.sourceWallet.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing change source: %v", err)
				return nil, fmt.Errorf("max amount change source error: %v", err)
			}
		} else {
			output, err := txhelper.MakeTxOutput(destination.Address, destination.AtomAmount, tx.sourceWallet.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing tx output: %v", err)
				return nil, fmt.Errorf("make tx output error: %v", err)
			}

			outputs = append(outputs, output)
		}
	}

	if changeSource == nil {
		// dcrwallet should ordinarily handle cases where a nil changeSource
		// is passed to `wallet.NewUnsignedTransaction` but the changeSource
		// generated there errors on internal gap address limit exhaustion
		// instead of wrapping around to a previously returned address.
		//
		// Generating a changeSource manually here, ensures that the gap address
		// limit exhaustion error is avoided.
		changeSource, err = tx.changeSource(ctx)
		if err != nil {
			return nil, err
		}
	}

	requiredConfirmations := tx.sourceWallet.RequiredConfirmations()
	return tx.sourceWallet.Internal().DCR.NewUnsignedTransaction(ctx, outputs, txrules.DefaultRelayFeePerKb, tx.sourceAccountNumber,
		requiredConfirmations, outputSelectionAlgorithm, changeSource, nil)
}

// changeSource derives an internal address from the source wallet and account
// for this unsigned tx, if a change address had not been previously derived.
// The derived (or previously derived) address is used to prepare a
// change source for receiving change from this tx back into the wallet.
func (tx *TxAuthor) changeSource(ctx context.Context) (txauthor.ChangeSource, error) {
	if tx.changeAddress == "" {
		var changeAccount uint32

		// MixedAccountNumber would be -1 if mixer config isn't set.
		if tx.sourceAccountNumber == uint32(tx.sourceWallet.MixedAccountNumber()) ||
			tx.sourceWallet.AccountMixerMixChange() {
			changeAccount = uint32(tx.sourceWallet.UnmixedAccountNumber())
		} else {
			changeAccount = tx.sourceAccountNumber
		}

		address, err := tx.sourceWallet.Internal().DCR.NewChangeAddress(ctx, changeAccount)
		if err != nil {
			return nil, fmt.Errorf("change address error: %v", err)
		}
		tx.changeAddress = address.String()
	}

	changeSource, err := txhelper.MakeTxChangeSource(tx.changeAddress, tx.sourceWallet.chainParams)
	if err != nil {
		log.Errorf("constructTransaction: error preparing change source: %v", err)
		return nil, fmt.Errorf("change source error: %v", err)
	}

	return changeSource, nil
}

// validateSendAmount validate the amount to send to a destination address
func (tx *TxAuthor) validateSendAmount(sendMax bool, atomAmount int64) error {
	if !sendMax && (atomAmount <= 0 || atomAmount > maxAmountAtom) {
		return errors.E(errors.Invalid, "invalid amount")
	}
	return nil
}
