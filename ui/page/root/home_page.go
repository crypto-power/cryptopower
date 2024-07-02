package root

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	dexdb "decred.org/dcrdex/client/db"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/appos"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/exchange"
	"github.com/crypto-power/cryptopower/ui/page/governance"
	"github.com/crypto-power/cryptopower/ui/page/receive"
	"github.com/crypto-power/cryptopower/ui/page/send"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/page/transaction"
	"github.com/crypto-power/cryptopower/ui/page/wallet"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	HomePageID = "Home"
)

var totalBalanceUSD string

type HomePage struct {
	*app.MasterPage

	*load.Load

	dexCtx context.Context

	ctx                 context.Context
	ctxCancel           context.CancelFunc
	sendReceiveNavItems []components.NavBarItem

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

	updateAvailableBtn *cryptomaterial.Clickable
	copyRedirectURL    *cryptomaterial.Clickable
	releaseResponse    *components.ReleaseResponse
}

func NewHomePage(dexCtx context.Context, l *load.Load) *HomePage {
	hp := &HomePage{
		Load:            l,
		MasterPage:      app.NewMasterPage(HomePageID),
		isConnected:     new(atomic.Bool),
		dexCtx:          dexCtx,
		copyRedirectURL: l.Theme.NewClickable(false),
	}

	hp.hideBalanceButton = hp.Theme.NewClickable(false)
	hp.appLevelSettingsButton = hp.Theme.NewClickable(false)
	hp.appNotificationButton = hp.Theme.NewClickable(false)
	_, hp.infoButton = components.SubpageHeaderButtons(l)
	hp.infoButton.Size = values.MarginPadding15
	hp.updateAvailableBtn = l.Theme.NewClickable(false)

	go func() {
		hp.isConnected.Store(libutils.IsOnline())
	}()

	// init shared page functions
	toggleSync := func(wallet sharedW.Asset, unlock load.NeedUnlockRestore) {
		if wallet.IsConnectedToNetwork() {
			go wallet.CancelSync()
			unlock(false)
		} else {
			hp.startSyncing(wallet, unlock)
		}
	}
	l.ToggleSync = toggleSync

	// initialize wallet page
	hp.walletSelectorPage = NewWalletSelectorPage(l)
	hp.showNavigationFunc = func(isHiddenNavigation bool) {
		hp.isHiddenNavigation = isHiddenNavigation
	}
	hp.walletSelectorPage.showNavigationFunc = hp.showNavigationFunc

	return hp
}

// initPageItems initializes navbar items that require the latest translation
// string and MUST be called from OnNavigatedTo.
func (hp *HomePage) initPageItems() {
	navigationTabTitles := []string{
		values.String(values.StrOverview),
		values.String(values.StrTransactions),
		values.String(values.StrWallets),
		values.String(values.StrGovernance),
	}

	// Add trade tab only if not on macOS
	if !appos.Current().IsDarwin() {
		// Determine the insertion point, which is second to last position
		insertionPoint := len(navigationTabTitles) - 1
		if insertionPoint < 0 {
			insertionPoint = 0
		}
		// Append at the second to last position
		navigationTabTitles = append(navigationTabTitles[:insertionPoint], append([]string{values.String(values.StrTrade)}, navigationTabTitles[insertionPoint:]...)...)
	}

	hp.navigationTab = hp.Theme.Tab(layout.Horizontal, false, navigationTabTitles)

	hp.sendReceiveNavItems = []components.NavBarItem{
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
			PageID:    receive.ReceivePageID,
		},
	}

	hp.initBottomNavItems()
	hp.bottomNavigationBar.OnViewCreated()
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
	hp.initPageItems()
	hp.initDEX()

	if hp.CurrentPage() == nil {
		hp.Display(NewOverviewPage(hp.Load, hp.showNavigationFunc))
	} else {
		hp.CurrentPage().OnNavigatedTo()
	}

	// Initiate the auto sync for all wallets with autosync set.
	allWallets := hp.AssetsManager.AllWallets()
	for _, wallet := range allWallets {
		if wallet.ReadBoolConfigValueForKey(sharedW.AutoSyncConfigKey, false) {
			hp.startSyncing(wallet, func(_ bool) {})
		}
	}

	if hp.AssetsManager.ExchangeRateFetchingEnabled() {
		go hp.CalculateAssetsUSDBalance()
	}
	hp.isBalanceHidden = hp.AssetsManager.IsTotalBalanceVisible()

	if hp.isUpdateAPIAllowed() {
		go hp.checkForUpdates()
	}
}

