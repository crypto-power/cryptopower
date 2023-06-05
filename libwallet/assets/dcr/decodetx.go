package dcr

import (
	"fmt"

	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/blockchain/stake/v4"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdscript"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrdata/v7/txhelpers"
	sharedW "gitlab.com/cryptopower/cryptopower/libwallet/assets/wallet"
	"gitlab.com/cryptopower/cryptopower/libwallet/txhelper"
)

const BlockValid = 1 << 0

// DecodeTransaction uses `walletTx.Hex` to retrieve detailed information for a transaction.
func (asset *DCRAsset) DecodeTransaction(walletTx *sharedW.TxInfoFromWallet, netParams *chaincfg.Params) (*sharedW.Transaction, error) {
	msgTx, txFee, txSize, txFeeRate, err := txhelper.MsgTxFeeSizeRate(walletTx.Hex)
	if err != nil {
		return nil, err
	}

	inputs, totalWalletInput, totalWalletUnmixedInputs := asset.decodeTxInputs(msgTx, walletTx.Inputs)
	outputs, totalWalletOutput, totalWalletMixedOutputs, mixedOutputsCount := asset.decodeTxOutputs(msgTx, netParams, walletTx.Outputs)

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

	txType := txhelper.FormatTransactionType(w.TxTransactionType(msgTx))
	if isMixedTx {
		txType = txhelper.TxTypeMixed

		mixChange := totalWalletOutput - totalWalletMixedOutputs
		txFee = dcrutil.Amount(totalWalletUnmixedInputs - (totalWalletMixedOutputs + mixChange))
	}

	return &sharedW.Transaction{
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

func (asset *DCRAsset) decodeTxInputs(mtx *wire.MsgTx, walletInputs []*sharedW.WalletInput) (inputs []*sharedW.TxInput, totalWalletInputs, totalWalletUnmixedInputs int64) {
	inputs = make([]*sharedW.TxInput, len(mtx.TxIn))
	unmixedAccountNumber := asset.ReadInt32ConfigValueForKey(sharedW.AccountMixerUnmixedAccount, -1)

	for i, txIn := range mtx.TxIn {
		input := &sharedW.TxInput{
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

func (asset *DCRAsset) decodeTxOutputs(mtx *wire.MsgTx, netParams *chaincfg.Params,
	walletOutputs []*sharedW.WalletOutput) (outputs []*sharedW.TxOutput, totalWalletOutput, totalWalletMixedOutputs int64, mixedOutputsCount int32) {
	outputs = make([]*sharedW.TxOutput, len(mtx.TxOut))
	txType := txhelpers.DetermineTxType(mtx, true)
	mixedAccountNumber := asset.ReadInt32ConfigValueForKey(sharedW.AccountMixerMixedAccount, -1)

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

		output := &sharedW.TxOutput{
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
