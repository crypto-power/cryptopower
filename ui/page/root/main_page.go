package root

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/gen2brain/beeep"
	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/listeners"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/page/dexclient"
	"gitlab.com/raedah/cryptopower/ui/page/governance"
	"gitlab.com/raedah/cryptopower/ui/page/info"
	"gitlab.com/raedah/cryptopower/ui/page/privacy"
	"gitlab.com/raedah/cryptopower/ui/page/seedbackup"
	"gitlab.com/raedah/cryptopower/ui/page/send"
	"gitlab.com/raedah/cryptopower/ui/page/staking"
	"gitlab.com/raedah/cryptopower/ui/page/transaction"
	"gitlab.com/raedah/cryptopower/ui/values"
	"gitlab.com/raedah/cryptopower/wallet"
)

const (
	MainPageID = "Main"
)

var (
	NavDrawerWidth          = unit.Dp(160)
	NavDrawerMinimizedWidth = unit.Dp(72)
)

type NavHandler struct {
	Clickable     *widget.Clickable
	Image         *cryptomaterial.Image
	ImageInactive *cryptomaterial.Image
	Title         string
	PageID        string
}

type MainPage struct {
	*app.MasterPage

	*load.Load
	*listeners.SyncProgressListener
	*listeners.TxAndBlockNotificationListener
	*listeners.ProposalNotificationListener

	ctx                  context.Context
	ctxCancel            context.CancelFunc
	drawerNav            components.NavDrawer
	bottomNavigationBar  components.BottomNavigationBar
	floatingActionButton components.BottomNavigationBar

	hideBalanceButton      *cryptomaterial.Clickable
	refreshExchangeRateBtn *cryptomaterial.Clickable
	openWalletSelector     *cryptomaterial.Clickable
	checkBox               cryptomaterial.CheckBoxStyle

	// page state variables
	dcrUsdtBittrex load.DCRUSDTBittrex
	totalBalance   dcrutil.Amount

	usdExchangeSet         bool
	isFetchingExchangeRate bool
	isBalanceHidden        bool

	setNavExpanded  func()
	totalBalanceUSD string
}

func NewMainPage(l *load.Load) *MainPage {
	mp := &MainPage{
		Load:       l,
		MasterPage: app.NewMasterPage(MainPageID),
		checkBox:   l.Theme.CheckBox(new(widget.Bool), values.String(values.StrAwareOfRisk)),
	}

	mp.hideBalanceButton = mp.Theme.NewClickable(false)
	mp.openWalletSelector = mp.Theme.NewClickable(false)
	mp.refreshExchangeRateBtn = mp.Theme.NewClickable(true)
	mp.setNavExpanded = func() {
		mp.drawerNav.DrawerToggled(mp.drawerNav.IsNavExpanded)
	}

	mp.bottomNavigationBar.OnViewCreated()
	mp.initNavItems()

	return mp
}

// ID is a unique string that identifies the page and may be used
// to differentiate this page from other pages.
// Part of the load.Page interface.
func (mp *MainPage) ID() string {
	return MainPageID
}

