package btc

import (
	"encoding/json"
	"strconv"

	"decred.org/dcrwallet/v2/errors"

	"github.com/btcsuite/btcd/chaincfg"
)

const (
	AddressGapLimit uint32 = 20
)

func (wallet *Wallet) GetAccounts() (string, error) {
	accountsResponse, err := wallet.GetAccountsRaw()
	if err != nil {
		return "", nil
	}

	result, _ := json.Marshal(accountsResponse)
	return string(result), nil
}

func (wallet *Wallet) GetAccountsRaw() (*AccountsResult, error) {
	resp, err := wallet.Internal().Accounts(wallet.GetScope())
	if err != nil {
		return nil, err
	}

	accounts := make([]*AccountResult, len(resp.Accounts))
	for i, a := range resp.Accounts {
		balance, err := wallet.GetAccountBalance(int32(a.AccountNumber))
		if err != nil {
			return nil, err
		}

		accounts[i] = &AccountResult{
			AccountProperties: AccountProperties{
				AccountNumber:    a.AccountNumber,
				AccountName:      a.AccountName,
				ExternalKeyCount: a.ExternalKeyCount + AddressGapLimit, // Add gap limit
				InternalKeyCount: a.InternalKeyCount + AddressGapLimit,
				ImportedKeyCount: a.ImportedKeyCount,
			},
			TotalBalance: balance.Total,
		}
	}

	return &AccountsResult{
		CurrentBlockHash:   resp.CurrentBlockHash,
		CurrentBlockHeight: resp.CurrentBlockHeight,
		Accounts:           accounts,
	}, nil
}

func (wallet *Wallet) GetAccount(accountNumber int32) (*AccountResult, error) {
	accounts, err := wallet.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	for _, account := range accounts.Accounts {
		if account.AccountNumber == uint32(accountNumber) {
			return account, nil
		}
	}

	return nil, errors.New(ErrNotExist)
}

func (wallet *Wallet) GetAccountBalance(accountNumber int32) (*Balances, error) {
	balance, err := wallet.Internal().CalculateAccountBalances(uint32(accountNumber), wallet.RequiredConfirmations())
	if err != nil {
		return nil, err
	}

	return &Balances{
		Total:          balance.Total,
		Spendable:      balance.Spendable,
		ImmatureReward: balance.ImmatureReward,
	}, nil
}

func (wallet *Wallet) SpendableForAccount(account int32) (int64, error) {
	bals, err := wallet.Internal().CalculateAccountBalances(uint32(account), wallet.RequiredConfirmations())
	if err != nil {
		return 0, translateError(err)
	}
	return int64(bals.Spendable), nil
}

func (wallet *Wallet) CreateNewAccount(accountName string, privPass []byte) (int32, error) {
	err := wallet.UnlockWallet(privPass)
	if err != nil {
		return -1, err
	}

	defer wallet.LockWallet()

	return wallet.NextAccount(accountName)
}

func (wallet *Wallet) NextAccount(accountName string) (int32, error) {

	if wallet.IsLocked() {
		return -1, errors.New(ErrWalletLocked)
	}

	accountNumber, err := wallet.Internal().NextAccount(wallet.GetScope(), accountName)
	if err != nil {
		return -1, err
	}

	return int32(accountNumber), nil
}

func (wallet *Wallet) RenameAccount(accountNumber int32, newName string) error {
	err := wallet.Internal().RenameAccount(wallet.GetScope(), uint32(accountNumber), newName)
	if err != nil {
		return translateError(err)
	}

	return nil
}

func (wallet *Wallet) AccountName(accountNumber int32) (string, error) {
	name, err := wallet.AccountNameRaw(uint32(accountNumber))
	if err != nil {
		return "", translateError(err)
	}
	return name, nil
}

func (wallet *Wallet) AccountNameRaw(accountNumber uint32) (string, error) {
	return wallet.Internal().AccountName(wallet.GetScope(), accountNumber)
}

func (wallet *Wallet) AccountNumber(accountName string) (int32, error) {
	accountNumber, err := wallet.Internal().AccountNumber(wallet.GetScope(), accountName)
	return int32(accountNumber), translateError(err)
}

func (wallet *Wallet) HasAccount(accountName string) bool {
	_, err := wallet.Internal().AccountNumber(wallet.GetScope(), accountName)
	return err == nil
}

func (wallet *Wallet) HDPathForAccount(accountNumber int32) (string, error) {
	var hdPath string
	if wallet.chainParams.Name == chaincfg.MainNetParams.Name {
		hdPath = MainnetHDPath
	} else {
		hdPath = TestnetHDPath
	}

	return hdPath + strconv.Itoa(int(accountNumber)), nil
}
