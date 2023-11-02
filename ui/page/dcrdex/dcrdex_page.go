package dcrdex

import (
	"context"

	"gioui.org/layout"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const DCRDEXID = "DCRDEXID"

type (
	C = layout.Context
	D = layout.Dimensions
)

type DEXPage struct {
	*app.MasterPage

	*load.Load

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	openTradeMainPage     *cryptomaterial.Clickable
	splashPageInfoButton  cryptomaterial.IconButton
	enableDEXBtn          cryptomaterial.Button
	navigateToSettingsBtn cryptomaterial.Button
	inited                bool // TODO: Set value
}

func NewDEXPage(l *load.Load) *DEXPage {
	dp := &DEXPage{
		Load:              l,
		MasterPage:        app.NewMasterPage(DCRDEXID),
		openTradeMainPage: l.Theme.NewClickable(false),
	}

	dp.initSplashPageWidgets()
	dp.navigateToSettingsBtn = dp.Theme.Button(values.String(values.StrStartTrading))
	return dp
}

// ID is a unique string that identifies the page and may be used to
// differentiate this page from other pages.
// Part of the load.Page interface.
func (pg *DEXPage) ID() string {
	return DCRDEXID
}

// OnNavigatedTo is called when the page is about to be displayed and may be
// used to initialize page features that are only relevant when the page is
// displayed.
// Part of the load.Page interface.
func (pg *DEXPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
	if pg.CurrentPage() == nil {
		// TODO: Handle pg.inited
		pg.Display(NewDEXOnboarding(pg.Load))
	}

	pg.CurrentPage().OnNavigatedTo()
}

// Layout draws the page UI components into the provided layout context to be
// eventually drawn on screen.
// Part of the load.Page interface.
func (pg *DEXPage) Layout(gtx C) D {
	if !pg.AssetsManager.IsDexFirstVisit() {
		return components.UniformPadding(gtx, pg.splashPage)
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.MatchParent,
				Orientation: layout.Vertical,
			}.Layout(gtx,
				layout.Flexed(1, pg.CurrentPage().Layout),
			)
		}),
	)
}

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (pg *DEXPage) HandleUserInteractions() {
	if pg.openTradeMainPage.Clicked() {
		pg.ParentNavigator().CloseCurrentPage()
	}
	if pg.CurrentPage() != nil {
		pg.CurrentPage().HandleUserInteractions()
	}
	if pg.splashPageInfoButton.Button.Clicked() {
		pg.showInfoModal()
	}
	if pg.navigateToSettingsBtn.Button.Clicked() {
		pg.AssetsManager.SetDexFirstVisit(true)
	}
}

// OnNavigatedFrom is called when the page is about to be removed from the
// displayed window. This method should ideally be used to disable features that
// are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *DEXPage) OnNavigatedFrom() {}
