package btc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/wallet"
)

// UnMinedTxHeight defines the height of the txs
const UnMinedTxHeight int32 = -1

// txCache helps to cache the transac
type txCache struct {
	blockHeight int32

	uminedTxs []sharedW.Transaction
	minedTxs  []sharedW.Transaction

	mu sync.RWMutex
}

func (asset *BTCAsset) PublishUnminedTransactions() error {
	loadedAsset := asset.Internal().BTC
	if loadedAsset == nil {
		return utils.ErrBTCNotInitialized
	}

	// Trigger the mempool txs to be updated if they are outdated
	_, err := asset.getTransactionsRaw(0, 0, true)
	if err != nil {
		return err
	}

	asset.txs.mu.RLock()
	mempoolTxs := asset.txs.uminedTxs
	asset.txs.mu.RUnlock()

	for _, tx := range mempoolTxs {
		decodeTx, err := asset.decodeTxHex(tx.Hex)
		if err != err {
			return err
		}
		if err := loadedAsset.PublishTransaction(decodeTx, tx.Label); err != nil {
			return err
		}
	}
	return nil
}

func (asset *BTCAsset) CountTransactions(txFilter int32) (int, error) {
	transactions, err := asset.getTransactionsRaw(0, 0, true)
	return len(transactions), err
}

func (asset *BTCAsset) GetTransactionRaw(txHash string) (*sharedW.Transaction, error) {
	transactions, err := asset.getTransactionsRaw(0, 0, true)
	for _, tx := range transactions {
		if tx.Hash == txHash {
			return &tx, nil
		}
	}
	return nil, err
}

func (asset *BTCAsset) TxMatchesFilter(tx *sharedW.Transaction, txFilter int32) bool {
	log.Warn(utils.ErrBTCMethodNotImplemented("TxMatchesFilter"))
	return false
}

func (asset *BTCAsset) GetTransactionsRaw(offset, limit, txFilter int32, newestFirst bool) (transactions []sharedW.Transaction, err error) {
	transactions, err = asset.getTransactionsRaw(offset, limit, newestFirst)
	return
}

// getTransactionsRaw returns the transactions between the start block and the endblock.
// start block height is equal to the offset and endblock is equal to the summation
// of the offset and the limit values.
// If startblock is less that the endblock the list return is in ascending order
// (starts with the oldest) otherwise its in descending (starts with the newest) order.
func (asset *BTCAsset) getTransactionsRaw(offset, limit int32, newestFirst bool) ([]sharedW.Transaction, error) {
	asset.txs.mu.RLock()
	allTxs := append(asset.txs.uminedTxs, asset.txs.uminedTxs...)
	txCacheHeight := asset.txs.blockHeight
	asset.txs.mu.RUnlock()

	if txCacheHeight == asset.GetBestBlockHeight() {
		// if the best block hasn't changed return the preset list of txs.
		return allTxs, nil
	}

	loadedAsset := asset.Internal().BTC
	if loadedAsset == nil {
		return nil, utils.ErrBTCNotInitialized
	}

	// if both offset and limit are each equal to zero, the transactions returned
	// include mempool contents and the mined txs.
	var startBlock, endBlock *wallet.BlockIdentifier
	if offset > 0 {
		if newestFirst { // Ascending order
			startBlock = wallet.NewBlockIdentifierFromHeight(offset)
		} else { // Descending Order
			endBlock = wallet.NewBlockIdentifierFromHeight(offset)
		}
	}

	refheight := offset + limit
	if refheight > 0 {
		if newestFirst { // Ascending order
			endBlock = wallet.NewBlockIdentifierFromHeight(refheight)
		} else { // Descending Order
			startBlock = wallet.NewBlockIdentifierFromHeight(refheight)
		}
	}

	txResult, err := loadedAsset.GetTransactions(startBlock, endBlock, "", asset.syncCtx.Done())
	if err != nil {
		return nil, err
	}

	unminedTxs := asset.decodeTransactionWithTxSummary(UnMinedTxHeight, txResult.UnminedTransactions)
	minedTxs := asset.extractTxs(txResult.MinedTransactions)

	// Cache the recent data.
	asset.txs.mu.Lock()
	asset.txs.uminedTxs = unminedTxs
	asset.txs.minedTxs = minedTxs
	asset.txs.blockHeight = asset.GetBestBlockHeight()
	asset.txs.mu.Unlock()

	// Return the unmined and the mined txs.
	return append(unminedTxs, minedTxs...), nil
}

func (asset *BTCAsset) extractTxs(blocks []wallet.Block) []sharedW.Transaction {
	txs := make([]sharedW.Transaction, 0)
	for _, block := range blocks {
		decodedTxs := asset.decodeTransactionWithTxSummary(block.Height, block.Transactions)
		txs = append(txs, decodedTxs...)
	}
	return txs
}

func (asset *BTCAsset) decodeTransactionWithTxSummary(blockheight int32, txsummary []wallet.TransactionSummary) []sharedW.Transaction {
	txs := make([]sharedW.Transaction, 0, len(txsummary))
	for _, rawtx := range txsummary {
		txHex := fmt.Sprintf("%x", rawtx.Transaction)
		decodedTx, _ := asset.decodeTxHex(txHex)
		txSize := decodedTx.SerializeSize()
		feeRate := rawtx.Fee * 1000 / btcutil.Amount(txSize)

		inputs, totalInputsAmount := asset.decodeTxInputs(decodedTx, rawtx.MyInputs)
		outputs, totalOutputsAmount := asset.decodeTxOutputs(decodedTx, rawtx.MyOutputs)
		amount, direction := txhelper.TransactionAmountAndDirection(totalInputsAmount, totalOutputsAmount, int64(rawtx.Fee))

		txs = append(txs, sharedW.Transaction{
			WalletID:    asset.GetWalletID(),
			Hash:        rawtx.Hash.String(),
			Type:        "",
			Hex:         txHex,
			Timestamp:   rawtx.Timestamp,
			BlockHeight: blockheight,

			Version:  decodedTx.Version,
			LockTime: int32(decodedTx.LockTime),
			Fee:      int64(rawtx.Fee),
			FeeRate:  int64(feeRate),
			Size:     txSize,
			Label:    rawtx.Label,

			Direction: direction,
			Amount:    amount,
			Inputs:    inputs,
			Outputs:   outputs,
		})
	}
	return txs
}

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
			log.Errorf("Error wheh extracting pkscript")
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
