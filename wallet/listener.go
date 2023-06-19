package wallet

import sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"

type RescanNotificationType int

const (

	// RescanStarted indicates a block rescan start signal
	RescanStarted RescanNotificationType = iota

	// RescanProgress indicates a block rescan progress signal
	RescanProgress

	// RescanEnded indicates a block rescan end signal
	RescanEnded
)

// SyncNotificationType represents the spv sync stage at which the assetsManager is currently
type SyncNotificationType int

const (
	// SyncStarted signifies that spv sync has started
	SyncStarted SyncNotificationType = iota

	// SyncCanceled is a pseudo stage that represents a canceled sync
	SyncCanceled

	// SyncCompleted signifies that spv sync has been completed
	SyncCompleted

	// CfiltersFetchProgress indicates a cfilters fetch signal
	CfiltersFetchProgress

	// HeadersFetchProgress indicates a headers fetch signal
	HeadersFetchProgress

	// AddressDiscoveryProgress indicates an address discovery signal
	AddressDiscoveryProgress

	// HeadersRescanProgress indicates an address rescan signal
	HeadersRescanProgress

	// PeersConnected indicates a peer connected signal
	PeersConnected

	// BlockAttached indicates a block attached signal
	BlockAttached

	// BlockConfirmed indicates a block update signal
	BlockConfirmed

	// AccountMixerStarted indicates on account mixer started
	AccountMixerStarted

	// AccountMixerEnded indicates on account mixer ended
	AccountMixerEnded

	// ProposalVoteFinished indicates that proposal voting is finished
	ProposalVoteFinished

	// ProposalVoteStarted indicates that proposal voting has started
	ProposalVoteStarted

	// ProposalSynced indicates that proposal has finished syncing
	ProposalSynced

	// ProposalAdded indicates that a new proposal was added
	ProposalAdded

	// OrderSynced indicates that order has finished syncing
	OrderSynced
)

const (
	// FetchHeadersStep is the first step when a wallet is syncing.
	FetchHeadersSteps = iota + 1

	// AddressDiscoveryStep is the third step when a wallet is syncing.
	AddressDiscoveryStep

	// RescanHeadersStep is the second step when a wallet is syncing.
	RescanHeadersStep
)

// TotalSyncSteps is the total number of steps to complete a sync process
const TotalSyncSteps = 3

type (
	// SyncStatusUpdate represents information about the status of the assetsManager spv sync
	SyncStatusUpdate struct {
		Stage          SyncNotificationType
		ProgressReport interface{}
		ConnectedPeers int32
		BlockInfo      NewBlock
		ConfirmedTxn   TxConfirmed
		AcctMixerInfo  AccountMixer
		Proposal       Proposal
		Order          Order
	}

	RescanUpdate struct {
		Stage          RescanNotificationType
		WalletID       int
		ProgressReport *sharedW.HeadersRescanProgressReport
	}
)