func (mp *MainPage) initNavItems() {
	mp.drawerNav = components.NavDrawer{
		Load:        mp.Load,
		CurrentPage: mp.CurrentPageID(),
		DrawerNavItems: []components.NavHandler{
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.OverviewIcon,
				ImageInactive: mp.Theme.Icons.OverviewIconInactive,
				Title:         values.String(values.StrInfo),
				PageID:        info.InfoID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.SendIcon,
				Title:         values.String(values.StrSend),
				ImageInactive: mp.Theme.Icons.SendInactiveIcon,
				PageID:        send.SendPageID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.ReceiveIcon,
				ImageInactive: mp.Theme.Icons.ReceiveInactiveIcon,
				Title:         values.String(values.StrReceive),
				PageID:        ReceivePageID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.TransactionsIcon,
				ImageInactive: mp.Theme.Icons.TransactionsIconInactive,
				Title:         values.String(values.StrTransactions),
				PageID:        transaction.TransactionsPageID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.Mixer,
				ImageInactive: mp.Theme.Icons.MixerInactive,
				Title:         values.String(values.StrStakeShuffle),
				PageID:        privacy.AccountMixerPageID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.StakeIcon,
				ImageInactive: mp.Theme.Icons.StakeIconInactive,
				Title:         values.String(values.StrStaking),
				PageID:        staking.OverviewPageID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.GovernanceActiveIcon,
				ImageInactive: mp.Theme.Icons.GovernanceInactiveIcon,
				Title:         values.String(values.StrGovernance),
				PageID:        governance.GovernancePageID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.MoreIcon,
				ImageInactive: mp.Theme.Icons.MoreIconInactive,
				Title:         values.String(values.StrSettings),
				PageID:        WalletSettingsPageID,
			},
		},
		MinimizeNavDrawerButton: mp.Theme.IconButton(mp.Theme.Icons.NavigationArrowBack),
		MaximizeNavDrawerButton: mp.Theme.IconButton(mp.Theme.Icons.NavigationArrowForward),
	}

	mp.bottomNavigationBar = components.BottomNavigationBar{
		Load:        mp.Load,
		CurrentPage: mp.CurrentPageID(),
		BottomNaigationItems: []components.BottomNavigationBarHandler{
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.OverviewIcon,
				ImageInactive: mp.Theme.Icons.OverviewIconInactive,
				Title:         values.String(values.StrInfo),
				PageID:        info.InfoID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.TransactionsIcon,
				ImageInactive: mp.Theme.Icons.TransactionsIconInactive,
				Title:         values.String(values.StrTransactions),
				PageID:        transaction.TransactionsPageID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.StakeIcon,
				ImageInactive: mp.Theme.Icons.StakeIconInactive,
				Title:         values.String(values.StrStaking),
				PageID:        staking.OverviewPageID,
			},
			{
				Clickable:     mp.Theme.NewClickable(true),
				Image:         mp.Theme.Icons.MoreIcon,
				ImageInactive: mp.Theme.Icons.MoreIconInactive,
				Title:         values.String(values.StrMore),
				PageID:        WalletSettingsPageID,
			},
		},
	}

	mp.floatingActionButton = components.BottomNavigationBar{
		Load:        mp.Load,
		CurrentPage: mp.CurrentPageID(),
		FloatingActionButton: []components.BottomNavigationBarHandler{
			{
				Clickable: mp.Theme.NewClickable(true),
				Image:     mp.Theme.Icons.SendIcon,
				Title:     values.String(values.StrSend),
				PageID:    send.SendPageID,
			},
			{
				Clickable: mp.Theme.NewClickable(true),
				Image:     mp.Theme.Icons.ReceiveIcon,
				Title:     values.String(values.StrReceive),
				PageID:    ReceivePageID,
			},
		},
	}
	mp.floatingActionButton.FloatingActionButton[0].Clickable.Hoverable = false
	mp.floatingActionButton.FloatingActionButton[1].Clickable.Hoverable = false
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (mp *MainPage) OnNavigatedTo() {
	mp.setNavExpanded()

	mp.ctx, mp.ctxCancel = context.WithCancel(context.TODO())
	mp.listenForNotifications()

	backupLater := mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(load.SeedBackupNotificationConfigKey, false)
	// reset the checkbox
	mp.checkBox.CheckBox.Value = false

	needBackup := mp.WL.SelectedWallet.Wallet.EncryptedSeed != nil
	if needBackup && !backupLater {
		mp.showBackupInfo()
	}

	if mp.CurrentPage() == nil {
		mp.Display(info.NewInfoPage(mp.Load)) // TODO: Should pagestack have a start page?
	}
	mp.CurrentPage().OnNavigatedTo()

	if mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(load.FetchProposalConfigKey, false) {
		if mp.WL.MultiWallet.Politeia.IsSyncing() {
			return
		}
		go mp.WL.MultiWallet.Politeia.Sync(mp.ctx)
	}

	mp.updateBalance()
	mp.updateExchangeSetting()
}

func (mp *MainPage) updateExchangeSetting() {
	currencyExchangeValue := mp.WL.MultiWallet.ReadStringConfigValueForKey(libwallet.CurrencyConversionConfigKey)
	if currencyExchangeValue == "" {
		mp.WL.MultiWallet.SaveUserConfigValue(libwallet.CurrencyConversionConfigKey, values.DefaultExchangeValue)
	}

	usdExchangeSet := currencyExchangeValue == values.USDExchangeValue
	if mp.usdExchangeSet == usdExchangeSet {
		return // nothing has changed
	}
	mp.usdExchangeSet = usdExchangeSet
	if mp.usdExchangeSet {
		go mp.fetchExchangeRate()
	}
}

