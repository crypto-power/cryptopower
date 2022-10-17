package wallet

import (
	"github.com/asdine/storm"
	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

// WalletConfig defines options for configuring wallet behaviour.
// This is a subset of the config used by dcrwallet.
type WalletConfig struct {
	// General
	GapLimit                uint32         // Allowed unused address gap between used addresses of accounts
	ManualTickets           bool           // Do not discover new tickets through network synchronization
	AllowHighFees           bool           // Do not perform high fee checks
	RelayFee                dcrutil.Amount // Transaction fee per kilobyte
	AccountGapLimit         int            // Allowed gap of unused accounts
	DisableCoinTypeUpgrades bool           // Never upgrade from legacy to SLIP0044 coin type keys

	// CSPP
	MixSplitLimit int // Connection limit to CoinShuffle++ server per change amount
}

// InitParams defines the basic parameters required to instantiate any
// wallet interface.
type InitParams struct {
	RootDir  string
	NetType  utils.NetworkType
	DB       *storm.DB
	DbDriver string
}

type WalletPassInfo struct {
	Name            string
	PrivatePass     string
	PrivatePassType int32
}

type BlockInfo struct {
	Height    int32
	Timestamp int64
}

type Amount struct {
	AtomValue int64
	DcrValue  float64
}

type TxFeeAndSize struct {
	Fee                 *Amount
	Change              *Amount
	EstimatedSignedSize int
}

type UnsignedTransaction struct {
	UnsignedTransaction       []byte
	EstimatedSignedSize       int
	ChangeIndex               int
	TotalOutputAmount         int64
	TotalPreviousOutputAmount int64
}

type Balance struct {
	Total                   int64
	Spendable               int64
	ImmatureReward          int64
	ImmatureStakeGeneration int64
	LockedByTickets         int64
	VotingAuthority         int64
	UnConfirmed             int64
}

type Account struct {
	WalletID         int
	Number           int32
	Name             string
	Balance          *Balance
	TotalBalance     int64
	ExternalKeyCount int32
	InternalKeyCount int32
	ImportedKeyCount int32
}

type Accounts struct {
	Count              int
	Acc                []*Account
	CurrentBlockHash   []byte
	CurrentBlockHeight int32
}

type PeerInfo struct {
	ID             int32  `json:"id"`
	Addr           string `json:"addr"`
	AddrLocal      string `json:"addr_local"`
	Services       string `json:"services"`
	Version        uint32 `json:"version"`
	SubVer         string `json:"sub_ver"`
	StartingHeight int64  `json:"starting_height"`
	BanScore       int32  `json:"ban_score"`
}

/** begin sync-related types */

type SyncProgressListener interface {
	OnSyncStarted(wasRestarted bool)
	OnPeerConnectedOrDisconnected(numberOfConnectedPeers int32)
	OnCFiltersFetchProgress(cfiltersFetchProgress *CFiltersFetchProgressReport)
	OnHeadersFetchProgress(headersFetchProgress *HeadersFetchProgressReport)
	OnAddressDiscoveryProgress(addressDiscoveryProgress *AddressDiscoveryProgressReport)
	OnHeadersRescanProgress(headersRescanProgress *HeadersRescanProgressReport)
	OnSyncCompleted()
	OnSyncCanceled(willRestart bool)
	OnSyncEndedWithError(err error)
	Debug(debugInfo *DebugInfo)
}

type GeneralSyncProgress struct {
	TotalSyncProgress         int32 `json:"totalSyncProgress"`
	TotalTimeRemainingSeconds int64 `json:"totalTimeRemainingSeconds"`
}

type CFiltersFetchProgressReport struct {
	*GeneralSyncProgress
	BeginFetchCFiltersTimeStamp int64
	StartCFiltersHeight         int32
	CfiltersFetchTimeSpent      int64
	TotalFetchedCFiltersCount   int32
	TotalCFiltersToFetch        int32 `json:"totalCFiltersToFetch"`
	CurrentCFilterHeight        int32 `json:"currentCFilterHeight"`
	CFiltersFetchProgress       int32 `json:"headersFetchProgress"`
}

type HeadersFetchProgressReport struct {
	*GeneralSyncProgress
	HeadersFetchTimeSpent    int64
	BeginFetchTimeStamp      int64
	StartHeaderHeight        int32
	TotalFetchedHeadersCount int32
	TotalHeadersToFetch      int32 `json:"totalHeadersToFetch"`
	CurrentHeaderHeight      int32 `json:"currentHeaderHeight"`
	CurrentHeaderTimestamp   int64 `json:"currentHeaderTimestamp"`
	HeadersFetchProgress     int32 `json:"headersFetchProgress"`
}

type AddressDiscoveryProgressReport struct {
	*GeneralSyncProgress
	AddressDiscoveryStartTime int64
	TotalDiscoveryTimeSpent   int64
	AddressDiscoveryProgress  int32 `json:"addressDiscoveryProgress"`
	WalletID                  int   `json:"walletID"`
}

type HeadersRescanProgressReport struct {
	*GeneralSyncProgress
	TotalHeadersToScan  int32 `json:"totalHeadersToScan"`
	CurrentRescanHeight int32 `json:"currentRescanHeight"`
	RescanProgress      int32 `json:"rescanProgress"`
	RescanTimeRemaining int64 `json:"rescanTimeRemaining"`
	WalletID            int   `json:"walletID"`
}

type DebugInfo struct {
	TotalTimeElapsed          int64
	TotalTimeRemaining        int64
	CurrentStageTimeElapsed   int64
	CurrentStageTimeRemaining int64
}

/** end sync-related types */

type TxAndBlockNotificationListener interface {
	OnTransaction(transaction string)
	OnBlockAttached(walletID int, blockHeight int32)
	OnTransactionConfirmed(walletID int, hash string, blockHeight int32)
}

type BlocksRescanProgressListener interface {
	OnBlocksRescanStarted(walletID int)
	OnBlocksRescanProgress(*HeadersRescanProgressReport)
	OnBlocksRescanEnded(walletID int, err error)
}

// Transaction is used with storm for tx indexing operations.
// For faster queries, the `Hash`, `Type` and `Direction` fields are indexed.
type Transaction struct {
	WalletID      int    `json:"walletID"`
	Hash          string `storm:"id,unique" json:"hash"`
	Type          string `storm:"index" json:"type"`
	Hex           string `json:"hex"`
	Timestamp     int64  `storm:"index" json:"timestamp"`
	BlockHeight   int32  `storm:"index" json:"block_height"`
	TicketSpender string `storm:"index" json:"ticket_spender"`

	MixDenomination int64 `json:"mix_denom"`
	MixCount        int32 `json:"mix_count"`

	Version  int32 `json:"version"`
	LockTime int32 `json:"lock_time"`
	Expiry   int32 `json:"expiry"`
	Fee      int64 `json:"fee"`
	FeeRate  int64 `json:"fee_rate"`
	Size     int   `json:"size"`

	Direction int32       `storm:"index" json:"direction"`
	Amount    int64       `json:"amount"`
	Inputs    []*TxInput  `json:"inputs"`
	Outputs   []*TxOutput `json:"outputs"`

	// Vote Info
	VoteVersion        int32  `json:"vote_version"`
	LastBlockValid     bool   `json:"last_block_valid"`
	VoteBits           string `json:"vote_bits"`
	VoteReward         int64  `json:"vote_reward"`
	TicketSpentHash    string `storm:"unique" json:"ticket_spent_hash"`
	DaysToVoteOrRevoke int32  `json:"days_to_vote_revoke"`
}

type TxInput struct {
	PreviousTransactionHash  string `json:"previous_transaction_hash"`
	PreviousTransactionIndex int32  `json:"previous_transaction_index"`
	PreviousOutpoint         string `json:"previous_outpoint"`
	Amount                   int64  `json:"amount"`
	AccountNumber            int32  `json:"account_number"`
}

type TxOutput struct {
	Index         int32  `json:"index"`
	Amount        int64  `json:"amount"`
	Version       int32  `json:"version"`
	ScriptType    string `json:"script_type"`
	Address       string `json:"address"`
	Internal      bool   `json:"internal"`
	AccountNumber int32  `json:"account_number"`
}

// TxInfoFromWallet contains tx data that relates to the querying wallet.
// This info is used with `DecodeTransaction` to compose the entire details of a transaction.
type TxInfoFromWallet struct {
	WalletID    int
	Hex         string
	Timestamp   int64
	BlockHeight int32
	Inputs      []*WalletInput
	Outputs     []*WalletOutput
}

type WalletInput struct {
	Index    int32 `json:"index"`
	AmountIn int64 `json:"amount_in"`
	*WalletAccount
}

type WalletOutput struct {
	Index     int32  `json:"index"`
	AmountOut int64  `json:"amount_out"`
	Internal  bool   `json:"internal"`
	Address   string `json:"address"`
	*WalletAccount
}

type WalletAccount struct {
	AccountNumber int32  `json:"account_number"`
	AccountName   string `json:"account_name"`
}

type TransactionDestination struct {
	Address    string
	AtomAmount int64
	SendMax    bool
}

type TransactionOverview struct {
	All         int
	Sent        int
	Received    int
	Transferred int
	Mixed       int
	Staking     int
	Coinbase    int
}

/** end tx-related types */
