package ltc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/errors"
	"github.com/ltcsuite/ltcd/chaincfg/chainhash"
	"github.com/ltcsuite/ltcd/ltcutil"
	"github.com/ltcsuite/ltcd/txscript"
	"github.com/ltcsuite/ltcd/wire"
	"github.com/ltcsuite/ltcwallet/wallet/txauthor"
	"github.com/ltcsuite/ltcwallet/wallet/txrules"
	"github.com/ltcsuite/ltcwallet/wallet/txsizes"
)

// TxAuthor holds the information required to construct a transaction that
// spends froma  wallet's account.
type TxAuthor struct {
	sourceAccountNumber uint32
	// A map is used in place of an array because every destination address
	// is supposed to be unique.
	destinations      map[string]*sharedW.TransactionDestination
	changeAddress     string
	inputs            []*wire.TxIn
	inputValues       []ltcutil.Amount
	txSpendAmount     ltcutil.Amount // Equal to fee + send amount
	changeDestination *sharedW.TransactionDestination

	unsignedTx     *txauthor.AuthoredTx
	needsConstruct bool

	selectedUXTOs []*sharedW.UnspentOutput

	mu sync.RWMutex
}

// NewUnsignedTx creates a new unsigned transaction.
func (asset *Asset) NewUnsignedTx(sourceAccountNumber int32, utxos []*sharedW.UnspentOutput) error {
	if asset == nil {
		return fmt.Errorf(utils.ErrWalletNotFound)
	}

	if len(utxos) == 0 {
		// Validate source account number if no utxos were passed.
		_, err := asset.GetAccount(sourceAccountNumber)
		if err != nil {
			return err
		}
	}

	asset.TxAuthoredInfo = &TxAuthor{
		sourceAccountNumber: uint32(sourceAccountNumber),
		destinations:        make(map[string]*sharedW.TransactionDestination, 0),
		needsConstruct:      true,
		selectedUXTOs:       utxos,
	}
	return nil
}

// GetUnsignedTx returns the unsigned transaction.
func (asset *Asset) GetUnsignedTx() *TxAuthor {
	return asset.TxAuthoredInfo
}

// IsUnsignedTxExist returns true if an unsigned transaction exists.
func (asset *Asset) IsUnsignedTxExist() bool {
	return asset.TxAuthoredInfo != nil
}

// ComputeTxSizeEstimation computes the estimated size of the final raw transaction.
func (asset *Asset) ComputeTxSizeEstimation(dstAddress string, utxos []*sharedW.UnspentOutput) (int, error) {
	if len(utxos) == 0 {
		return 0, nil
	}

	if dstAddress == "" {
		return -1, errors.New("destination address missing")
	}

	var sendAmount int64
	for _, c := range utxos {
		sendAmount += c.Amount.ToInt()
	}

	output, err := txhelper.MakeLTCTxOutput(dstAddress, sendAmount, asset.chainParams)
	if err != nil {
		return -1, fmt.Errorf("computing utxo size failed: %v", err)
	}

	estimatedSize := txsizes.EstimateSerializeSize(len(utxos), []*wire.TxOut{output}, true)
	return estimatedSize, nil
}

// AddSendDestination adds a destination address to the transaction.
// The amount to be sent to the address is specified in litoshi.
// If sendMax is true, the amount is ignored and the maximum amount is sent.
func (asset *Asset) AddSendDestination(address string, litoshiAmount int64, sendMax bool) error {
	_, err := ltcutil.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return utils.TranslateError(err)
	}

	if err := asset.validateSendAmount(sendMax, litoshiAmount); err != nil {
		return err
	}

	asset.TxAuthoredInfo.mu.Lock()
	defer asset.TxAuthoredInfo.mu.Unlock()

	asset.TxAuthoredInfo.destinations[address] = &sharedW.TransactionDestination{
		Address:    address,
		UnitAmount: litoshiAmount,
		SendMax:    sendMax,
	}
	asset.TxAuthoredInfo.needsConstruct = true

	return nil
}

