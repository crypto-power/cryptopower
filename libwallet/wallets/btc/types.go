package btc

import (
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
)

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
	AccountPubKey *hdkeychain.ExtendedKey

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
	// AddrSchema *ScopeAddrSchema
}

// AccountResult is a single account result for the AccountsResult type.
type AccountResult struct {
	AccountProperties
	TotalBalance btcutil.Amount
}

// AccountsResult is the result of the wallet's Accounts method.  See that
// method for more details.
type AccountsResult struct {
	Accounts           []*AccountResult
	CurrentBlockHash   *chainhash.Hash
	CurrentBlockHeight int32
}

// AddressType represents the various address types waddrmgr is currently able
// to generate, and maintain.
//
// NOTE: These MUST be stable as they're used for scope address schema
// recognition within the database.
type AddressType uint8

// Balances records total, spendable (by policy), and immature coinbase
// reward balance amounts.
type Balances struct {
	Total          btcutil.Amount
	Spendable      btcutil.Amount
	ImmatureReward btcutil.Amount
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

type ListUnspentResult struct {
	TxID          string  `json:"txid"`
	Vout          uint32  `json:"vout"`
	Address       string  `json:"address"`
	Account       string  `json:"account"`
	ScriptPubKey  string  `json:"scriptPubKey"`
	RedeemScript  string  `json:"redeemScript,omitempty"`
	Amount        float64 `json:"amount"`
	Confirmations int64   `json:"confirmations"`
	Spendable     bool    `json:"spendable"`
}

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