func (mp *MainPage) fetchExchangeRate() {
	if mp.isFetchingExchangeRate {
		return
	}
	maxAttempts := 5
	delayBtwAttempts := 2 * time.Second
	mp.isFetchingExchangeRate = true
	desc := "for getting dcrUsdtBittrex exchange rate value"
	attempts, err := components.RetryFunc(maxAttempts, delayBtwAttempts, desc, func() error {
		return load.GetUSDExchangeValue(&mp.dcrUsdtBittrex)
	})
	if err != nil {
		log.Errorf("error fetching usd exchange rate value after %d attempts: %v", attempts, err)
	} else if mp.dcrUsdtBittrex.LastTradeRate == "" {
		log.Errorf("no error while fetching usd exchange rate in %d tries, but no rate was fetched", attempts)
	} else {
		log.Infof("exchange rate value fetched: %s", mp.dcrUsdtBittrex.LastTradeRate)
		mp.updateBalance()
		mp.ParentWindow().Reload()
	}
	mp.isFetchingExchangeRate = false
}

func (mp *MainPage) updateBalance() {
	totalBalance, err := components.CalculateTotalWalletsBalance(mp.Load)
	if err == nil {
		mp.totalBalance = totalBalance.Total

		if mp.usdExchangeSet && mp.dcrUsdtBittrex.LastTradeRate != "" {
			usdExchangeRate, err := strconv.ParseFloat(mp.dcrUsdtBittrex.LastTradeRate, 64)
			if err == nil {
				balanceInUSD := totalBalance.Total.ToCoin() * usdExchangeRate
				mp.totalBalanceUSD = load.FormatUSDBalance(mp.Printer, balanceInUSD)
			}
		}
	}
}

// OnDarkModeChanged is triggered whenever the dark mode setting is changed
// to enable restyling UI elements where necessary.
// Satisfies the load.AppSettingsChangeHandler interface.
func (mp *MainPage) OnDarkModeChanged(isDarkModeOn bool) {
	// TODO: currentPage will likely be the Settings page when this method
	// is called. If that page implements the AppSettingsChangeHandler interface,
	// the following code will trigger the OnDarkModeChanged method of that
	// page.
	if currentPage, ok := mp.CurrentPage().(load.AppSettingsChangeHandler); ok {
		currentPage.OnDarkModeChanged(isDarkModeOn)
	}

	mp.initNavItems()
	mp.setNavExpanded()
	mp.bottomNavigationBar.OnViewCreated()
}

