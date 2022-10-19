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
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

func (asset *DCRAsset) GetAccounts() (string, error) {
	accountsResponse, err := asset.GetAccountsRaw()
	if err != nil {
		return "", nil
	}

	result, _ := json.Marshal(accountsResponse)
	return string(result), nil
}

func (asset *DCRAsset) GetAccountsRaw() (*sharedW.Accounts, error) {
	ctx, _ := asset.ShutdownContextWithCancel()
	resp, err := asset.Internal().DCR.Accounts(ctx)
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
			WalletID:         asset.ID,
			Number:           int32(a.AccountNumber),
			Name:             a.AccountName,
			Balance:          balance,
			TotalBalance:     balance.Total,
			ExternalKeyCount: int32(a.LastUsedExternalIndex + AddressGapLimit), // Add gap limit
			InternalKeyCount: int32(a.LastUsedInternalIndex + AddressGapLimit),
			ImportedKeyCount: int32(a.ImportedKeyCount),
		}
	}

	return &sharedW.Accounts{
		Count:              len(resp.Accounts),
		CurrentBlockHash:   resp.CurrentBlockHash[:],
		CurrentBlockHeight: resp.CurrentBlockHeight,
		Acc:                accounts,
	}, nil
}

func (asset *DCRAsset) AccountsIterator() (*AccountsIterator, error) {
	accounts, err := asset.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	return &AccountsIterator{
		currentIndex: 0,
		accounts:     accounts.Acc,
	}, nil
}

func (accountsInterator *AccountsIterator) Next() *sharedW.Account {
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

func (asset *DCRAsset) GetAccount(accountNumber int32) (*sharedW.Account, error) {
	accounts, err := asset.GetAccountsRaw()
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

func (asset *DCRAsset) GetAccountBalance(accountNumber int32) (*sharedW.Balance, error) {
	ctx, _ := asset.ShutdownContextWithCancel()
	balance, err := asset.Internal().DCR.AccountBalance(ctx, uint32(accountNumber), asset.RequiredConfirmations())
	if err != nil {
		return nil, err
	}

	return &sharedW.Balance{
		Total:                   int64(balance.Total),
		Spendable:               int64(balance.Spendable),
		ImmatureReward:          int64(balance.ImmatureCoinbaseRewards),
		ImmatureStakeGeneration: int64(balance.ImmatureStakeGeneration),
		LockedByTickets:         int64(balance.LockedByTickets),
		VotingAuthority:         int64(balance.VotingAuthority),
		UnConfirmed:             int64(balance.Unconfirmed),
	}, nil
}

func (asset *DCRAsset) SpendableForAccount(account int32) (int64, error) {
	ctx, _ := asset.ShutdownContextWithCancel()
	bals, err := asset.Internal().DCR.AccountBalance(ctx, uint32(account), asset.RequiredConfirmations())
	if err != nil {
		log.Error(err)
		return 0, utils.TranslateError(err)
	}
	return int64(bals.Spendable), nil
}

func (asset *DCRAsset) UnspentOutputs(account int32) ([]*UnspentOutput, error) {
	policy := w.OutputSelectionPolicy{
		Account:               uint32(account),
		RequiredConfirmations: asset.RequiredConfirmations(),
	}

	// fetch all utxos in account to extract details for the utxos selected by user
	// use targetAmount = 0 to fetch ALL utxos in account
	ctx, _ := asset.ShutdownContextWithCancel()
	inputDetail, err := asset.Internal().DCR.SelectInputs(ctx, dcrutil.Amount(0), policy)

	if err != nil {
		return nil, err
	}

	unspentOutputs := make([]*UnspentOutput, len(inputDetail.Inputs))

	for i, input := range inputDetail.Inputs {
		outputInfo, err := asset.Internal().DCR.OutputInfo(ctx, &input.PreviousOutPoint)
		if err != nil {
			return nil, err
		}

		// unique key to identify utxo
		outputKey := fmt.Sprintf("%s:%d", input.PreviousOutPoint.Hash, input.PreviousOutPoint.Index)

		addresses := addresshelper.PkScriptAddresses(asset.chainParams, inputDetail.Scripts[i])

		var confirmations int32
		inputBlockHeight := int32(input.BlockHeight)
		if inputBlockHeight != -1 {
			confirmations = asset.GetBestBlockHeight() - inputBlockHeight + 1
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

func (asset *DCRAsset) CreateNewAccount(accountName string, privPass string) (int32, error) {
	err := asset.UnlockWallet(privPass)
	if err != nil {
		return -1, err
	}

	defer asset.LockWallet()

	return asset.NextAccount(accountName)
}

func (asset *DCRAsset) NextAccount(accountName string) (int32, error) {

	if asset.IsLocked() {
		return -1, errors.New(utils.ErrWalletLocked)
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	accountNumber, err := asset.Internal().DCR.NextAccount(ctx, accountName)
	if err != nil {
		return -1, err
	}

	return int32(accountNumber), nil
}

func (asset *DCRAsset) RenameAccount(accountNumber int32, newName string) error {
	ctx, _ := asset.ShutdownContextWithCancel()
	err := asset.Internal().DCR.RenameAccount(ctx, uint32(accountNumber), newName)
	if err != nil {
		return utils.TranslateError(err)
	}

	return nil
}

func (asset *DCRAsset) AccountName(accountNumber int32) (string, error) {
	name, err := asset.AccountNameRaw(uint32(accountNumber))
	if err != nil {
		return "", utils.TranslateError(err)
	}
	return name, nil
}

func (asset *DCRAsset) AccountNameRaw(accountNumber uint32) (string, error) {
	ctx, _ := asset.ShutdownContextWithCancel()
	return asset.Internal().DCR.AccountName(ctx, accountNumber)
}

func (asset *DCRAsset) AccountNumber(accountName string) (int32, error) {
	ctx, _ := asset.ShutdownContextWithCancel()
	accountNumber, err := asset.Internal().DCR.AccountNumber(ctx, accountName)
	return int32(accountNumber), utils.TranslateError(err)
}

func (asset *DCRAsset) HasAccount(accountName string) bool {
	ctx, _ := asset.ShutdownContextWithCancel()
	_, err := asset.Internal().DCR.AccountNumber(ctx, accountName)
	return err == nil
}

func (asset *DCRAsset) HDPathForAccount(accountNumber int32) (string, error) {
	ctx, _ := asset.ShutdownContextWithCancel()
	cointype, err := asset.Internal().DCR.CoinType(ctx)
	if err != nil {
		return "", utils.TranslateError(err)
	}

	var hdPath string
	isLegacyCoinType := cointype == asset.chainParams.LegacyCoinType
	if asset.chainParams.Name == chaincfg.MainNetParams().Name {
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

func (asset *DCRAsset) GetExtendedPubKey(account int32) (string, error) {
	loadedAsset := asset.Internal().DCR
	if loadedAsset == nil {
		return "", fmt.Errorf("dcr asset not initialised")
	}
	ctx, _ := asset.ShutdownContextWithCancel()
	extendedPublicKey, err := loadedAsset.AccountXpub(ctx, uint32(account))
	if err != nil {
		return "", err
	}
	return extendedPublicKey.String(), nil
}
