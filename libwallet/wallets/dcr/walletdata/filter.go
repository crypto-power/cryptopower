package walletdata

import (
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"gitlab.com/raedah/cryptopower/libwallet/txhelper"
)

const (
	TxFilterAll         int32 = 0
	TxFilterSent        int32 = 1
	TxFilterReceived    int32 = 2
	TxFilterTransferred int32 = 3
	TxFilterStaking     int32 = 4
	TxFilterCoinBase    int32 = 5
	TxFilterRegular     int32 = 6
	TxFilterMixed       int32 = 7
	TxFilterVoted       int32 = 8
	TxFilterRevoked     int32 = 9
	TxFilterImmature    int32 = 10
	TxFilterLive        int32 = 11
	TxFilterUnmined     int32 = 12
	TxFilterExpired     int32 = 13
	TxFilterTickets     int32 = 14
)

func (db *DB) prepareTxQuery(txFilter, requiredConfirmations, bestBlock int32) (query storm.Query) {
	// tickets with block height less than this are matured.
	maturityBlock := bestBlock - int32(db.chainParams.TicketMaturity)

	// tickets with block height less than this are expired.
	expiryBlock := bestBlock - int32(db.chainParams.TicketMaturity+uint16(db.chainParams.TicketExpiry))

	switch txFilter {
	case TxFilterSent:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRegular),
			q.Eq("Direction", txhelper.TxDirectionSent),
		)
	case TxFilterReceived:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRegular),
			q.Eq("Direction", txhelper.TxDirectionReceived),
		)
	case TxFilterTransferred:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRegular),
			q.Eq("Direction", txhelper.TxDirectionTransferred),
		)
	case TxFilterStaking:
		query = db.walletDataDB.Select(
			q.Or(
				q.Eq("Type", txhelper.TxTypeTicketPurchase),
				q.Eq("Type", txhelper.TxTypeVote),
				q.Eq("Type", txhelper.TxTypeRevocation),
			),
		)
	case TxFilterCoinBase:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeCoinBase),
		)
	case TxFilterRegular:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRegular),
		)
	case TxFilterMixed:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeMixed),
		)
	case TxFilterVoted:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeVote),
		)
	case TxFilterRevoked:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRevocation),
		)
	case TxFilterImmature:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeTicketPurchase),
			q.And(
				q.Gt("BlockHeight", maturityBlock),
			),
		)
	case TxFilterLive:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeTicketPurchase),
			q.Eq("TicketSpender", ""),           // not spent by a vote or revoke
			q.Gt("BlockHeight", 0),              // mined
			q.Lte("BlockHeight", maturityBlock), // must be matured
			q.Gt("BlockHeight", expiryBlock),    // not expired
		)
	case TxFilterUnmined:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeTicketPurchase),
			q.Or(
				q.Eq("BlockHeight", -1),
			),
		)
	case TxFilterExpired:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeTicketPurchase),
			q.Eq("TicketSpender", ""), // not spent by a vote or revoke
			q.Gt("BlockHeight", 0),    // mined
			q.Lte("BlockHeight", expiryBlock),
		)
	case TxFilterTickets:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeTicketPurchase),
		)
	default:
		query = db.walletDataDB.Select(
			q.True(),
		)
	}

	return
}