func (mp *MainPage) OnCurrencyChanged() {
	mp.updateExchangeSetting()
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (mp *MainPage) HandleUserInteractions() {
	if mp.CurrentPage() != nil {
		mp.CurrentPage().HandleUserInteractions()
	}

	if mp.refreshExchangeRateBtn.Clicked() {
		go mp.fetchExchangeRate()
	}

	for mp.openWalletSelector.Clicked() {
		onWalSelected := func() {
			mp.ParentNavigator().ClearStackAndDisplay(NewMainPage(mp.Load))
		}
		onDexServerSelected := func(server string) {
			log.Info("Not implemented yet...", server)
		}
		mp.ParentWindow().Display(NewWalletDexServerSelector(mp.Load, onWalSelected, onDexServerSelected))
	}

	mp.drawerNav.CurrentPage = mp.CurrentPageID()
	mp.bottomNavigationBar.CurrentPage = mp.CurrentPageID()
	mp.floatingActionButton.CurrentPage = mp.CurrentPageID()

	for mp.drawerNav.MinimizeNavDrawerButton.Button.Clicked() {
		mp.drawerNav.IsNavExpanded = true
		mp.setNavExpanded()
	}

	for mp.drawerNav.MaximizeNavDrawerButton.Button.Clicked() {
		mp.drawerNav.IsNavExpanded = false
		mp.setNavExpanded()
	}

	for _, item := range mp.drawerNav.DrawerNavItems {
		for item.Clickable.Clicked() {
			var pg app.Page
			switch item.PageID {
			case send.SendPageID:
				pg = send.NewSendPage(mp.Load)
			case ReceivePageID:
				pg = NewReceivePage(mp.Load)
			case info.InfoID:
				pg = info.NewInfoPage(mp.Load)
			case transaction.TransactionsPageID:
				pg = transaction.NewTransactionsPage(mp.Load)
			case privacy.AccountMixerPageID:
				pg = privacy.NewAccountMixerPage(mp.Load)
			case staking.OverviewPageID:
				pg = staking.NewStakingPage(mp.Load)
			case governance.GovernancePageID:
				pg = governance.NewGovernancePage(mp.Load)
			case dexclient.MarketPageID:
				pg = dexclient.NewMarketPage(mp.Load)
			case WalletSettingsPageID:
				pg = NewWalletSettingsPage(mp.Load)
			}

			if pg == nil || mp.ID() == mp.CurrentPageID() {
				continue
			}

			// check if wallet is synced and clear stack
			if mp.ID() == send.SendPageID || mp.ID() == ReceivePageID {
				if mp.WL.MultiWallet.IsSynced() {
					mp.Display(pg)
				} else if mp.WL.MultiWallet.IsSyncing() {
					errModal := modal.NewErrorModal(mp.Load, values.String(values.StrNotConnected), modal.DefaultClickFunc())
					mp.ParentWindow().ShowModal(errModal)
				} else {
					errModal := modal.NewErrorModal(mp.Load, values.String(values.StrWalletSyncing), modal.DefaultClickFunc())
					mp.ParentWindow().ShowModal(errModal)
				}
			} else {
				mp.Display(pg)
			}
		}
	}

	for _, item := range mp.bottomNavigationBar.BottomNaigationItems {
		for item.Clickable.Clicked() {
			var pg app.Page
			switch item.PageID {
			case transaction.TransactionsPageID:
				pg = transaction.NewTransactionsPage(mp.Load)
			case staking.OverviewPageID:
				pg = staking.NewStakingPage(mp.Load)
			case info.InfoID:
				pg = info.NewInfoPage(mp.Load)
			case WalletSettingsPageID:
				pg = NewWalletSettingsPage(mp.Load)
			}

			if pg == nil || mp.ID() == mp.CurrentPageID() {
				continue
			}

			// clear stack
			mp.Display(pg)
		}
	}

	for i, item := range mp.floatingActionButton.FloatingActionButton {
		for item.Clickable.Clicked() {
			var pg app.Page
			if i == 0 {
				pg = send.NewSendPage(mp.Load)
			} else {
				pg = NewReceivePage(mp.Load)
			}

			if mp.ID() == mp.CurrentPageID() {
				continue
			}

			if mp.WL.MultiWallet.IsSynced() {
				mp.Display(pg)
			} else if mp.WL.MultiWallet.IsSyncing() {
				errModal := modal.NewErrorModal(mp.Load, values.String(values.StrWalletSyncing), modal.DefaultClickFunc())
				mp.ParentWindow().ShowModal(errModal)
			} else {
				errModal := modal.NewErrorModal(mp.Load, values.String(values.StrNotConnected), modal.DefaultClickFunc())
				mp.ParentWindow().ShowModal(errModal)
			}
		}
	}

	mp.isBalanceHidden = mp.WL.MultiWallet.ReadBoolConfigValueForKey(load.HideBalanceConfigKey, false)
	for mp.hideBalanceButton.Clicked() {
		mp.isBalanceHidden = !mp.isBalanceHidden
		mp.WL.MultiWallet.SetBoolConfigValueForKey(load.HideBalanceConfigKey, mp.isBalanceHidden)
	}
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this page wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (mp *MainPage) KeysToHandle() key.Set {
	if currentPage := mp.CurrentPage(); currentPage != nil {
		if keyEvtHandler, ok := currentPage.(load.KeyEventHandler); ok {
			return keyEvtHandler.KeysToHandle()
		}
	}
	return ""
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (mp *MainPage) HandleKeyPress(evt *key.Event) {
	if currentPage := mp.CurrentPage(); currentPage != nil {
		if keyEvtHandler, ok := currentPage.(load.KeyEventHandler); ok {
			keyEvtHandler.HandleKeyPress(evt)
		}
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (mp *MainPage) OnNavigatedFrom() {
	// Also disappear all child pages.
	if mp.CurrentPage() != nil {
		mp.CurrentPage().OnNavigatedFrom()
	}

	mp.WL.SelectedWallet.Wallet.SaveUserConfigValue(load.SeedBackupNotificationConfigKey, false)
	mp.ctxCancel()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (mp *MainPage) Layout(gtx C) D {
	mp.Load.SetCurrentAppWidth(gtx.Constraints.Max.X)
	if mp.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return mp.layoutMobile(gtx)
	}
	return mp.layoutDesktop(gtx)
}

func (mp *MainPage) layoutDesktop(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.MatchParent,
				Orientation: layout.Vertical,
			}.Layout(gtx,
				layout.Rigid(mp.LayoutTopBar),
				layout.Rigid(func(gtx C) D {
					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.MatchParent,
						Height:      cryptomaterial.MatchParent,
						Orientation: layout.Horizontal,
					}.Layout(gtx,
						layout.Rigid(mp.drawerNav.LayoutNavDrawer),
						layout.Rigid(func(gtx C) D {
							if mp.CurrentPage() == nil {
								return D{}
							}
							return mp.CurrentPage().Layout(gtx)
						}),
					)
				}),
			)
		}),
	)
}