// RemoveSendDestination removes a destination address from the transaction.
func (asset *Asset) RemoveSendDestination(address string) {
	asset.TxAuthoredInfo.mu.Lock()
	defer asset.TxAuthoredInfo.mu.Unlock()

	if _, ok := asset.TxAuthoredInfo.destinations[address]; ok {
		delete(asset.TxAuthoredInfo.destinations, address)
		asset.TxAuthoredInfo.needsConstruct = true
	}
}

// SendDestination returns a list of all destination addresses added to the transaction.
func (asset *Asset) SendDestination(address string) *sharedW.TransactionDestination {
	asset.TxAuthoredInfo.mu.RLock()
	defer asset.TxAuthoredInfo.mu.RUnlock()

	return asset.TxAuthoredInfo.destinations[address]
}

// SetChangeDestination sets the change address for the transaction.
func (asset *Asset) SetChangeDestination(address string) {
	asset.TxAuthoredInfo.mu.Lock()
	defer asset.TxAuthoredInfo.mu.Unlock()

	asset.TxAuthoredInfo.changeDestination = &sharedW.TransactionDestination{
		Address: address,
	}
	asset.TxAuthoredInfo.needsConstruct = true
}

// RemoveChangeDestination removes the change address from the transaction.
func (asset *Asset) RemoveChangeDestination() {
	asset.TxAuthoredInfo.mu.RLock()
	defer asset.TxAuthoredInfo.mu.RUnlock()

	asset.TxAuthoredInfo.changeDestination = nil
	asset.TxAuthoredInfo.needsConstruct = true
}

// TotalSendAmount returns the total amount to be sent in the transaction.
func (asset *Asset) TotalSendAmount() *sharedW.Amount {
	asset.TxAuthoredInfo.mu.RLock()
	defer asset.TxAuthoredInfo.mu.RUnlock()

	var totalSendAmountLitoshi int64 = 0
	for _, destination := range asset.TxAuthoredInfo.destinations {
		totalSendAmountLitoshi += destination.UnitAmount
	}

	return &sharedW.Amount{
		UnitValue: totalSendAmountLitoshi,
		CoinValue: ltcutil.Amount(totalSendAmountLitoshi).ToBTC(),
	}
}

// EstimateFeeAndSize estimates the fee and size of the transaction.
func (asset *Asset) EstimateFeeAndSize() (*sharedW.TxFeeAndSize, error) {
	// compute the amount to be sent in the current tx.
	sendAmount := ltcutil.Amount(asset.TotalSendAmount().UnitValue)

	asset.TxAuthoredInfo.mu.Lock()
	defer asset.TxAuthoredInfo.mu.Unlock()

	unsignedTx, err := asset.unsignedTransaction()
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	// Since the fee is already calculated when computing the change source out
	// or single destination to send max amount, no need to repeat calculations again.
	feeToSpend := asset.TxAuthoredInfo.txSpendAmount - sendAmount
	feeAmount := &sharedW.Amount{
		UnitValue: int64(feeToSpend),
		CoinValue: feeToSpend.ToBTC(),
	}

	var change *sharedW.Amount
	if unsignedTx.ChangeIndex >= 0 {
		txOut := unsignedTx.Tx.TxOut[unsignedTx.ChangeIndex]
		change = &sharedW.Amount{
			UnitValue: txOut.Value,
			CoinValue: AmountLTC(txOut.Value),
		}
	}

	// TODO: confirm if the size on UI needs to be in vB to B.
	// This estimation returns size in Bytes (B).
	estimatedSize := txsizes.EstimateSerializeSize(len(unsignedTx.Tx.TxIn), unsignedTx.Tx.TxOut, true)
	// This estimation returns size in virtualBytes (vB).
	// estimatedSize := feeToSpend.ToBTC() / fallBackFeeRate.ToBTC()

	return &sharedW.TxFeeAndSize{
		FeeRate:             asset.GetUserFeeRate().ToInt(),
		EstimatedSignedSize: estimatedSize,
		Fee:                 feeAmount,
		Change:              change,
	}, nil
}

