package btc

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"decred.org/dcrwallet/v2/errors"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcwallet/waddrmgr"
	sharedW "gitlab.com/cryptopower/cryptopower/libwallet/assets/wallet"
	"gitlab.com/cryptopower/cryptopower/libwallet/utils"
)

const (
	// AddressGapLimit is the number of consecutive unused addresses that
	// will be tracked before the wallet stops searching for new transactions.
	AddressGapLimit uint32 = 20
	// ImportedAccountNumber is the account number used for imported addresses.
	ImportedAccountNumber = waddrmgr.ImportedAddrAccount
	// DefaultAccountNum is the account number used for the default account.
	DefaultAccountNum = waddrmgr.DefaultAccountNum
)

// GetAccounts returns a list of all accounts for the wallet.
func (asset *Asset) GetAccounts() (string, error) {
	accountsResponse, err := asset.GetAccountsRaw()
	if err != nil {
		return "", err
	}

	result, err := json.Marshal(accountsResponse)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// GetAccountsRaw returns a list of all accounts for the wallet
// without marshalling the response.
func (asset *Asset) GetAccountsRaw() (*sharedW.Accounts, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrBTCNotInitialized
	}

	resp, err := asset.Internal().BTC.Accounts(GetScope())
	if err != nil {
		return nil, err
	}

	accounts := make([]*sharedW.Account, len(resp.Accounts))
	for i, a := range resp.Accounts {
		balance, err := asset.GetAccountBalance(int32(a.AccountNumber))
		if err != nil {
			return nil, err
		}

		accounts[i] = &sharedW.Account{
			AccountProperties: sharedW.AccountProperties{
				AccountNumber:    a.AccountNumber,
				AccountName:      a.AccountName,
				ExternalKeyCount: a.ExternalKeyCount + AddressGapLimit, // Add gap limit
				InternalKeyCount: a.InternalKeyCount + AddressGapLimit,
				ImportedKeyCount: a.ImportedKeyCount,
			},
			Number:   int32(a.AccountNumber),
			Name:     a.AccountName,
			WalletID: asset.ID,
			Balance:  balance,
		}
	}

	return &sharedW.Accounts{
		CurrentBlockHash:   resp.CurrentBlockHash[:],
		CurrentBlockHeight: resp.CurrentBlockHeight,
		Accounts:           accounts,
	}, nil
}

// GetAccount returns the account for the provided account number.
// If the account does not exist, an error is returned.
func (asset *Asset) GetAccount(accountNumber int32) (*sharedW.Account, error) {
	accounts, err := asset.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	for _, account := range accounts.Accounts {
		if account.AccountNumber == uint32(accountNumber) {
			return account, nil
		}
	}

	return nil, errors.New(utils.ErrNotExist)
}

// GetAccountBalance returns the balance for the provided account number.
func (asset *Asset) GetAccountBalance(accountNumber int32) (*sharedW.Balance, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrBTCNotInitialized
	}

	balance, err := asset.Internal().BTC.CalculateAccountBalances(uint32(accountNumber), asset.RequiredConfirmations())
	if err != nil {
		return nil, err
	}

	return &sharedW.Balance{
		Total:          Amount(balance.Total),
		Spendable:      Amount(balance.Spendable),
		ImmatureReward: Amount(balance.ImmatureReward),
	}, nil
}

// SpendableForAccount returns the spendable balance for the provided account
func (asset *Asset) SpendableForAccount(account int32) (int64, error) {
	if !asset.WalletOpened() {
		return -1, utils.ErrBTCNotInitialized
	}

	bals, err := asset.Internal().BTC.CalculateAccountBalances(uint32(account), asset.RequiredConfirmations())
	if err != nil {
		return 0, utils.TranslateError(err)
	}
	return int64(bals.Spendable), nil
}

