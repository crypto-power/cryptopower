package btc

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wallet/txrules"
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

func (asset *BTCAsset) IsUnsignedTxExist() bool {
	return asset.TxAuthoredInfo != nil
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

	inputSource := makeInputSource(outputs)

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

func makeInputSource(unspents []*wire.TxOut) txauthor.InputSource {
	// Return outputs in order.
	currentTotal := btcutil.Amount(0)
	currentInputs := make([]*wire.TxIn, 0, len(unspents))
	currentInputValues := make([]btcutil.Amount, 0, len(unspents))
	f := func(target btcutil.Amount) (btcutil.Amount, []*wire.TxIn, []btcutil.Amount, [][]byte, error) {
		for currentTotal < target && len(unspents) != 0 {
			u := unspents[0]
			unspents = unspents[1:]
			nextInput := wire.NewTxIn(&wire.OutPoint{}, nil, nil)
			currentTotal += btcutil.Amount(u.Value)
			currentInputs = append(currentInputs, nextInput)
			currentInputValues = append(currentInputValues, btcutil.Amount(u.Value))
		}
		return currentTotal, currentInputs, currentInputValues, make([][]byte, len(currentInputs)), nil
	}
	return txauthor.InputSource(f)
}
