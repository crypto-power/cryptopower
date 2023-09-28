package dcrdex

import (
	"context"

	"gioui.org/layout"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const DCRDEXID = "DCRDEXID"

type (
	C = layout.Context
	D = layout.Dimensions
)

type DCRDEXPage struct {
	*app.MasterPage

	*load.Load

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	openTradeMainPage *cryptomaterial.Clickable
	inited            bool // TODO: Set value
}

func NewDCRDEXPage(l *load.Load) *DCRDEXPage {
	dp := &DCRDEXPage{
		Load:              l,
		MasterPage:        app.NewMasterPage(DCRDEXID),
		openTradeMainPage: l.Theme.NewClickable(false),
	}
	return dp
}

// ID is a unique string that identifies the page and may be used to
// differentiate this page from other pages.
// Part of the load.Page interface.
func (dp *DCRDEXPage) ID() string {
	return DCRDEXID
}

// OnNavigatedTo is called when the page is about to be displayed and may be
// used to initialize page features that are only relevant when the page is
// displayed.
// Part of the load.Page interface.
func (dp *DCRDEXPage) OnNavigatedTo() {
	dp.ctx, dp.ctxCancel = context.WithCancel(context.TODO())

	if dp.CurrentPage() == nil {
		if dp.inited {
			// Show Market Page
		} else {
			dp.Display(NewDEXOnboarding(dp.Load))
		}
	}

	dp.CurrentPage().OnNavigatedTo()
}

// Layout draws the page UI components into the provided layout context to be
// eventually drawn on screen.
// Part of the load.Page interface.
func (dp *DCRDEXPage) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.MatchParent,
				Orientation: layout.Vertical,
			}.Layout(gtx,
				layout.Rigid(dp.topBar),
				layout.Flexed(1, dp.CurrentPage().Layout),
			)
		}),
	)
}

func (dp *DCRDEXPage) topBar(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
		Clickable:   dp.openTradeMainPage,
		Padding:     layout.UniformInset(values.MarginPadding12),
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return dp.Theme.Icons.ChevronLeft.LayoutSize(gtx, values.MarginPadding24)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			txt := dp.Theme.Label(values.TextSize16, values.String(values.StrDcrDex))
			txt.Color = dp.Theme.Color.Gray1
			return txt.Layout(gtx)
		}),
	)
}

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (dp *DCRDEXPage) HandleUserInteractions() {
	if dp.openTradeMainPage.Clicked() {
		dp.ParentNavigator().CloseCurrentPage()
	}
	if dp.CurrentPage() != nil {
		dp.CurrentPage().HandleUserInteractions()
	}
}

// OnNavigatedFrom is called when the page is about to be removed from the
// displayed window. This method should ideally be used to disable features that
// are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (dp *DCRDEXPage) OnNavigatedFrom() {}