func (mp *MainPage) layoutMobile(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(mp.LayoutTopBar),
		layout.Flexed(1, func(gtx C) D {
			return layout.Stack{Alignment: layout.N}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					currentPage := mp.CurrentPage()
					if currentPage == nil {
						return D{}
					}
					return currentPage.Layout(gtx)
				}),
				layout.Stacked(func(gtx C) D {
					return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, mp.floatingActionButton.LayoutSendReceive)
				}),
			)
		}),
		layout.Rigid(mp.bottomNavigationBar.LayoutBottomNavigationBar),
	)
}

func (mp *MainPage) LayoutUSDBalance(gtx C) D {
	if !mp.usdExchangeSet {
		return D{}
	}
	switch {
	case mp.isFetchingExchangeRate && mp.dcrUsdtBittrex.LastTradeRate == "":
		gtx.Constraints.Max.Y = gtx.Dp(values.MarginPadding18)
		gtx.Constraints.Max.X = gtx.Constraints.Max.Y
		return layout.Inset{
			Top:  values.MarginPadding8,
			Left: values.MarginPadding5,
		}.Layout(gtx, func(gtx C) D {
			loader := material.Loader(mp.Theme.Base)
			return loader.Layout(gtx)
		})
	case !mp.isFetchingExchangeRate && mp.dcrUsdtBittrex.LastTradeRate == "":
		return layout.Inset{
			Top:  values.MarginPadding7,
			Left: values.MarginPadding5,
		}.Layout(gtx, func(gtx C) D {
			return mp.refreshExchangeRateBtn.Layout(gtx, func(gtx C) D {
				return mp.Theme.Icons.Restore.Layout16dp(gtx)
			})
		})
	case len(mp.totalBalanceUSD) > 0:
		lbl := mp.Theme.Label(values.TextSize20, fmt.Sprintf("/ %s", mp.totalBalanceUSD))
		lbl.Color = mp.Theme.Color.PageNavText
		inset := layout.Inset{Left: values.MarginPadding8}
		return inset.Layout(gtx, lbl.Layout)
	default:
		return D{}
	}
}

func (mp *MainPage) totalDCRBalance(gtx C) D {
	if mp.isBalanceHidden {
		hiddenBalanceText := mp.Theme.Label(values.TextSize18*0.8, "*******************")
		return layout.Inset{Bottom: values.MarginPadding0, Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
			hiddenBalanceText.Color = mp.Theme.Color.PageNavText
			return hiddenBalanceText.Layout(gtx)
		})
	}
	return components.LayoutBalanceWithUnit(gtx, mp.Load, mp.totalBalance.String())
}

