package dcr

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/libwallet/addresshelper"
	mainW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

func (wallet *Wallet) GetAccounts() (string, error) {
	accountsResponse, err := wallet.GetAccountsRaw()
	if err != nil {
		return "", nil
	}

	result, _ := json.Marshal(accountsResponse)
	return string(result), nil
}

func (wallet *Wallet) GetAccountsRaw() (*mainW.Accounts, error) {
	ctx, _ := wallet.ShutdownContextWithCancel()
	resp, err := wallet.Internal().DCR.Accounts(ctx)
	if err != nil {
		return nil, err
	}

	accounts := make([]*mainW.Account, len(resp.Accounts))
	for i, a := range resp.Accounts {
		balance, err := wallet.GetAccountBalance(int32(a.AccountNumber))
		if err != nil {
			return nil, err
		}

		accounts[i] = &mainW.Account{
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

	return &mainW.Accounts{
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

func (accountsInterator *AccountsIterator) Next() *mainW.Account {
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

func (wallet *Wallet) GetAccount(accountNumber int32) (*mainW.Account, error) {
	accounts, err := wallet.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	for _, account := range accounts.Acc {
		if account.Number == accountNumber {
			return account, nil
		}
	}

	return nil, errors.New(utils.ErrNotExist)
}

func (wallet *Wallet) GetAccountBalance(accountNumber int32) (*mainW.Balance, error) {
	ctx, _ := wallet.ShutdownContextWithCancel()
	balance, err := wallet.Internal().DCR.AccountBalance(ctx, uint32(accountNumber), wallet.RequiredConfirmations())
	if err != nil {
		return nil, err
	}

	return &mainW.Balance{
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
	ctx, _ := wallet.ShutdownContextWithCancel()
	bals, err := wallet.Internal().DCR.AccountBalance(ctx, uint32(account), wallet.RequiredConfirmations())
	if err != nil {
		log.Error(err)
		return 0, utils.TranslateError(err)
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
	ctx, _ := wallet.ShutdownContextWithCancel()
	inputDetail, err := wallet.Internal().DCR.SelectInputs(ctx, dcrutil.Amount(0), policy)

	if err != nil {
		return nil, err
	}

	unspentOutputs := make([]*UnspentOutput, len(inputDetail.Inputs))

	for i, input := range inputDetail.Inputs {
		outputInfo, err := wallet.Internal().DCR.OutputInfo(ctx, &input.PreviousOutPoint)
		if err != nil {
			return nil, err
		}

		// unique key to identify utxo
		outputKey := fmt.Sprintf("%s:%d", input.PreviousOutPoint.Hash, input.PreviousOutPoint.Index)

		addresses := addresshelper.PkScriptAddresses(wallet.chainParams, inputDetail.Scripts[i])

		var confirmations int32
		inputBlockHeight := int32(input.BlockHeight)
		if inputBlockHeight != -1 {
			confirmations = wallet.GetBestBlockHeight() - inputBlockHeight + 1
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
		return -1, errors.New(utils.ErrWalletLocked)
	}

	ctx, _ := wallet.ShutdownContextWithCancel()
	accountNumber, err := wallet.Internal().DCR.NextAccount(ctx, accountName)
	if err != nil {
		return -1, err
	}

	return int32(accountNumber), nil
}

func (wallet *Wallet) RenameAccount(accountNumber int32, newName string) error {
	ctx, _ := wallet.ShutdownContextWithCancel()
	err := wallet.Internal().DCR.RenameAccount(ctx, uint32(accountNumber), newName)
	if err != nil {
		return utils.TranslateError(err)
	}

	return nil
}

func (wallet *Wallet) AccountName(accountNumber int32) (string, error) {
	name, err := wallet.AccountNameRaw(uint32(accountNumber))
	if err != nil {
		return "", utils.TranslateError(err)
	}
	return name, nil
}

func (wallet *Wallet) AccountNameRaw(accountNumber uint32) (string, error) {
	ctx, _ := wallet.ShutdownContextWithCancel()
	return wallet.Internal().DCR.AccountName(ctx, accountNumber)
}

func (wallet *Wallet) AccountNumber(accountName string) (int32, error) {
	ctx, _ := wallet.ShutdownContextWithCancel()
	accountNumber, err := wallet.Internal().DCR.AccountNumber(ctx, accountName)
	return int32(accountNumber), utils.TranslateError(err)
}

func (wallet *Wallet) HasAccount(accountName string) bool {
	ctx, _ := wallet.ShutdownContextWithCancel()
	_, err := wallet.Internal().DCR.AccountNumber(ctx, accountName)
	return err == nil
}

func (wallet *Wallet) HDPathForAccount(accountNumber int32) (string, error) {
	ctx, _ := wallet.ShutdownContextWithCancel()
	cointype, err := wallet.Internal().DCR.CoinType(ctx)
	if err != nil {
		return "", utils.TranslateError(err)
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

func (wallet *Wallet) GetExtendedPubKey(account int32) (string, error) {
	loadedWallet := wallet.Internal().DCR
	if loadedWallet == nil {
		return "", fmt.Errorf("dcr asset not initialised")
	}
	ctx, _ := wallet.ShutdownContextWithCancel()
	extendedPublicKey, err := loadedWallet.AccountXpub(ctx, uint32(account))
	if err != nil {
		return "", err
	}
	return extendedPublicKey.String(), nil
}
