package txhelper

const (
	TxDirectionInvalid     int32 = -1
	TxDirectionSent        int32 = 0
	TxDirectionReceived    int32 = 1
	TxDirectionTransferred int32 = 2

	TxTypeRegular        = "Regular"
	TxTypeMixed          = "Mixed"
	TxTypeCoinBase       = "Coinbase"
	TxTypeTicketPurchase = "Ticket"
	TxTypeVote           = "Vote"
	TxTypeRevocation     = "Revocation"
)
