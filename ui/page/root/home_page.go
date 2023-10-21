package root

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/governance"
	"github.com/crypto-power/cryptopower/ui/page/send"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/page/transaction"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	HomePageID = "Home"
)

type HomePage struct {
	*app.MasterPage

	*load.Load

	ctx       context.Context
	ctxCancel context.CancelFunc
	drawerNav components.NavDrawer

	navigationTab          *cryptomaterial.Tab
	appLevelSettingsButton *cryptomaterial.Clickable
	appNotificationButton  *cryptomaterial.Clickable
	hideBalanceButton      *cryptomaterial.Clickable
	infoButton             cryptomaterial.IconButton // TOD0: use *cryptomaterial.Clickable

	walletSelectorPage *WalletSelectorPage

	bottomNavigationBar  components.BottomNavigationBar
	floatingActionButton components.BottomNavigationBar

	// page state variables
	isBalanceHidden,
	isHiddenNavigation bool

	isConnected        *atomic.Bool
	showNavigationFunc showNavigationFunc
	startSpvSync       uint32

	totalBalanceUSD string
}

var navigationTabTitles = []string{
	values.String(values.StrOverview),
	values.String(values.StrTransactions),
	values.String(values.StrWallets),
	values.String(values.StrTrade),
	values.String(values.StrGovernance),
}

func NewHomePage(l *load.Load) *HomePage {
	hp := &HomePage{
		Load:        l,
		MasterPage:  app.NewMasterPage(HomePageID),
		isConnected: new(atomic.Bool),
	}

	hp.hideBalanceButton = hp.Theme.NewClickable(false)
	hp.appLevelSettingsButton = hp.Theme.NewClickable(false)
	hp.appNotificationButton = hp.Theme.NewClickable(false)

	hp.navigationTab = l.Theme.Tab(layout.Horizontal, false, navigationTabTitles)

	_, hp.infoButton = components.SubpageHeaderButtons(l)
	hp.infoButton.Size = values.MarginPadding15

	hp.drawerNav = components.NavDrawer{
		Load:        hp.Load,
		CurrentPage: hp.CurrentPageID(),
		AppNavBarItems: []components.NavHandler{
			{
				Clickable: hp.Theme.NewClickable(true),
				Image:     hp.Theme.Icons.SendIcon,
				Title:     values.String(values.StrSend),
				PageID:    send.SendPageID,
			},
			{
				Clickable: hp.Theme.NewClickable(true),
				Image:     hp.Theme.Icons.ReceiveIcon,
				Title:     values.String(values.StrReceive),
				PageID:    ReceivePageID,
			},
		},
	}

	go func() {
		hp.isConnected.Store(libutils.IsOnline())
	}()

	// init shared page functions
	toggleSync := func(unlock load.NeedUnlockRestore) {
		if hp.WL.SelectedWallet.Wallet.IsConnectedToNetwork() {
			go hp.WL.SelectedWallet.Wallet.CancelSync()
			unlock(false)
		} else {
			hp.startSyncing(hp.WL.SelectedWallet.Wallet, unlock)
		}
	}
	l.ToggleSync = toggleSync

	// initialize wallet page
	hp.walletSelectorPage = NewWalletSelectorPage(l)
	hp.showNavigationFunc = func(isHiddenNavigation bool) {
		hp.isHiddenNavigation = isHiddenNavigation
	}
	hp.walletSelectorPage.showNavigationFunc = hp.showNavigationFunc

	hp.initBottomNavItems()
	hp.bottomNavigationBar.OnViewCreated()

	return hp
}

