package dcr

import (
	"bytes"
	"context"
	"fmt"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/txauthor"
	"decred.org/dcrwallet/v2/wallet/txrules"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
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

func (asset *DCRAsset) IsUnsignedTxExist() bool {
	return asset.TxAuthoredInfo != nil
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
