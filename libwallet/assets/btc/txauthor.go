package btc

import (
	"fmt"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wallet/txrules"
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

// noInputValue describes an error returned by the input source when no inputs
// were selected because each previous output value was zero.  Callers of
// txauthor.NewUnsignedTransaction need not report these errors to the user.
type noInputValue struct {
}

func (noInputValue) Error() string { return "no input value" }

func (asset *BTCAsset) NewUnsignedTx(sourceAccountNumber int32) error {
	if asset == nil {
		return fmt.Errorf(utils.ErrWalletNotFound)
	}

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

func (asset *BTCAsset) GetUnsignedTx() *TxAuthor {
	return asset.TxAuthoredInfo
}

func (asset *BTCAsset) AddSendDestination(address string, satoshiAmount int64, sendMax bool) error {
	_, err := btcutil.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return utils.TranslateError(err)
	}

	if err := asset.validateSendAmount(sendMax, satoshiAmount); err != nil {
		return err
	}

	asset.TxAuthoredInfo.destinations = append(asset.TxAuthoredInfo.destinations, sharedW.TransactionDestination{
		Address:    address,
		UnitAmount: satoshiAmount,
		SendMax:    sendMax,
	})
	asset.TxAuthoredInfo.needsConstruct = true

	return nil
}

func (asset *BTCAsset) UpdateSendDestination(index int, address string, satoshiAmount int64, sendMax bool) error {
	if err := asset.validateSendAmount(sendMax, satoshiAmount); err != nil {
		return err
	}

	if len(asset.TxAuthoredInfo.destinations) < index {
		return errors.New(utils.ErrIndexOutOfRange)
	}

	asset.TxAuthoredInfo.destinations[index] = sharedW.TransactionDestination{
		Address:    address,
		UnitAmount: satoshiAmount,
		SendMax:    sendMax,
	}
	asset.TxAuthoredInfo.needsConstruct = true
	return nil
}

func (asset *BTCAsset) RemoveSendDestination(index int) {
	if len(asset.TxAuthoredInfo.destinations) > index {
		asset.TxAuthoredInfo.destinations = append(asset.TxAuthoredInfo.destinations[:index], asset.TxAuthoredInfo.destinations[index+1:]...)
		asset.TxAuthoredInfo.needsConstruct = true
	}
}

func (asset *BTCAsset) SendDestination(atIndex int) *sharedW.TransactionDestination {
	return &asset.TxAuthoredInfo.destinations[atIndex]
}

func (asset *BTCAsset) SetChangeDestination(address string) {
	asset.TxAuthoredInfo.changeDestination = &sharedW.TransactionDestination{
		Address: address,
	}
	asset.TxAuthoredInfo.needsConstruct = true
}

func (asset *BTCAsset) RemoveChangeDestination() {
	asset.TxAuthoredInfo.changeDestination = nil
	asset.TxAuthoredInfo.needsConstruct = true
}

func (asset *BTCAsset) TotalSendAmount() *sharedW.Amount {
	var totalSendAmountSatoshi int64 = 0
	for _, destination := range asset.TxAuthoredInfo.destinations {
		totalSendAmountSatoshi += destination.UnitAmount
	}

	return &sharedW.Amount{
		UnitValue: totalSendAmountSatoshi,
		CoinValue: btcutil.Amount(totalSendAmountSatoshi).ToBTC(),
	}
}

func (asset *BTCAsset) EstimateFeeAndSize() (*sharedW.TxFeeAndSize, error) {
	unsignedTx, err := asset.unsignedTransaction()
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	estimatedSignedSerializeSize := asset.TxAuthoredInfo.unsignedTx.Tx.SerializeSize()
	feeToSendTx := txrules.FeeForSerializeSize(txrules.DefaultRelayFeePerKb, estimatedSignedSerializeSize)
	feeAmount := &sharedW.Amount{
		UnitValue: int64(feeToSendTx),
		CoinValue: feeToSendTx.ToBTC(),
	}

	var change *sharedW.Amount
	if unsignedTx.ChangeIndex >= 0 {
		txOut := unsignedTx.Tx.TxOut[unsignedTx.ChangeIndex]
		change = &sharedW.Amount{
			UnitValue: txOut.Value,
			CoinValue: AmountBTC(txOut.Value),
		}
	}

	return &sharedW.TxFeeAndSize{
		EstimatedSignedSize: estimatedSignedSerializeSize,
		Fee:                 feeAmount,
		Change:              change,
	}, nil
}

func (asset *BTCAsset) unsignedTransaction() (*txauthor.AuthoredTx, error) {
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

func (asset *BTCAsset) constructTransaction() (*txauthor.AuthoredTx, error) {
	if len(asset.TxAuthoredInfo.inputs) != 0 {
		return asset.constructCustomTransaction()
	}

	var err error
	var outputs = make([]*wire.TxOut, 0)
	var changeSource *txauthor.ChangeSource

	for _, destination := range asset.TxAuthoredInfo.destinations {
		if err := asset.validateSendAmount(destination.SendMax, destination.UnitAmount); err != nil {
			return nil, err
		}

		// check if multiple destinations are set to receive max amount
		if destination.SendMax && changeSource != nil {
			return nil, fmt.Errorf("cannot send max amount to multiple recipients")
		}

		if destination.SendMax {
			// Use this destination address to make a changeSource rather than a tx output.
			changeSource, err = txhelper.MakeBTCTxChangeSource(destination.Address, asset.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing change source: %v", err)
				return nil, fmt.Errorf("max amount change source error: %v", err)
			}
		} else {
			output, err := txhelper.MakeBTCTxOutput(destination.Address, destination.UnitAmount, asset.chainParams)
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
		changeSource, err = asset.changeSource()
		if err != nil {
			return nil, err
		}
	}

	unspents, err := asset.UnspentOutputs(int32(asset.TxAuthoredInfo.sourceAccountNumber))
	if err != nil {
		return nil, err
	}
	inputSource := makeInputSource(unspents)

	return txauthor.NewUnsignedTransaction(outputs, txrules.DefaultRelayFeePerKb, inputSource, changeSource)
}

// changeSource derives an internal address from the source wallet and account
// for this unsigned tx, if a change address had not been previously derived.
// The derived (or previously derived) address is used to prepare a
// change source for receiving change from this tx back into the sharedW.
func (asset *BTCAsset) changeSource() (*txauthor.ChangeSource, error) {
	if asset.TxAuthoredInfo.changeAddress == "" {

		changeAccount := asset.TxAuthoredInfo.sourceAccountNumber

		address, err := asset.Internal().BTC.NewChangeAddress(changeAccount, asset.GetScope())
		if err != nil {
			return nil, fmt.Errorf("change address error: %v", err)
		}
		asset.TxAuthoredInfo.changeAddress = address.String()
	}

	changeSource, err := txhelper.MakeBTCTxChangeSource(asset.TxAuthoredInfo.changeAddress, asset.chainParams)
	if err != nil {
		log.Errorf("constructTransaction: error preparing change source: %v", err)
		return nil, fmt.Errorf("change source error: %v", err)
	}

	return changeSource, nil
}

// validateSendAmount validate the amount to send to a destination address
func (asset *BTCAsset) validateSendAmount(sendMax bool, satoshiAmount int64) error {
	if !sendMax && (satoshiAmount <= 0 || satoshiAmount > maxAmountSatoshi) {
		return errors.E(errors.Invalid, "invalid amount")
	}
	return nil
}

// makeInputSource creates an InputSource that creates inputs for every unspent
// output with non-zero output values.  The target amount is ignored since every
// output is consumed.  The InputSource does not return any previous output
// scripts as they are not needed for creating the unsinged transaction and are
// looked up again by the wallet during the call to signrawtransaction.
func makeInputSource(outputs []*ListUnspentResult) txauthor.InputSource {
	var (
		totalInputValue btcutil.Amount
		inputs          = make([]*wire.TxIn, 0, len(outputs))
		inputValues     = make([]btcutil.Amount, 0, len(outputs))
		sourceErr       error
	)
	for _, output := range outputs {
		output := output

		outputAmount, err := btcutil.NewAmount(output.Amount)
		if err != nil {
			sourceErr = fmt.Errorf(
				"invalid amount `%v` in listunspent result",
				output.Amount)
			break
		}
		if outputAmount == 0 {
			continue
		}
		if !saneOutputValue(outputAmount) {
			sourceErr = fmt.Errorf(
				"impossible output amount `%v` in listunspent result",
				outputAmount)
			break
		}
		totalInputValue += outputAmount

		previousOutPoint, err := parseOutPoint(output)
		if err != nil {
			sourceErr = fmt.Errorf(
				"invalid data in listunspent result: %v",
				err)
			break
		}

		inputs = append(inputs, wire.NewTxIn(&previousOutPoint, nil, nil))
		inputValues = append(inputValues, outputAmount)
	}

	if sourceErr == nil && totalInputValue == 0 {
		sourceErr = noInputValue{}
	}

	return func(btcutil.Amount) (btcutil.Amount, []*wire.TxIn, []btcutil.Amount, [][]byte, error) {
		return totalInputValue, inputs, inputValues, nil, sourceErr
	}
}

func saneOutputValue(amount btcutil.Amount) bool {
	return amount >= 0 && amount <= btcutil.MaxSatoshi
}

func parseOutPoint(input *ListUnspentResult) (wire.OutPoint, error) {
	txHash, err := chainhash.NewHashFromStr(input.TxID)
	if err != nil {
		return wire.OutPoint{}, err
	}
	return wire.OutPoint{Hash: *txHash, Index: input.Vout}, nil
}
