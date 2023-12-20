package exchange

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/dexc"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/dcrdex"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	TradePageID = "Trade"
)

var tabTitles = []string{
	values.String(values.StrDcrDex),
	values.String(values.StrCentralizedExchange),
	values.String(values.StrTradeHistory),
}

type TradePage struct {
	*load.Load
	*app.MasterPage

	// Might be nil but TradePage does not care because DEXPage is in the best
	// position to handle a nil DEX client.
	dexc *dexc.DEXClient

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
		shadowBox:  l.Theme.Shadow(),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
	}
	filteredTabTitles := tabTitles
	if l.IsMobileView() {
		// Remove dcrdex for mobile view,
		// dcrdex isn't supported on mobile yet
		filteredTabTitles = filteredTabTitles[1:]
	}

	pg.tab = l.Theme.SegmentedControl(filteredTabTitles, cryptomaterial.SegmentTypeGroup)

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
	// on mobile view, we display the cex tab by default
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
	selectedIndex := pg.tab.SelectedIndex()
	if pg.IsMobileView() {
		selectedIndex++ // Adjust index for mobile view
	}

	switch selectedIndex {
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

	pg.CurrentPage().HandleUserInteractions()
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
	return pg.tab.Layout(gtx, pg.CurrentPage().Layout, pg.IsMobileView())
}
