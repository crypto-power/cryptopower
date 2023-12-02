package root

import (
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

var tabTitles = []string{
	values.String(values.StrDcrDex),
	values.String(values.StrCentralizedExchange),
}

type TradePage struct {
	*load.Load
	*app.MasterPage

	scrollContainer *widget.List

	tab *cryptomaterial.SegmentedControl

	shadowBox   *cryptomaterial.Shadow
	exchangeBtn *cryptomaterial.Clickable
	dcrdexBtn   *cryptomaterial.Clickable
}

func NewTradePage(l *load.Load) *TradePage {
	pg := &TradePage{
		Load:       l,
		MasterPage: app.NewMasterPage(TradePageID),

		shadowBox: l.Theme.Shadow(),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
	}

	pg.tab = l.Theme.SegmentedControl(tabTitles)

	rad := cryptomaterial.Radius(14)
	pg.exchangeBtn = l.Theme.NewClickable(false)
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
	if activeTab := pg.CurrentPage(); activeTab != nil {
		activeTab.OnNavigatedTo()
	} else {
		pg.Display(dcrdex.NewDEXPage(pg.Load))
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *TradePage) HandleUserInteractions() {
	if activeTab := pg.CurrentPage(); activeTab != nil {
		activeTab.HandleUserInteractions()
	}

	if pg.tab.SelectedIndex() == 0 {
		pg.Display(dcrdex.NewDEXPage(pg.Load))
	}
	if pg.tab.SelectedIndex() == 1 {
		pg.Display(exchange.NewCreateOrderPage(pg.Load))
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
	if activeTab := pg.CurrentPage(); activeTab != nil {
		activeTab.OnNavigatedFrom()
	}
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *TradePage) Layout(gtx C) D {
	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *TradePage) layoutDesktop(gtx C) D {
	return components.UniformPadding(gtx, func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Rigid(pg.sectionNavTab),
			layout.Flexed(1, func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					return pg.CurrentPage().Layout(gtx)
				})
			}),
		)
	})
}

func (pg *TradePage) layoutMobile(gtx C) D {
	return components.UniformMobile(gtx, false, true, func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Rigid(pg.sectionNavTab),
			layout.Flexed(1, func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					return pg.CurrentPage().Layout(gtx)
				})
			}),
		)
	})
}

func (pg *TradePage) sectionNavTab(gtx C) D {
	return pg.tab.Layout(gtx)
}
