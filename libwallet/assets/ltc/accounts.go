package ltc

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"decred.org/dcrwallet/v3/errors"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/dcrlabs/ltcwallet/waddrmgr"
	"github.com/ltcsuite/ltcd/chaincfg"
	"github.com/ltcsuite/ltcd/ltcutil"
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
		return nil, utils.ErrLTCNotInitialized
	}

	resp, err := asset.Internal().LTC.Accounts(GetScope())
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
		return nil, utils.ErrLTCNotInitialized
	}

	balance, err := asset.Internal().LTC.CalculateAccountBalances(uint32(accountNumber), asset.RequiredConfirmations())
	if err != nil {
		return nil, err
	}

	// Account for locked amount.
	lockedAmount, err := asset.lockedAmount()
	if err != nil {
		return nil, err
	}

	return &sharedW.Balance{
		Total:          Amount(balance.Total),
		Spendable:      Amount(balance.Spendable - lockedAmount),
		ImmatureReward: Amount(balance.ImmatureReward),
		Locked:         Amount(lockedAmount),
	}, nil
}

// lockedAmount is the total value of locked outputs, as locked with
// LockUnspent.
func (asset *Asset) lockedAmount() (ltcutil.Amount, error) {
	lockedOutpoints := asset.Internal().LTC.LockedOutpoints()
	var sum int64
	for _, op := range lockedOutpoints {
		tx, err := asset.GetTransactionRaw(op.Txid)
		if err != nil {
			return 0, err
		}
		sum += tx.Amount
	}
	return ltcutil.Amount(sum), nil
}

// SpendableForAccount returns the spendable balance for the provided account
func (asset *Asset) SpendableForAccount(account int32) (int64, error) {
	if !asset.WalletOpened() {
		return -1, utils.ErrLTCNotInitialized
	}

	bals, err := asset.Internal().LTC.CalculateAccountBalances(uint32(account), asset.RequiredConfirmations())
	if err != nil {
		return 0, utils.TranslateError(err)
	}

	// Account for locked amount.
	lockedAmount, err := asset.lockedAmount()
	if err != nil {
		return 0, err
	}

	return int64(bals.Spendable - lockedAmount), nil
}

// UnspentOutputs returns all the unspent outputs available for the provided
// account index.
func (asset *Asset) UnspentOutputs(account int32) ([]*sharedW.UnspentOutput, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrLTCNotInitialized
	}

	accountName, err := asset.AccountName(account)
	if err != nil {
		return nil, err
	}

	// Only return UTXOs with the required number of confirmations.
	unspents, err := asset.Internal().LTC.ListUnspent(asset.RequiredConfirmations(),
		math.MaxInt32, accountName)
	if err != nil {
		return nil, err
	}
	resp := make([]*sharedW.UnspentOutput, 0, len(unspents))

	for _, utxo := range unspents {
		// error returned is ignored because the amount value is from upstream
		// and doesn't require an extra layer of validation.
		amount, _ := ltcutil.NewAmount(utxo.Amount)

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
		return -1, utils.ErrLTCNotInitialized
	}

	if asset.IsLocked() {
		return -1, errors.New(utils.ErrWalletLocked)
	}

	accountNumber, err := asset.Internal().LTC.NextAccount(GetScope(), accountName)
	if err != nil {
		return -1, err
	}

	return int32(accountNumber), nil
}

// RenameAccount renames the account with the provided account number.
func (asset *Asset) RenameAccount(accountNumber int32, newName string) error {
	if !asset.WalletOpened() {
		return utils.ErrLTCNotInitialized
	}

	err := asset.Internal().LTC.RenameAccount(GetScope(), uint32(accountNumber), newName)
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
		return "", utils.ErrLTCNotInitialized
	}

	return asset.Internal().LTC.AccountName(GetScope(), accountNumber)
}

// AccountNumber returns the account number for the provided account name.
func (asset *Asset) AccountNumber(accountName string) (int32, error) {
	if !asset.WalletOpened() {
		return -1, utils.ErrLTCNotInitialized
	}

	accountNumber, err := asset.Internal().LTC.AccountNumber(GetScope(), accountName)
	return int32(accountNumber), utils.TranslateError(err)
}

// HasAccount returns true if there is an account with the provided account name.
func (asset *Asset) HasAccount(accountName string) bool {
	if !asset.WalletOpened() {
		log.Error(utils.ErrLTCNotInitialized)
		return false
	}

	_, err := asset.Internal().LTC.AccountNumber(GetScope(), accountName)
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