// ID is a unique string that identifies the page and may be used
// to differentiate this page from other pages.
// Part of the load.Page interface.
func (hp *HomePage) ID() string {
	return HomePageID
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (hp *HomePage) OnNavigatedTo() {
	hp.ctx, hp.ctxCancel = context.WithCancel(context.TODO())

	go hp.CalculateAssetsUSDBalance()

	if hp.CurrentPage() == nil {
		hp.Display(NewOverviewPage(hp.Load, hp.showNavigationFunc))
	}

	// Initiate the auto sync for all the DCR wallets with set autosync.
	for _, wallet := range hp.WL.SortedWalletList(libutils.DCRWalletAsset) {
		if wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false) {
			hp.startSyncing(wallet, func(isUnlock bool) {})
		}
	}

	// Initiate the auto sync for all the BTC wallets with set autosync.
	for _, wallet := range hp.WL.SortedWalletList(libutils.BTCWalletAsset) {
		if wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false) {
			hp.startSyncing(wallet, func(isUnlock bool) {})
		}
	}

	// Initiate the auto sync for all the LTC wallets with set autosync.
	for _, wallet := range hp.WL.SortedWalletList(libutils.LTCWalletAsset) {
		if wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false) {
			hp.startSyncing(wallet, func(isUnlock bool) {})
		}
	}

	hp.isBalanceHidden = hp.WL.AssetsManager.IsTotalBalanceVisible()
}

