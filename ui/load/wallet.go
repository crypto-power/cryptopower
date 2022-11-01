package load

import (
	"errors"
	"fmt"
	"sort"

	"code.cryptopower.dev/group/cryptopower/libwallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/wallet"
)

type WalletItem struct {
	Wallet       sharedW.Asset
	TotalBalance string
}

type WalletLoad struct {
	MultiWallet *libwallet.AssetsManager
	TxAuthor    dcr.TxAuthor

	UnspentOutputs *wallet.UnspentOutputs
	Wallet         *wallet.Wallet

	SelectedWallet  *WalletItem
	SelectedAccount *int
}

// SortedWalletList can return sorted wallets based on the current selected wallet
// type of on the basis of the provided asset type variadic variable.
func (wl *WalletLoad) SortedWalletList(assetType ...utils.AssetType) []sharedW.Asset {
	var wallets []sharedW.Asset
	if len(assetType) > 0 {
		wallets = wl.getAssets(assetType[0])
	} else {
		// On app start up SelectedWallet is usually not set thus the else use.
		wallets = wl.getAssets()
	}

	if wallets == nil {
		return nil
	}

	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].GetWalletID() < wallets[j].GetWalletID()
	})

	return wallets
}

func (wl *WalletLoad) TotalWalletsBalance() (sharedW.AssetAmount, error) {
	totalBalance := int64(0)
	var wallets = wl.getAssets()
	if wallets == nil {
		return wl.nilAmount(), nil
	}

	for _, w := range wallets {
		accountsResult, err := w.GetAccountsRaw()
		if err != nil {
			return wl.nilAmount(), err
		}
		totalBalance += wl.getAssetTotalbalance(accountsResult).ToInt()
	}
	return wl.SelectedWallet.Wallet.ToAmount(totalBalance), nil
}

func (wl *WalletLoad) getAssets(assetType ...utils.AssetType) []sharedW.Asset {
	var wType utils.AssetType
	if len(assetType) > 0 {
		wType = assetType[0]
	} else {
		// On app start up SelectedWallet is usually not set thus the else use.
		wType = wl.SelectedWallet.Wallet.GetAssetType()
	}

	switch wType {
	case utils.BTCWalletAsset:
		return wl.MultiWallet.AllBTCWallets()
	case utils.DCRWalletAsset:
		return wl.MultiWallet.AllDCRWallets()
	default:
		return nil
	}
}

func (wl *WalletLoad) TotalWalletBalance(walletID int) (sharedW.AssetAmount, error) {
	wallet := wl.MultiWallet.WalletWithID(walletID)
	if wallet == nil {
		return wl.nilAmount(), errors.New(utils.ErrNotExist)
	}

	accountsResult, err := wallet.GetAccountsRaw()
	if err != nil {
		return wl.nilAmount(), err
	}

	return wl.getAssetTotalbalance(accountsResult), nil
}

func (wl *WalletLoad) SpendableWalletBalance(walletID int) (sharedW.AssetAmount, error) {
	wallet := wl.MultiWallet.WalletWithID(walletID)
	if wallet == nil {
		return wl.nilAmount(), errors.New(utils.ErrNotExist)
	}

	accountsResult, err := wallet.GetAccountsRaw()
	if err != nil {
		return wl.nilAmount(), err
	}
	return wl.getAssetSpendablebalance(accountsResult), nil
}

func (wl *WalletLoad) DCRHDPrefix() string {
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

func (wl *WalletLoad) nilAmount() sharedW.AssetAmount {
	return wl.SelectedWallet.Wallet.ToAmount(-1)
}

func (wl *WalletLoad) getAssetTotalbalance(accountsResult *sharedW.Accounts) sharedW.AssetAmount {
	var totalBalance int64
	for _, account := range accountsResult.Accounts {
		totalBalance += account.Balance.Total.ToInt()
	}
	return wl.SelectedWallet.Wallet.ToAmount(totalBalance)
}

func (wl *WalletLoad) getAssetSpendablebalance(accountsResult *sharedW.Accounts) sharedW.AssetAmount {
	var totalBalance int64
	for _, account := range accountsResult.Accounts {
		totalBalance += account.Balance.Spendable.ToInt()
	}
	return wl.SelectedWallet.Wallet.ToAmount(totalBalance)
}
