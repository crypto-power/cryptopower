package load

import (
	"errors"
	"fmt"
	"sort"

	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/libwallet/assets/btc"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
	"gitlab.com/raedah/cryptopower/wallet"
)

type WalletItem struct {
	Wallet       *dcr.DCRAsset
	TotalBalance string
}

type BTCWalletItem struct {
	Wallet       *btc.BTCAsset
	TotalBalance string
}

type WalletLoad struct {
	MultiWallet *libwallet.AssetsManager
	TxAuthor    dcr.TxAuthor

	UnspentOutputs *wallet.UnspentOutputs
	Wallet         *wallet.Wallet

	SelectedWallet     *WalletItem
	SelectedBTCWallet  *BTCWalletItem
	SelectedAccount    *int
	SelectedWalletType string
}

func (wl *WalletLoad) SortedWalletList() []*dcr.DCRAsset {
	wallets := wl.MultiWallet.AllDCRWallets()

	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].ID < wallets[j].ID
	})

	return wallets
}

func (wl *WalletLoad) SortedBTCWalletList() []*btc.BTCAsset {
	wallets := wl.MultiWallet.AllBTCWallets()

	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].ID < wallets[j].ID
	})

	return wallets
}

func (wl *WalletLoad) TotalWalletsBalance() (dcrutil.Amount, error) {
	totalBalance := int64(0)
	for _, w := range wl.MultiWallet.AllDCRWallets() {
		accountsResult, err := w.GetAccountsRaw()
		if err != nil {
			return -1, err
		}

		for _, account := range accountsResult.Acc {
			totalBalance += account.TotalBalance
		}
	}

	return dcrutil.Amount(totalBalance), nil
}

func (wl *WalletLoad) TotalWalletBalance(walletID int) (dcrutil.Amount, error) {
	totalBalance := int64(0)
	wallet := wl.MultiWallet.DCRWalletWithID(walletID)
	if wallet == nil {
		return -1, errors.New(utils.ErrNotExist)
	}

	accountsResult, err := wallet.GetAccountsRaw()
	if err != nil {
		return -1, err
	}

	for _, account := range accountsResult.Acc {
		totalBalance += account.TotalBalance
	}

	return dcrutil.Amount(totalBalance), nil
}

func (wl *WalletLoad) SpendableWalletBalance(walletID int) (dcrutil.Amount, error) {
	spendableBal := int64(0)
	wallet := wl.MultiWallet.DCRWalletWithID(walletID)
	if wallet == nil {
		return -1, errors.New(utils.ErrNotExist)
	}

	accountsResult, err := wallet.GetAccountsRaw()
	if err != nil {
		return -1, err
	}

	for _, account := range accountsResult.Acc {
		spendableBal += account.Balance.Spendable
	}

	return dcrutil.Amount(spendableBal), nil
}

func (wl *WalletLoad) HDPrefix() string {
	switch wl.Wallet.Net {
	case string(utils.Testnet):
		return dcr.TestnetHDPath
	case string(utils.Mainnet):
		return dcr.MainnetHDPath
	default:
		return ""
	}
}

func (wl *WalletLoad) BTCHDPrefix() string {
	switch wl.Wallet.Net {
	case string(utils.Testnet):
		return btc.TestnetHDPath
	case string(utils.Mainnet):
		return btc.MainnetHDPath
	default:
		return ""
	}
}

func (wl *WalletLoad) WalletDirectory() string {
	return fmt.Sprintf("%s/%s", wl.Wallet.Root, wl.Wallet.Net)
}

func (wl *WalletLoad) DataSize() string {
	v, err := wl.MultiWallet.RootDirFileSizeInBytes()
	if err != nil {
		return "Unknown"
	}
	return fmt.Sprintf("%f GB", float64(v)*1e-9)
}
