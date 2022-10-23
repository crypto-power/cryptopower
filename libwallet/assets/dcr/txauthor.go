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
	"decred.org/dcrwallet/wallet/txsizes"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/txhelper"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

type TxAuthor struct {
	sourceAccountNumber uint32
	destinations        []sharedW.TransactionDestination
	changeAddress       string
	inputs              []*wire.TxIn
	changeDestination   *sharedW.TransactionDestination

	unsignedTx     *txauthor.AuthoredTx
	needsConstruct bool
}

func (asset *DCRAsset) NewUnsignedTx(sourceAccountNumber int32) error {
	_, err := asset.GetAccount(sourceAccountNumber)
	if err != nil {
		return err
	}

	asset.TxAuthoredInfo = &TxAuthor{
		sourceAccountNumber: uint32(sourceAccountNumber),
		destinations:        make([]sharedW.TransactionDestination, 0),
		needsConstruct:      true,
	}
	return nil
}

func (asset *DCRAsset) GetUnsignedTx() *TxAuthor {
	return asset.TxAuthoredInfo
}

func (asset *DCRAsset) AddSendDestination(address string, atomAmount int64, sendMax bool) error {
	_, err := stdaddr.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return utils.TranslateError(err)
	}

	if err := asset.validateSendAmount(sendMax, atomAmount); err != nil {
		return err
	}

	asset.TxAuthoredInfo.destinations = append(asset.TxAuthoredInfo.destinations, sharedW.TransactionDestination{
		Address:    address,
		UnitAmount: atomAmount,
		SendMax:    sendMax,
	})
	asset.TxAuthoredInfo.needsConstruct = true

	return nil
}

func (asset *DCRAsset) UpdateSendDestination(index int, address string, atomAmount int64, sendMax bool) error {
	if err := asset.validateSendAmount(sendMax, atomAmount); err != nil {
		return err
	}

	if len(asset.TxAuthoredInfo.destinations) < index {
		return errors.New(utils.ErrIndexOutOfRange)
	}

	asset.TxAuthoredInfo.destinations[index] = sharedW.TransactionDestination{
		Address:    address,
		UnitAmount: atomAmount,
		SendMax:    sendMax,
	}
	asset.TxAuthoredInfo.needsConstruct = true
	return nil
}

func (asset *DCRAsset) RemoveSendDestination(index int) {
	if len(asset.TxAuthoredInfo.destinations) > index {
		asset.TxAuthoredInfo.destinations = append(asset.TxAuthoredInfo.destinations[:index], asset.TxAuthoredInfo.destinations[index+1:]...)
		asset.TxAuthoredInfo.needsConstruct = true
	}
}

func (asset *DCRAsset) SendDestination(atIndex int) *sharedW.TransactionDestination {
	return &asset.TxAuthoredInfo.destinations[atIndex]
}

func (asset *DCRAsset) SetChangeDestination(address string) {
	asset.TxAuthoredInfo.changeDestination = &sharedW.TransactionDestination{
		Address: address,
	}
	asset.TxAuthoredInfo.needsConstruct = true
}

func (asset *DCRAsset) RemoveChangeDestination() {
	asset.TxAuthoredInfo.changeDestination = nil
	asset.TxAuthoredInfo.needsConstruct = true
}

func (asset *DCRAsset) TotalSendAmount() *sharedW.Amount {
	var totalSendAmountAtom int64 = 0
	for _, destination := range asset.TxAuthoredInfo.destinations {
		totalSendAmountAtom += destination.UnitAmount
	}

	return &sharedW.Amount{
		UnitValue: totalSendAmountAtom,
		CoinValue: dcrutil.Amount(totalSendAmountAtom).ToCoin(),
	}
}

func (asset *DCRAsset) EstimateFeeAndSize() (*sharedW.TxFeeAndSize, error) {
	unsignedTx, err := asset.unsignedTransaction()
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	feeToSendTx := txrules.FeeForSerializeSize(txrules.DefaultRelayFeePerKb, unsignedTx.EstimatedSignedSerializeSize)
	feeAmount := &sharedW.Amount{
		UnitValue: int64(feeToSendTx),
		CoinValue: feeToSendTx.ToCoin(),
	}

	var change *sharedW.Amount
	if unsignedTx.ChangeIndex >= 0 {
		txOut := unsignedTx.Tx.TxOut[unsignedTx.ChangeIndex]
		change = &sharedW.Amount{
			UnitValue: txOut.Value,
			CoinValue: asset.ToAmount(txOut.Value).ToCoin(),
		}
	}

	return &sharedW.TxFeeAndSize{
		EstimatedSignedSize: unsignedTx.EstimatedSignedSerializeSize,
		Fee:                 feeAmount,
		Change:              change,
	}, nil
}

