package libwallet

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/udb"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/libwallet/addresshelper"
)

const (
	AddressGapLimit       uint32 = 20
	ImportedAccountNumber        = udb.ImportedAddrAccount
	DefaultAccountNum            = udb.DefaultAccountNum
)

func (wallet *Wallet) GetAccounts() (string, error) {
	accountsResponse, err := wallet.GetAccountsRaw()
	if err != nil {
		return "", nil
	}

	result, _ := json.Marshal(accountsResponse)
	return string(result), nil
}

func (wallet *Wallet) GetAccountsRaw() (*Accounts, error) {
	resp, err := wallet.Internal().Accounts(wallet.shutdownContext())
	if err != nil {
		return nil, err
	}

	accounts := make([]*Account, len(resp.Accounts))
	for i, a := range resp.Accounts {
		balance, err := wallet.GetAccountBalance(int32(a.AccountNumber))
		if err != nil {
			return nil, err
		}

		accounts[i] = &Account{
			WalletID:         wallet.ID,
			Number:           int32(a.AccountNumber),
			Name:             a.AccountName,
			Balance:          balance,
			TotalBalance:     balance.Total,
			ExternalKeyCount: int32(a.LastUsedExternalIndex + AddressGapLimit), // Add gap limit
			InternalKeyCount: int32(a.LastUsedInternalIndex + AddressGapLimit),
			ImportedKeyCount: int32(a.ImportedKeyCount),
		}
	}

	return &Accounts{
		Count:              len(resp.Accounts),
		CurrentBlockHash:   resp.CurrentBlockHash[:],
		CurrentBlockHeight: resp.CurrentBlockHeight,
		Acc:                accounts,
	}, nil
}

func (wallet *Wallet) AccountsIterator() (*AccountsIterator, error) {
	accounts, err := wallet.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	return &AccountsIterator{
		currentIndex: 0,
		accounts:     accounts.Acc,
	}, nil
}

func (accountsInterator *AccountsIterator) Next() *Account {
	if accountsInterator.currentIndex < len(accountsInterator.accounts) {
		account := accountsInterator.accounts[accountsInterator.currentIndex]
		accountsInterator.currentIndex++
		return account
	}

	return nil
}

func (accountsInterator *AccountsIterator) Reset() {
	accountsInterator.currentIndex = 0
}

func (wallet *Wallet) GetAccount(accountNumber int32) (*Account, error) {
	accounts, err := wallet.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	for _, account := range accounts.Acc {
		if account.Number == accountNumber {
			return account, nil
		}
	}

	return nil, errors.New(ErrNotExist)
}

func (wallet *Wallet) GetAccountBalance(accountNumber int32) (*Balance, error) {
	balance, err := wallet.Internal().AccountBalance(wallet.shutdownContext(), uint32(accountNumber), wallet.RequiredConfirmations())
	if err != nil {
		return nil, err
	}

	return &Balance{
		Total:                   int64(balance.Total),
		Spendable:               int64(balance.Spendable),
		ImmatureReward:          int64(balance.ImmatureCoinbaseRewards),
		ImmatureStakeGeneration: int64(balance.ImmatureStakeGeneration),
		LockedByTickets:         int64(balance.LockedByTickets),
		VotingAuthority:         int64(balance.VotingAuthority),
		UnConfirmed:             int64(balance.Unconfirmed),
	}, nil
}

func (wallet *Wallet) SpendableForAccount(account int32) (int64, error) {
	bals, err := wallet.Internal().AccountBalance(wallet.shutdownContext(), uint32(account), wallet.RequiredConfirmations())
	if err != nil {
		log.Error(err)
		return 0, translateError(err)
	}
	return int64(bals.Spendable), nil
}

func (wallet *Wallet) UnspentOutputs(account int32) ([]*UnspentOutput, error) {
	policy := w.OutputSelectionPolicy{
		Account:               uint32(account),
		RequiredConfirmations: wallet.RequiredConfirmations(),
	}

	// fetch all utxos in account to extract details for the utxos selected by user
	// use targetAmount = 0 to fetch ALL utxos in account
	inputDetail, err := wallet.Internal().SelectInputs(wallet.shutdownContext(), dcrutil.Amount(0), policy)

	if err != nil {
		return nil, err
	}

	unspentOutputs := make([]*UnspentOutput, len(inputDetail.Inputs))

	for i, input := range inputDetail.Inputs {
		outputInfo, err := wallet.Internal().OutputInfo(wallet.shutdownContext(), &input.PreviousOutPoint)
		if err != nil {
			return nil, err
		}

		// unique key to identify utxo
		outputKey := fmt.Sprintf("%s:%d", input.PreviousOutPoint.Hash, input.PreviousOutPoint.Index)

		addresses := addresshelper.PkScriptAddresses(wallet.chainParams, inputDetail.Scripts[i])

		var confirmations int32
		inputBlockHeight := int32(input.BlockHeight)
		if inputBlockHeight != -1 {
			confirmations = wallet.GetBestBlock() - inputBlockHeight + 1
		}

		unspentOutputs[i] = &UnspentOutput{
			TransactionHash: input.PreviousOutPoint.Hash[:],
			OutputIndex:     input.PreviousOutPoint.Index,
			OutputKey:       outputKey,
			Tree:            int32(input.PreviousOutPoint.Tree),
			Amount:          int64(outputInfo.Amount),
			PkScript:        inputDetail.Scripts[i],
			ReceiveTime:     outputInfo.Received.Unix(),
			FromCoinbase:    outputInfo.FromCoinbase,
			Addresses:       strings.Join(addresses, ", "),
			Confirmations:   confirmations,
		}
	}

	return unspentOutputs, nil
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

	ctx := wallet.shutdownContext()

	accountNumber, err := wallet.Internal().NextAccount(ctx, accountName)
	if err != nil {
		return -1, err
	}

	return int32(accountNumber), nil
}

func (wallet *Wallet) RenameAccount(accountNumber int32, newName string) error {
	err := wallet.Internal().RenameAccount(wallet.shutdownContext(), uint32(accountNumber), newName)
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
	return wallet.Internal().AccountName(wallet.shutdownContext(), accountNumber)
}

func (wallet *Wallet) AccountNumber(accountName string) (int32, error) {
	accountNumber, err := wallet.Internal().AccountNumber(wallet.shutdownContext(), accountName)
	return int32(accountNumber), translateError(err)
}

func (wallet *Wallet) HasAccount(accountName string) bool {
	_, err := wallet.Internal().AccountNumber(wallet.shutdownContext(), accountName)
	return err == nil
}

func (wallet *Wallet) HDPathForAccount(accountNumber int32) (string, error) {
	cointype, err := wallet.Internal().CoinType(wallet.shutdownContext())
	if err != nil {
		return "", translateError(err)
	}

	var hdPath string
	isLegacyCoinType := cointype == wallet.chainParams.LegacyCoinType
	if wallet.chainParams.Name == chaincfg.MainNetParams().Name {
		if isLegacyCoinType {
			hdPath = LegacyMainnetHDPath
		} else {
			hdPath = MainnetHDPath
		}
	} else {
		if isLegacyCoinType {
			hdPath = LegacyTestnetHDPath
		} else {
			hdPath = TestnetHDPath
		}
	}

	return hdPath + strconv.Itoa(int(accountNumber)), nil
}