// OnDarkModeChanged is triggered whenever the dark mode setting is changed
// to enable restyling UI elements where necessary.
// Satisfies the load.AppSettingsChangeHandler interface.
func (hp *HomePage) OnDarkModeChanged(isDarkModeOn bool) {
	// TODO: currentPage will likely be the Settings page when this method
	// is called. If that page implements the AppSettingsChangeHandler interface,
	// the following code will trigger the OnDarkModeChanged method of that
	// page.
	if currentPage, ok := hp.CurrentPage().(load.AppSettingsChangeHandler); ok {
		currentPage.OnDarkModeChanged(isDarkModeOn)
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (hp *HomePage) HandleUserInteractions() {
	if hp.CurrentPage() != nil {
		hp.CurrentPage().HandleUserInteractions()
	}

	if hp.navigationTab.Changed() {
		var pg app.Page
		switch hp.navigationTab.SelectedTab() {
		case values.String(values.StrOverview):
			pg = NewOverviewPage(hp.Load, hp.showNavigationFunc)
		case values.String(values.StrTransactions):
			pg = transaction.NewTransactionsPage(hp.Load, true)
		case values.String(values.StrWallets):
			pg = hp.walletSelectorPage
		case values.String(values.StrTrade):
			pg = NewTradePage(hp.Load)
		case values.String(values.StrGovernance):
			pg = governance.NewGovernancePage(hp.Load)
		}

		hp.Display(pg)
	}

	// set the page to the active nav, especially when navigating from over pages
	// like the overview page slider.
	if hp.CurrentPageID() == WalletSelectorPageID && hp.navigationTab.SelectedTab() != values.String(values.StrWallets) {
		hp.navigationTab.SetSelectedTab(values.String(values.StrWallets))
	} else if hp.CurrentPageID() == TradePageID && hp.navigationTab.SelectedTab() != values.String(values.StrTrade) {
		hp.navigationTab.SetSelectedTab(values.String(values.StrTrade))
	}

	for _, item := range hp.drawerNav.AppNavBarItems {
		for item.Clickable.Clicked() {
			switch strings.ToLower(item.PageID) {
			case values.StrReceive:
				hp.ParentWindow().ShowModal(components.NewReceiveModal(hp.Load))
			case values.StrSend:
				allWallets := hp.WL.AssetsManager.AllWallets()
				isSendAvailable := false
				for _, wallet := range allWallets {
					if !wallet.IsWatchingOnlyWallet() {
						isSendAvailable = true
					}
				}
				if !isSendAvailable {
					hp.showWarningNoWallet()
					return
				}
				hp.ParentWindow().ShowModal(send.NewSendPage(hp.Load, true))
			}
		}
	}

	if hp.infoButton.Button.Clicked() {
		infoModal := modal.NewCustomModal(hp.Load).
			Title(values.String(values.StrTotalValue)).
			SetupWithTemplate(modal.TotalValueInfoTemplate).
			SetCancelable(true).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetPositiveButtonText(values.String(values.StrOk))
		hp.ParentWindow().ShowModal(infoModal)
	}

	if hp.appNotificationButton.Clicked() {
		// TODO: Use real values as these are dummy so lint will pass
		hp.ParentNavigator().Display(settings.NewSettingsPage(hp.Load))
	}

	for hp.appLevelSettingsButton.Clicked() {
		hp.ParentNavigator().Display(settings.NewSettingsPage(hp.Load))
	}

	for hp.hideBalanceButton.Clicked() {
		hp.isBalanceHidden = !hp.isBalanceHidden
		hp.WL.AssetsManager.SetTotalBalanceVisibility(hp.isBalanceHidden)
	}

	hp.bottomNavigationBar.CurrentPage = hp.CurrentPageID()
	hp.floatingActionButton.CurrentPage = hp.CurrentPageID()
	for _, item := range hp.bottomNavigationBar.BottomNaigationItems {
		for item.Clickable.Clicked() {
			var pg app.Page
			switch item.Title {
			case values.String(values.StrOverview):
				pg = NewOverviewPage(hp.Load, hp.showNavigationFunc)
			case values.String(values.StrWallets):
				pg = hp.walletSelectorPage
			case values.String(values.StrTrade):
				pg = NewTradePage(hp.Load)
			}

			if pg == nil || hp.ID() == hp.CurrentPageID() {
				continue
			}

			// clear stack
			hp.Display(pg)
		}
	}

	for _, item := range hp.floatingActionButton.FloatingActionButton {
		for item.Clickable.Clicked() {
			if strings.ToLower(item.PageID) == values.StrReceive {
				receiveModal := components.NewReceiveModal(hp.Load)
				hp.ParentWindow().ShowModal(receiveModal)
			}
		}
	}
}

func (hp *HomePage) showWarningNoWallet() {
	go func() {
		info := modal.NewCustomModal(hp.Load).
			PositiveButtonStyle(hp.Theme.Color.Primary, hp.Theme.Color.Surface).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			Body(values.String(values.StrNoWalletsAvailable))
		hp.ParentWindow().ShowModal(info)
	}()
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this page wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (hp *HomePage) KeysToHandle() key.Set {
	if currentPage := hp.CurrentPage(); currentPage != nil {
		if keyEvtHandler, ok := currentPage.(load.KeyEventHandler); ok {
			return keyEvtHandler.KeysToHandle()
		}
	}
	return ""
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (hp *HomePage) HandleKeyPress(evt *key.Event) {
	if currentPage := hp.CurrentPage(); currentPage != nil {
		if keyEvtHandler, ok := currentPage.(load.KeyEventHandler); ok {
			keyEvtHandler.HandleKeyPress(evt)
		}
	}
}

func (hp *HomePage) OnCurrencyChanged() {
	go hp.CalculateAssetsUSDBalance()
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (hp *HomePage) OnNavigatedFrom() {
	// Also remove all child pages.
	if activeTab := hp.CurrentPage(); activeTab != nil {
		activeTab.OnNavigatedFrom()
	}

	hp.ctxCancel()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (hp *HomePage) Layout(gtx C) D {
	hp.Load.SetCurrentAppWidth(gtx.Constraints.Max.X)
	if hp.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return hp.layoutMobile(gtx)
	}
	return hp.layoutDesktop(gtx)
}

func (hp *HomePage) layoutDesktop(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.MatchParent,
				Orientation: layout.Vertical,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if hp.isHiddenNavigation {
						return D{}
					}
					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.MatchParent,
						Height:      cryptomaterial.WrapContent,
						Orientation: layout.Vertical,
					}.Layout(gtx,
						layout.Rigid(hp.LayoutTopBar),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Left: values.MarginPadding20,
							}.Layout(gtx, hp.navigationTab.Layout)
						}),
						layout.Rigid(hp.Theme.Separator().Layout),
					)
				}),
				layout.Flexed(1, hp.CurrentPage().Layout),
			)
		}),
	)
}

