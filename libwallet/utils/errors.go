package utils

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	"github.com/asdine/storm"
)

const (
	// Error Codes
	ErrInsufficientBalance          = "insufficient_balance"
	ErrInvalid                      = "invalid"
	ErrWalletLocked                 = "wallet_locked"
	ErrWalletDatabaseInUse          = "wallet_db_in_use"
	ErrWalletNotLoaded              = "wallet_not_loaded"
	ErrWalletNotFound               = "wallet_not_found"
	ErrWalletNameExist              = "wallet_name_exists"
	ErrReservedWalletName           = "wallet_name_reserved"
	ErrWalletIsRestored             = "wallet_is_restored"
	ErrWalletIsWatchOnly            = "watch_only_wallet"
	ErrUnusableSeed                 = "unusable_seed"
	ErrPassphraseRequired           = "passphrase_required"
	ErrInvalidPassphrase            = "invalid_passphrase"
	ErrNotConnected                 = "not_connected"
	ErrExist                        = "exists"
	ErrNotExist                     = "not_exists"
	ErrEmptySeed                    = "empty_seed"
	ErrInvalidAddress               = "invalid_address"
	ErrInvalidAuth                  = "invalid_auth"
	ErrUnavailable                  = "unavailable"
	ErrContextCanceled              = "context_canceled"
	ErrFailedPrecondition           = "failed_precondition"
	ErrSyncAlreadyInProgress        = "sync_already_in_progress"
	ErrNoPeers                      = "no_peers"
	ErrInvalidPeers                 = "invalid_peers"
	ErrListenerAlreadyExist         = "listener_already_exist"
	ErrLoggerAlreadyRegistered      = "logger_already_registered"
	ErrLogRotatorAlreadyInitialized = "log_rotator_already_initialized"
	ErrAddressDiscoveryNotDone      = "address_discovery_not_done"
	ErrChangingPassphrase           = "err_changing_passphrase"
	ErrSavingWallet                 = "err_saving_wallet"
	ErrIndexOutOfRange              = "err_index_out_of_range"
	ErrNoMixableOutput              = "err_no_mixable_output"
	ErrInvalidVoteBit               = "err_invalid_vote_bit"
	ErrNotSynced                    = "err_not_synced"
)

var (
	ErrInvalidNet        = errors.New("invalid network type found")
	ErrAssetUnknown      = errors.New("unknown asset found")
	ErrBTCNotInitialized = errors.New("btc asset not initialized")
)

// todo, should update this method to translate more error kinds.
func TranslateError(err error) error {
	if err, ok := err.(*errors.Error); ok {
		switch err.Kind {
		case errors.InsufficientBalance:
			return errors.New(ErrInsufficientBalance)
		case errors.NotExist, storm.ErrNotFound:
			return errors.New(ErrNotExist)
		case errors.Passphrase:
			return errors.New(ErrInvalidPassphrase)
		case errors.NoPeers:
			return errors.New(ErrNoPeers)
		}
	}
	return err
}

func ErrBTCMethodNotImplemented(method string) error {
	return fmt.Errorf("%v not implemented for the %v Asset", method, BTCWalletAsset)
}

func ErrDCRMethodNotImplemented(method string) error {
	return fmt.Errorf("%v not implemented for the %v Asset", method, DCRWalletAsset)
}
