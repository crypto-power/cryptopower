package btc

import (
	"bytes"
	"encoding/hex"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/wallet"
)

func (asset *BTCAsset) decodeTxHex(txHex string) (*wire.MsgTx, error) {
	b, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, err
	}

	tx := &wire.MsgTx{}
	if err = tx.Deserialize(bytes.NewReader(b)); err != nil {
		return nil, err
	}

	return tx, nil
}

func (asset *BTCAsset) decodeTxInputs(mtx *wire.MsgTx,
	walletInputs []wallet.TransactionSummaryInput) (inputs []*sharedW.TxInput, totalWalletInputs int64) {
	inputs = make([]*sharedW.TxInput, len(mtx.TxIn))

	for i, txIn := range mtx.TxIn {
		input := &sharedW.TxInput{
			PreviousTransactionHash:  txIn.PreviousOutPoint.Hash.String(),
			PreviousTransactionIndex: int32(txIn.PreviousOutPoint.Index),
			PreviousOutpoint:         txIn.PreviousOutPoint.String(),
			AccountNumber:            -1, // correct account number is set below if this is a wallet output
		}

		// override account details if this is wallet input
		for _, walletInput := range walletInputs {
			if int(walletInput.Index) == i {
				input.AccountNumber = int32(walletInput.PreviousAccount)
				input.Amount = int64(walletInput.PreviousAmount)
				break
			}
		}

		if input.AccountNumber != -1 {
			totalWalletInputs += input.Amount
		}

		inputs[i] = input
	}
	return
}

func (asset *BTCAsset) decodeTxOutputs(mtx *wire.MsgTx,
	walletOutputs []wallet.TransactionSummaryOutput) (outputs []*sharedW.TxOutput, totalWalletOutput int64) {
	outputs = make([]*sharedW.TxOutput, len(mtx.TxOut))

	for i, txOut := range mtx.TxOut {
		// get address and script type for output
		var address, scriptType string
		scriptClass, addrs, _, err := txscript.ExtractPkScriptAddrs(txOut.PkScript, asset.chainParams)
		if err != nil {
			log.Errorf("Error extracting pkscript")
		}

		if len(addrs) > 0 {
			address = addrs[0].String()
		}
		scriptType = scriptClass.String()

		output := &sharedW.TxOutput{
			Index:         int32(i),
			Amount:        txOut.Value,
			ScriptType:    scriptType,
			Address:       address, // correct address, account name and number set below if this is a wallet output
			AccountNumber: -1,
		}

		// override address and account details if this is wallet output
		for _, walletOutput := range walletOutputs {
			if int32(walletOutput.Index) == output.Index {
				output.Internal = walletOutput.Internal
				output.AccountNumber = int32(walletOutput.Account)
				break
			}
		}

		if output.AccountNumber != -1 {
			totalWalletOutput += output.Amount
		}

		outputs[i] = output
	}

	return
}