// initDEX initializes a new dex client if dex is not ready.
func (hp *HomePage) initDEX() {
	if hp.AssetsManager.DEXCInitialized() {
		return // do nothing
	}

	go func() {
		hp.AssetsManager.InitializeDEX(hp.dexCtx)

		// If all went well, the dex client must be ready.
		dexClient := hp.AssetsManager.DexClient()
		if dexClient == nil {
			return // nothing to do
		}

		// Wait until dex is ready
		<-dexClient.Ready()

		activeOrders, _, err := dexClient.ActiveOrders() // we just initialized dexc, no inflight order expected
		if err != nil {
			log.Errorf("dexClient.ActiveOrders error: %w", err)
		}

		var expiredBonds []*dexdb.Bond
		xcs := dexClient.Exchanges()
		for _, xc := range xcs {
			if len(xc.Auth.ExpiredBonds) == 0 {
				continue // nothing to do.
			}

			expiredBonds = append(expiredBonds, xc.Auth.ExpiredBonds...)
		}

		if len(activeOrders) == 0 && len(expiredBonds) == 0 {
			return // nothing to do
		}

		showDEXLoginModal := func() {
			dexPassEditor := hp.Theme.EditorPassword(new(widget.Editor), values.String(values.StrDexPassword))
			dexPassEditor.Editor.SingleLine, dexPassEditor.IsRequired = true, true

			loginModal := modal.NewCustomModal(hp.Load).
				UseCustomWidget(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, hp.Theme.Body1(values.String(values.StrLoginDEXForActiveOrdersOrExpiredBonds)).Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return dexPassEditor.Layout(gtx)
						}),
					)
				}).
				SetNegativeButtonText(values.String(values.StrIWillLoginLater)).
				SetPositiveButtonText(values.String(values.StrLogin)).
				SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
					dexPassEditor.SetError("")
					err := dexClient.Login([]byte(dexPassEditor.Editor.Text()))
					if err != nil {
						dexPassEditor.SetError(err.Error())
						return false
					}
					return true
				}).
				SetCancelable(false)
			hp.ParentWindow().ShowModal(loginModal)
		}

		// DEX client has active orders or expired bonds, retrieve the
		// wallets involved and ensure they are synced or syncing.
		walletsToSyncMap := make(map[uint32]*struct{})
		for _, orders := range activeOrders {
			for _, ord := range orders {
				walletsToSyncMap[ord.BaseID] = &struct{}{}
				walletsToSyncMap[ord.QuoteID] = &struct{}{}
			}
		}

		for _, bond := range expiredBonds {
			walletsToSyncMap[bond.AssetID] = &struct{}{}
		}

		var namesOfWalletsToSync []string
		var walletsToSync []sharedW.Asset
		for assetID := range walletsToSyncMap {
			walletID, err := dexClient.WalletIDForAsset(assetID)
			if err != nil {
				log.Errorf("dexClient.WalletIDForAsset(%d) error: %w", assetID, err)
				continue
			}

			if walletID == nil {
				continue // impossible but better safe than sorry
			}

			wallet := hp.AssetsManager.WalletWithID(*walletID)
			if wallet == nil { // impossible but better safe than sorry
				log.Error("dex wallet with ID %d is missing", walletID)
				continue
			}

			if wallet.IsSynced() || wallet.IsSyncing() {
				continue // ok
			}

			walletsToSync = append(walletsToSync, wallet)
			namesOfWalletsToSync = append(namesOfWalletsToSync, fmt.Sprintf("%s (%s)", wallet.GetWalletName(), wallet.GetAssetType()))
		}

		if len(walletsToSync) == 0 {
			showDEXLoginModal() // Request dex login now.
			return
		}

		walletSyncRequestModal := modal.NewCustomModal(hp.Load).
			Title(values.String(values.StrWalletsNeedToSync)).
			Body(values.StringF(values.StrWalletsNeedToSyncMsg, strings.Join(namesOfWalletsToSync, ", "))).
			SetNegativeButtonText(values.String(values.StrIWillSyncLater)).
			SetNegativeButtonCallback(showDEXLoginModal).
			SetPositiveButtonText(values.String(values.StrOkaySync)).
			SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
				if !hp.isConnected.Load() {
					hp.Toast.NotifyError(values.String(values.StrNotConnected))
				} else {
					for _, w := range walletsToSync {
						err := w.SpvSync()
						if err != nil {
							log.Error(err)
						}
					}
				}

				showDEXLoginModal()
				return true
			}).SetCancelable(false)
		hp.ParentWindow().ShowModal(walletSyncRequestModal)

	}()
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
func (hp *HomePage) HandleUserInteractions(gtx C) {
	if hp.CurrentPage() != nil {
		hp.CurrentPage().HandleUserInteractions(gtx)
	}

	if hp.navigationTab.Changed() {
		hp.displaySelectedPage(hp.navigationTab.SelectedTab())
	}

	// set the page to the active nav, especially when navigating from over pages
	// like the overview page slider.
	if hp.CurrentPageID() == WalletSelectorPageID && hp.navigationTab.SelectedTab() != values.String(values.StrWallets) {
		hp.navigationTab.SetSelectedTab(values.String(values.StrWallets))
	} else if hp.CurrentPageID() == exchange.TradePageID && hp.navigationTab.SelectedTab() != values.String(values.StrTrade) {
		hp.navigationTab.SetSelectedTab(values.String(values.StrTrade))
	} else if hp.CurrentPageID() == transaction.TransactionsPageID && hp.navigationTab.SelectedTab() != values.String(values.StrTransactions) {
		hp.navigationTab.SetSelectedTab(values.String(values.StrTransactions))
	} else if hp.CurrentPageID() == governance.GovernancePageID && hp.navigationTab.SelectedTab() != values.String(values.StrGovernance) {
		hp.navigationTab.SetSelectedTab(values.String(values.StrGovernance))
	}
	for i, item := range hp.sendReceiveNavItems {
		if item.Clickable.Clicked(gtx) {
			switch strings.ToLower(item.PageID) {
			case values.StrReceive:
				hp.ParentWindow().ShowModal(receive.NewReceivePage(hp.Load, nil))
			case values.StrSend:
				allWallets := hp.AssetsManager.AllWallets()
				isSendAvailable := false
				for _, wallet := range allWallets {
					if !wallet.IsWatchingOnlyWallet() {
						isSendAvailable = true
					}
				}
				if !isSendAvailable {
					hp.showWarningNoSpendableWallet()
					return
				}
				hp.ParentWindow().ShowModal(send.NewSendPage(hp.Load, nil))
			}

			// This line fix hover send and receive button
			hp.sendReceiveNavItems[i].Clickable = hp.Theme.NewClickable(true)
		}
	}

	if hp.infoButton.Button.Clicked(gtx) {
		infoModal := modal.NewCustomModal(hp.Load).
			Title(values.String(values.StrTotalValue)).
			SetupWithTemplate(modal.TotalValueInfoTemplate).
			SetCancelable(true).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetPositiveButtonText(values.String(values.StrOk))
		hp.ParentWindow().ShowModal(infoModal)
	}

	if hp.appNotificationButton.Clicked(gtx) {
		// TODO: Use real values as these are dummy so lint will pass
		hp.ParentNavigator().Display(settings.NewAppSettingsPage(hp.Load))
	}

	for hp.appLevelSettingsButton.Clicked(gtx) {
		hp.ParentNavigator().Display(settings.NewAppSettingsPage(hp.Load))
	}

	for hp.hideBalanceButton.Clicked(gtx) {
		hp.isBalanceHidden = !hp.isBalanceHidden
		hp.AssetsManager.SetTotalBalanceVisibility(hp.isBalanceHidden)
	}

	hp.bottomNavigationBar.CurrentPage = hp.CurrentPageID()
	hp.floatingActionButton.CurrentPage = hp.CurrentPageID()
	for _, item := range hp.bottomNavigationBar.BottomNavigationItems {
		if item.Clickable.Clicked(gtx) {
			if hp.ID() == hp.CurrentPageID() {
				continue
			}
			hp.displaySelectedPage(item.Title)
		}
	}

	for _, item := range hp.floatingActionButton.FloatingActionButton {
		if item.Clickable.Clicked(gtx) {
			if strings.ToLower(item.PageID) == values.StrReceive {
				hp.ParentWindow().ShowModal(receive.NewReceivePage(hp.Load, nil))
			}

			if strings.ToLower(item.PageID) == values.StrSend {
				hp.ParentWindow().ShowModal(send.NewSendPage(hp.Load, nil))
			}
		}
	}

	if hp.updateAvailableBtn.Clicked(gtx) {
		info := modal.NewCustomModal(hp.Load).
			Title(fmt.Sprintf(values.String(values.StrNewUpdateText), hp.releaseResponse.TagName)).
			Body(values.String(values.StrCopyLink)).
			SetCancelable(true).
			UseCustomWidget(func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx C) D {
						border := widget.Border{Color: hp.Theme.Color.Gray4, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
						wrapper := hp.Theme.Card()
						wrapper.Color = hp.Theme.Color.Gray4
						return border.Layout(gtx, func(gtx C) D {
							return wrapper.Layout(gtx, func(gtx C) D {
								return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
									return layout.Flex{}.Layout(gtx,
										layout.Flexed(0.9, hp.Theme.Body1(hp.releaseResponse.URL).Layout),
										layout.Flexed(0.1, func(gtx C) D {
											return layout.E.Layout(gtx, func(gtx C) D {
												if hp.copyRedirectURL.Clicked(gtx) {
													gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(hp.releaseResponse.URL))})
													hp.Toast.Notify(values.String(values.StrCopied))
												}
												return hp.copyRedirectURL.Layout(gtx, hp.Theme.NewIcon(hp.Theme.Icons.CopyIcon).Layout24dp)
											})
										}),
									)
								})
							})
						})
					}),
					layout.Stacked(func(gtx C) D {
						return layout.Inset{
							Top:  values.MarginPaddingMinus10,
							Left: values.MarginPadding10,
						}.Layout(gtx, func(gtx C) D {
							label := hp.Theme.Body2(values.String(values.StrWebURL))
							label.Color = hp.Theme.Color.GrayText2
							return label.Layout(gtx)
						})
					}),
				)
			}).
			SetPositiveButtonText(values.String(values.StrGotIt))
		hp.ParentWindow().ShowModal(info)
	}
}

