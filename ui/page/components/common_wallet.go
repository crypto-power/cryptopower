package components

import (
	"fmt"

	"gitlab.com/raedah/cryptopower/libwallet/assets/btc"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	"gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/ui/utils"
)

type CommonWallets struct {
	dcr        *dcr.DCRAsset
	btc        *btc.BTCAsset
	walletType utils.WalletType
}

func NewDCRCommonWallet(dcr *dcr.DCRAsset) *CommonWallets {
	return &CommonWallets{
		dcr:        dcr,
		walletType: utils.DCRWalletAsset,
	}
}

func NewBTCCommonWallet(btc *btc.BTCAsset) *CommonWallets {
	return &CommonWallets{
		btc:        btc,
		walletType: utils.BTCWalletAsset,
	}
}

func (wallt *CommonWallets) ID() int {
	switch wallt.walletType {
	case utils.DCRWalletAsset:
		return wallt.dcr.ID
	case utils.BTCWalletAsset:
		return wallt.btc.ID
	default:
		return -1
	}
}

func (wallt *CommonWallets) Name() string {
	switch wallt.walletType {
	case utils.DCRWalletAsset:
		return wallt.dcr.Name
	case utils.BTCWalletAsset:
		return wallt.btc.Name
	default:
		return ""
	}
}

func (wallt *CommonWallets) GetAccountsRaw() (*wallet.Accounts, error) {
	switch wallt.walletType {
	case utils.DCRWalletAsset:
		return wallt.dcr.GetAccountsRaw()
	case utils.BTCWalletAsset:
		return wallt.btc.GetAccountsRawX()
	default:
		return nil, fmt.Errorf("wallet type missing")
	}
}

func (wallt *CommonWallets) AddTxAndBlockNotificationListener(txAndBlockNotificationListener wallet.TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error {
	switch wallt.walletType {
	case utils.DCRWalletAsset:
		return wallt.dcr.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, async, uniqueIdentifier)
	case utils.BTCWalletAsset:
		return fmt.Errorf("btc wallet not have function")
	default:
		return fmt.Errorf("wallet type missing")
	}
}

func (wallt *CommonWallets) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	switch wallt.walletType {
	case utils.DCRWalletAsset:
		wallt.dcr.RemoveTxAndBlockNotificationListener(uniqueIdentifier)
	case utils.BTCWalletAsset:
		//TODO: implement btc fucntion
		fmt.Printf("Error: btc wallet not have function")
	default:
		fmt.Printf("wallet type missing")
	}
}

func (wallt *CommonWallets) IsSynced() bool {
	switch wallt.walletType {
	case utils.DCRWalletAsset:
		return wallt.dcr.IsSynced()
	case utils.BTCWalletAsset:
		//TODO: implement btc fucntion
		return false
	default:
		return false
	}
}

func (wallt *CommonWallets) IsWatchingOnlyWallet() bool {
	switch wallt.walletType {
	case utils.DCRWalletAsset:
		return wallt.dcr.IsWatchingOnlyWallet()
	case utils.BTCWalletAsset:
		return wallt.btc.IsWatchingOnlyWallet()
	default:
		return false
	}
}

func (wallt *CommonWallets) CurrentAddress(account int32) (string, error) {
	switch wallt.walletType {
	case utils.DCRWalletAsset:
		return wallt.dcr.CurrentAddress(account)
	case utils.BTCWalletAsset:
		return wallt.btc.CurrentAddress(account)
	default:
		return "", fmt.Errorf("wallet type missing")
	}
}
