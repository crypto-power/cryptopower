package dcr

import (
	"fmt"

	"decred.org/dcrwallet/v4/wallet/udb"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/internal/vsp"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
)

// Amount implements the Asset amount interface for the DCR asset
type Amount dcrutil.Amount

// ToCoin returns the float64 version of the DCR formatted asset amount.
func (a Amount) ToCoin() float64 {
	return dcrutil.Amount(a).ToCoin()
}

// String returns the string version of the DCR formatted asset amount.
func (a Amount) String() string {
	return dcrutil.Amount(a).String()
}

// MulF64 multiplys the Amount with the provided float64 value.
func (a Amount) MulF64(f float64) sharedW.AssetAmount {
	return Amount(dcrutil.Amount(a).MulF64(f))
}

// ToInt return the original unformatted amount DCRs
func (a Amount) ToInt() int64 {
	return int64(dcrutil.Amount(a))
}

const (
	AddressGapLimit       uint32 = 20
	ImportedAccountNumber        = udb.ImportedAddrAccount
	DefaultAccountNum            = udb.DefaultAccountNum
)

type AccountsIterator struct {
	currentIndex int
	accounts     []*sharedW.Account
}

type WalletsIterator struct {
	CurrentIndex int
	Wallets      []*Asset
}

type CSPPConfig struct {
	// Mixing option activates the new version of the coins mixer which is a
	// replacement of the old client-server mechanism. Now peer to peer
	// mechanism is in place. Ref: https://github.com/decred/dcrwallet/pull/2351
	Mixing             bool
	MixedAccount       uint32
	MixedAccountBranch uint32
	TicketSplitAccount uint32
	ChangeAccount      uint32
}

type AccountMixerNotificationListener struct {
	OnAccountMixerStarted func(walletID int)
	OnAccountMixerEnded   func(walletID int)
}

/** begin ticket-related types */

type TicketPriceResponse struct {
	TicketPrice int64
	Height      int32
}

type StakingOverview struct {
	All      int
	Unmined  int
	Immature int
	Live     int
	Voted    int
	Revoked  int
	Expired  int
}

// TicketBuyerConfig defines configuration parameters for running
// an automated ticket buyer.
type TicketBuyerConfig struct {
	VspHost           string
	PurchaseAccount   int32
	BalanceToMaintain int64

	VspClient *vsp.Client
}

// VSPFeeStatus represents the current fee status of a ticket.
type VSPFeeStatus uint8

const (
	// VSPFeeProcessStarted represents the state which process has being
	// called but fee still not paid.
	VSPFeeProcessStarted VSPFeeStatus = iota
	// VSPFeeProcessPaid represents the state where the process has being
	// paid, but not published.
	VSPFeeProcessPaid
	VSPFeeProcessErrored
	// VSPFeeProcessConfirmed represents the state where the fee has been
	// confirmed by the VSP.
	VSPFeeProcessConfirmed
)

// String returns a human-readable interpretation of the vsp fee status.
func (status VSPFeeStatus) String() string {
	switch udb.FeeStatus(status) {
	case udb.VSPFeeProcessStarted:
		return "fee process started"
	case udb.VSPFeeProcessPaid:
		return "fee paid"
	case udb.VSPFeeProcessErrored:
		return "fee payment errored"
	case udb.VSPFeeProcessConfirmed:
		return "fee confirmed by vsp"
	default:
		return fmt.Sprintf("invalid fee status %d", status)
	}
}

// VSPTicketInfo is information about a ticket that is assigned to a VSP.
type VSPTicketInfo struct {
	VSP         string
	FeeTxHash   string
	FeeTxStatus VSPFeeStatus
	// Client defines the vsp client needed to process more tickets requests.
	Client *vsp.Client
	// ConfirmedByVSP is nil if the ticket status could not be obtained
	// from the VSP, false if the VSP hasn't confirmed the fee and true
	// if the VSP has fully registered the ticket.
	ConfirmedByVSP bool
	// VoteChoices is only set if the ticket status was obtained from the
	// VSP.
	VoteChoices map[string]string
}

/** end ticket-related types */

/** end politea proposal types */

/** begin vspd-related types */
type VspInfoResponse struct {
	APIVersions   []int64 `json:"apiversions"`
	Timestamp     int64   `json:"timestamp"`
	PubKey        []byte  `json:"pubkey"`
	FeePercentage float64 `json:"feepercentage"`
	VspClosed     bool    `json:"vspclosed"`
	Network       string  `json:"network"`
	VspdVersion   string  `json:"vspdversion"`
	Voting        int64   `json:"voting"`
	Voted         int64   `json:"voted"`
	Revoked       int64   `json:"revoked"`
}

type VSP struct {
	Host string
	*VspInfoResponse
}

/** end vspd-related types */

/** begin agenda types */

// Agenda contains information about a consensus deployment
type Agenda struct {
	AgendaID         string            `json:"agenda_id"`
	Description      string            `json:"description"`
	Mask             uint32            `json:"mask"`
	Choices          []chaincfg.Choice `json:"choices"`
	VotingPreference string            `json:"voting_preference"`
	StartTime        int64             `json:"start_time"`
	ExpireTime       int64             `json:"expire_time"`
	Status           string            `json:"status"`
}

// DcrdataAgenda models agenda information for the active network from the
// dcrdata api https://dcrdata.decred.org/api/agendas for mainnet or
// https://testnet.decred.org/api/agendas for testnet.
type DcrdataAgenda struct {
	Name          string `json:"name"`
	Description   string `json:"-"`
	Status        string `json:"status"`
	VotingStarted int64  `json:"-"`
	VotingDone    int64  `json:"-"`
	Activated     int64  `json:"-"`
	HardForked    int64  `json:"-"`
	StartTime     string `json:"-"`
	ExpireTime    string `json:"-"`
	VoteVersion   uint32 `json:"-"`
	Mask          uint16 `json:"-"`
}

/** end agenda types */

// TreasuryKeyPolicy records the voting policy for treasury spend transactions
// by a particular key, and possibly for a particular ticket being voted on by a
// VSP.
type TreasuryKeyPolicy struct {
	PiKey      string `json:"pi_key"`
	TicketHash string `json:"ticket_hash"` // nil unless for per-ticket VSP policies
	Policy     string `json:"policy"`
}
