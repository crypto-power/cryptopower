package exchange

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/appos"
	"github.com/crypto-power/cryptopower/dexc"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/dcrdex"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	TradePageID = "Trade"
)

var tab *cryptomaterial.SegmentedControl

var tabTitles = []string{
	values.StrDcrDex,
	values.StrCentralizedExchange,
	values.StrTradeHistory,
}

type TradePage struct {
	*load.Load
	*app.MasterPage

	// Might be nil but TradePage does not care because DEXPage is in the best
	// position to handle a nil DEX client.
	dexc *dexc.DEXClient

	scrollContainer *widget.List
	shadowBox       *cryptomaterial.Shadow
	exchangeBtn     *cryptomaterial.Clickable
	dcrdexBtn       *cryptomaterial.Clickable
}

func NewTradePage(l *load.Load) *TradePage {
	pg := &TradePage{
		Load:       l,
		MasterPage: app.NewMasterPage(TradePageID),
		shadowBox:  l.Theme.Shadow(),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
	}
	filteredTabTitles := tabTitles
	if appos.Current().IsIOS() {
		// Remove dcrdex for ios view, dcrdex isn't supported on ios devices yet.
		filteredTabTitles = filteredTabTitles[1:]
	}

	tab = l.Theme.SegmentedControl(filteredTabTitles, cryptomaterial.SegmentTypeGroup)
	tab.AutoScrollToItem = true
	tab.SetEnableSwipe(false)
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
	// on ios devices, we display the cex tab by default
	if appos.Current().IsIOS() {
		tab.SetSelectedSegment(tabTitles[1])
		pg.Display(NewCreateOrderPage(pg.Load))
	} else if activeTab := pg.CurrentPage(); activeTab != nil {
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
func (pg *TradePage) HandleUserInteractions(gtx C) {
	if pg.CurrentPage() == nil || tab.Changed() {
		switch tab.SelectedIndex() {
		case 0: // DCRDEX
			if pg.CurrentPageID() != dcrdex.DCRDEXPageID {
				pg.Display(dcrdex.NewDEXPage(pg.Load))
			}
		case 1: // Centralized Exchange
			if pg.CurrentPageID() != CreateOrderPageID {
				pg.Display(NewCreateOrderPage(pg.Load))
			}
		case 2: // Trade History
			if pg.CurrentPageID() != OrderHistoryPageID {
				pg.Display(NewOrderHistoryPage(pg.Load))
			}
		}
	}

	pg.CurrentPage().HandleUserInteractions(gtx)
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
	return tab.Layout(gtx, pg.CurrentPage().Layout, pg.IsMobileView())
}
