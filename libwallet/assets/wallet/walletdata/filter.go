package walletdata

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/txhelper"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
)

func (db *DB) prepareTxQuery(txFilter, requiredConfirmations, bestBlock int32) (query storm.Query) {
	// tickets with block height less than this are matured.
	maturityBlock := bestBlock - db.ticketMaturity

	// tickets with block height less than this are expired.
	expiryBlock := bestBlock - (db.ticketMaturity + db.ticketExpiry)

	switch txFilter {
	case utils.TxFilterSent:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRegular),
			q.Eq("Direction", txhelper.TxDirectionSent),
		)
	case utils.TxFilterReceived:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRegular),
			q.Eq("Direction", txhelper.TxDirectionReceived),
		)
	case utils.TxFilterTransferred:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRegular),
			q.Eq("Direction", txhelper.TxDirectionTransferred),
		)
	case utils.TxFilterStaking:
		query = db.walletDataDB.Select(
			q.Or(
				q.Eq("Type", txhelper.TxTypeTicketPurchase),
				q.Eq("Type", txhelper.TxTypeVote),
				q.Eq("Type", txhelper.TxTypeRevocation),
			),
		)
	case utils.TxFilterCoinBase:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeCoinBase),
		)
	case utils.TxFilterRegular:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRegular),
		)
	case utils.TxFilterMixed:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeMixed),
		)
	case utils.TxFilterVoted:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeVote),
		)
	case utils.TxFilterRevoked:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeRevocation),
		)
	case utils.TxFilterImmature:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeTicketPurchase),
			q.And(
				q.Gt("BlockHeight", maturityBlock),
			),
		)
	case utils.TxFilterLive:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeTicketPurchase),
			q.Eq("TicketSpender", ""),           // not spent by a vote or revoke
			q.Gt("BlockHeight", 0),              // mined
			q.Lte("BlockHeight", maturityBlock), // must be matured
			q.Gt("BlockHeight", expiryBlock),    // not expired
		)
	case utils.TxFilterUnmined:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeTicketPurchase),
			q.Or(
				q.Eq("BlockHeight", -1),
			),
		)
	case utils.TxFilterExpired:
		query = db.walletDataDB.Select(
			q.Eq("Type", txhelper.TxTypeTicketPurchase),
			q.Eq("TicketSpender", ""), // not spent by a vote or revoke
			q.Gt("BlockHeight", 0),    // mined
			q.Lte("BlockHeight", expiryBlock),
		)
	case utils.TxFilterTickets:
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
