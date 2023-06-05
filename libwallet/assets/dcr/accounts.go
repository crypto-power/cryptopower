package dcr

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/chaincfg/v3"
	"gitlab.com/cryptopower/cryptopower/libwallet/addresshelper"
	sharedW "gitlab.com/cryptopower/cryptopower/libwallet/assets/wallet"
	"gitlab.com/cryptopower/cryptopower/libwallet/utils"
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
	if !asset.WalletOpened() {
		return nil, utils.ErrDCRNotInitialized
	}

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
			AccountProperties: sharedW.AccountProperties{
				AccountNumber: a.AccountNumber,
				AccountName:   a.AccountName,
			},
			WalletID:         asset.ID,
			Number:           int32(a.AccountNumber),
			Name:             a.AccountName,
			Balance:          balance,
			ExternalKeyCount: int32(a.LastUsedExternalIndex + AddressGapLimit), // Add gap limit
			InternalKeyCount: int32(a.LastUsedInternalIndex + AddressGapLimit),
			ImportedKeyCount: int32(a.ImportedKeyCount),
		}
	}

	return &sharedW.Accounts{
		CurrentBlockHash:   resp.CurrentBlockHash[:],
		CurrentBlockHeight: resp.CurrentBlockHeight,
		Accounts:           accounts,
	}, nil
}

func (asset *DCRAsset) AccountsIterator() (*AccountsIterator, error) {
	accounts, err := asset.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	return &AccountsIterator{
		currentIndex: 0,
		accounts:     accounts.Accounts,
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

	for _, account := range accounts.Accounts {
		if account.Number == accountNumber {
			return account, nil
		}
	}

	return nil, errors.New(utils.ErrNotExist)
}

func (asset *DCRAsset) GetAccountBalance(accountNumber int32) (*sharedW.Balance, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrDCRNotInitialized
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	balance, err := asset.Internal().DCR.AccountBalance(ctx, uint32(accountNumber), asset.RequiredConfirmations())
	if err != nil {
		return nil, err
	}

	return &sharedW.Balance{
		Total:                   DCRAmount(balance.Total),
		Spendable:               DCRAmount(balance.Spendable),
		ImmatureReward:          DCRAmount(balance.ImmatureCoinbaseRewards),
		ImmatureStakeGeneration: DCRAmount(balance.ImmatureStakeGeneration),
		LockedByTickets:         DCRAmount(balance.LockedByTickets),
		VotingAuthority:         DCRAmount(balance.VotingAuthority),
		UnConfirmed:             DCRAmount(balance.Unconfirmed),
	}, nil
}

func (asset *DCRAsset) SpendableForAccount(account int32) (int64, error) {
	if !asset.WalletOpened() {
		return -1, utils.ErrDCRNotInitialized
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	bals, err := asset.Internal().DCR.AccountBalance(ctx, uint32(account), asset.RequiredConfirmations())
	if err != nil {
		log.Error(err)
		return 0, utils.TranslateError(err)
	}
	return int64(bals.Spendable), nil
}

func (asset *DCRAsset) UnspentOutputs(account int32) ([]*sharedW.UnspentOutput, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrDCRNotInitialized
	}

	policy := w.OutputSelectionPolicy{
		Account:               uint32(account),
		RequiredConfirmations: asset.RequiredConfirmations(),
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	unspents, err := asset.Internal().DCR.UnspentOutputs(ctx, policy)
	if err != nil {
		return nil, err
	}

	unspentOutputs := make([]*sharedW.UnspentOutput, 0, len(unspents))
	for _, utxo := range unspents {
		addresses := addresshelper.PkScriptAddresses(asset.chainParams, utxo.Output.PkScript)

		var confirmations int32
		inputBlockHeight := utxo.ContainingBlock.Height
		if inputBlockHeight != -1 {
			confirmations = asset.GetBestBlockHeight() - inputBlockHeight + 1
		}

		addr := ""
		if len(addresses) > 0 {
			addr = addresses[0]
		}

		unspentOutputs = append(unspentOutputs, &sharedW.UnspentOutput{
			TxID:          utxo.OutPoint.Hash.String(),
			Vout:          utxo.OutPoint.Index,
			Address:       addr,
			Amount:        DCRAmount(utxo.Output.Value),
			ScriptPubKey:  hex.EncodeToString(utxo.Output.PkScript),
			ReceiveTime:   utxo.ReceiveTime,
			Confirmations: confirmations,
			Spendable:     true,
			Tree:          utxo.OutPoint.Tree,
		})
	}

	return unspentOutputs, nil
}

func (asset *DCRAsset) CreateNewAccount(accountName, privPass string) (int32, error) {
	err := asset.UnlockWallet(privPass)
	if err != nil {
		return -1, err
	}

	defer asset.LockWallet()

	return asset.NextAccount(accountName)
}

func (asset *DCRAsset) NextAccount(accountName string) (int32, error) {
	if !asset.WalletOpened() {
		return -1, utils.ErrDCRNotInitialized
	}

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
	if !asset.WalletOpened() {
		return utils.ErrDCRNotInitialized
	}

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
	if !asset.WalletOpened() {
		return "", utils.ErrDCRNotInitialized
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	return asset.Internal().DCR.AccountName(ctx, accountNumber)
}

func (asset *DCRAsset) AccountNumber(accountName string) (int32, error) {
	if !asset.WalletOpened() {
		return -1, utils.ErrDCRNotInitialized
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	accountNumber, err := asset.Internal().DCR.AccountNumber(ctx, accountName)
	return int32(accountNumber), utils.TranslateError(err)
}

func (asset *DCRAsset) HasAccount(accountName string) bool {
	if !asset.WalletOpened() {
		return false
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	_, err := asset.Internal().DCR.AccountNumber(ctx, accountName)
	return err == nil
}

func (asset *DCRAsset) HDPathForAccount(accountNumber int32) (string, error) {
	if !asset.WalletOpened() {
		return "", utils.ErrDCRNotInitialized
	}

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
	if !asset.WalletOpened() {
		return "", utils.ErrDCRNotInitialized
	}

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