func (asset *DCRAsset) EstimateMaxSendAmount() (*sharedW.Amount, error) {
	txFeeAndSize, err := asset.EstimateFeeAndSize()
	if err != nil {
		return nil, err
	}

	spendableAccountBalance, err := asset.SpendableForAccount(int32(asset.TxAuthoredInfo.sourceAccountNumber))
	if err != nil {
		return nil, err
	}

	maxSendableAmount := spendableAccountBalance - txFeeAndSize.Fee.UnitValue

	return &sharedW.Amount{
		UnitValue: maxSendableAmount,
		CoinValue: dcrutil.Amount(maxSendableAmount).ToCoin(),
	}, nil
}

func (asset *DCRAsset) UseInputs(utxoKeys []string) error {
	// first clear any previously set inputs
	// so that an outdated set of inputs isn't used if an error occurs from this function
	asset.TxAuthoredInfo.inputs = nil
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
		ctx, _ := asset.ShutdownContextWithCancel()
		outputInfo, err := asset.Internal().DCR.OutputInfo(ctx, op)
		if err != nil {
			return fmt.Errorf("no valid utxo found for '%s' in the source account", utxoKey)
		}

		input := wire.NewTxIn(op, int64(outputInfo.Amount), nil)
		inputs = append(inputs, input)
	}

	asset.TxAuthoredInfo.inputs = inputs
	asset.TxAuthoredInfo.needsConstruct = true
	return nil
}

func (asset *DCRAsset) Broadcast(privatePassphrase string) ([]byte, error) {
	n, err := asset.Internal().DCR.NetworkBackend()
	if err != nil {
		log.Error(err)
		return nil, err
	}

	unsignedTx, err := asset.unsignedTransaction()
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

	ctx, _ := asset.ShutdownContextWithCancel()
	err = asset.Internal().DCR.Unlock(ctx, []byte(privatePassphrase), lock)
	if err != nil {
		log.Error(err)
		return nil, errors.New(utils.ErrInvalidPassphrase)
	}

	var additionalPkScripts map[wire.OutPoint][]byte

	invalidSigs, err := asset.Internal().DCR.SignTransaction(ctx, &msgTx, txscript.SigHashAll, additionalPkScripts, nil, nil)
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

	txHash, err := asset.Internal().DCR.PublishTransaction(ctx, &msgTx, n)
	if err != nil {
		return nil, utils.TranslateError(err)
	}
	return txHash[:], nil
}

func (asset *DCRAsset) unsignedTransaction() (*txauthor.AuthoredTx, error) {
	if asset.TxAuthoredInfo.needsConstruct || asset.TxAuthoredInfo.unsignedTx == nil {
		unsignedTx, err := asset.constructTransaction()
		if err != nil {
			return nil, err
		}

		asset.TxAuthoredInfo.needsConstruct = false
		asset.TxAuthoredInfo.unsignedTx = unsignedTx
	}

	return asset.TxAuthoredInfo.unsignedTx, nil
}

func (asset *DCRAsset) constructTransaction() (*txauthor.AuthoredTx, error) {
	if len(asset.TxAuthoredInfo.inputs) != 0 {
		return asset.constructCustomTransaction()
	}

	var err error
	var outputs = make([]*wire.TxOut, 0)
	var outputSelectionAlgorithm w.OutputSelectionAlgorithm = w.OutputSelectionAlgorithmDefault
	var changeSource txauthor.ChangeSource

	ctx, _ := asset.ShutdownContextWithCancel()
	for _, destination := range asset.TxAuthoredInfo.destinations {
		if err := asset.validateSendAmount(destination.SendMax, destination.UnitAmount); err != nil {
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
			changeSource, err = txhelper.MakeTxChangeSource(destination.Address, asset.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing change source: %v", err)
				return nil, fmt.Errorf("max amount change source error: %v", err)
			}
		} else {
			output, err := txhelper.MakeTxOutput(destination.Address, destination.UnitAmount, asset.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing tx output: %v", err)
				return nil, fmt.Errorf("make tx output error: %v", err)
			}

			outputs = append(outputs, output)
		}
	}

	if changeSource == nil {
		// dcrwallet should ordinarily handle cases where a nil changeSource
		// is passed to `sharedW.NewUnsignedTransaction` but the changeSource
		// generated there errors on internal gap address limit exhaustion
		// instead of wrapping around to a previously returned address.
		//
		// Generating a changeSource manually here, ensures that the gap address
		// limit exhaustion error is avoided.
		changeSource, err = asset.changeSource(ctx)
		if err != nil {
			return nil, err
		}
	}

	requiredConfirmations := asset.RequiredConfirmations()
	return asset.Internal().DCR.NewUnsignedTransaction(ctx, outputs, txrules.DefaultRelayFeePerKb, asset.TxAuthoredInfo.sourceAccountNumber,
		requiredConfirmations, outputSelectionAlgorithm, changeSource, nil)
}

