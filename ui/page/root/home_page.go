package root

import (
	"context"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
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
	hp.infoButton.Size = values.MarginPadding20

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
			pg = NewWalletDexServerSelector(hp.Load)
		case values.String(values.StrTrade):
			pg = NewTradePage(hp.Load)
		}

		hp.Display(pg)
	}

	if hp.infoButton.Button.Clicked() {
		// TODO: Use real values as these are dummy so lint will pass
		hp.ParentNavigator().Display(settings.NewSettingsPage(hp.Load))
	}

	if hp.appNotificationButton.Clicked() {
		// TODO: Use real values as these are dummy so lint will pass
		hp.ParentNavigator().Display(settings.NewSettingsPage(hp.Load))
	}

	for hp.appLevelSettingsButton.Clicked() {
		hp.ParentNavigator().Display(settings.NewSettingsPage(hp.Load))
	}

	for hp.hideBalanceButton.Clicked() {
		// TODO: these comments are still needed. They will be updated in my next PR
		// hp.isBalanceHidden = hp.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(sharedW.HideBalanceConfigKey, false)
		hp.isBalanceHidden = !hp.isBalanceHidden
		// hp.WL.SelectedWallet.Wallet.SetBoolConfigValueForKey(sharedW.HideBalanceConfigKey, hp.isBalanceHidden)
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
				layout.Flexed(1, func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, hp.CurrentPage().Layout)
				}),
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
			// Alignment:   layout.Middle,
		}.Layout(gtx,
			layout.Rigid(hp.totalBalanceTextAndIconButtonLayout),
			layout.Rigid(hp.balanceLayout),
		)
	})
}

func (hp *HomePage) balanceLayout(gtx C) D {
	return layout.E.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(hp.LayoutUSDBalance),
			layout.Rigid(func(gtx C) D {
				icon := hp.Theme.Icons.RevealIcon
				if hp.isBalanceHidden {
					icon = hp.Theme.Icons.ConcealIcon
				}
				return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
					return hp.hideBalanceButton.Layout(gtx, icon.Layout24dp)
				})
			}),
		)
	})
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
			return layout.Inset{
				Right: values.MarginPadding5,
			}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(hp.infoButton.Layout),
	)
}

func (hp *HomePage) notificationSettingsLayout(gtx C) D {
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
							return hp.appNotificationButton.Layout(gtx, hp.Theme.Icons.Notification.Layout24dp)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return hp.appLevelSettingsButton.Layout(gtx, hp.Theme.Icons.SettingsIcon.Layout24dp)
					}),
				)
			})
		}),
	)
}