func (hp *HomePage) layoutMobile(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(hp.LayoutTopBar),
		layout.Rigid(hp.Theme.Separator().Layout),
		layout.Flexed(1, func(gtx C) D {
			return layout.Stack{Alignment: layout.N}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					currentPage := hp.CurrentPage()
					if currentPage == nil {
						return D{}
					}
					return hp.CurrentPage().Layout(gtx)
				}),
			)
		}),
		layout.Rigid(hp.bottomNavigationBar.LayoutBottomNavigationBar),
	)
}

func (hp *HomePage) LayoutTopBar(gtx C) D {
	padding20 := values.MarginPadding20
	padding10 := values.MarginPadding10

	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
		Padding: layout.Inset{
			Right:  padding20,
			Left:   padding20,
			Top:    padding10,
			Bottom: padding10,
		},
	}.Layout(gtx,
		layout.Rigid(hp.totalBalanceLayout),
		layout.Rigid(func(gtx C) D {
			if hp.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
				return D{}
			}

			return hp.notificationSettingsLayout(gtx)
		}),
	)
}

func (hp *HomePage) initBottomNavItems() {
	hp.bottomNavigationBar = components.BottomNavigationBar{
		Load:        hp.Load,
		CurrentPage: hp.CurrentPageID(),
		BottomNaigationItems: []components.BottomNavigationBarHandler{
			{
				Clickable:     hp.Theme.NewClickable(true),
				Image:         hp.Theme.Icons.OverviewIcon,
				ImageInactive: hp.Theme.Icons.OverviewIconInactive,
				Title:         values.String(values.StrOverview),
				PageID:        OverviewPageID,
			},
			{
				Clickable:     hp.Theme.NewClickable(true),
				Image:         hp.Theme.Icons.WalletIcon,
				ImageInactive: hp.Theme.Icons.WalletIconInactive,
				Title:         values.String(values.StrWallets),
				PageID:        WalletSelectorPageID,
			},
			{
				Clickable:     hp.Theme.NewClickable(true),
				Image:         hp.Theme.Icons.TradeIconActive,
				ImageInactive: hp.Theme.Icons.TradeIconInactive,
				Title:         values.String(values.StrTrade),
				PageID:        TradePageID,
			},
			{
				Clickable:     hp.Theme.NewClickable(true),
				Image:         hp.Theme.Icons.GovernanceActiveIcon,
				ImageInactive: hp.Theme.Icons.GovernanceInactiveIcon,
				Title:         values.String(values.StrGovernance),
				PageID:        governance.GovernancePageID,
			},
		},
	}

	hp.floatingActionButton = components.BottomNavigationBar{
		Load:        hp.Load,
		CurrentPage: hp.CurrentPageID(),
		FloatingActionButton: []components.BottomNavigationBarHandler{
			{
				Clickable: hp.Theme.NewClickable(true),
				Image:     hp.Theme.Icons.SendIcon,
				Title:     values.String(values.StrSend),
				PageID:    send.SendPageID,
			},
			{
				Clickable: hp.Theme.NewClickable(true),
				Image:     hp.Theme.Icons.ReceiveIcon,
				Title:     values.String(values.StrReceive),
				PageID:    ReceivePageID,
			},
		},
	}
	hp.floatingActionButton.FloatingActionButton[0].Clickable.Hoverable = false
	hp.floatingActionButton.FloatingActionButton[1].Clickable.Hoverable = false
}

