package root

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/listeners"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/governance"
	"github.com/crypto-power/cryptopower/ui/page/info"
	"github.com/crypto-power/cryptopower/ui/page/privacy"
	"github.com/crypto-power/cryptopower/ui/page/seedbackup"
	"github.com/crypto-power/cryptopower/ui/page/send"
	"github.com/crypto-power/cryptopower/ui/page/staking"
	"github.com/crypto-power/cryptopower/ui/page/transaction"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/crypto-power/cryptopower/wallet"
	"github.com/gen2brain/beeep"
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
	*listeners.OrderNotificationListener

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
	usdExchangeRate float64
	totalBalance    sharedW.AssetAmount

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

	mp.isBalanceHidden = mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.HideBalanceConfigKey, false)
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
		DCRDrawerNavItems: []components.NavHandler{
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
				Title:         values.String(values.StrVoting),
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
		BTCDrawerNavItems: []components.NavHandler{
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
	// load wallet account balance first before rendering page contents.
	// It loads balance for the current selected wallet.
	mp.updateBalance()
	// updateExchangeSetting also calls updateBalance() but because of the API
	// call it may take a while before the balance and USD conversion is updated.
	// updateBalance() is called above first to prevent crash when balance value
	// is required before updateExchangeSetting() returns.
	mp.updateExchangeSetting()

	backupLater := mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.SeedBackupNotificationConfigKey, false)
	// reset the checkbox
	mp.checkBox.CheckBox.Value = false

	needBackup := mp.WL.SelectedWallet.Wallet.GetEncryptedSeed() != ""
	if needBackup && !backupLater {
		mp.showBackupInfo()
	}

	if mp.CurrentPage() == nil {
		mp.Display(info.NewInfoPage(mp.Load)) // TODO: Should pagestack have a start page?
	}

	mp.listenForNotifications() // start sync notifications listening.

	switch mp.WL.SelectedWallet.Wallet.GetAssetType() {
	case libutils.DCRWalletAsset:
		if mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.FetchProposalConfigKey, false) && mp.isProposalsAPIAllowed() {
			if mp.WL.AssetsManager.Politeia.IsSyncing() {
				return
			}
			go mp.WL.AssetsManager.Politeia.Sync(mp.ctx)
		}
	case libutils.BTCWalletAsset:
	case libutils.LTCWalletAsset:
	}

	mp.CurrentPage().OnNavigatedTo()
}

func (mp *MainPage) isProposalsAPIAllowed() bool {
	return mp.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
}

func (mp *MainPage) updateExchangeSetting() {
	mp.usdExchangeSet = false
	if components.IsFetchExchangeRateAPIAllowed(mp.WL) {
		go mp.fetchExchangeRate()
	}
}

func (mp *MainPage) fetchExchangeRate() {
	if mp.isFetchingExchangeRate {
		return
	}

	mp.isFetchingExchangeRate = true
	var market string
	switch mp.WL.SelectedWallet.Wallet.GetAssetType() {
	case libutils.DCRWalletAsset:
		market = values.DCRUSDTMarket
	case libutils.BTCWalletAsset:
		market = values.BTCUSDTMarket
	case libutils.LTCWalletAsset:
		market = values.LTCUSDTMarket
	default:
		log.Errorf("Unsupported asset type: %s", mp.WL.SelectedWallet.Wallet.GetAssetType())
		mp.isFetchingExchangeRate = false
		return
	}

	rate := mp.WL.AssetsManager.RateSource.GetTicker(market)
	if rate == nil || rate.LastTradePrice <= 0 {
		mp.isFetchingExchangeRate = false
		return
	}

	mp.usdExchangeRate = rate.LastTradePrice
	mp.updateBalance()
	mp.usdExchangeSet = true
	mp.ParentWindow().Reload()
	mp.isFetchingExchangeRate = false
}