func (hp *HomePage) displaySelectedPage(title string) {
	var pg app.Page
	switch title {
	case values.String(values.StrOverview):
		pg = NewOverviewPage(hp.Load, hp.showNavigationFunc)
	case values.String(values.StrTransactions):
		pg = transaction.NewTransactionsPage(hp.Load, nil)
	case values.String(values.StrWallets):
		pg = hp.walletSelectorPage
	case values.String(values.StrTrade):
		if !hp.AssetsManager.DEXCInitialized() {
			// Attempt to initialize dex again.
			hp.AssetsManager.InitializeDEX(hp.dexCtx)
		}
		pg = exchange.NewTradePage(hp.Load)
	case values.String(values.StrGovernance):
		pg = governance.NewGovernancePage(hp.Load)
	}
	hp.Display(pg)
}

func (hp *HomePage) showWarningNoSpendableWallet() {
	go func() {
		info := modal.NewCustomModal(hp.Load).
			PositiveButtonStyle(hp.Theme.Color.Primary, hp.Theme.Color.Surface).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			Body(values.String(values.StrCannotSpendWatchOnlyWallet))
		hp.ParentWindow().ShowModal(info)
	}()
}

// KeysToHandle returns a Filter's slice that describes a set of key combinations
// that this modal wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (hp *HomePage) KeysToHandle() []event.Filter {
	if currentPage := hp.CurrentPage(); currentPage != nil {
		if keyEvtHandler, ok := currentPage.(load.KeyEventHandler); ok {
			return keyEvtHandler.KeysToHandle()
		}
	}
	return nil
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (hp *HomePage) HandleKeyPress(gtx C, evt *key.Event) {
	if currentPage := hp.CurrentPage(); currentPage != nil {
		if keyEvtHandler, ok := currentPage.(load.KeyEventHandler); ok {
			keyEvtHandler.HandleKeyPress(gtx, evt)
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
	if hp.Load.IsMobileView() {
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
						layout.Rigid(hp.layoutTopBar),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Left: values.MarginPadding20,
							}.Layout(gtx, hp.navigationTab.Layout)
						}),
						layout.Rigid(hp.Theme.Separator().Layout),
					)
				}),
				layout.Flexed(1, hp.CurrentPage().Layout),
				layout.Rigid(func(gtx C) D {
					if hp.isUpdateAPIAllowed() && hp.releaseResponse != nil {
						return hp.layoutUpdateAvailable(gtx)
					}

					return D{}
				}),
			)
		}),
	)
}

