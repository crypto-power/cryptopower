package ltc

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

// GetAccountsRaw returns a list of all accounts for the wallet
// without marshalling the response.
func (asset *Asset) GetAccountsRaw() (*sharedW.Accounts, error) {
	return nil, utils.ErrLTCMethodNotImplemented("GetAccountsRaw")
}

// GetAccount returns the account for the provided account number.
// If the account does not exist, an error is returned.
func (asset *Asset) GetAccount(accountNumber int32) (*sharedW.Account, error) {
	return nil, utils.ErrLTCMethodNotImplemented("GetAccount")
}

// GetAccountBalance returns the balance for the provided account number.
func (asset *Asset) GetAccountBalance(accountNumber int32) (*sharedW.Balance, error) {
	return nil, utils.ErrLTCMethodNotImplemented("GetAccountBalance")
}

// UnspentOutputs returns all the unspent outputs available for the provided
// account index.
func (asset *Asset) UnspentOutputs(account int32) ([]*sharedW.UnspentOutput, error) {
	return nil, utils.ErrLTCMethodNotImplemented("UnspentOutputs")
}

// CreateNewAccount creates a new account with the provided account name.
func (asset *Asset) CreateNewAccount(accountName, privPass string) (int32, error) {
	return 0, utils.ErrLTCMethodNotImplemented("CreateNewAccount")
}

// RenameAccount renames the account with the provided account number.
func (asset *Asset) RenameAccount(accountNumber int32, newName string) error {
	return utils.ErrLTCMethodNotImplemented("RenameAccount")
}

// AccountName returns the account name for the provided account number.
func (asset *Asset) AccountName(accountNumber int32) (string, error) {
	return "", utils.ErrLTCMethodNotImplemented("AccountName")
}

// AccountNameRaw returns the account name for the provided account number
// from the internal wallet.
func (asset *Asset) AccountNameRaw(accountNumber uint32) (string, error) {
	return "", utils.ErrLTCMethodNotImplemented("AccountNameRaw")
}

// AccountNumber returns the account number for the provided account name.
func (asset *Asset) AccountNumber(accountName string) (int32, error) {
	return 0, utils.ErrLTCMethodNotImplemented("AccountNumber")
}
