package root

import (
	"context"
	"fmt"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/send"
	"github.com/crypto-power/cryptopower/ui/page/settings"
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

	totalUSDValueSwitch    *cryptomaterial.Switch
	navigationTab          *cryptomaterial.Tab
	appLevelSettingsButton *cryptomaterial.Clickable
	appNotificationButton  *cryptomaterial.Clickable
	hideBalanceButton      *cryptomaterial.Clickable
	checkBox               cryptomaterial.CheckBoxStyle
	infoButton             cryptomaterial.IconButton // TOD0: use *cryptomaterial.Clickable

	// page state variables
	isBalanceHidden bool
}

var navigationTabTitles = []string{
	values.String(values.StrOverview),
	values.String(values.StrWallets),
	values.String(values.StrTrade),
}

func NewHomePage(l *load.Load) *HomePage {
	hp := &HomePage{
		Load:       l,
		MasterPage: app.NewMasterPage(HomePageID),
	}

	hp.hideBalanceButton = hp.Theme.NewClickable(false)
	hp.appLevelSettingsButton = hp.Theme.NewClickable(false)
	hp.appNotificationButton = hp.Theme.NewClickable(false)

	hp.navigationTab = l.Theme.Tab(layout.Horizontal, false, navigationTabTitles)

	_, hp.infoButton = components.SubpageHeaderButtons(l)
	hp.infoButton.Size = values.MarginPadding15

	hp.totalUSDValueSwitch = hp.Theme.Switch()

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

	if hp.CurrentPage() == nil {
		hp.Display(NewOverviewPage(hp.Load))
	}

	hp.totalUSDValueSwitch.SetChecked(true)

	hp.CurrentPage().OnNavigatedTo()
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
			pg = NewOverviewPage(hp.Load)
		case values.String(values.StrWallets):
			pg = NewWalletSelectorPage(hp.Load)
		case values.String(values.StrTrade):
			pg = NewTradePage(hp.Load)
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
			// TODO: Implement click functionality
			fmt.Println(item.PageID, "clicked")
			if strings.ToLower(item.PageID) == values.StrReceive {
				receiveModal := components.NewReceiveModal(hp.Load)
				hp.ParentWindow().ShowModal(receiveModal)
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
		// TODO use assetManager config settings
		hp.isBalanceHidden = !hp.isBalanceHidden
	}

	if hp.totalUSDValueSwitch.Changed() {
		// TODO use assetManager config settings
		hp.totalUSDValueSwitch.SetChecked(hp.totalUSDValueSwitch.IsChecked())
	}
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
				layout.Rigid(hp.LayoutTopBar),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Left: values.MarginPadding20,
					}.Layout(gtx, hp.navigationTab.Layout)
				}),
				layout.Rigid(hp.Theme.Separator().Layout),
				layout.Flexed(1, hp.CurrentPage().Layout),
			)
		}),
	)
}

func (hp *HomePage) layoutMobile(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.MatchParent,
				Orientation: layout.Vertical,
			}.Layout(gtx,
				layout.Rigid(hp.LayoutTopBar),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Left: values.MarginPadding20,
					}.Layout(gtx, hp.navigationTab.Layout)
				}),
				layout.Rigid(hp.Theme.Separator().Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
						return hp.CurrentPage().Layout(gtx)
					})
				}),
			)
		}),
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
		layout.Rigid(hp.notificationSettingsLayout),
	)
}

func (hp *HomePage) totalBalanceLayout(gtx C) D {
	return layout.W.Layout(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.WrapContent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(hp.totalBalanceTextAndIconButtonLayout),
			layout.Rigid(hp.balanceLayout),
		)
	})
}

func (hp *HomePage) balanceLayout(gtx C) D {
	if hp.totalUSDValueSwitch.IsChecked() {
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
	lblText.Color = hp.Theme.Color.PageNavText
	return lblText.Layout(gtx)
}

// TODO: use real values
func (hp *HomePage) LayoutUSDBalance(gtx C) D {
	lblText := hp.Theme.Label(values.TextSize30, "$0.00")
	lblText.Color = hp.Theme.Color.PageNavText

	if hp.isBalanceHidden {
		lblText = hp.Theme.Label(values.TextSize24, "********")
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
			lbl.Color = hp.Theme.Color.PageNavText
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Left:  values.MarginPadding5,
				Right: values.MarginPadding10,
			}.Layout(gtx, hp.infoButton.Layout)
		}),
		layout.Rigid(hp.totalUSDValueSwitch.Layout),
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
					layout.Rigid(hp.drawerNav.LayoutTopBar),
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