func (hp *HomePage) totalBalanceLayout(gtx C) D {
	return layout.W.Layout(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.WrapContent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if hp.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(hp.totalBalanceTextAndIconButtonLayout),
						layout.Rigid(hp.notificationSettingsLayout),
					)
				}
				return hp.totalBalanceTextAndIconButtonLayout(gtx)
			}),
			layout.Rigid(hp.balanceLayout),
			layout.Rigid(func(gtx C) D {
				if hp.Load.GetCurrentAppWidth() > gtx.Dp(values.StartMobileView) {
					return D{}
				}
				card := hp.Theme.Card()
				radius := cryptomaterial.CornerRadius{TopLeft: 20, BottomLeft: 20, TopRight: 20, BottomRight: 20}
				card.Radius = cryptomaterial.Radius(8)
				card.Color = hp.Theme.Color.Gray2
				padding8 := values.MarginPadding8
				padding16 := values.MarginPadding16
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
							return card.Layout(gtx, func(gtx C) D {
								return cryptomaterial.LinearLayout{
									Width:       cryptomaterial.WrapContent,
									Height:      cryptomaterial.WrapContent,
									Orientation: layout.Horizontal,
									Clickable:   hp.floatingActionButton.FloatingActionButton[0].Clickable,
									Alignment:   layout.Middle,
									Border: cryptomaterial.Border{
										Radius: radius,
									},
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return layout.Inset{
											Top:    padding8,
											Bottom: padding8,
											Left:   padding16,
											Right:  padding8,
										}.Layout(gtx, func(gtx C) D {
											return layout.Center.Layout(gtx, hp.floatingActionButton.FloatingActionButton[0].Image.Layout16dp)
										})
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{
											Top:    padding8,
											Bottom: padding8,
											Right:  padding16,
											Left:   values.MarginPadding0,
										}.Layout(gtx, func(gtx C) D {
											return layout.Center.Layout(gtx, hp.Theme.Body2(hp.floatingActionButton.FloatingActionButton[0].Title).Layout)
										})
									}),
								)
							})
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
							return card.Layout(gtx, func(gtx C) D {
								return cryptomaterial.LinearLayout{
									Width:       cryptomaterial.WrapContent,
									Height:      cryptomaterial.WrapContent,
									Orientation: layout.Horizontal,
									Clickable:   hp.floatingActionButton.FloatingActionButton[1].Clickable,
									Alignment:   layout.Middle,
									Border: cryptomaterial.Border{
										Radius: radius,
									},
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return layout.Inset{
											Top:    padding8,
											Bottom: padding8,
											Left:   padding8,
											Right:  padding8,
										}.Layout(gtx, func(gtx C) D {
											return layout.Center.Layout(gtx, hp.floatingActionButton.FloatingActionButton[1].Image.Layout16dp)
										})
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{
											Top:    padding8,
											Bottom: padding8,
											Right:  padding8,
											Left:   values.MarginPadding0,
										}.Layout(gtx, func(gtx C) D {
											return layout.Center.Layout(gtx, hp.Theme.Body2(hp.floatingActionButton.FloatingActionButton[1].Title).Layout)
										})
									}),
								)
							})
						})
					}),
				)
			}),
		)
	})
}

func (hp *HomePage) balanceLayout(gtx C) D {
	if components.IsFetchExchangeRateAPIAllowed(hp.WL) && hp.totalBalanceUSD != "" {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(hp.LayoutUSDBalance),
			layout.Rigid(func(gtx C) D {
				icon := hp.Theme.Icons.RevealIcon
				if hp.isBalanceHidden {
					icon = hp.Theme.Icons.ConcealIcon
				}
				return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
					return hp.hideBalanceButton.Layout(gtx, icon.Layout20dp)
				})
			}),
		)
	}

	lblText := hp.Theme.Label(values.TextSize30, "--")
	return lblText.Layout(gtx)
}

// TODO: use real values
func (hp *HomePage) LayoutUSDBalance(gtx C) D {
	lblText := hp.Theme.Label(values.TextSize30, hp.totalBalanceUSD)

	if hp.isBalanceHidden {
		lblText = hp.Theme.Label(values.TextSize24, "******")
	}
	inset := layout.Inset{Right: values.MarginPadding8}
	return inset.Layout(gtx, lblText.Layout)
}

func (hp *HomePage) totalBalanceTextAndIconButtonLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := hp.Theme.Label(values.TextSize14, values.String(values.StrTotalValue))
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Left:  values.MarginPadding5,
				Right: values.MarginPadding10,
			}.Layout(gtx, hp.infoButton.Layout)
		}),
	)
}