func (mp *MainPage) LayoutTopBar(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Background:  mp.Theme.Color.Surface,
		Orientation: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			h := values.MarginPadding24
			v := values.MarginPadding8
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Horizontal,
				Alignment:   layout.Middle,
				Padding: layout.Inset{
					Right:  h,
					Left:   values.MarginPadding10,
					Top:    v,
					Bottom: v,
				},
			}.GradientLayout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.W.Layout(gtx, func(gtx C) D {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Orientation: layout.Horizontal,
							Alignment:   layout.Middle,
							Clickable:   mp.openWalletSelector,
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Left:  values.MarginPadding12,
									Right: values.MarginPadding24,
								}.Layout(gtx, func(gtx C) D {
									return mp.Theme.Icons.ChevronLeft.LayoutSize(gtx, values.MarginPadding12)
								})
							}),
							layout.Rigid(func(gtx C) D {
								if mp.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
									return mp.Theme.Icons.DcrWatchOnly.Layout24dp(gtx)
								}
								return mp.Theme.Icons.DecredSymbol2.Layout24dp(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								lbl := mp.Theme.H6(mp.WL.SelectedWallet.Wallet.Name)
								lbl.Color = mp.Theme.Color.PageNavText
								return layout.Inset{
									Left: values.MarginPadding10,
								}.Layout(gtx, lbl.Layout)
							}),
						)
					})
				}),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.E.Layout(gtx, func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								icon := mp.Theme.Icons.RevealIcon
								if mp.isBalanceHidden {
									icon = mp.Theme.Icons.ConcealIcon
								}
								return layout.Inset{
									Top:   values.MarginPadding5,
									Right: values.MarginPadding9,
								}.Layout(gtx, func(gtx C) D {
									return mp.hideBalanceButton.Layout(gtx, icon.Layout16dp)
								})
							}),
							layout.Rigid(func(gtx C) D {
								return mp.totalDCRBalance(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								if !mp.isBalanceHidden {
									return mp.LayoutUSDBalance(gtx)
								}
								return D{}
							}),
						)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return mp.Theme.Separator().Layout(gtx)
		}),
	)
}

// postDdesktopNotification posts notifications to the desktop.
func (mp *MainPage) postDesktopNotification(notifier interface{}) {
	var notification string
	switch t := notifier.(type) {
	case wallet.NewTransaction:

		switch t.Transaction.Type {
		case libwallet.TxTypeRegular:
			if t.Transaction.Direction != libwallet.TxDirectionReceived {
				return
			}
			// remove trailing zeros from amount and convert to string
			amount := strconv.FormatFloat(libwallet.AmountCoin(t.Transaction.Amount), 'f', -1, 64)
			notification = values.StringF(values.StrDcrReceived, amount)
		case libwallet.TxTypeVote:
			reward := strconv.FormatFloat(libwallet.AmountCoin(t.Transaction.VoteReward), 'f', -1, 64)
			notification = values.StringF(values.StrTicektVoted, reward)
		case libwallet.TxTypeRevocation:
			notification = values.String(values.StrTicketRevoked)
		default:
			return
		}

		if mp.WL.MultiWallet.OpenedWalletsCount() > 1 {
			wallet := mp.WL.MultiWallet.WalletWithID(t.Transaction.WalletID)
			if wallet == nil {
				return
			}

			notification = fmt.Sprintf("[%s] %s", wallet.Name, notification)
		}

		initializeBeepNotification(notification)
	case wallet.Proposal:
		proposalNotification := mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(load.ProposalNotificationConfigKey, false)
		if !proposalNotification {
			return
		}
		switch {
		case t.ProposalStatus == wallet.NewProposalFound:
			notification = values.StringF(values.StrProposalAddedNotif, t.Proposal.Name)
		case t.ProposalStatus == wallet.VoteStarted:
			notification = values.StringF(values.StrVoteStartedNotif, t.Proposal.Name)
		case t.ProposalStatus == wallet.VoteFinished:
			notification = values.StringF(values.StrVoteEndedNotif, t.Proposal.Name)
		default:
			notification = values.StringF(values.StrNewProposalUpdate, t.Proposal.Name)
		}
		initializeBeepNotification(notification)
	}
}

func initializeBeepNotification(n string) {
	absoluteWdPath, err := components.GetAbsolutePath()
	if err != nil {
		log.Error(err.Error())
	}

	err = beeep.Notify("Cryptopower Wallet", n, filepath.Join(absoluteWdPath, "ui/assets/decredicons/qrcodeSymbol.png"))
	if err != nil {
		log.Info("could not initiate desktop notification, reason:", err.Error())
	}
}

