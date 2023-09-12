package root

import (
	"context"

	// "gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	// "github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/listeners"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	// "github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
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

type MainPage struct {
	*app.MasterPage

	*load.Load
	*listeners.SyncProgressListener
	*listeners.TxAndBlockNotificationListener
	*listeners.ProposalNotificationListener
	*listeners.OrderNotificationListener

	ctx       context.Context
	ctxCancel context.CancelFunc

	navigationTab          *cryptomaterial.Tab_Nav
	appLevelSettingsButton *cryptomaterial.Clickable
	appNotificationButton  *cryptomaterial.Clickable
	hideBalanceButton      *cryptomaterial.Clickable
	checkBox               cryptomaterial.CheckBoxStyle
	infoButton             cryptomaterial.IconButton // TOD0: use *cryptomaterial.Clickable

	// page state variables
	usdExchangeRate       float64
	totalBalance          sharedW.AssetAmount
	currencyExchangeValue string

	usdExchangeSet         bool
	isFetchingExchangeRate bool
	isBalanceHidden        bool

	totalBalanceUSD string
}

var navigationTabTitles = []string{
	values.String(values.StrOverview),
	values.String(values.StrWallets),
	values.String(values.StrTrade),
}

func NewMainPage(l *load.Load) *MainPage {
	mp := &MainPage{
		Load:       l,
		MasterPage: app.NewMasterPage(MainPageID),
		checkBox:   l.Theme.CheckBox(new(widget.Bool), values.String(values.StrAwareOfRisk)),
	}

	mp.hideBalanceButton = mp.Theme.NewClickable(false)
	mp.appLevelSettingsButton = mp.Theme.NewClickable(false)
	mp.appNotificationButton = mp.Theme.NewClickable(false)

	mp.navigationTab = l.Theme.Tab_Nav(layout.Horizontal, false)

	_, mp.infoButton = components.SubpageHeaderButtons(l)
	mp.infoButton.Size = values.MarginPadding20

	// mp.bottomNavigationBar.OnViewCreated()

	return mp
}

// ID is a unique string that identifies the page and may be used
// to differentiate this page from other pages.
// Part of the load.Page interface.
func (mp *MainPage) ID() string {
	return MainPageID
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (mp *MainPage) OnNavigatedTo() {
	mp.ctx, mp.ctxCancel = context.WithCancel(context.TODO())
	// load wallet account balance first before rendering page contents.
	// It loads balance for the current selected wallet.
	// mp.updateBalance()
	// updateExchangeSetting also calls updateBalance() but because of the API
	// call it may take a while before the balance and USD conversion is updated.
	// updateBalance() is called above first to prevent crash when balance value
	// is required before updateExchangeSetting() returns.
	// mp.updateExchangeSetting()

	// if mp.CurrentPage() == nil {
	// 	mp.Display(info.NewInfoPage(mp.Load, redirect)) // TODO: Should pagestack have a start page?
	// }

	// mp.listenForNotifications() // start sync notifications listening.

	// switch mp.WL.SelectedWallet.Wallet.GetAssetType() {
	// case libutils.DCRWalletAsset:
	// 	if mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.FetchProposalConfigKey, false) && mp.isProposalsAPIAllowed() {
	// 		if mp.WL.AssetsManager.Politeia.IsSyncing() {
	// 			return
	// 		}
	// 		go mp.WL.AssetsManager.Politeia.Sync(mp.ctx)
	// 	}
	// case libutils.BTCWalletAsset:
	// case libutils.LTCWalletAsset:
	// }

	// mp.CurrentPage().OnNavigatedTo()
}

func (mp *MainPage) isProposalsAPIAllowed() bool {
	return mp.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
}

func (mp *MainPage) updateExchangeSetting() {
	mp.usdExchangeSet = false
	if components.IsFetchExchangeRateAPIAllowed(mp.WL) {
		mp.currencyExchangeValue = mp.WL.AssetsManager.GetCurrencyConversionExchange()
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

	rate, err := mp.WL.AssetsManager.ExternalService.GetTicker(mp.currencyExchangeValue, market)
	if err != nil {
		log.Error(err)
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
	mp.totalBalanceUSD = utils.FormatUSDBalance(mp.Printer, balanceInUSD)
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

	// mp.bottomNavigationBar.OnViewCreated()
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

	if mp.infoButton.Button.Clicked() {
		// infoModal := modal.NewCustomModal(mp.Load).
		// 	Title(values.String(values.StrProposal)).
		// 	Body(values.String(values.StrOffChainVote)).
		// 	SetCancelable(true).
		// 	SetPositiveButtonText(values.String(values.StrGotIt))
		// mp.ParentWindow().ShowModal(infoModal)
	}

	if mp.appNotificationButton.Clicked() {
		// go mp.fetchExchangeRate()
	}

	for mp.appLevelSettingsButton.Clicked() {
		// mp.ParentNavigator().Display(settings.NewSettingsPage(mp.Load))
	}

	// displayPage := func(pg app.Page) {
	// 	// Load the current wallet balance on page reload.
	// 	mp.updateBalance()
	// 	mp.Display(pg)
	// }

	for mp.hideBalanceButton.Clicked() {
		mp.isBalanceHidden = mp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.HideBalanceConfigKey, false)
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
	// if mp.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
	// return mp.layoutMobile(gtx)
	// }
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
					return layout.Inset{
						Left: values.MarginPadding20,
					}.Layout(gtx, func(gtx C) D {
						return mp.navigationTab.Layout(gtx, navigationTabTitles)
					})
				}),
				layout.Rigid(mp.Theme.Separator().Layout),
				// layout.Flexed(1, func(gtx C) D {
				// 	return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
				// 		return mp.CurrentPage().Layout(gtx)
				// 	})
				// }),
			)
		}),
	)
}

