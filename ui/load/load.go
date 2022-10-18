// The load package contains data structures that are shared by components in the ui package. It is not a dumping ground
// for code you feel might be shared with other components in the future. Before adding code here, ask yourself, can
// the code be isolated in the package you're calling it from? Is it really needed by other packages in the ui package?
// or you're just planning for a use case that might never used.

package load

import (
	"golang.org/x/text/message"

	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/ui/assets"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/notification"
	"gitlab.com/raedah/cryptopower/wallet"
)

type DCRUSDTBittrex struct {
	LastTradeRate string
}

type BTCUSDTBittrex struct {
	LastTradeRate string
}

type Load struct {
	Theme *cryptomaterial.Theme

	WL              *WalletLoad
	Printer         *message.Printer
	Network         string
	CurrentAppWidth int

	Toast *notification.Toast

	SelectedUTXO map[int]map[int32]map[string]*wallet.UnspentOutput

	DarkModeSettingChanged func(bool)
	LanguageSettingChanged func()
	CurrencySettingChanged func()
	ToggleSync             func()
}

func (l *Load) RefreshTheme(window app.WindowNavigator) {
	isDarkModeOn := l.WL.MultiWallet.IsDarkModeOn()
	l.Theme.SwitchDarkMode(isDarkModeOn, assets.DecredIcons)
	l.DarkModeSettingChanged(isDarkModeOn)
	l.LanguageSettingChanged()
	l.CurrencySettingChanged()
	window.Reload()
}

func (l *Load) Dexc() *libwallet.DexClient {
	return l.WL.MultiWallet.DexClient()
}