// EstimateMaxSendAmount estimates the maximum amount that can be sent in the transaction.
func (asset *Asset) EstimateMaxSendAmount() (*sharedW.Amount, error) {
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
		CoinValue: ltcutil.Amount(maxSendableAmount).ToBTC(),
	}, nil
}

// Broadcast broadcasts the transaction to the network.
func (asset *Asset) Broadcast(privatePassphrase, transactionLabel string) ([]byte, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrLTCNotInitialized
	}

	asset.TxAuthoredInfo.mu.Lock()
	defer asset.TxAuthoredInfo.mu.Unlock()

	unsignedTx, err := asset.unsignedTransaction()
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	// If the change output is the only one, no need to change position.
	if unsignedTx.ChangeIndex > 0 {
		unsignedTx.RandomizeChangePosition()
	}

	// Test encode and decode the tx to check its validity after being signed.
	msgTx := unsignedTx.Tx

	lock := make(chan time.Time, 1)
	defer func() {
		lock <- time.Time{}
	}()

	err = asset.Internal().LTC.Unlock([]byte(privatePassphrase), lock)
	if err != nil {
		log.Errorf("unlocking the wallet failed: %v", err)
		return nil, errors.New(utils.ErrInvalidPassphrase)
	}

	// To discourage fee sniping, LockTime is explicity set in the raw tx.
	// More documentation on this:
	// https://bitcoin.stackexchange.com/questions/48384/why-bitcoin-core-creates-time-locked-transactions-by-default
	msgTx.LockTime = uint32(asset.GetBestBlockHeight())

	for index, txIn := range msgTx.TxIn {
		_, previousTXout, _, _, err := asset.Internal().LTC.FetchInputInfo(&txIn.PreviousOutPoint)
		if err != nil {
			log.Errorf("fetch previous outpoint txout failed: %v", err)
			return nil, err
		}

		// prevOutScript := unsignedTx.PrevScripts[index]
		prevOutAmount := int64(asset.TxAuthoredInfo.inputValues[index])
		// prevOutFetcher := txscript.NewCannedPrevOutputFetcher(prevOutScript, prevOutAmount)
		sigHashes := txscript.NewTxSigHashes(msgTx)

		witness, signature, err := asset.Internal().LTC.ComputeInputScript(
			msgTx, previousTXout, index, sigHashes, txscript.SigHashAll, nil,
		)
		if err != nil {
			log.Errorf("generating input signatures failed: %v", err)
			return nil, err
		}

		msgTx.TxIn[index].Witness = witness
		msgTx.TxIn[index].SignatureScript = signature

		// Prove that the transaction has been validly signed by executing the
		// script pair.
		flags := txscript.ScriptBip16 | txscript.ScriptVerifyDERSignatures |
			txscript.ScriptStrictMultiSig | txscript.ScriptDiscourageUpgradableNops
		vm, err := txscript.NewEngine(previousTXout.PkScript, msgTx, 0, flags, nil, nil,
			prevOutAmount)
		if err != nil {
			log.Errorf("creating validation engine failed: %v", err)
			return nil, err
		}
		if err := vm.Execute(); err != nil {
			log.Errorf("executing the validation engine failed: %v", err)
			return nil, err
		}
	}

	var serializedTransaction bytes.Buffer
	serializedTransaction.Grow(msgTx.SerializeSize())
	err = msgTx.Serialize(&serializedTransaction)
	if err != nil {
		log.Errorf("encoding the tx to test its validity failed: %v", err)
		return nil, err
	}

	err = msgTx.Deserialize(bytes.NewReader(serializedTransaction.Bytes()))
	if err != nil {
		// Invalid tx
		log.Errorf("decoding the tx to test its validity failed: %v", err)
		return nil, err
	}

	err = asset.Internal().LTC.PublishTransaction(msgTx, transactionLabel)
	return nil, utils.TranslateError(err)
}

