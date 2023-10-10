package root

import (
	"context"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/dcrdex"
	"github.com/crypto-power/cryptopower/ui/page/exchange"
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

	scrollContainer *widget.List

	shadowBox   *cryptomaterial.Shadow
	exchangeBtn *cryptomaterial.Clickable
	dcrdexBtn   *cryptomaterial.Clickable
}

func NewTradePage(l *load.Load) *TradePage {
	pg := &TradePage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TradePageID),

		shadowBox: l.Theme.Shadow(),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
	}

	rad := cryptomaterial.Radius(14)
	pg.exchangeBtn = l.Theme.NewClickable(true)
	pg.exchangeBtn.Radius = rad

	pg.dcrdexBtn = l.Theme.NewClickable(true)
	pg.dcrdexBtn.Radius = rad

	return pg
}

// ID is a unique string that identifies the page and may be used
// to differentiate this page from other pages.
// Part of the load.Page interface.
func (pg *TradePage) ID() string {
	return TradePageID
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *TradePage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *TradePage) HandleUserInteractions() {
	if pg.exchangeBtn.Clicked() {
		pg.ParentNavigator().Display(exchange.NewCreateOrderPage(pg.Load))
	}
	if pg.dcrdexBtn.Clicked() {
		pg.ParentNavigator().Display(dcrdex.NewDEXPage(pg.Load))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *TradePage) OnNavigatedFrom() {
	pg.ctxCancel()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *TradePage) Layout(gtx C) D {
	pg.Load.SetCurrentAppWidth(gtx.Constraints.Max.X)
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *TradePage) layoutDesktop(gtx C) D {
	return layout.UniformInset(values.MarginPadding20).Layout(gtx, pg.pageContentLayout)
}

func (pg *TradePage) layoutMobile(gtx C) D {
	return components.UniformMobile(gtx, false, false, pg.pageContentLayout)
}

func (pg *TradePage) pageContentLayout(gtx C) D {
	return components.UniformPadding(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:     cryptomaterial.MatchParent,
			Height:    cryptomaterial.MatchParent,
			Direction: layout.Center,
			Margin: layout.Inset{
				Top:    values.MarginPadding8,
				Bottom: values.MarginPadding80,
			},
			Orientation: layout.Vertical,
		}.Layout2(gtx, func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:  cryptomaterial.WrapContent,
				Height: cryptomaterial.MatchParent,
				Margin: layout.Inset{
					Bottom: values.MarginPadding30,
				},
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrExchangeIntro)).Layout),
						layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrExchangeIntroPt2)).Layout),
						layout.Rigid(pg.boxContentLayout),
					)
				}),
			)

		})
	})
}

func (pg *TradePage) boxContentLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
		Direction:   layout.Center,
		Alignment:   layout.Middle,
		Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(8)},
		Padding: layout.Inset{
			Top:    values.MarginPadding80,
			Bottom: values.MarginPadding80,
			Left:   values.MarginPadding124,
			Right:  values.MarginPadding124,
		},
		Margin:     layout.Inset{Top: values.MarginPadding20},
		Background: pg.Theme.Color.Gray2,
	}.Layout2(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(pg.layoutAddMoreButton(pg.exchangeBtn, values.String(values.StrCentralized), values.String(values.StrExchange), pg.Theme.Icons.TradeExchange.Layout24dp)),
			layout.Rigid(pg.layoutAddMoreButton(pg.dcrdexBtn, values.String(values.StrDcrDex), "", pg.Theme.Icons.DcrDex.Layout24dp)),
		)
	})
}

func (pg *TradePage) layoutAddMoreButton(clk *cryptomaterial.Clickable, buttonText1, buttonText2 string, ic func(gtx C) D) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: values.MarginPadding24}.Layout(gtx, func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       gtx.Dp(130),
				Height:      gtx.Dp(130),
				Orientation: layout.Vertical,
				Direction:   layout.Center,
				Alignment:   layout.Middle,
				Clickable:   clk,
				Padding: layout.Inset{
					Top:    values.MarginPadding30,
					Bottom: values.MarginPadding10,
					Left:   values.MarginPadding20,
					Right:  values.MarginPadding20,
				},
				Border:     cryptomaterial.Border{Radius: cryptomaterial.Radius(8)},
				Background: pg.Theme.Color.Surface,
			}.Layout(gtx,
				layout.Rigid(ic),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, pg.Theme.Label(values.TextSize16, buttonText1).Layout)
				}),
				layout.Rigid(pg.Theme.Label(values.TextSize16, buttonText2).Layout),
			)
		})
	}
}
