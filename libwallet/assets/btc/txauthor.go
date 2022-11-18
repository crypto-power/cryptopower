package btc

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wallet/txrules"
)

type TxAuthor struct {
	sourceAccountNumber uint32
	// A map is used in place of an array because every destination address
	// is supposed to be unique.
	destinations      map[string]*sharedW.TransactionDestination
	changeAddress     string
	inputs            []*wire.TxIn
	changeDestination *sharedW.TransactionDestination

	unsignedTx     *txauthor.AuthoredTx
	needsConstruct bool
}

// fallbackfeerate defines the default fee rate to be used if API source of the
// current fee rates fails.
const fallbackfeerate = txrules.DefaultRelayFeePerKb

// noInputValue describes an error returned by the input source when no inputs
// were selected because each previous output value was zero.  Callers of
// txauthor.NewUnsignedTransaction need not report these errors to the user.
type noInputValue struct {
	confirmations int32
}

func (in noInputValue) Error() string {
	return fmt.Sprintf("inputs not spendable or have less than %d confirmations", in.confirmations)
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
		destinations:        make(map[string]*sharedW.TransactionDestination, 0),
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

	asset.TxAuthoredInfo.destinations[address] = &sharedW.TransactionDestination{
		Address:    address,
		UnitAmount: satoshiAmount,
		SendMax:    sendMax,
	}
	asset.TxAuthoredInfo.needsConstruct = true

	return nil
}

func (asset *BTCAsset) RemoveSendDestination(address string) {
	if _, ok := asset.TxAuthoredInfo.destinations[address]; ok {
		delete(asset.TxAuthoredInfo.destinations, address)
		asset.TxAuthoredInfo.needsConstruct = true
	}
}

func (asset *BTCAsset) SendDestination(address string) *sharedW.TransactionDestination {
	return asset.TxAuthoredInfo.destinations[address]
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
	feeToSendTx := txrules.FeeForSerializeSize(fallbackfeerate, estimatedSignedSerializeSize)
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

func (asset *BTCAsset) EstimateMaxSendAmount() (*sharedW.Amount, error) {
	txFeeAndSize, err := asset.EstimateFeeAndSize()
	if err != nil {
		return nil, err
	}

	if asset.TxAuthoredInfo == nil {
		return nil, fmt.Errorf("TxAuthoredInfo is nil")
	}

	spendableAccountBalance, err := asset.SpendableForAccount(int32(asset.TxAuthoredInfo.sourceAccountNumber))
	if err != nil {
		return nil, err
	}

	maxSendableAmount := spendableAccountBalance - txFeeAndSize.Fee.UnitValue

	return &sharedW.Amount{
		UnitValue: maxSendableAmount,
		CoinValue: btcutil.Amount(maxSendableAmount).ToBTC(),
	}, nil
}

func (asset *BTCAsset) UseInputs(utxoKeys []string) error {
	if asset.TxAuthoredInfo == nil {
		return fmt.Errorf("TxAuthoredInfo is nil")
	}
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

		input := wire.NewTxIn(op, nil, nil)
		inputs = append(inputs, input)
	}

	asset.TxAuthoredInfo.inputs = inputs
	asset.TxAuthoredInfo.needsConstruct = true
	return nil
}

