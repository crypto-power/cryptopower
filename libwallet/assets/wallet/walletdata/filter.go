package walletdata

import (
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"gitlab.com/cryptopower/cryptopower/libwallet/txhelper"
	"gitlab.com/cryptopower/cryptopower/libwallet/utils"
)

func (db *DB) prepareTxQuery(txFilter, requiredConfirmations, bestBlock int32) (query storm.Query) {
	// tickets with block height less than this are matured.
	maturityBlock := bestBlock - db.ticketMaturity

	// tickets with block height less than this are expired.
	expiryBlock := bestBlock - (db.ticketMaturity + db.ticketExpiry)

	switch txFilter {
	case utils.TxFilterSent:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeRegular),
			q.Eq(utils.DirectionFilter, txhelper.TxDirectionSent),
		)
	case utils.TxFilterReceived:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeRegular),
			q.Eq(utils.DirectionFilter, txhelper.TxDirectionReceived),
		)
	case utils.TxFilterTransferred:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeRegular),
			q.Eq(utils.DirectionFilter, txhelper.TxDirectionTransferred),
		)
	case utils.TxFilterStaking:
		query = db.walletDataDB.Select(
			q.Or(
				q.Eq(utils.TypeFilter, txhelper.TxTypeTicketPurchase),
				q.Eq(utils.TypeFilter, txhelper.TxTypeVote),
				q.Eq(utils.TypeFilter, txhelper.TxTypeRevocation),
			),
		)
	case utils.TxFilterCoinBase:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeCoinBase),
		)
	case utils.TxFilterRegular:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeRegular),
		)
	case utils.TxFilterMixed:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeMixed),
		)
	case utils.TxFilterVoted:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeVote),
		)
	case utils.TxFilterRevoked:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeRevocation),
		)
	case utils.TxFilterImmature:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeTicketPurchase),
			q.And(
				q.Gt(utils.HeightFilter, maturityBlock),
			),
		)
	case utils.TxFilterLive:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeTicketPurchase),
			q.Eq(utils.TicketSpenderFilter, ""),      // not spent by a vote or revoke
			q.Gt(utils.HeightFilter, 0),              // mined
			q.Lte(utils.HeightFilter, maturityBlock), // must be matured
			q.Gt(utils.HeightFilter, expiryBlock),    // not expired
		)
	case utils.TxFilterUnmined:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeTicketPurchase),
			q.Or(
				q.Eq(utils.HeightFilter, -1),
			),
		)
	case utils.TxFilterExpired:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeTicketPurchase),
			q.Eq(utils.TicketSpenderFilter, ""), // not spent by a vote or revoke
			q.Gt(utils.HeightFilter, 0),         // mined
			q.Lte(utils.HeightFilter, expiryBlock),
		)
	case utils.TxFilterTickets:
		query = db.walletDataDB.Select(
			q.Eq(utils.TypeFilter, txhelper.TxTypeTicketPurchase),
		)
	default:
		query = db.walletDataDB.Select(
			q.True(),
		)
	}

	return
}
