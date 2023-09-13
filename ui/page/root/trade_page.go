package root

import (
	"context"

	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	TradePageID = "Trade"
)

type TradePage struct {
	*app.GenericPageModal
	*load.Load

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func NewTradePage(l *load.Load) *TradePage {
	tp := &TradePage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TradePageID),
	}

	return tp
}

// ID is a unique string that identifies the page and may be used
// to differentiate this page from other pages.
// Part of the load.Page interface.
func (tp *TradePage) ID() string {
	return TradePageID
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (tp *TradePage) OnNavigatedTo() {
	tp.ctx, tp.ctxCancel = context.WithCancel(context.TODO())
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (tp *TradePage) HandleUserInteractions() {

}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (tp *TradePage) OnNavigatedFrom() {
	tp.ctxCancel()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (tp *TradePage) Layout(gtx C) D {
	tp.Load.SetCurrentAppWidth(gtx.Constraints.Max.X)
	if tp.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return tp.layoutMobile(gtx)
	}
	return tp.layoutDesktop(gtx)
}

func (tp *TradePage) layoutDesktop(gtx C) D {
	return tp.comingSoonLayout(gtx)
}

func (tp *TradePage) layoutMobile(gtx C) D {
	return tp.comingSoonLayout(gtx)
}

func (tp *TradePage) comingSoonLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.MatchParent,
		Orientation: layout.Vertical,
		Alignment:   layout.Middle,
		Direction:   layout.Center,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lblText := tp.Theme.H4("Trade Page Coming soon.....")
			lblText.Color = tp.Theme.Color.PageNavText
			return lblText.Layout(gtx)
		}),
	)
}
