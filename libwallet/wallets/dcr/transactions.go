package dcr

import (
	"encoding/json"

	"github.com/asdine/storm"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"gitlab.com/raedah/cryptopower/libwallet/txhelper"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/dcr/walletdata"
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

func (wallet *Wallet) PublishUnminedTransactions() error {
	n, err := wallet.Internal().NetworkBackend()
	if err != nil {
		log.Error(err)
		return err
	}

	return wallet.Internal().PublishUnminedTransactions(wallet.ShutdownContext(), n)
}

func (wallet *Wallet) GetTransaction(txHash string) (string, error) {
	transaction, err := wallet.GetTransactionRaw(txHash)
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

func (wallet *Wallet) GetTransactionRaw(txHash string) (*Transaction, error) {
	hash, err := chainhash.NewHashFromStr(txHash)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	txSummary, _, blockHash, err := wallet.Internal().TransactionSummary(wallet.ShutdownContext(), hash)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return wallet.decodeTransactionWithTxSummary(txSummary, blockHash)
}

func (wallet *Wallet) GetTransactions(offset, limit, txFilter int32, newestFirst bool) (string, error) {
	transactions, err := wallet.GetTransactionsRaw(offset, limit, txFilter, newestFirst)
	if err != nil {
		return "", err
	}

	jsonEncodedTransactions, err := json.Marshal(&transactions)
	if err != nil {
		return "", err
	}

	return string(jsonEncodedTransactions), nil
}

func (wallet *Wallet) GetTransactionsRaw(offset, limit, txFilter int32, newestFirst bool) (transactions []Transaction, err error) {
	err = wallet.WalletDataDB.Read(offset, limit, txFilter, newestFirst, wallet.RequiredConfirmations(), wallet.GetBestBlockHeight(), &transactions)
	return
}

func (wallet *Wallet) CountTransactions(txFilter int32) (int, error) {
	return wallet.WalletDataDB.Count(txFilter, wallet.RequiredConfirmations(), wallet.GetBestBlockHeight(), &Transaction{})
}

func (wallet *Wallet) TicketHasVotedOrRevoked(ticketHash string) (bool, error) {
	err := wallet.WalletDataDB.FindOne("TicketSpentHash", ticketHash, &Transaction{})
	if err != nil {
		if err == storm.ErrNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (wallet *Wallet) TicketSpender(ticketHash string) (*Transaction, error) {
	var spender Transaction
	err := wallet.WalletDataDB.FindOne("TicketSpentHash", ticketHash, &spender)
	if err != nil {
		if err == storm.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &spender, nil
}

func (wallet *Wallet) TransactionOverview() (txOverview *TransactionOverview, err error) {

	txOverview = &TransactionOverview{}

	txOverview.Sent, err = wallet.CountTransactions(TxFilterSent)
	if err != nil {
		return
	}

	txOverview.Received, err = wallet.CountTransactions(TxFilterReceived)
	if err != nil {
		return
	}

	txOverview.Transferred, err = wallet.CountTransactions(TxFilterTransferred)
	if err != nil {
		return
	}

	txOverview.Mixed, err = wallet.CountTransactions(TxFilterMixed)
	if err != nil {
		return
	}

	txOverview.Staking, err = wallet.CountTransactions(TxFilterStaking)
	if err != nil {
		return
	}

	txOverview.Coinbase, err = wallet.CountTransactions(TxFilterCoinBase)
	if err != nil {
		return
	}

	txOverview.All = txOverview.Sent + txOverview.Received + txOverview.Transferred + txOverview.Mixed +
		txOverview.Staking + txOverview.Coinbase

	return txOverview, nil
}

func (wallet *Wallet) TxMatchesFilter(tx *Transaction, txFilter int32) bool {
	bestBlock := wallet.GetBestBlockHeight()

	// tickets with block height less than this are matured.
	maturityBlock := bestBlock - int32(wallet.chainParams.TicketMaturity)

	// tickets with block height less than this are expired.
	expiryBlock := bestBlock - int32(wallet.chainParams.TicketMaturity+uint16(wallet.chainParams.TicketExpiry))

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

func (wallet *Wallet) TxMatchesFilter2(direction, blockHeight int32, txType, ticketSpender string, txFilter int32) bool {
	tx := Transaction{
		Type:          txType,
		Direction:     direction,
		BlockHeight:   blockHeight,
		TicketSpender: ticketSpender,
	}
	return wallet.TxMatchesFilter(&tx, txFilter)
}

func (tx Transaction) Confirmations(bestBlock int32) int32 {
	if tx.BlockHeight == BlockHeightInvalid {
		return 0
	}

	return (bestBlock - tx.BlockHeight) + 1
}

func (tx Transaction) TicketStatus(ticketMaturity, ticketExpiry, bestBlock int32) string {
	if tx.Type != TxTypeTicketPurchase {
		return ""
	}

	confirmations := tx.Confirmations(bestBlock)
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
