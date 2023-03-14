package wallet

import (
	"time"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/asdine/storm"
	btchdkeychain "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/decred/dcrd/dcrutil/v4"
)

type AssetAmount interface {
	// ToCoin returns an asset formatted amount in float64.
	ToCoin() float64
	// String returns an asset formatted amount in string.
	String() string
	// MulF64 multiplies an Amount by a floating point value.
	MulF64(f float64) AssetAmount
	// ToInt() returns the complete int64 value without formatting.
	ToInt() int64
}

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
	LogDir   string
}

// WalletAuthInfo defines the complete information required to either create a
// new wallet or restore an old wallet.
type WalletAuthInfo struct {
	Name            string
	PrivatePass     string
	PrivatePassType int32
}

type BlockInfo struct {
	Height    int32
	Timestamp int64
}

type Amount struct {
	// UnitValue holds the base monetary unit value for a cryptocurrency.
	// The field is currently used for both BTC and DCR.
	// For Decred it holds the number of Atoms per DCR.
	// For Bitcoin it holds the number of satoshis per BTC.
	UnitValue int64
	// CoinValue holds the monetary amount counted in a cryptocurrency base
	// units, converted to a floating point value representing the amount
	// of said cryptocurrency.
	CoinValue float64
}

type TxFeeAndSize struct {
	Fee                 *Amount
	Change              *Amount
	FeeRate             int64 // calculated in Sat/kvB
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
	// fields common to both DCR and BTC
	Total          AssetAmount
	Spendable      AssetAmount
	ImmatureReward AssetAmount

	// DCR only fields
	ImmatureStakeGeneration AssetAmount
	LockedByTickets         AssetAmount
	VotingAuthority         AssetAmount
	UnConfirmed             AssetAmount
}

type Account struct {
	// DCR fields
	ExternalKeyCount int32
	InternalKeyCount int32
	ImportedKeyCount int32

	// BTC fields
	AccountProperties

	// Has some fields common to both BTC and DCR
	WalletID int
	Balance  *Balance
	Number   int32
	Name     string
}

type Accounts struct {
	Accounts           []*Account
	CurrentBlockHash   []byte
	CurrentBlockHeight int32
}

// AccountProperties contains properties associated with each account, such as
// the account name, number, and the nubmer of derived and imported keys.
type AccountProperties struct {
	// AccountNumber is the internal number used to reference the account.
	AccountNumber uint32

	// AccountName is the user-identifying name of the account.
	AccountName string

	// ExternalKeyCount is the number of internal keys that have been
	// derived for the account.
	ExternalKeyCount uint32

	// InternalKeyCount is the number of internal keys that have been
	// derived for the account.
	InternalKeyCount uint32

	// ImportedKeyCount is the number of imported keys found within the
	// account.
	ImportedKeyCount uint32

	// AccountPubKey is the account's public key that can be used to
	// derive any address relevant to said account.
	//
	// NOTE: This may be nil for imported accounts.
	AccountPubKey *btchdkeychain.ExtendedKey

	// MasterKeyFingerprint represents the fingerprint of the root key
	// corresponding to the master public key (also known as the key with
	// derivation path m/). This may be required by some hardware wallets
	// for proper identification and signing.
	MasterKeyFingerprint uint32

	// KeyScope is the key scope the account belongs to.
	KeyScope KeyScope

	// IsWatchOnly indicates whether the is set up as watch-only, i.e., it
	// doesn't contain any private key information.
	IsWatchOnly bool

	// AddrSchema, if non-nil, specifies an address schema override for
	// address generation only applicable to the account.
	AddrSchema *ScopeAddrSchema
}

// KeyScope represents a restricted key scope from the primary root key within
// the HD chain. From the root manager (m/) we can create a nearly arbitrary
// number of ScopedKeyManagers of key derivation path: m/purpose'/cointype'.
// These scoped managers can then me managed indecently, as they house the
// encrypted cointype key and can derive any child keys from there on.
type KeyScope struct {
	// Purpose is the purpose of this key scope. This is the first child of
	// the master HD key.
	Purpose uint32

	// Coin is a value that represents the particular coin which is the
	// child of the purpose key. With this key, any accounts, or other
	// children can be derived at all.
	Coin uint32
}

// AddressType represents the various address types waddrmgr is currently able
// to generate, and maintain.
//
// NOTE: These MUST be stable as they're used for scope address schema
// recognition within the database.
type AddressType uint8