func (hp *HomePage) layoutMobile(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(hp.layoutTopBar),
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

func (hp *HomePage) layoutTopBar(gtx C) D {
	padding20 := values.MarginPadding20
	padding10 := values.MarginPadding10

	topBottomPadding := padding10
	// Remove top and bottom padding if on the SingleWalletMasterPage.
	// This hides the gap between the top bar and the page content.
	if hp.CurrentPageID() == wallet.MainPageID {
		topBottomPadding = values.MarginPadding0
	}

	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
		Padding: layout.Inset{
			Right:  padding20,
			Left:   padding20,
			Top:    topBottomPadding,
			Bottom: topBottomPadding,
		},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			// Hide the total asset balance usd amount while
			// on the SingleWalletMasterPage.
			if hp.CurrentPageID() == wallet.MainPageID {
				return D{}
			}

			return hp.totalBalanceLayout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if hp.Load.IsMobileView() {
				return D{}
			}

			return hp.notificationSettingsLayout(gtx)
		}),
	)
}

func (hp *HomePage) initBottomNavItems() {
	items := []components.BottomNavigationBarHandler{
		{
			Clickable:     hp.Theme.NewClickable(true),
			Image:         hp.Theme.Icons.OverviewIcon,
			ImageInactive: hp.Theme.Icons.OverviewIconInactive,
			Title:         values.String(values.StrOverview),
			PageID:        OverviewPageID,
		},
		{
			Clickable:     hp.Theme.NewClickable(true),
			Image:         hp.Theme.Icons.TransactionsIcon,
			ImageInactive: hp.Theme.Icons.TransactionsIconInactive,
			Title:         values.String(values.StrTransactions),
			PageID:        transaction.TransactionsPageID,
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
			Image:         hp.Theme.Icons.GovernanceActiveIcon,
			ImageInactive: hp.Theme.Icons.GovernanceInactiveIcon,
			Title:         values.String(values.StrGovernance),
			PageID:        governance.GovernancePageID,
		},
	}

	// Add the trade tab only if not on iOS
	if !appos.Current().IsIOS() {
		tradeTab := components.BottomNavigationBarHandler{
			Clickable:     hp.Theme.NewClickable(true),
			Image:         hp.Theme.Icons.TradeIconActive,
			ImageInactive: hp.Theme.Icons.TradeIconInactive,
			Title:         values.String(values.StrTrade),
			PageID:        exchange.TradePageID,
		}
		// Determine the insertion point, which is second to last position
		insertionPoint := len(items) - 1
		if insertionPoint < 0 {
			insertionPoint = 0
		}
		// Append at the second to last position
		items = append(items[:insertionPoint], append([]components.BottomNavigationBarHandler{tradeTab}, items[insertionPoint:]...)...)
	}

	hp.bottomNavigationBar = components.BottomNavigationBar{
		Load:                  hp.Load,
		CurrentPage:           hp.CurrentPageID(),
		BottomNavigationItems: items,
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
				PageID:    receive.ReceivePageID,
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
				// Check if exchange rate fetching is enabled and total balance is available
				if hp.AssetsManager.ExchangeRateFetchingEnabled() && totalBalanceUSD != "" {
					// Render total balance text and icon button
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(hp.totalBalanceTextAndIconButtonLayout),
						layout.Rigid(hp.notificationSettingsLayout), // Include notification layout here
					)
				}

				// Render notification settings icon
				return hp.notificationSettingsLayout(gtx)
			}),
			layout.Rigid(hp.balanceLayout),
			layout.Rigid(func(gtx C) D {
				if !hp.Load.IsMobileView() {
					return D{}
				}

				// Hide the top bar send/receive buttons while on mobile view
				// and on the SingleWalletMasterPage.
				if hp.CurrentPageID() == wallet.MainPageID {
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
	if hp.AssetsManager.ExchangeRateFetchingEnabled() && totalBalanceUSD != "" {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(hp.LayoutUSDBalance),
			layout.Rigid(func(gtx C) D {
				icon := hp.Theme.Icons.VisibilityOffIcon
				if hp.isBalanceHidden {
					icon = hp.Theme.Icons.VisibilityIcon
				}
				return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
					return hp.hideBalanceButton.Layout(gtx, hp.Theme.NewIcon(icon).Layout24dp)
				})
			}),
		)
	}

	return D{}
}

