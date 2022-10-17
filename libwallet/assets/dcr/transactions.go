package dcr

import (
	"encoding/json"

	"github.com/asdine/storm"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/assets/wallet/walletdata"
	"gitlab.com/raedah/cryptopower/libwallet/txhelper"
)

const (
	// Export constants for use in mobile apps
	// since gomobile excludes fields from sub packages.
	TxFilterAll         = walletdata.TxFilterAll
	TxFilterSent        = walletdata.TxFilterSent
	TxFilterReceived    = walletdata.TxFilterReceived
	TxFilterTransferred = walletdata.TxFilterTransferred
	TxFilterStaking     = walletdata.TxFilterStaking
	TxFilterCoinBase    = walletdata.TxFilterCoinBase
	TxFilterRegular     = walletdata.TxFilterRegular
	TxFilterMixed       = walletdata.TxFilterMixed
	TxFilterVoted       = walletdata.TxFilterVoted
	TxFilterRevoked     = walletdata.TxFilterRevoked
	TxFilterImmature    = walletdata.TxFilterImmature
	TxFilterLive        = walletdata.TxFilterLive
	TxFilterUnmined     = walletdata.TxFilterUnmined
	TxFilterExpired     = walletdata.TxFilterExpired
	TxFilterTickets     = walletdata.TxFilterTickets

	TxDirectionInvalid     = txhelper.TxDirectionInvalid
	TxDirectionSent        = txhelper.TxDirectionSent
	TxDirectionReceived    = txhelper.TxDirectionReceived
	TxDirectionTransferred = txhelper.TxDirectionTransferred

	TxTypeRegular        = txhelper.TxTypeRegular
	TxTypeCoinBase       = txhelper.TxTypeCoinBase
	TxTypeTicketPurchase = txhelper.TxTypeTicketPurchase
	TxTypeVote           = txhelper.TxTypeVote
	TxTypeRevocation     = txhelper.TxTypeRevocation
	TxTypeMixed          = txhelper.TxTypeMixed

	TicketStatusUnmined        = "unmined"
	TicketStatusImmature       = "immature"
	TicketStatusLive           = "live"
	TicketStatusVotedOrRevoked = "votedrevoked"
	TicketStatusExpired        = "expired"
)