// ScopeAddrSchema is the address schema of a particular KeyScope. This will be
// persisted within the database, and will be consulted when deriving any keys
// for a particular scope to know how to encode the public keys as addresses.
type ScopeAddrSchema struct {
	// ExternalAddrType is the address type for all keys within branch 0.
	ExternalAddrType AddressType

	// InternalAddrType is the address type for all keys within branch 1
	// (change addresses).
	InternalAddrType AddressType
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
	OnSyncStarted()
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
	HeadersFetchTimeSpent int64
	BeginFetchTimeStamp   time.Time
	StartHeaderHeight     *int32
	TotalHeadersToFetch   int32 `json:"totalHeadersToFetch"`
	HeadersFetchProgress  int32 `json:"headersFetchProgress"`
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

/** begin tx-related types */

// AsyncTxAndBlockNotificationListener is a TxAndBlockNotificationListener that
// triggers notifcation callbacks asynchronously.
type AsyncTxAndBlockNotificationListener struct {
	TxAndBlockNotificationListener
}

// OnTransaction satisfies the TxAndBlockNotificationListener interface and
// starts a goroutine to actually handle the notification using the embedded
// listener.
func (asyncTxBlockListener *AsyncTxAndBlockNotificationListener) OnTransaction(transaction string) {
	go asyncTxBlockListener.TxAndBlockNotificationListener.OnTransaction(transaction)
}

// OnBlockAttached satisfies the TxAndBlockNotificationListener interface and
// starts a goroutine to actually handle the notification using the embedded
// listener.
func (asyncTxBlockListener *AsyncTxAndBlockNotificationListener) OnBlockAttached(walletID int, blockHeight int32) {
	go asyncTxBlockListener.TxAndBlockNotificationListener.OnBlockAttached(walletID, blockHeight)
}

// OnTransactionConfirmed satisfies the TxAndBlockNotificationListener interface
// and starts a goroutine to actually handle the notification using the embedded
// listener.
func (asyncTxBlockListener *AsyncTxAndBlockNotificationListener) OnTransactionConfirmed(walletID int, hash string, blockHeight int32) {
	go asyncTxBlockListener.TxAndBlockNotificationListener.OnTransactionConfirmed(walletID, hash, blockHeight)
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
	Type          string `storm:"index" json:"type,omitempty"`
	Hex           string `json:"hex"`
	Timestamp     int64  `storm:"index" json:"timestamp"`
	BlockHeight   int32  `storm:"index" json:"block_height"`
	TicketSpender string `storm:"index" json:"ticket_spender,omitempty"` // (DCR Field)

	MixDenomination int64 `json:"mix_denom,omitempty"` // (DCR Field)
	MixCount        int32 `json:"mix_count,omitempty"` // (DCR Field)

	Version  int32  `json:"version"`
	LockTime int32  `json:"lock_time"`
	Expiry   int32  `json:"expiry,omitempty"` // (DCR Field)
	Fee      int64  `json:"fee"`
	FeeRate  int64  `json:"fee_rate"`
	Size     int    `json:"size"`
	Label    string `json:"label"`

	Direction int32       `storm:"index" json:"direction"`
	Amount    int64       `json:"amount"`
	Inputs    []*TxInput  `json:"inputs"`
	Outputs   []*TxOutput `json:"outputs"`

	// Vote Info (DCR fields)
	VoteVersion        int32  `json:"vote_version,omitempty"`
	LastBlockValid     bool   `json:"last_block_valid,omitempty"`
	VoteBits           string `json:"vote_bits,omitempty"`
	VoteReward         int64  `json:"vote_reward,omitempty"`
	TicketSpentHash    string `storm:"unique" json:"ticket_spent_hash,omitempty"`
	DaysToVoteOrRevoke int32  `json:"days_to_vote_revoke,omitempty"`
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
	Version       int32  `json:"version,omitempty"` // (DCR Field)
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
	// Shared fields.
	Address    string
	SendMax    bool
	UnitAmount int64
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

// ExchangeConfig defines configuration parameters for creating
// an exchange order.
type ExchangeConfig struct {
	SourceAsset      utils.AssetType
	DestinationAsset utils.AssetType

	SourceWalletID      int32
	DestinationWalletID int32

	SourceAccountNumber      int32
	DestinationAccountNumber int32
}
