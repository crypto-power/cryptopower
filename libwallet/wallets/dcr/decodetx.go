package dcr

import (
	"fmt"

	"decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/blockchain/stake/v4"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdscript"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrdata/v7/txhelpers"
	"gitlab.com/raedah/cryptopower/libwallet/txhelper"
)

const BlockValid = 1 << 0

// DecodeTransaction uses `walletTx.Hex` to retrieve detailed information for a transaction.
func (w *Wallet) DecodeTransaction(walletTx *TxInfoFromWallet, netParams *chaincfg.Params) (*Transaction, error) {
	msgTx, txFee, txSize, txFeeRate, err := txhelper.MsgTxFeeSizeRate(walletTx.Hex)
	if err != nil {
		return nil, err
	}

	inputs, totalWalletInput, totalWalletUnmixedInputs := w.decodeTxInputs(msgTx, walletTx.Inputs)
	outputs, totalWalletOutput, totalWalletMixedOutputs, mixedOutputsCount := w.decodeTxOutputs(msgTx, netParams, walletTx.Outputs)

	amount, direction := txhelper.TransactionAmountAndDirection(totalWalletInput, totalWalletOutput, int64(txFee))

	ssGenVersion, lastBlockValid, voteBits, ticketSpentHash := voteInfo(msgTx)

	// ticketSpentHash will be empty if this isn't a vote tx
	if txhelpers.IsSSRtx(msgTx) {
		ticketSpentHash = msgTx.TxIn[0].PreviousOutPoint.Hash.String()
		// set first tx input as amount for revoked txs
		amount = msgTx.TxIn[0].ValueIn
	} else if stake.IsSStx(msgTx) {
		// set first tx output as amount for ticket txs
		amount = msgTx.TxOut[0].Value
	}

	isMixedTx, mixDenom, _ := txhelpers.IsMixTx(msgTx)

	txType := txhelper.FormatTransactionType(wallet.TxTransactionType(msgTx))
	if isMixedTx {
		txType = txhelper.TxTypeMixed

		mixChange := totalWalletOutput - totalWalletMixedOutputs
		txFee = dcrutil.Amount(totalWalletUnmixedInputs - (totalWalletMixedOutputs + mixChange))
	}

	return &Transaction{
		WalletID:    walletTx.WalletID,
		Hash:        msgTx.TxHash().String(),
		Type:        txType,
		Hex:         walletTx.Hex,
		Timestamp:   walletTx.Timestamp,
		BlockHeight: walletTx.BlockHeight,

		MixDenomination: mixDenom,
		MixCount:        mixedOutputsCount,

		Version:  int32(msgTx.Version),
		LockTime: int32(msgTx.LockTime),
		Expiry:   int32(msgTx.Expiry),
		Fee:      int64(txFee),
		FeeRate:  int64(txFeeRate),
		Size:     txSize,

		Direction: direction,
		Amount:    amount,
		Inputs:    inputs,
		Outputs:   outputs,

		VoteVersion:     int32(ssGenVersion),
		LastBlockValid:  lastBlockValid,
		VoteBits:        voteBits,
		TicketSpentHash: ticketSpentHash,
	}, nil
}

func (wallet *Wallet) decodeTxInputs(mtx *wire.MsgTx, walletInputs []*WalletInput) (inputs []*TxInput, totalWalletInputs, totalWalletUnmixedInputs int64) {
	inputs = make([]*TxInput, len(mtx.TxIn))
	unmixedAccountNumber := wallet.ReadInt32ConfigValueForKey(AccountMixerUnmixedAccount, -1)

	for i, txIn := range mtx.TxIn {
		input := &TxInput{
			PreviousTransactionHash:  txIn.PreviousOutPoint.Hash.String(),
			PreviousTransactionIndex: int32(txIn.PreviousOutPoint.Index),
			PreviousOutpoint:         txIn.PreviousOutPoint.String(),
			Amount:                   txIn.ValueIn,
			AccountNumber:            -1, // correct account number is set below if this is a wallet output
		}

		// override account details if this is wallet input
		for _, walletInput := range walletInputs {
			if walletInput.Index == int32(i) {
				input.AccountNumber = walletInput.AccountNumber
				break
			}
		}

		if input.AccountNumber != -1 {
			totalWalletInputs += input.Amount
			if input.AccountNumber == unmixedAccountNumber {
				totalWalletUnmixedInputs += input.Amount
			}
		}

		inputs[i] = input
	}

	return
}

func (wallet *Wallet) decodeTxOutputs(mtx *wire.MsgTx, netParams *chaincfg.Params,
	walletOutputs []*WalletOutput) (outputs []*TxOutput, totalWalletOutput, totalWalletMixedOutputs int64, mixedOutputsCount int32) {
	outputs = make([]*TxOutput, len(mtx.TxOut))
	txType := txhelpers.DetermineTxType(mtx, true)
	mixedAccountNumber := wallet.ReadInt32ConfigValueForKey(AccountMixerMixedAccount, -1)

	for i, txOut := range mtx.TxOut {
		// get address and script type for output
		var address, scriptType string
		if (txType == stake.TxTypeSStx) && (stake.IsStakeCommitmentTxOut(i)) {
			addr, err := stake.AddrFromSStxPkScrCommitment(txOut.PkScript, netParams)
			if err == nil {
				address = addr.String()
			}
			scriptType = stdscript.STStakeSubmissionPubKeyHash.String()
		} else {
			// Ignore the error here since an error means the script
			// couldn't parse and there is no additional information
			// about it anyways.
			scriptClass, addrs := stdscript.ExtractAddrs(txOut.Version, txOut.PkScript, netParams)
			if len(addrs) > 0 {
				address = addrs[0].String()
			}
			scriptType = scriptClass.String()
		}

		output := &TxOutput{
			Index:         int32(i),
			Amount:        txOut.Value,
			Version:       int32(txOut.Version),
			ScriptType:    scriptType,
			Address:       address, // correct address, account name and number set below if this is a wallet output
			AccountNumber: -1,
		}

		// override address and account details if this is wallet output
		for _, walletOutput := range walletOutputs {
			if walletOutput.Index == output.Index {
				output.Internal = walletOutput.Internal
				output.Address = walletOutput.Address
				output.AccountNumber = walletOutput.AccountNumber
				break
			}
		}

		if output.AccountNumber != -1 {
			totalWalletOutput += output.Amount
			if output.AccountNumber == mixedAccountNumber {
				totalWalletMixedOutputs += output.Amount
				mixedOutputsCount++
			}
		}

		outputs[i] = output
	}

	return
}

func voteInfo(msgTx *wire.MsgTx) (ssGenVersion uint32, lastBlockValid bool, voteBits string, ticketSpentHash string) {
	if stake.IsSSGen(msgTx, true) {
		ssGenVersion = stake.SSGenVersion(msgTx)
		bits := stake.SSGenVoteBits(msgTx)
		voteBits = fmt.Sprintf("%#04x", bits)
		lastBlockValid = bits&uint16(BlockValid) != 0
		ticketSpentHash = msgTx.TxIn[1].PreviousOutPoint.Hash.String()
	}
	return
}