// TODO: use real values
func (hp *HomePage) LayoutUSDBalance(gtx C) D {
	lblText := hp.Theme.Label(values.TextSize30, totalBalanceUSD)

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
						if hp.Load.IsMobileView() {
							return D{}
						}
						return components.LayoutNavigationBar(gtx, hp.Theme, hp.sendReceiveNavItems)
					}),
					// layout.Rigid(func(gtx C) D { // TODO: Uncomment when notifications are implemented
					// 	return layout.Inset{
					// 		Left:  values.MarginPadding10,
					// 		Right: values.MarginPadding10,
					// 	}.Layout(gtx, func(gtx C) D {
					// 		return hp.appNotificationButton.Layout(gtx, hp.Theme.Icons.Notification.Layout20dp)
					// 	})
					// }),
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
		Title(values.String(values.StrUnlockWithPassword)).
		SetDescription(values.StringF(values.StrResumeAccountDiscoveryInfo, wal.GetAssetType(), wal.GetWalletName())).
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
	if hp.AssetsManager.ExchangeRateFetchingEnabled() {
		assetsBalance, err := hp.AssetsManager.CalculateTotalAssetsBalance(true)
		if err != nil {
			log.Error(err)
			return
		}

		assetsTotalUSDBalance, err := hp.AssetsManager.CalculateAssetsUSDBalance(assetsBalance)
		if err != nil {
			log.Error(err)
			return
		}

		var totalBalance float64
		for _, balance := range assetsTotalUSDBalance {
			totalBalance += balance
		}

		totalBalanceUSD = utils.FormatAsUSDString(hp.Printer, totalBalance)
		hp.ParentWindow().Reload()
	}
}

