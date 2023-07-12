package dcr

import (
	"fmt"

	w "decred.org/dcrwallet/v3/wallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/decred/dcrd/chaincfg/chainhash"
)

func (asset *Asset) decodeTransactionWithTxSummary(txSummary *w.TransactionSummary,
	blockHash *chainhash.Hash,
) (*sharedW.Transaction, error) {
	var blockHeight int32 = sharedW.UnminedTxHeight
	if blockHash != nil {
		blockIdentifier := w.NewBlockIdentifierFromHash(blockHash)
		ctx, _ := asset.ShutdownContextWithCancel()
		blockInfo, err := asset.Internal().DCR.BlockInfo(ctx, blockIdentifier)
		if err != nil {
			log.Error(err)
		} else {
			blockHeight = blockInfo.Height
		}
	}

	walletInputs := make([]*sharedW.WInput, len(txSummary.MyInputs))
	for i, input := range txSummary.MyInputs {
		accountNumber := int32(input.PreviousAccount)
		accountName, err := asset.AccountName(accountNumber)
		if err != nil {
			log.Error(err)
		}

		walletInputs[i] = &sharedW.WInput{
			Index:    int32(input.Index),
			AmountIn: int64(input.PreviousAmount),
			WAccount: &sharedW.WAccount{
				AccountNumber: accountNumber,
				AccountName:   accountName,
			},
		}
	}

	walletOutputs := make([]*sharedW.WOutput, len(txSummary.MyOutputs))
	for i, output := range txSummary.MyOutputs {
		accountNumber := int32(output.Account)
		accountName, err := asset.AccountName(accountNumber)
		if err != nil {
			log.Error(err)
		}

		walletOutputs[i] = &sharedW.WOutput{
			Index:     int32(output.Index),
			AmountOut: int64(output.Amount),
			Internal:  output.Internal,
			Address:   output.Address.String(),
			WAccount: &sharedW.WAccount{
				AccountNumber: accountNumber,
				AccountName:   accountName,
			},
		}
	}

	walletTx := &sharedW.TxInfoFromWallet{
		WalletID:    asset.ID,
		BlockHeight: blockHeight,
		Timestamp:   txSummary.Timestamp,
		Hex:         fmt.Sprintf("%x", txSummary.Transaction),
		Inputs:      walletInputs,
		Outputs:     walletOutputs,
	}

	decodedTx, err := asset.DecodeTransaction(walletTx, asset.chainParams)
	if err != nil {
		return nil, err
	}

	if decodedTx.TicketSpentHash != "" {
		ticketPurchaseTx, err := asset.GetTransactionRaw(decodedTx.TicketSpentHash)
		if err != nil {
			return nil, err
		}

		timeDifferenceInSeconds := decodedTx.Timestamp - ticketPurchaseTx.Timestamp
		decodedTx.DaysToVoteOrRevoke = int32(timeDifferenceInSeconds / 86400) // seconds to days conversion

		// calculate reward
		var ticketInvestment int64
		for _, input := range ticketPurchaseTx.Inputs {
			if input.AccountNumber > -1 {
				ticketInvestment += input.Amount
			}
		}

		var ticketOutput int64
		for _, output := range walletTx.Outputs {
			if output.AccountNumber > -1 {
				ticketOutput += output.AmountOut
			}
		}

		decodedTx.VoteReward = ticketOutput - ticketInvestment

		// update ticket with spender hash
		ticketPurchaseTx.TicketSpender = decodedTx.Hash
		asset.GetWalletDataDb().SaveOrUpdate(&sharedW.Transaction{}, ticketPurchaseTx)
	}

	return decodedTx, nil
}