// changeSource derives an internal address from the source wallet and account
// for this unsigned tx, if a change address had not been previously derived.
// The derived (or previously derived) address is used to prepare a
// change source for receiving change from this tx back into the sharedW.
func (asset *DCRAsset) changeSource(ctx context.Context) (txauthor.ChangeSource, error) {
	if asset.TxAuthoredInfo.changeAddress == "" {
		var changeAccount uint32

		// MixedAccountNumber would be -1 if mixer config isn't set.
		if asset.TxAuthoredInfo.sourceAccountNumber == uint32(asset.MixedAccountNumber()) ||
			asset.AccountMixerMixChange() {
			changeAccount = uint32(asset.UnmixedAccountNumber())
		} else {
			changeAccount = asset.TxAuthoredInfo.sourceAccountNumber
		}

		address, err := asset.Internal().DCR.NewChangeAddress(ctx, changeAccount)
		if err != nil {
			return nil, fmt.Errorf("change address error: %v", err)
		}
		asset.TxAuthoredInfo.changeAddress = address.String()
	}

	changeSource, err := txhelper.MakeTxChangeSource(asset.TxAuthoredInfo.changeAddress, asset.chainParams)
	if err != nil {
		log.Errorf("constructTransaction: error preparing change source: %v", err)
		return nil, fmt.Errorf("change source error: %v", err)
	}

	return changeSource, nil
}

// validateSendAmount validate the amount to send to a destination address
func (asset *DCRAsset) validateSendAmount(sendMax bool, atomAmount int64) error {
	if !sendMax && (atomAmount <= 0 || atomAmount > dcrutil.MaxAmount) {
		return errors.E(errors.Invalid, "invalid amount")
	}
	return nil
}

type nextAddressFunc func() (address string, err error)

func calculateChangeScriptSize(changeAddress string, chainParams *chaincfg.Params) (int, error) {
	changeSource, err := txhelper.MakeTxChangeSource(changeAddress, chainParams)
	if err != nil {
		return 0, fmt.Errorf("change address error: %v", err)
	}
	return changeSource.ScriptSize(), nil
}

// ParseOutputsAndChangeDestination generates and returns TxOuts
// using the provided slice of transaction destinations.
// Any destination set to receive max amount is not included in the TxOuts returned,
// but is instead returned as a change destination.
// Returns an error if more than 1 max amount recipients identified or
// if any other error is encountered while processing the addresses and amounts.
func (asset *DCRAsset) parseOutputsAndChangeDestination(txdestinations []sharedW.TransactionDestination) ([]*wire.TxOut, int64, string, error) {
	var outputs = make([]*wire.TxOut, 0)
	var totalSendAmount int64
	var maxAmountRecipientAddress string

	for _, destination := range txdestinations {
		if err := asset.validateSendAmount(destination.SendMax, destination.UnitAmount); err != nil {
			return nil, 0, "", err
		}

		// check if multiple destinations are set to receive max amount
		if destination.SendMax && maxAmountRecipientAddress != "" {
			return nil, 0, "", fmt.Errorf("cannot send max amount to multiple recipients")
		}

		if destination.SendMax {
			maxAmountRecipientAddress = destination.Address
			continue // do not prepare a tx output for this destination
		}

		output, err := txhelper.MakeTxOutput(destination.Address, destination.UnitAmount, asset.chainParams)
		if err != nil {
			return nil, 0, "", fmt.Errorf("make tx output error: %v", err)
		}

		totalSendAmount += output.Value
		outputs = append(outputs, output)
	}

	return outputs, totalSendAmount, maxAmountRecipientAddress, nil
}