func (asset *DCRAsset) PublishUnminedTransactions() error {
	n, err := asset.Internal().DCR.NetworkBackend()
	if err != nil {
		log.Error(err)
		return err
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	return asset.Internal().DCR.PublishUnminedTransactions(ctx, n)
}

func (asset *DCRAsset) GetTransaction(txHash string) (string, error) {
	transaction, err := asset.GetTransactionRaw(txHash)
	if err != nil {
		log.Error(err)
		return "", err
	}

	result, err := json.Marshal(transaction)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func (asset *DCRAsset) GetTransactionRaw(txHash string) (*wallet.Transaction, error) {
	hash, err := chainhash.NewHashFromStr(txHash)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	txSummary, _, blockHash, err := asset.Internal().DCR.TransactionSummary(ctx, hash)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return asset.decodeTransactionWithTxSummary(txSummary, blockHash)
}

func (asset *DCRAsset) GetTransactions(offset, limit, txFilter int32, newestFirst bool) (string, error) {
	transactions, err := asset.GetTransactionsRaw(offset, limit, txFilter, newestFirst)
	if err != nil {
		return "", err
	}

	jsonEncodedTransactions, err := json.Marshal(&transactions)
	if err != nil {
		return "", err
	}

	return string(jsonEncodedTransactions), nil
}

func (asset *DCRAsset) GetTransactionsRaw(offset, limit, txFilter int32, newestFirst bool) (transactions []wallet.Transaction, err error) {
	err = asset.GetWalletDataDb().Read(offset, limit, txFilter, newestFirst, asset.RequiredConfirmations(), asset.GetBestBlockHeight(), &transactions)
	return
}

func (asset *DCRAsset) CountTransactions(txFilter int32) (int, error) {
	return asset.GetWalletDataDb().Count(txFilter, asset.RequiredConfirmations(), asset.GetBestBlockHeight(), &wallet.Transaction{})
}

func (asset *DCRAsset) TicketHasVotedOrRevoked(ticketHash string) (bool, error) {
	err := asset.GetWalletDataDb().FindOne("TicketSpentHash", ticketHash, &wallet.Transaction{})
	if err != nil {
		if err == storm.ErrNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (asset *DCRAsset) TicketSpender(ticketHash string) (*wallet.Transaction, error) {
	var spender wallet.Transaction
	err := asset.GetWalletDataDb().FindOne("TicketSpentHash", ticketHash, &spender)
	if err != nil {
		if err == storm.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &spender, nil
}

func (asset *DCRAsset) TransactionOverview() (txOverview *wallet.TransactionOverview, err error) {

	txOverview = &wallet.TransactionOverview{}

	txOverview.Sent, err = asset.CountTransactions(TxFilterSent)
	if err != nil {
		return
	}

	txOverview.Received, err = asset.CountTransactions(TxFilterReceived)
	if err != nil {
		return
	}

	txOverview.Transferred, err = asset.CountTransactions(TxFilterTransferred)
	if err != nil {
		return
	}

	txOverview.Mixed, err = asset.CountTransactions(TxFilterMixed)
	if err != nil {
		return
	}

	txOverview.Staking, err = asset.CountTransactions(TxFilterStaking)
	if err != nil {
		return
	}

	txOverview.Coinbase, err = asset.CountTransactions(TxFilterCoinBase)
	if err != nil {
		return
	}

	txOverview.All = txOverview.Sent + txOverview.Received + txOverview.Transferred + txOverview.Mixed +
		txOverview.Staking + txOverview.Coinbase

	return txOverview, nil
}

func (asset *DCRAsset) TxMatchesFilter(tx *wallet.Transaction, txFilter int32) bool {
	bestBlock := asset.GetBestBlockHeight()

	// tickets with block height less than this are matured.
	maturityBlock := bestBlock - int32(asset.chainParams.TicketMaturity)

	// tickets with block height less than this are expired.
	expiryBlock := bestBlock - int32(asset.chainParams.TicketMaturity+uint16(asset.chainParams.TicketExpiry))

	switch txFilter {
	case TxFilterSent:
		return tx.Type == TxTypeRegular && tx.Direction == TxDirectionSent
	case TxFilterReceived:
		return tx.Type == TxTypeRegular && tx.Direction == TxDirectionReceived
	case TxFilterTransferred:
		return tx.Type == TxTypeRegular && tx.Direction == TxDirectionTransferred
	case TxFilterStaking:
		switch tx.Type {
		case TxTypeTicketPurchase:
			fallthrough
		case TxTypeVote:
			fallthrough
		case TxTypeRevocation:
			return true
		}

		return false
	case TxFilterCoinBase:
		return tx.Type == TxTypeCoinBase
	case TxFilterRegular:
		return tx.Type == TxTypeRegular
	case TxFilterMixed:
		return tx.Type == TxTypeMixed
	case TxFilterVoted:
		return tx.Type == TxTypeVote
	case TxFilterRevoked:
		return tx.Type == TxTypeRevocation
	case walletdata.TxFilterImmature:
		return tx.Type == TxTypeTicketPurchase &&
			(tx.BlockHeight > maturityBlock) // not matured
	case TxFilterLive:
		// ticket is live if we don't have the spender hash and it hasn't expired.
		// we cannot detect missed tickets over spv.
		return tx.Type == TxTypeTicketPurchase &&
			tx.TicketSpender == "" &&
			tx.BlockHeight > 0 &&
			tx.BlockHeight <= maturityBlock &&
			tx.BlockHeight > expiryBlock // not expired
	case TxFilterUnmined:
		return tx.Type == TxTypeTicketPurchase && tx.BlockHeight == -1
	case TxFilterExpired:
		return tx.Type == TxTypeTicketPurchase &&
			tx.TicketSpender == "" &&
			tx.BlockHeight > 0 &&
			tx.BlockHeight <= expiryBlock
	case TxFilterTickets:
		return tx.Type == TxTypeTicketPurchase
	case TxFilterAll:
		return true
	}

	return false
}

func (asset *DCRAsset) TxMatchesFilter2(direction, blockHeight int32, txType, ticketSpender string, txFilter int32) bool {
	tx := wallet.Transaction{
		Type:          txType,
		Direction:     direction,
		BlockHeight:   blockHeight,
		TicketSpender: ticketSpender,
	}
	return asset.TxMatchesFilter(&tx, txFilter)
}

func Confirmations(bestBlock int32, tx wallet.Transaction) int32 {
	if tx.BlockHeight == BlockHeightInvalid {
		return 0
	}

	return (bestBlock - tx.BlockHeight) + 1
}

func TicketStatus(ticketMaturity, ticketExpiry, bestBlock int32, tx wallet.Transaction) string {
	if tx.Type != TxTypeTicketPurchase {
		return ""
	}

	confirmations := Confirmations(bestBlock, tx)
	if confirmations == 0 {
		return TicketStatusUnmined
	} else if confirmations <= ticketMaturity {
		return TicketStatusImmature
	} else if confirmations > (ticketMaturity + ticketExpiry) {
		return TicketStatusExpired
	} else if tx.TicketSpender != "" {
		return TicketStatusVotedOrRevoked
	}

	return TicketStatusLive
}