func (asset *BTCAsset) Broadcast(privatePassphrase, transactionLabel string) error {
	unsignedTx, err := asset.unsignedTransaction()
	if err != nil {
		return utils.TranslateError(err)
	}

	if unsignedTx.ChangeIndex >= 0 {
		unsignedTx.RandomizeChangePosition()
	}

	var txBuf bytes.Buffer
	txBuf.Grow(unsignedTx.Tx.SerializeSize())
	err = unsignedTx.Tx.Serialize(&txBuf)
	if err != nil {
		log.Error(err)
		return err
	}

	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(txBuf.Bytes()))
	if err != nil {
		log.Error(err)
		// Bytes do not represent a valid raw transaction
		return err
	}

	lock := make(chan time.Time, 1)
	defer func() {
		lock <- time.Time{}
	}()

	err = asset.Internal().BTC.Unlock([]byte(privatePassphrase), lock)
	if err != nil {
		log.Error(err)
		return errors.New(utils.ErrInvalidPassphrase)
	}

	msgTx.LockTime = uint32(asset.GetBestBlockHeight()) + 1
	sigHashes := txscript.NewTxSigHashes(&msgTx)

	for index, txIn := range msgTx.TxIn {
		_, previousTXout, _, _, err := asset.Internal().BTC.FetchInputInfo(&txIn.PreviousOutPoint)
		if err != nil {
			log.Errorf("fetch previous outpoint txout failed: %v", err)
			return err
		}

		witness, signature, err := asset.Internal().BTC.ComputeInputScript(
			&msgTx, previousTXout, index, sigHashes, txscript.SigHashAll, nil,
		)

		if err != nil {
			log.Errorf("generating input signatures failed: %v", err)
			return err
		}

		msgTx.TxIn[index].Witness = witness
		msgTx.TxIn[index].SignatureScript = signature
	}

	var serializedTransaction bytes.Buffer
	serializedTransaction.Grow(msgTx.SerializeSize())
	err = msgTx.Serialize(&serializedTransaction)
	if err != nil {
		log.Error(err)
		return err
	}

	err = msgTx.Deserialize(bytes.NewReader(serializedTransaction.Bytes()))
	if err != nil {
		// Invalid tx
		log.Error(err)
		return err
	}

	err = asset.Internal().BTC.PublishTransaction(&msgTx, transactionLabel)
	if err != nil {
		return utils.TranslateError(err)
	}
	return nil
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
	// if len(asset.TxAuthoredInfo.inputs) != 0 {
	// 	return asset.constructCustomTransaction()
	// }

	var err error
	var outputs = make([]*wire.TxOut, 0)
	var changeSource *txauthor.ChangeSource
	var sendMax bool

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
			sendMax = true

		} else {
			output, err := txhelper.MakeBTCTxOutput(destination.Address, destination.UnitAmount, asset.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing tx output: %v", err)
				return nil, fmt.Errorf("make tx output error: %v", err)
			}

			// confirm that the txout will not be labelled dust on hitting mempool.
			if !txrules.IsDustOutput(output, fallbackfeerate) {
				outputs = append(outputs, output)
				continue
			}

			// txout failed the dust check validation.
			// Despite what the actual fee rate could be, defaultrelayfee rate is needed here.
			minAmount := txrules.GetDustThreshold(len(output.PkScript), fallbackfeerate)
			return nil, fmt.Errorf("minimum amount to send should be %v", minAmount)
		}
	}

	// Case activated when sendMax is false.
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

	inputSource := asset.makeInputSource(unspents, sendMax)
	unsignedTx, err := txauthor.NewUnsignedTransaction(outputs, fallbackfeerate, inputSource, changeSource)
	if err != nil {
		return nil, fmt.Errorf("creating unsigned tx failed: %v", err)
	}

	if unsignedTx.ChangeIndex == -1 {
		// The change amount is zero or the Txout is likely to be considered as dust
		// if sent to the mempool and thus rejected.
		return nil, errors.New("adding the change Txout or sendMax tx failed")
	}

	return unsignedTx, nil
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
// output with non-zero output values. The importsource aims to create the leanest
// transaction if its possible not to spend all the utxos available when servicing
// the current transaction spending amount. The sendMax shows that all utxos must
// be spent without any balances left in the account.
func (asset *BTCAsset) makeInputSource(outputs []*ListUnspentResult, sendMax bool) txauthor.InputSource {
	var (
		totalInputValue btcutil.Amount
		inputs          = make([]*wire.TxIn, 0, len(outputs))
		inputValues     = make([]btcutil.Amount, 0, len(outputs))
		sourceErr       error
	)

	// Sorts the outputs in the descending order (utxo with largest amount start)
	// This descending order helps in selecting the least number of utxos need
	// in the transactions to be made.
	sort.Slice(outputs, func(i, j int) bool { return outputs[i].Amount > outputs[j].Amount })

	// validates the utxo amounts and if an invalid amount is discovered and
	// error is returned.
	for _, output := range outputs {
		// Ignore unspendable utxos
		if !output.Spendable {
			continue
		}

		outputAmount, err := btcutil.NewAmount(output.Amount)
		if err != nil {
			sourceErr = fmt.Errorf("invalid amount `%v` in listunspent result", output.Amount)
			break
		}

		if outputAmount == 0 {
			continue
		}

		if !saneOutputValue(outputAmount) {
			sourceErr = fmt.Errorf("impossible output amount `%v` in listunspent result", outputAmount)
			break
		}
		totalInputValue += outputAmount

		previousOutPoint, err := parseOutPoint(output)
		if err != nil {
			sourceErr = fmt.Errorf("invalid data in listunspent result: %v", err)
			break
		}

		inputs = append(inputs, wire.NewTxIn(previousOutPoint, nil, nil))
		inputValues = append(inputValues, outputAmount)
	}

	if sourceErr == nil && totalInputValue == 0 {
		sourceErr = noInputValue{confirmations: asset.RequiredConfirmations()}
	}

	return func(target btcutil.Amount) (btcutil.Amount, []*wire.TxIn, []btcutil.Amount, [][]byte, error) {
		// if error found return it
		if sourceErr != nil {
			return 0, nil, nil, nil, sourceErr
		}

		// All utxo are to be spent with no change amount expected.
		if sendMax {
			return totalInputValue, inputs, inputValues, nil, nil
		}

		var totalutxo btcutil.Amount
		var index int
		for i, utxoAmount := range inputValues {
			if totalutxo < target {
				totalutxo += utxoAmount
			} else {
				// Found utxo(s) we can spend in the current tx.
				index = i + 1
				break
			}
		}
		return totalutxo, inputs[:index], inputValues[:index], nil, nil
	}
}

func saneOutputValue(amount btcutil.Amount) bool {
	return amount >= 0 && amount <= btcutil.MaxSatoshi
}

func parseOutPoint(input *ListUnspentResult) (*wire.OutPoint, error) {
	txHash, err := chainhash.NewHashFromStr(input.TxID)
	if err != nil {
		return nil, err
	}
	return wire.NewOutPoint(txHash, input.Vout), nil
}