func (asset *Asset) unsignedTransaction() (*txauthor.AuthoredTx, error) {
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

func (asset *Asset) constructTransaction() (*txauthor.AuthoredTx, error) {
	var err error
	outputs := make([]*wire.TxOut, 0)
	var changeSource *txauthor.ChangeSource
	setFeeRate := ltcutil.Amount(asset.GetUserFeeRate().ToInt())
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
			changeSource, err = txhelper.MakeLTCTxChangeSource(destination.Address, asset.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing change source: %v", err)
				return nil, fmt.Errorf("max amount change source error: %v", err)
			}
			sendMax = true

		} else {
			output, err := txhelper.MakeLTCTxOutput(destination.Address, destination.UnitAmount, asset.chainParams)
			if err != nil {
				log.Errorf("constructTransaction: error preparing tx output: %v", err)
				return nil, fmt.Errorf("make tx output error: %v", err)
			}

			// confirm that the txout will not be rejected on hitting the mempool.
			if err = txrules.CheckOutput(output, setFeeRate); err != nil {
				return nil, fmt.Errorf("main txOut validation failed %v", err)
			}
			outputs = append(outputs, output)
		}
	}

	// Case activated when sendMax is false.
	if changeSource == nil {
		// ltcwallet should ordinarily handle cases where a nil changeSource
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

	// if preset with a selected list of UTXOs exists, use them instead.
	unspents := asset.TxAuthoredInfo.selectedUXTOs
	if len(unspents) == 0 {
		unspents, err = asset.UnspentOutputs(int32(asset.TxAuthoredInfo.sourceAccountNumber))
		if err != nil {
			return nil, err
		}
	}

	inputSource := asset.makeInputSource(unspents, sendMax)
	unsignedTx, err := txauthor.NewUnsignedTransaction(outputs, setFeeRate, inputSource, changeSource)
	if err != nil {
		return nil, fmt.Errorf("creating unsigned tx failed: %v", err)
	}

	if unsignedTx.ChangeIndex == -1 {
		// The change amount is zero or the Txout is likely to be considered as dust
		// if sent to the mempool the whole tx will be rejected.
		return nil, errors.New("adding the change txOut or sendMax tx failed")
	}

	// Confirm that the change output is valid too.
	if err = txrules.CheckOutput(unsignedTx.Tx.TxOut[unsignedTx.ChangeIndex], setFeeRate); err != nil {
		return nil, fmt.Errorf("change txOut validation failed %v", err)
	}

	return unsignedTx, nil
}

// changeSource derives an internal address from the source wallet and account
// for this unsigned tx, if a change address had not been previously derived.
// The derived (or previously derived) address is used to prepare a
// change source for receiving change from this tx back into the sharedW.
func (asset *Asset) changeSource() (*txauthor.ChangeSource, error) {
	if asset.TxAuthoredInfo.changeAddress == "" {
		changeAccount := asset.TxAuthoredInfo.sourceAccountNumber
		address, err := asset.Internal().LTC.NewChangeAddress(changeAccount, asset.GetScope())
		if err != nil {
			return nil, fmt.Errorf("change address error: %v", err)
		}
		asset.TxAuthoredInfo.changeAddress = address.String()
	}

	changeSource, err := txhelper.MakeLTCTxChangeSource(asset.TxAuthoredInfo.changeAddress, asset.chainParams)
	if err != nil {
		log.Errorf("constructTransaction: error preparing change source: %v", err)
		return nil, fmt.Errorf("change source error: %v", err)
	}

	return changeSource, nil
}

// validateSendAmount validate the amount to send to a destination address
func (asset *Asset) validateSendAmount(sendMax bool, litoshiAmount int64) error {
	if !sendMax && (litoshiAmount <= 0 || litoshiAmount > maxAmountLitoshi) {
		return errors.E(errors.Invalid, "invalid amount")
	}
	return nil
}