// func (mp *MainPage) layoutMobile(gtx C) D {
// 	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
// 		layout.Rigid(mp.LayoutTopBar),
// 		layout.Flexed(1, func(gtx C) D {
// 			return layout.Stack{Alignment: layout.N}.Layout(gtx,
// 				layout.Expanded(func(gtx C) D {
// 					currentPage := mp.CurrentPage()
// 					if currentPage == nil {
// 						return D{}
// 					}
// 					return currentPage.Layout(gtx)
// 				}),
// 				layout.Stacked(func(gtx C) D {
// 					return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, mp.floatingActionButton.LayoutSendReceive)
// 				}),
// 			)
// 		}),
// 		layout.Rigid(mp.bottomNavigationBar.LayoutBottomNavigationBar),
// 	)
// }

func (mp *MainPage) LayoutTopBar(gtx C) D {
	v := values.MarginPadding20
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
		Padding: layout.Inset{
			Right:  v,
			Left:   v,
			Top:    values.MarginPadding10,
			Bottom: v,
		},
	}.Layout(gtx,
		layout.Rigid(mp.totalBalanceLayout),
		layout.Rigid(mp.notificationSettingsLayout),
	)
}

func (mp *MainPage) totalBalanceLayout(gtx C) D {
	return layout.W.Layout(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.WrapContent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
			// Alignment:   layout.Middle,
		}.Layout(gtx,
			layout.Rigid(mp.totalBalanceTextAndIconButtonLayout),
			layout.Rigid(mp.balanceLayout),
		)
	})
}

func (mp *MainPage) balanceLayout(gtx C) D {
	return layout.E.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(mp.LayoutUSDBalance),
			layout.Rigid(func(gtx C) D {
				icon := mp.Theme.Icons.RevealIcon
				if mp.isBalanceHidden {
					icon = mp.Theme.Icons.ConcealIcon
				}
				return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
					return mp.hideBalanceButton.Layout(gtx, icon.Layout24dp)
				})
			}),
		)
	})
}

// TODO: use real values
func (mp *MainPage) LayoutUSDBalance(gtx C) D {
	lblText := mp.Theme.Label(values.TextSize30, "$0.00")
	lblText.Color = mp.Theme.Color.PageNavText

	if mp.isBalanceHidden {
		lblText = mp.Theme.Label(values.TextSize24, "********")
	}
	inset := layout.Inset{Right: values.MarginPadding8}
	return inset.Layout(gtx, lblText.Layout)
}

func (mp *MainPage) totalBalanceTextAndIconButtonLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := mp.Theme.Label(values.TextSize14, values.String(values.StrTotalValue))
			lbl.Color = mp.Theme.Color.PageNavText
			return layout.Inset{
				Right: values.MarginPadding5,
			}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(mp.infoButton.Layout),
	)
}

func (mp *MainPage) notificationSettingsLayout(gtx C) D {
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.WrapContent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Horizontal,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding10,
						}.Layout(gtx, func(gtx C) D {
							return mp.appNotificationButton.Layout(gtx, mp.Theme.Icons.Notification.Layout24dp)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return mp.appLevelSettingsButton.Layout(gtx, mp.Theme.Icons.SettingsIcon.Layout24dp)
					}),
				)
			})
		}),
	)
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
						// update := wallet.NewTransaction{
						// 	Transaction: n.Transaction,
						// }
						// mp.postDesktopNotification(update)
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
					// mp.postDesktopNotification(notification)
				}
			case notification := <-mp.OrderNotifChan:
				// Post desktop notification for all events except the synced event.
				if notification.OrderStatus != wallet.OrderStatusSynced {
					// mp.postDesktopNotification(notification)
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

func redirect(l *load.Load, pg app.WindowNavigator) {
	onWalSelected := func() {
		pg.ClearStackAndDisplay(NewMainPage(l))
	}
	onDexServerSelected := func(server string) {
		log.Info("Not implemented yet...", server)
	}
	pg.Display(NewWalletDexServerSelector(l, onWalSelected, onDexServerSelected))
}
