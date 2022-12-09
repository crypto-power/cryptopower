package btc

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/btcsuite/btcwallet/wallet/txsizes"
)

type nextAddressFunc func() (address string, err error)

func calculateChangeScriptSize(changeAddress string, chainParams *chaincfg.Params) (int, error) {
	changeSource, err := txhelper.MakeBTCTxChangeSource(changeAddress, chainParams)
	if err != nil {
		return 0, fmt.Errorf("change address error: %v", err)
	}
	return changeSource.ScriptSize, nil
}

// ParseOutputsAndChangeDestination generates and returns TxOuts
// using the provided slice of transaction destinations.
// Any destination set to receive max amount is not included in the TxOuts returned,
// but is instead returned as a change destination.
// Returns an error if more than 1 max amount recipients identified or
// if any other error is encountered while processing the addresses and amounts.
func (asset *BTCAsset) ParseOutputsAndChangeDestination(txDestinations map[string]*sharedW.TransactionDestination) ([]*wire.TxOut, int64, string, error) {
	var outputs = make([]*wire.TxOut, 0)
	var totalSendAmount int64
	var maxAmountRecipientAddress string

	for _, destination := range txDestinations {
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

		output, err := txhelper.MakeBTCTxOutput(destination.Address, destination.UnitAmount, asset.chainParams)
		if err != nil {
			return nil, 0, "", fmt.Errorf("make tx output error: %v", err)
		}

		totalSendAmount += output.Value
		outputs = append(outputs, output)
	}

	return outputs, totalSendAmount, maxAmountRecipientAddress, nil
}

func (asset *BTCAsset) constructCustomTransaction() (*txauthor.AuthoredTx, error) {
	// Used to generate an internal address for change,
	// if no change destination is provided and
	// no recipient is set to receive max amount.
	nextInternalAddress := func() (string, error) {
		addr, err := asset.Internal().BTC.NewChangeAddress(asset.TxAuthoredInfo.sourceAccountNumber, asset.GetScope())
		if err != nil {
			return "", err
		}
		return addr.String(), nil
	}

	return asset.newUnsignedTxUTXO(nextInternalAddress)
}

func (asset *BTCAsset) newUnsignedTxUTXO(nextInternalAddress nextAddressFunc) (*txauthor.AuthoredTx, error) {
	sendDestinations := asset.TxAuthoredInfo.destinations
	outputs, totalSendAmount, maxAmountRecipientAddress, err := asset.ParseOutputsAndChangeDestination(sendDestinations)
	if err != nil {
		return nil, err
	}

	changeDestination := asset.TxAuthoredInfo.changeDestination
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

	var txMsg = wire.NewMsgTx(wire.TxVersion)

	var totalInputAmount int64
	inputs := asset.TxAuthoredInfo.inputs
	inputValues := asset.TxAuthoredInfo.inputValues
	inputScriptSizes := make([]int, len(inputs))
	inputScripts := make([][]byte, len(inputs))
	for i, input := range inputs {
		totalInputAmount += int64(inputValues[i])
		inputScriptSizes[i] = txsizes.RedeemP2PKHSigScriptSize
		inputScripts[i] = input.SignatureScript

		txMsg.AddTxIn(input)
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

	maxSignedSize := txsizes.EstimateSerializeSize(len(inputScriptSizes), outputs, changeScriptSize > 0)
	maxRequiredFee := txrules.FeeForSerializeSize(btcutil.Amount(asset.GetUserFeeRate().ToInt()), maxSignedSize)
	changeAmount := totalInputAmount - totalSendAmount - int64(maxRequiredFee)

	if changeAmount < 0 {
		return nil, errors.New(utils.ErrInsufficientBalance)
	}

	// IsDustAmount has been deprecated, there seems to be a newere method IsDustOutput.
	// see documentation https://pkg.go.dev/github.com/btcsuite/btcwallet/wallet/txrules#IsDustOutput
	if changeAmount != 0 /* && !txrules.IsDustAmount(btcutil.Amount(changeAmount), changeScriptSize, txrules.DefaultRelayFeePerKb)*/ {
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

	// Add the outputs.
	txMsg.TxOut = append(txMsg.TxOut, outputs...)
	txMsg.LockTime = uint32(asset.GetBestBlockHeight())

	return &txauthor.AuthoredTx{
		TotalInput:      btcutil.Amount(totalInputAmount),
		PrevInputValues: inputValues,
		Tx:              txMsg,
	}, nil
}

func (asset *BTCAsset) changeOutput(changeAmount int64, maxAmountRecipientAddress string, outputs []*wire.TxOut) ([]*wire.TxOut, error) {
	changeOutput, err := txhelper.MakeBTCTxOutput(maxAmountRecipientAddress, changeAmount, asset.chainParams)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, changeOutput)
	txauthor.RandomizeOutputPosition(outputs, len(outputs)-1)
	return outputs, nil
}
