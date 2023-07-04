package politeia

import (
	"decred.org/dcrwallet/v3/errors"
	"github.com/asdine/storm"
)

const (
	ErrInsufficientBalance   = "insufficient_balance"
	ErrSyncAlreadyInProgress = "sync_already_in_progress"
	ErrNotExist              = "not_exists"
	ErrInvalid               = "invalid"
	ErrListenerAlreadyExist  = "listener_already_exist"
	ErrInvalidAddress        = "invalid_address"
	ErrInvalidPassphrase     = "invalid_passphrase"
	ErrNoPeers               = "no_peers"
)

func translateError(err error) error {
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