// listenForNotifications starts a goroutine to watch for notifications
// and update the UI accordingly.
func (mp *MainPage) listenForNotifications() {
	// Return if any of the listener is not nil.
	switch {
	case mp.SyncProgressListener != nil:
		return
	case mp.TxAndBlockNotificationListener != nil:
		return
	case mp.ProposalNotificationListener != nil:
		return
	}

	mp.SyncProgressListener = listeners.NewSyncProgress()
	err := mp.WL.MultiWallet.AddSyncProgressListener(mp.SyncProgressListener, MainPageID)
	if err != nil {
		log.Errorf("Error adding sync progress listener: %v", err)
		return
	}

	mp.TxAndBlockNotificationListener = listeners.NewTxAndBlockNotificationListener()
	err = mp.WL.MultiWallet.AddTxAndBlockNotificationListener(mp.TxAndBlockNotificationListener, true, MainPageID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	mp.ProposalNotificationListener = listeners.NewProposalNotificationListener()
	err = mp.WL.MultiWallet.Politeia.AddNotificationListener(mp.ProposalNotificationListener, MainPageID)
	if err != nil {
		log.Errorf("Error adding politeia notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-mp.TxAndBlockNotifChan:
				switch n.Type {
				case listeners.NewTransaction:
					mp.updateBalance()
					transactionNotification := mp.WL.MultiWallet.ReadBoolConfigValueForKey(load.TransactionNotificationConfigKey, false)
					if transactionNotification {
						update := wallet.NewTransaction{
							Transaction: n.Transaction,
						}
						mp.postDesktopNotification(update)
					}
					mp.ParentWindow().Reload()
				case listeners.BlockAttached:
					beep := mp.WL.MultiWallet.ReadBoolConfigValueForKey(libwallet.BeepNewBlocksConfigKey, false)
					if beep {
						err := beeep.Beep(5, 1)
						if err != nil {
							log.Error(err.Error)
						}
					}

					mp.updateBalance()
					mp.ParentWindow().Reload()
				case listeners.TxConfirmed:
					mp.updateBalance()
					mp.ParentWindow().Reload()

				}
			case notification := <-mp.ProposalNotifChan:
				// Post desktop notification for all events except the synced event.
				if notification.ProposalStatus != wallet.Synced {
					mp.postDesktopNotification(notification)
				}
			case n := <-mp.SyncStatusChan:
				if n.Stage == wallet.SyncCompleted {
					mp.updateBalance()
					mp.ParentWindow().Reload()
				}
			case <-mp.ctx.Done():
				mp.WL.MultiWallet.RemoveSyncProgressListener(MainPageID)
				mp.WL.MultiWallet.RemoveTxAndBlockNotificationListener(MainPageID)
				mp.WL.MultiWallet.Politeia.RemoveNotificationListener(MainPageID)

				close(mp.SyncStatusChan)
				close(mp.TxAndBlockNotifChan)
				close(mp.ProposalNotifChan)

				mp.SyncProgressListener = nil
				mp.TxAndBlockNotificationListener = nil
				mp.ProposalNotificationListener = nil

				return
			}
		}
	}()
}

func (mp *MainPage) showBackupInfo() {
	backupNowOrLaterModal := modal.NewCustomModal(mp.Load).
		SetupWithTemplate(modal.WalletBackupInfoTemplate).
		SetCancelable(false).
		SetContentAlignment(layout.W, layout.Center).
		CheckBox(mp.checkBox, true).
		SetNegativeButtonText(values.String(values.StrBackupLater)).
		SetNegativeButtonCallback(func() {
			mp.WL.SelectedWallet.Wallet.SaveUserConfigValue(load.SeedBackupNotificationConfigKey, true)
		}).
		PositiveButtonStyle(mp.Load.Theme.Color.Primary, mp.Load.Theme.Color.InvText).
		SetPositiveButtonText(values.String(values.StrBackupNow)).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			mp.WL.SelectedWallet.Wallet.SaveUserConfigValue(load.SeedBackupNotificationConfigKey, true)
			mp.ParentNavigator().Display(seedbackup.NewBackupInstructionsPage(mp.Load, mp.WL.SelectedWallet.Wallet))
			return true
		})
	mp.ParentWindow().ShowModal(backupNowOrLaterModal)
}