func (asset *DCRAsset) constructCustomTransaction() (*txauthor.AuthoredTx, error) {
	// Used to generate an internal address for change,
	// if no change destination is provided and
	// no recipient is set to receive max amount.
	nextInternalAddress := func() (string, error) {
		ctx, _ := asset.ShutdownContextWithCancel()
		addr, err := asset.Internal().DCR.NewChangeAddress(ctx, asset.TxAuthoredInfo.sourceAccountNumber)
		if err != nil {
			return "", err
		}
		return addr.String(), nil
	}

	tx := asset.TxAuthoredInfo
	return asset.newUnsignedTxUTXO(tx.inputs, tx.destinations, tx.changeDestination, nextInternalAddress)
}

func (asset *DCRAsset) newUnsignedTxUTXO(inputs []*wire.TxIn, senddestinations []sharedW.TransactionDestination, changeDestination *sharedW.TransactionDestination,
	nextInternalAddress nextAddressFunc) (*txauthor.AuthoredTx, error) {
	outputs, totalSendAmount, maxAmountRecipientAddress, err := asset.parseOutputsAndChangeDestination(senddestinations)
	if err != nil {
		return nil, err
	}

	if maxAmountRecipientAddress != "" && changeDestination != nil {
		return nil, errors.E(errors.Invalid, "no change is generated when sending max amount,"+
			" change destinations must not be provided")
	}

	if maxAmountRecipientAddress == "" && changeDestination == nil {
		// no change specified, generate new internal address to use as change (max amount recipient)
		maxAmountRecipientAddress, err = nextInternalAddress()
		if err != nil {
			return nil, fmt.Errorf("error generating internal address to use as change: %s", err.Error())
		}
	}

	var totalInputAmount int64
	inputScriptSizes := make([]int, len(inputs))
	inputScripts := make([][]byte, len(inputs))
	for i, input := range inputs {
		totalInputAmount += input.ValueIn
		inputScriptSizes[i] = txsizes.RedeemP2PKHSigScriptSize
		inputScripts[i] = input.SignatureScript
	}

	var changeScriptSize int
	if maxAmountRecipientAddress != "" {
		changeScriptSize, err = calculateChangeScriptSize(maxAmountRecipientAddress, asset.chainParams)
	} else {
		changeScriptSize, err = calculateChangeScriptSize(changeDestination.Address, asset.chainParams)
	}
	if err != nil {
		return nil, err
	}

	maxSignedSize := txsizes.EstimateSerializeSize(inputScriptSizes, outputs, changeScriptSize)
	maxRequiredFee := txrules.FeeForSerializeSize(txrules.DefaultRelayFeePerKb, maxSignedSize)
	changeAmount := totalInputAmount - totalSendAmount - int64(maxRequiredFee)

	if changeAmount < 0 {
		return nil, errors.New(utils.ErrInsufficientBalance)
	}

	if changeAmount != 0 && !txrules.IsDustAmount(dcrutil.Amount(changeAmount), changeScriptSize, txrules.DefaultRelayFeePerKb) {
		if changeScriptSize > txscript.MaxScriptElementSize {
			return nil, fmt.Errorf("script size exceed maximum bytes pushable to the stack")
		}
		if maxAmountRecipientAddress != "" {
			outputs, err = asset.changeOutput(changeAmount, maxAmountRecipientAddress, outputs)
		} else if changeDestination != nil {
			outputs, err = asset.changeOutput(changeAmount, changeDestination.Address, outputs)
		}
		if err != nil {
			return nil, fmt.Errorf("change address error: %v", err)
		}
	}

	return &txauthor.AuthoredTx{
		TotalInput:                   dcrutil.Amount(totalInputAmount),
		EstimatedSignedSerializeSize: maxSignedSize,
		Tx: &wire.MsgTx{
			SerType:  wire.TxSerializeFull,
			Version:  wire.TxVersion,
			TxIn:     inputs,
			TxOut:    outputs,
			LockTime: 0,
			Expiry:   0,
		},
	}, nil
}

func (asset *DCRAsset) changeOutput(changeAmount int64, maxAmountRecipientAddress string, outputs []*wire.TxOut) ([]*wire.TxOut, error) {
	changeOutput, err := txhelper.MakeTxOutput(maxAmountRecipientAddress, changeAmount, asset.chainParams)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, changeOutput)
	txauthor.RandomizeOutputPosition(outputs, len(outputs)-1)
	return outputs, nil
}