func (mp *MainPage) updateBalance() {
	totalBalance, err := components.CalculateTotalWalletsBalance(mp.Load)
	if err != nil {
		log.Error(err)
		return
	}
	mp.totalBalance = totalBalance.Total
	balanceInUSD := totalBalance.Total.MulF64(mp.usdExchangeRate).ToCoin()
	mp.totalBalanceUSD = utils.FormatAsUSDString(mp.Printer, balanceInUSD)
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
		mp.ParentNavigator().CloseCurrentPage()
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

	displayPage := func(pg app.Page) {
		// Load the current wallet balance on page reload.
		mp.updateBalance()
		mp.Display(pg)
	}

	for _, item := range mp.drawerNav.DCRDrawerNavItems {
		for item.Clickable.Clicked() {
			var pg app.Page
			switch item.PageID {
			case send.SendPageID:
				pg = send.NewSendPage(mp.Load, false)
			case ReceivePageID:
				pg = NewReceivePage(mp.Load)
			case info.InfoID:
				pg = info.NewInfoPage(mp.Load)
			case transaction.TransactionsPageID:
				pg = transaction.NewTransactionsPage(mp.Load)
			case privacy.AccountMixerPageID:
				dcrUniqueImpl := mp.WL.SelectedWallet.Wallet.(*dcr.Asset)
				if dcrUniqueImpl != nil {
					if !dcrUniqueImpl.AccountMixerConfigIsSet() {
						pg = privacy.NewSetupPrivacyPage(mp.Load)
					} else {
						pg = privacy.NewAccountMixerPage(mp.Load)
					}
				}
			case staking.OverviewPageID:
				pg = staking.NewStakingPage(mp.Load)
			case governance.GovernancePageID:
				pg = governance.NewGovernancePage(mp.Load)
			case WalletSettingsPageID:
				pg = NewWalletSettingsPage(mp.Load)
			}

			if pg == nil || mp.ID() == mp.CurrentPageID() {
				continue
			}

			displayPage(pg)
		}
	}

	for _, item := range mp.drawerNav.BTCDrawerNavItems {
		for item.Clickable.Clicked() {
			var pg app.Page
			switch item.PageID {
			case WalletSettingsPageID:
				pg = NewWalletSettingsPage(mp.Load)
			case ReceivePageID:
				pg = NewReceivePage(mp.Load)
			case send.SendPageID:
				pg = send.NewSendPage(mp.Load, false)
			case transaction.TransactionsPageID:
				pg = transaction.NewTransactionsPage(mp.Load)
			case info.InfoID:
				pg = info.NewInfoPage(mp.Load)
			}

			if pg == nil || mp.ID() == mp.CurrentPageID() {
				continue
			}
			displayPage(pg)
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
			displayPage(pg)
		}
	}

	for i, item := range mp.floatingActionButton.FloatingActionButton {
		for item.Clickable.Clicked() {
			var pg app.Page
			if i == 0 {
				pg = send.NewSendPage(mp.Load, false)
			} else {
				pg = NewReceivePage(mp.Load)
			}

			if mp.ID() == mp.CurrentPageID() {
				continue
			}

			if mp.WL.SelectedWallet.Wallet.IsSynced() {
				mp.Display(pg)
			} else if mp.WL.SelectedWallet.Wallet.IsSyncing() {
				errModal := modal.NewErrorModal(mp.Load, values.String(values.StrWalletSyncing), modal.DefaultClickFunc())
				mp.ParentWindow().ShowModal(errModal)
			} else {
				errModal := modal.NewErrorModal(mp.Load, values.String(values.StrNotConnected), modal.DefaultClickFunc())
				mp.ParentWindow().ShowModal(errModal)
			}
		}
	}

	for mp.hideBalanceButton.Clicked() {
		mp.isBalanceHidden = !mp.isBalanceHidden
		mp.WL.SelectedWallet.Wallet.SetBoolConfigValueForKey(sharedW.HideBalanceConfigKey, mp.isBalanceHidden)
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

	// The encrypted seed exists by default and is cleared after wallet is backed up.
	// Activate the modal requesting the user to backup their current wallet on
	// every wallet open request until the encrypted seed is cleared (backup happens).
	if mp.WL.SelectedWallet.Wallet.GetEncryptedSeed() != "" {
		mp.WL.SelectedWallet.Wallet.SaveUserConfigValue(sharedW.SeedBackupNotificationConfigKey, false)
	}

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
						layout.Rigid(func(gtx C) D {
							var drawer D
							switch mp.WL.SelectedWallet.Wallet.GetAssetType() {
							case libutils.BTCWalletAsset:
								drawer = mp.drawerNav.LayoutNavDrawer(gtx, mp.drawerNav.BTCDrawerNavItems)
							case libutils.DCRWalletAsset:
								drawer = mp.drawerNav.LayoutNavDrawer(gtx, mp.drawerNav.DCRDrawerNavItems)
							case libutils.LTCWalletAsset:
								drawer = mp.drawerNav.LayoutNavDrawer(gtx, mp.drawerNav.BTCDrawerNavItems)
							}
							return drawer
						}),
						layout.Rigid(func(gtx C) D {
							if mp.CurrentPage() == nil {
								return D{}
							}
							switch mp.CurrentPage().ID() {
							case ReceivePageID, send.SendPageID, staking.OverviewPageID,
								transaction.TransactionsPageID, privacy.AccountMixerPageID:
								// Disable page functionality if a page is not synced or rescanning is in progress.
								if !mp.WL.SelectedWallet.Wallet.IsSynced() || mp.WL.SelectedWallet.Wallet.IsRescanning() {
									return components.DisablePageWithOverlay(mp.Load, mp.CurrentPage(), gtx,
										values.String(values.StrFunctionUnavailable), nil)
								}
								fallthrough
							default:
								return mp.CurrentPage().Layout(gtx)
							}
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
	case mp.isFetchingExchangeRate && mp.usdExchangeRate == 0:
		gtx.Constraints.Max.Y = gtx.Dp(values.MarginPadding18)
		gtx.Constraints.Max.X = gtx.Constraints.Max.Y
		return layout.Inset{
			Top:  values.MarginPadding8,
			Left: values.MarginPadding5,
		}.Layout(gtx, func(gtx C) D {
			loader := material.Loader(mp.Theme.Base)
			return loader.Layout(gtx)
		})
	case !mp.isFetchingExchangeRate && mp.usdExchangeRate == 0:
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

func (mp *MainPage) totalAssetBalance(gtx C) D {
	if mp.isBalanceHidden || mp.totalBalance == nil {
		hiddenBalanceText := mp.Theme.Label(values.TextSize18*0.8, "*******************")
		return layout.Inset{Bottom: values.MarginPadding0, Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
			hiddenBalanceText.Color = mp.Theme.Color.PageNavText
			return hiddenBalanceText.Layout(gtx)
		})
	}
	return components.LayoutBalanceWithUnit(gtx, mp.Load, mp.totalBalance.String())
}

func (mp *MainPage) LayoutTopBar(gtx C) D {
	assetType := mp.WL.SelectedWallet.Wallet.GetAssetType()
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Background:  mp.Theme.Color.White,
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
			}.GradientLayout(gtx, assetType,
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
									Right: values.MarginPadding12,
								}.Layout(gtx, func(gtx C) D {
									return mp.Theme.Icons.ChevronLeft.LayoutSize(gtx, values.MarginPadding12)
								})
							}),
							layout.Rigid(func(gtx C) D {
								image := components.CoinImageBySymbol(mp.Load, assetType,
									mp.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet())
								if image != nil {
									return image.Layout24dp(gtx)
								}
								return D{}
							}),
							layout.Rigid(func(gtx C) D {
								lbl := mp.Theme.H6(mp.WL.SelectedWallet.Wallet.GetWalletName())
								lbl.Color = mp.Theme.Color.PageNavText
								return layout.Inset{
									Left: values.MarginPadding10,
								}.Layout(gtx, lbl.Layout)
							}),
							layout.Rigid(func(gtx C) D {
								if mp.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
									return layout.Inset{
										Left: values.MarginPadding10,
									}.Layout(gtx, func(gtx C) D {
										return walletHightlighLabel(mp.Theme, gtx, values.String(values.StrWatchOnly))
									})
								}
								return D{}
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
								return mp.totalAssetBalance(gtx)
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

// postDesktopNotification posts notifications to the desktop.
func (mp *MainPage) postDesktopNotification(notifier interface{}) {
	var notification string
	switch t := notifier.(type) {
	case wallet.NewTransaction:
		wal := mp.WL.SelectedWallet.Wallet
		switch t.Transaction.Type {
		case dcr.TxTypeRegular:
			if t.Transaction.Direction != dcr.TxDirectionReceived {
				return
			}
			// remove trailing zeros from amount and convert to string
			amount := strconv.FormatFloat(wal.ToAmount(t.Transaction.Amount).ToCoin(), 'f', -1, 64)
			notification = values.StringF(values.StrDcrReceived, amount)
		case dcr.TxTypeVote:
			reward := strconv.FormatFloat(wal.ToAmount(t.Transaction.VoteReward).ToCoin(), 'f', -1, 64)
			notification = values.StringF(values.StrTicektVoted, reward)
		case dcr.TxTypeRevocation:
			notification = values.String(values.StrTicketRevoked)
		default:
			return
		}

		if mp.WL.AssetsManager.OpenedWalletsCount() > 1 {
			wallet := mp.WL.AssetsManager.WalletWithID(t.Transaction.WalletID)
			if wallet == nil {
				return
			}

			notification = fmt.Sprintf("[%s] %s", wallet.GetWalletName(), notification)
		}

		initializeBeepNotification(notification)
	case wallet.Proposal:
		proposalNotification := mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.ProposalNotificationConfigKey, false) ||
			!mp.WL.AssetsManager.IsPrivacyModeOn()
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
	absoluteWdPath, err := utils.GetAbsolutePath()
	if err != nil {
		log.Error(err.Error())
	}

	err = beeep.Notify(values.String(values.StrAppWallet), n,
		filepath.Join(absoluteWdPath, "ui/assets/decredicons/qrcodeSymbol.png"))
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
	case mp.OrderNotificationListener != nil:
		return
	}

	selectedWallet := mp.WL.SelectedWallet.Wallet

	mp.SyncProgressListener = listeners.NewSyncProgress()
	err := selectedWallet.AddSyncProgressListener(mp.SyncProgressListener, MainPageID)
	if err != nil {
		log.Errorf("Error adding sync progress listener: %v", err)
		return
	}

	mp.TxAndBlockNotificationListener = listeners.NewTxAndBlockNotificationListener()
	err = selectedWallet.AddTxAndBlockNotificationListener(mp.TxAndBlockNotificationListener, true, MainPageID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	mp.ProposalNotificationListener = listeners.NewProposalNotificationListener()
	if mp.isProposalsAPIAllowed() {
		err = mp.WL.AssetsManager.Politeia.AddNotificationListener(mp.ProposalNotificationListener, MainPageID)
		if err != nil {
			log.Errorf("Error adding politeia notification listener: %v", err)
			return
		}
	}

	mp.OrderNotificationListener = listeners.NewOrderNotificationListener()
	err = mp.WL.AssetsManager.InstantSwap.AddNotificationListener(mp.OrderNotificationListener, MainPageID)
	if err != nil {
		log.Errorf("Error adding instantswap notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-mp.TxAndBlockNotifChan():
				switch n.Type {
				case listeners.NewTransaction:
					mp.updateBalance()
					if mp.WL.AssetsManager.IsTransactionNotificationsOn() {
						update := wallet.NewTransaction{
							Transaction: n.Transaction,
						}
						mp.postDesktopNotification(update)
					}
					mp.ParentWindow().Reload()
				case listeners.BlockAttached:
					beep := mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.BeepNewBlocksConfigKey, false)
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
			case notification := <-mp.OrderNotifChan:
				// Post desktop notification for all events except the synced event.
				if notification.OrderStatus != wallet.OrderStatusSynced {
					mp.postDesktopNotification(notification)
				}
			case n := <-mp.SyncStatusChan:
				if n.Stage == wallet.SyncCompleted {
					mp.updateBalance()
					mp.ParentWindow().Reload()
				}
			case <-mp.ctx.Done():
				selectedWallet.RemoveSyncProgressListener(MainPageID)
				selectedWallet.RemoveTxAndBlockNotificationListener(MainPageID)
				mp.WL.AssetsManager.Politeia.RemoveNotificationListener(MainPageID)
				mp.WL.AssetsManager.InstantSwap.RemoveNotificationListener(MainPageID)

				close(mp.SyncStatusChan)
				mp.CloseTxAndBlockChan()
				close(mp.ProposalNotifChan)
				close(mp.OrderNotifChan)

				mp.SyncProgressListener = nil
				mp.TxAndBlockNotificationListener = nil
				mp.ProposalNotificationListener = nil
				mp.OrderNotificationListener = nil

				return
			}
		}
	}()
}

func (mp *MainPage) showBackupInfo() {
	backupNowOrLaterModal := modal.NewCustomModal(mp.Load).
		SetupWithTemplate(modal.WalletBackupInfoTemplate).
		SetCancelable(false).
		SetContentAlignment(layout.W, layout.W, layout.Center).
		CheckBox(mp.checkBox, true).
		SetNegativeButtonText(values.String(values.StrBackupLater)).
		SetNegativeButtonCallback(func() {
			mp.WL.SelectedWallet.Wallet.SaveUserConfigValue(sharedW.SeedBackupNotificationConfigKey, true)
		}).
		PositiveButtonStyle(mp.Load.Theme.Color.Primary, mp.Load.Theme.Color.InvText).
		SetPositiveButtonText(values.String(values.StrBackupNow)).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			mp.WL.SelectedWallet.Wallet.SaveUserConfigValue(sharedW.SeedBackupNotificationConfigKey, true)
			currentPage := mp.ParentWindow().CurrentPageID()
			mp.ParentWindow().Display(seedbackup.NewBackupInstructionsPage(mp.Load, mp.WL.SelectedWallet.Wallet, func(load *load.Load, navigator app.WindowNavigator) {
				navigator.ClosePagesAfter(currentPage)
			}))
			return true
		})
	mp.ParentWindow().ShowModal(backupNowOrLaterModal)
}

func walletHightlighLabel(theme *cryptomaterial.Theme, gtx C, content string) D {
	indexLabel := theme.Label(values.TextSize16, content)
	indexLabel.Color = theme.Color.PageNavText
	indexLabel.Font.Weight = font.Medium
	return cryptomaterial.LinearLayout{
		Width:      gtx.Dp(values.MarginPadding100),
		Height:     gtx.Dp(values.MarginPadding22),
		Direction:  layout.Center,
		Background: theme.Color.Gray8,
		Margin:     layout.Inset{Right: values.MarginPadding8},
		Border:     cryptomaterial.Border{Radius: cryptomaterial.Radius(9), Color: theme.Color.Gray3, Width: values.MarginPadding1},
	}.Layout2(gtx, indexLabel.Layout)
}