func (hp *HomePage) notificationSettingsLayout(gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.WrapContent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Horizontal,
					Alignment:   layout.Middle,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if hp.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
							return D{}
						}
						return hp.drawerNav.LayoutTopBar(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Left:  values.MarginPadding10,
							Right: values.MarginPadding10,
						}.Layout(gtx, func(gtx C) D {
							return hp.appNotificationButton.Layout(gtx, hp.Theme.Icons.Notification.Layout20dp)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return hp.appLevelSettingsButton.Layout(gtx, hp.Theme.Icons.SettingsIcon.Layout20dp)
					}),
				)
			})
		}),
	)
}

func (hp *HomePage) startSyncing(wallet sharedW.Asset, unlock load.NeedUnlockRestore) {
	// Watchonly wallets do not have any password neither need one.
	if !wallet.ContainsDiscoveredAccounts() && wallet.IsLocked() && !wallet.IsWatchingOnlyWallet() {
		hp.unlockWalletForSyncing(wallet, unlock)
		return
	}
	unlock(true)

	if hp.isConnected.Load() {
		// once network connection has been established proceed to
		// start the wallet sync.
		if err := wallet.SpvSync(); err != nil {
			log.Debugf("Error starting sync: %v", err)
		}
	}

	if !atomic.CompareAndSwapUint32(&hp.startSpvSync, 0, 1) {
		// internet connection checking goroutine is already running.
		return
	}

	// Since internet connectivity isn't available, the goroutine will keep
	// checking for the internet connectivity. on every 5th poll it will keep
	// increasing the wait duration by 5 seconds till the internet connectivity
	// is restored or the app is shutdown.
	go func() {
		count := 0
		duration := time.Second * 5

		for {
			select {
			case <-hp.ctx.Done():
				return
			case <-time.After(duration):
				if libutils.IsOnline() {
					log.Info("Internet connection has been established")
					// once network connection has been established proceed to
					// start the wallet sync.
					if err := wallet.SpvSync(); err != nil {
						log.Debugf("Error starting sync: %v", err)
					}

					if hp.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.ExchangeHTTPAPI) {
						hp.WL.AssetsManager.InstantSwap.StopSync() // in case it was syncing before losing network connection
						hp.WL.AssetsManager.InstantSwap.Sync()
					}

					// Trigger UI update
					hp.ParentWindow().Reload()

					hp.isConnected.Store(true)
					// Allow another goroutine to be spun up later on if need be.
					atomic.StoreUint32(&hp.startSpvSync, 0)
					return
				}

				// At the 5th ticker count, increase the duration interval
				// by 5 seconds up to thirty seconds.
				if count%5 == 0 && count > 0 && duration < time.Second*30 {
					duration += time.Second * 5
				}
				count++
				log.Debugf("Attempting to check for internet connection in %s", duration)
			}
		}
	}()
}

func (hp *HomePage) unlockWalletForSyncing(wal sharedW.Asset, unlock load.NeedUnlockRestore) {
	spendingPasswordModal := modal.NewCreatePasswordModal(hp.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrResumeAccountDiscoveryTitle)).
		PasswordHint(values.String(values.StrSpendingPassword)).
		SetPositiveButtonText(values.String(values.StrUnlock)).
		SetCancelable(false).
		SetNegativeButtonCallback(func() {
			unlock(false)
		}).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := wal.UnlockWallet(password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			unlock(true)
			pm.Dismiss()
			hp.startSyncing(wal, unlock)
			return true
		})
	hp.ParentWindow().ShowModal(spendingPasswordModal)
}

func (hp *HomePage) CalculateAssetsUSDBalance() {
	if components.IsFetchExchangeRateAPIAllowed(hp.WL) {
		assetsBalance, err := components.CalculateTotalAssetsBalance(hp.Load)
		if err != nil {
			log.Error(err)
			return
		}

		assetsTotalUSDBalance, err := components.CalculateAssetsUSDBalance(hp.Load, assetsBalance)
		if err != nil {
			log.Error(err)
			return
		}

		var totalBalance float64
		for _, balance := range assetsTotalUSDBalance {
			totalBalance += balance
		}

		hp.totalBalanceUSD = utils.FormatAsUSDString(hp.Printer, totalBalance)
		hp.ParentWindow().Reload()
	}
}
