package eth

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

func (asset *Asset) ContainsDiscoveredAccounts() bool {
	if !asset.WalletOpened() {
		log.Warnf("discovered accounts check failed: %v", utils.ErrETHNotInitialized)
		return false
	}
	return len(asset.Internal().ETH.Keystore.Accounts()) > 0
}

func (asset *Asset) GetAccountsRaw() (*sharedW.Accounts, error) {
	return nil, utils.ErrETHMethodNotImplemented("GetAccountsRaw")
}

func (asset *Asset) GetAccount(accountNumber int32) (*sharedW.Account, error) {
	return nil, utils.ErrETHMethodNotImplemented("GetAccount")
}

func (asset *Asset) AccountName(accountNumber int32) (string, error) {
	return "", utils.ErrETHMethodNotImplemented("AccountName")
}

func (asset *Asset) CreateNewAccount(accountName, privPass string) (int32, error) {
	return -1, utils.ErrETHMethodNotImplemented("CreateNewAccount")
}

func (asset *Asset) RenameAccount(accountNumber int32, newName string) error {
	return utils.ErrETHMethodNotImplemented("RenameAccount")
}

func (asset *Asset) AccountNumber(accountName string) (int32, error) {
	return -1, utils.ErrETHMethodNotImplemented("AccountNumber")
}

func (asset *Asset) AccountNameRaw(accountNumber uint32) (string, error) {
	return "", utils.ErrETHMethodNotImplemented("AccountNameRaw")
}

func (asset *Asset) GetAccountBalance(accountNumber int32) (*sharedW.Balance, error) {
	return nil, utils.ErrETHMethodNotImplemented("GetAccountBalance")
}

func (asset *Asset) UnspentOutputs(account int32) ([]*sharedW.UnspentOutput, error) {
	return nil, utils.ErrETHMethodNotImplemented("UnspentOutputs")
}
