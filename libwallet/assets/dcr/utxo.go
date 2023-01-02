package dcr

import (
	"fmt"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"decred.org/dcrwallet/v2/wallet/txauthor"
	"decred.org/dcrwallet/v2/wallet/txrules"
	"decred.org/dcrwallet/v2/wallet/txsizes"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
)

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