func (hp *HomePage) layoutUpdateAvailable(gtx C) D {
	return cryptomaterial.LinearLayout{
		Orientation: layout.Horizontal,
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Background:  hp.Theme.Color.DefaultThemeColors().SurfaceHighlight,
		Clickable:   hp.updateAvailableBtn,
		Margin: layout.Inset{
			Top:    values.MarginPaddingMinus10,
			Bottom: values.MarginPadding10,
			Right:  values.MarginPadding40,
		},
		Border:    cryptomaterial.Border{Radius: hp.updateAvailableBtn.Radius},
		Direction: layout.E,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := hp.Theme.Label(values.TextSize14, values.String(values.StrUpdateAvailable))
			txt.Color = hp.Theme.Color.DefaultThemeColors().Primary
			txt.Font.Weight = font.SemiBold
			return layout.Inset{
				Left: values.MarginPadding4,
			}.Layout(gtx, txt.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			txt := hp.Theme.Label(values.TextSize14, hp.releaseResponse.TagName)
			txt.Font.Weight = font.SemiBold
			return layout.Inset{
				Left: values.MarginPadding4,
			}.Layout(gtx, txt.Layout)
		}),
	)
}

func (hp *HomePage) checkForUpdates() {
	hp.releaseResponse = components.CheckForUpdate(hp.Load)
}

func (hp *HomePage) isUpdateAPIAllowed() bool {
	return hp.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.UpdateAPI)
}