// UnspentOutputs returns all the unspent outputs available for the provided
// account index.
func (asset *Asset) UnspentOutputs(account int32) ([]*sharedW.UnspentOutput, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrBTCNotInitialized
	}

	accountName, err := asset.AccountName(account)
	if err != nil {
		return nil, err
	}

	// Only return UTXOs with the required number of confirmations.
	unspents, err := asset.Internal().BTC.ListUnspent(asset.RequiredConfirmations(),
		math.MaxInt32, accountName)
	if err != nil {
		return nil, err
	}
	resp := make([]*sharedW.UnspentOutput, 0, len(unspents))

	for _, utxo := range unspents {
		// error returned is ignored because the amount value is from upstream
		// and doesn't require an extra layer of validation.
		amount, _ := btcutil.NewAmount(utxo.Amount)

		txInfo, err := asset.GetTransactionRaw(utxo.TxID)
		if err != nil {
			return nil, fmt.Errorf("invalid TxID %v : error: %v", utxo.TxID, err)
		}

		resp = append(resp, &sharedW.UnspentOutput{
			TxID:          utxo.TxID,
			Vout:          utxo.Vout,
			Address:       utxo.Address,
			ScriptPubKey:  utxo.ScriptPubKey,
			RedeemScript:  utxo.RedeemScript,
			Amount:        Amount(amount),
			Confirmations: int32(utxo.Confirmations),
			Spendable:     utxo.Spendable,
			ReceiveTime:   time.Unix(txInfo.Timestamp, 0),
		})
	}

	return resp, nil
}

// CreateNewAccount creates a new account with the provided account name.
func (asset *Asset) CreateNewAccount(accountName, privPass string) (int32, error) {
	err := asset.UnlockWallet(privPass)
	if err != nil {
		return -1, err
	}

	defer asset.LockWallet()

	return asset.NextAccount(accountName)
}

// NextAccount returns the next account number for the provided account name.
func (asset *Asset) NextAccount(accountName string) (int32, error) {
	if !asset.WalletOpened() {
		return -1, utils.ErrBTCNotInitialized
	}

	if asset.IsLocked() {
		return -1, errors.New(utils.ErrWalletLocked)
	}

	accountNumber, err := asset.Internal().BTC.NextAccount(GetScope(), accountName)
	if err != nil {
		return -1, err
	}

	return int32(accountNumber), nil
}

// RenameAccount renames the account with the provided account number.
func (asset *Asset) RenameAccount(accountNumber int32, newName string) error {
	if !asset.WalletOpened() {
		return utils.ErrBTCNotInitialized
	}

	err := asset.Internal().BTC.RenameAccount(GetScope(), uint32(accountNumber), newName)
	if err != nil {
		return utils.TranslateError(err)
	}

	return nil
}

// AccountName returns the account name for the provided account number.
func (asset *Asset) AccountName(accountNumber int32) (string, error) {
	name, err := asset.AccountNameRaw(uint32(accountNumber))
	if err != nil {
		return "", utils.TranslateError(err)
	}
	return name, nil
}

// AccountNameRaw returns the account name for the provided account number
// from the internal wallet.
func (asset *Asset) AccountNameRaw(accountNumber uint32) (string, error) {
	if !asset.WalletOpened() {
		return "", utils.ErrBTCNotInitialized
	}

	return asset.Internal().BTC.AccountName(GetScope(), accountNumber)
}

// AccountNumber returns the account number for the provided account name.
func (asset *Asset) AccountNumber(accountName string) (int32, error) {
	if !asset.WalletOpened() {
		return -1, utils.ErrBTCNotInitialized
	}

	accountNumber, err := asset.Internal().BTC.AccountNumber(GetScope(), accountName)
	return int32(accountNumber), utils.TranslateError(err)
}

// HasAccount returns true if there is an account with the provided account name.
func (asset *Asset) HasAccount(accountName string) bool {
	if !asset.WalletOpened() {
		return false
	}

	_, err := asset.Internal().BTC.AccountNumber(GetScope(), accountName)
	return err == nil
}

// HDPathForAccount returns the HD path for the provided account number.
func (asset *Asset) HDPathForAccount(accountNumber int32) (string, error) {
	var hdPath string
	if asset.chainParams.Name == chaincfg.MainNetParams.Name {
		hdPath = MainnetHDPath
	} else {
		hdPath = TestnetHDPath
	}

	return hdPath + strconv.Itoa(int(accountNumber)), nil
}