// makeInputSource creates an InputSource that creates inputs for every unspent
// output with non-zero output values. The importsource aims to create the leanest
// transaction possible. It plans not to spend all the utxos available when servicing
// the current transaction spending amount if possible. The sendMax shows that
// all utxos must be spent without any balance(unspent utxo) left in the account.
func (asset *Asset) makeInputSource(outputs []*sharedW.UnspentOutput, sendMax bool) txauthor.InputSource {
	var (
		sourceErr       error
		totalInputValue ltcutil.Amount

		inputs      = make([]*wire.TxIn, 0, len(outputs))
		inputValues = make([]ltcutil.Amount, 0, len(outputs))
		pkScripts   = make([][]byte, 0, len(outputs))
	)

	// sorting is only necessary when send max is false.
	if !sendMax {
		// Sorts the outputs in the descending order (utxo with largest amount start)
		// This descending order helps in selecting the least number of utxos needed
		// in the servicing the transaction to be made.
		sort.Slice(outputs, func(i, j int) bool { return outputs[i].Amount.ToCoin() > outputs[j].Amount.ToCoin() })
	}

	// validates the utxo amounts and if an invalid amount is discovered an
	// error is returned.
	for _, output := range outputs {
		// Ignore unspendable utxos
		if !output.Spendable {
			continue
		}

		if output.Amount == nil || output.Amount.ToCoin() == 0 {
			continue
		}

		if !saneOutputValue(output.Amount.(Amount)) {
			sourceErr = fmt.Errorf("impossible output amount `%v` in listunspent result", output.Amount)
			break
		}

		previousOutPoint, err := parseOutPoint(output)
		if err != nil {
			sourceErr = fmt.Errorf("invalid TxIn data found: %v", err)
			break
		}

		script, err := hex.DecodeString(output.ScriptPubKey)
		if err != nil {
			sourceErr = fmt.Errorf("invalid TxIn pkScript data found: %v", err)
			break
		}

		// Determine whether this transaction output is considered dust
		if txrules.IsDustOutput(wire.NewTxOut(output.Amount.ToInt(), script), txrules.DefaultRelayFeePerKb) {
			log.Errorf("transaction contains a dust output with value: %v", output.Amount.String())
			continue
		}

		totalInputValue += ltcutil.Amount(output.Amount.(Amount))
		pkScripts = append(pkScripts, script)
		inputValues = append(inputValues, ltcutil.Amount(output.Amount.(Amount)))
		inputs = append(inputs, wire.NewTxIn(previousOutPoint, nil, nil))
	}

	if sourceErr == nil && totalInputValue == 0 {
		// Constructs an error describing the possible reasons why the
		// wallet balance cannot be spent.
		sourceErr = fmt.Errorf("inputs not spendable or have less than %d confirmations",
			asset.RequiredConfirmations())
	}

	return func(target ltcutil.Amount) (ltcutil.Amount, []*wire.TxIn, []ltcutil.Amount, [][]byte, error) {
		// If an error was found return it first.
		if sourceErr != nil {
			return 0, nil, nil, nil, sourceErr
		}

		// This sets the amount the tx will spend if utxos to balance it exists.
		// This spend amount will be crucial in calculating the projected tx fee.
		asset.TxAuthoredInfo.txSpendAmount = target

		// All utxos are to be spent with no change amount expected.
		if sendMax {
			asset.TxAuthoredInfo.inputs = inputs
			asset.TxAuthoredInfo.inputValues = inputValues
			return totalInputValue, inputs, inputValues, pkScripts, nil
		}

		var index int
		var totalUtxo ltcutil.Amount

		for _, utxoAmount := range inputValues {
			if totalUtxo < target {
				// Found some utxo(s) we can spend in the current tx.
				index++

				totalUtxo += utxoAmount
				continue
			}
			break
		}
		asset.TxAuthoredInfo.inputs = inputs[:index]
		asset.TxAuthoredInfo.inputValues = inputValues[:index]
		return totalUtxo, inputs[:index], inputValues[:index], pkScripts[:index], nil
	}
}

func saneOutputValue(amount Amount) bool {
	return amount >= 0 && amount <= ltcutil.MaxSatoshi
}

func parseOutPoint(input *sharedW.UnspentOutput) (*wire.OutPoint, error) {
	txHash, err := chainhash.NewHashFromStr(input.TxID)
	if err != nil {
		return nil, err
	}
	return wire.NewOutPoint(txHash, input.Vout), nil
}
