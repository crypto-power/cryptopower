package dcrdex

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"decred.org/dcrdex/client/comms"
	"decred.org/dcrdex/client/core"
	"decred.org/dcrdex/client/orderbook"
	"decred.org/dcrdex/dex"
	"decred.org/dcrdex/dex/order"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/dexc"
	"github.com/crypto-power/cryptopower/libwallet"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/ext"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
	pageutils "github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	DEXMarketPageID = "dex_market"
	// maxOrderDisplayedInOrderBook is the maximum number of orders that can be
	// accommodated/displayed on the order book.
	maxOrderDisplayedInOrderBook = 9
)

var (
	dp5   = values.MarginPadding5
	dp8   = values.MarginPadding8
	dp300 = values.MarginPadding300
	// orderFormAndOrderBookHeight is a an arbitrary height that accommodates
	// the current order form elements and maxOrderDisplayedInOrderBook. Use
	// this to ensure they (order form and orderbook) have the same height as
	// they are displayed sided by side.
	orderFormAndOrderBookHeight = values.MarginPadding620

	orderTypes = []cryptomaterial.DropDownItem{
		{
			Text: values.String(values.StrLimit),
		},
		{
			Text: values.String(values.StrMarket),
		},
	}

	buyAndSellBtnStrings = []string{
		values.String(values.StrBuy),
		values.String(values.StrSell),
	}

	vertical   = layout.Vertical
	horizontal = layout.Horizontal
)

type DEXMarketPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context
	cancelCtx context.CancelFunc

	scrollContainer                    *widget.List
	openOrdersAndOrderHistoryContainer *widget.List

	serverSelector        *cryptomaterial.DropDown
	lastSelectedDEXServer string
	addServerBtn          *cryptomaterial.Clickable
	xc                    *core.Exchange

	marketSelector               *cryptomaterial.DropDown
	noMarketOrServerDisconnected atomic.Bool

	toggleBuyAndSellBtn *cryptomaterial.SegmentedControl
	orderTypesDropdown  *cryptomaterial.DropDown

	priceEditor  cryptomaterial.Editor
	lotsEditor   cryptomaterial.Editor
	totalEditor  cryptomaterial.Editor
	amountEditor cryptomaterial.Editor
	lotsInfoBtn  *cryptomaterial.Clickable

	maxBuyOrSellStr     string
	orderFeeEstimateStr string

	loginBtn               cryptomaterial.Button
	postBondBtn            cryptomaterial.Button
	createOrderBtn         cryptomaterial.Button
	immediateOrderCheckbox cryptomaterial.CheckBoxStyle
	immediateOrderInfoBtn  *cryptomaterial.Clickable

	addWalletToDEX  cryptomaterial.Button
	walletSelector  *components.WalletDropdown
	accountSelector *components.AccountDropdown

	seeFullOrderBookBtn     cryptomaterial.Button
	selectedMarketOrderBook orderbookInfo
	closeOrderBookListener  func()

	orders                      []*clickableOrder
	openOrdersBtn               cryptomaterial.Button
	orderHistoryBtn             cryptomaterial.Button
	ordersTableHorizontalScroll *widget.List

	openOrdersDisplayed bool
	showLoader          bool
}

type orderbookInfo struct {
	quote, base             uint32
	quoteSymbol, baseSymbol string
	marketID                string
	book                    *orderbook.OrderBook
}

type clickableOrder struct {
	*core.Order
	cancelBtn *cryptomaterial.Clickable
}

// NewDEXMarketPage prepares and initializes a *DEXMarketPage. Specify
// selectServer to select the provided server.
func NewDEXMarketPage(l *load.Load, selectServer string) *DEXMarketPage {
	th := l.Theme
	pg := &DEXMarketPage{
		Load:                               l,
		GenericPageModal:                   app.NewGenericPageModal(DEXMarketPageID),
		scrollContainer:                    &widget.List{List: layout.List{Axis: vertical, Alignment: layout.Middle}},
		openOrdersAndOrderHistoryContainer: &widget.List{List: layout.List{Axis: vertical, Alignment: layout.Middle}},
		addServerBtn:                       th.NewClickable(false),
		toggleBuyAndSellBtn:                th.SegmentedControl(buyAndSellBtnStrings, cryptomaterial.SegmentTypeGroup),
		orderTypesDropdown:                 th.NewCommonDropDown(orderTypes, nil, values.MarginPadding100, values.DEXOrderTypes, false),
		priceEditor:                        newTextEditor(l.Theme, values.String(values.StrPrice), "", false),
		lotsEditor:                         newTextEditor(l.Theme, values.String(values.StrLots), "", false),
		lotsInfoBtn:                        th.NewClickable(false),
		totalEditor:                        newTextEditor(th, values.String(values.StrTotal), "", false),
		amountEditor:                       newTextEditor(th, values.String(values.StrAmount), "", false),
		maxBuyOrSellStr:                    "---",
		orderFeeEstimateStr:                "------",
		loginBtn:                           th.Button(values.String(values.StrLogin)),
		postBondBtn:                        th.Button(values.String(values.StrPostBond)),
		addWalletToDEX:                     th.Button(values.String(values.StrAddWallet)),
		createOrderBtn:                     th.Button(values.String(values.StrBuy)),
		immediateOrderCheckbox:             th.CheckBox(new(widget.Bool), values.String(values.StrImmediate)),
		immediateOrderInfoBtn:              th.NewClickable(false),
		seeFullOrderBookBtn:                th.Button(values.String(values.StrSeeMore)),
		openOrdersBtn:                      th.Button(values.String(values.StrOpenOrders)),
		orderHistoryBtn:                    th.Button(values.String(values.StrTradeHistory)),
		ordersTableHorizontalScroll:        &widget.List{List: layout.List{Axis: horizontal, Alignment: layout.Middle}},
		openOrdersDisplayed:                true,
		lastSelectedDEXServer:              selectServer,
	}

	btnPadding := layout.Inset{Top: dp8, Right: dp20, Left: dp20, Bottom: dp8}
	pg.toggleBuyAndSellBtn.Padding = btnPadding
	pg.openOrdersBtn.Inset, pg.orderHistoryBtn.Inset = btnPadding, btnPadding
	pg.openOrdersBtn.Font.Weight, pg.orderHistoryBtn.Font.Weight = font.SemiBold, font.SemiBold

	pg.orderTypesDropdown.CollapsedLayoutTextDirection = layout.E

	pg.priceEditor.IsTitleLabel, pg.lotsEditor.IsTitleLabel, pg.totalEditor.IsTitleLabel, pg.amountEditor.IsTitleLabel = false, false, false, false

	pg.amountEditor.Editor.ReadOnly = true
	pg.totalEditor.Editor.ReadOnly = true

	pg.seeFullOrderBookBtn.HighlightColor, pg.seeFullOrderBookBtn.Background = color.NRGBA{}, color.NRGBA{}
	pg.seeFullOrderBookBtn.Color = th.Color.Primary
	pg.seeFullOrderBookBtn.Font.Weight = font.SemiBold
	pg.seeFullOrderBookBtn.Inset = layout.Inset{}

	pg.immediateOrderCheckbox.Font.Weight = font.SemiBold

	pg.refreshOrderForm()

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *DEXMarketPage) OnNavigatedTo() {
	if pg.isDEXReset() {
		return
	}

	pg.ctx, pg.cancelCtx = context.WithCancel(context.Background())

	pg.showLoader = true
	dexc := pg.AssetsManager.DexClient()
	noteFeed := dexc.NotificationFeed()
	go func() {
		// Ensure dex client is ready.
		<-dexc.Ready()
		pg.showLoader = false
		pg.ParentWindow().Reload()

		defer func() {
			noteFeed.ReturnFeed()
		}()
		for {
			// Always check if the dex client is ready. We want to exit if there
			// was a reset.
			if pg.isDEXReset() {
				return
			}

			select {
			case <-pg.ctx.Done():
				return
			case n := <-noteFeed.C:
				if n == nil || !pg.AssetsManager.DEXCInitialized() {
					return
				}

				switch n.Type() {
				case core.NoteTypeConnEvent:
					switch n.Topic() {
					case core.TopicDEXConnected:
						pg.noMarketOrServerDisconnected.Store(true)
						pg.setServerMarkets()
					case core.TopicDEXDisconnected, core.TopicDexConnectivity:
						if n.Topic() == core.TopicDEXDisconnected {
							pg.noMarketOrServerDisconnected.Store(false)
						}
					}

					pg.ParentWindow().Reload()

				case core.NoteTypeOrder, core.NoteTypeMatch:
					if n.Topic() == core.TopicAsyncOrderFailure {
						pg.notifyError(n.Details())
					}

					pg.refreshOrders()
					pg.ParentWindow().Reload()
				case core.NoteTypeBalance, core.NoteTypeSpots:
					pg.ParentWindow().Reload()
				}
			}
		}
	}()

	pg.resetServerAndMarkets()
	if pg.priceEditor.Editor.Text() == "" {
		mkt := pg.selectedMarketInfo()
		if price := pg.orderPrice(mkt); price > 0 {
			pg.priceEditor.Editor.SetText(trimmedConventionalAmtString(price))
		}
	}

	pg.priceEditor.SetFocus()

	if dexc.IsLoggedIn() {
		go pg.refreshOrders()
		return // All good, return early.
	}

	// Prompt user to login now.
	pg.ParentWindow().ShowModal(dexLoginModal(pg.Load, dexc, func(_ string) {
		// This will reset pg.xc so we have update server details.
		pg.resetServerAndMarkets()
	}))
}

func dexLoginModal(load *load.Load, dexClient libwallet.DEXClient, positiveBtnCallback func(password string)) *modal.CreatePasswordModal {
	dexPasswordModal := modal.NewCreatePasswordModal(load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrLogin)).
		SetDescription(values.String(values.StrLoginWithDEXPassword)).
		PasswordHint(values.String(values.StrDexPassword)).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := dexClient.Login([]byte(password))
			if err != nil {
				pm.SetError(err.Error())
				return false
			}

			if positiveBtnCallback != nil {
				positiveBtnCallback(password)
			}
			return true
		}).SetCancelable(false)

	dexPasswordModal.SetPasswordTitleVisibility(false)
	return dexPasswordModal
}

func (pg *DEXMarketPage) isDEXReset() bool {
	return !pg.AssetsManager.DEXCInitialized() || !pg.AssetsManager.DexClient().InitializedWithPassword()
}

func (pg *DEXMarketPage) resetServerAndMarkets() {
	xcs := pg.AssetsManager.DexClient().Exchanges()
	var servers []cryptomaterial.DropDownItem
	for _, xc := range xcs {
		servers = append(servers, cryptomaterial.DropDownItem{
			Text: xc.Host,
		})
	}

	// Include the "Add Server" button as part of pg.serverSelector items.
	// TODO: The pg.addServerBtn should open a modal or page to add a new server
	// to DEX when clicked.
	servers = append(servers, cryptomaterial.DropDownItem{
		Text:             values.String(values.StrAddServer),
		DisplayFn:        components.IconButton(pg.Theme.Icons.ContentAdd, values.String(values.StrAddServer), layout.Inset{}, pg.Theme, pg.addServerBtn),
		PreventSelection: true,
	})

	lastSelectedDexServer := &cryptomaterial.DropDownItem{
		Text: pg.lastSelectedDEXServer,
	}
	pg.serverSelector = pg.Theme.NewCommonDropDown(servers, lastSelectedDexServer, cryptomaterial.MatchParent, values.DEXServerDropdownGroup, false)
	pg.setServerMarkets()
}

func (pg *DEXMarketPage) setServerMarkets() {
	// Set available market pairs.
	dexc := pg.AssetsManager.DexClient()
	var markets []cryptomaterial.DropDownItem
	var lastSelectedItem *cryptomaterial.DropDownItem
	var serverIsDisconnected bool
	if pg.serverSelector.Selected() != values.String(values.StrAddServer) {
		host := pg.serverSelector.Selected()
		xc, err := dexc.Exchange(host)
		if err != nil {
			pg.notifyError(err.Error())
		} else {
			pg.xc = xc
			serverIsDisconnected = xc.ConnectionStatus != comms.Connected
			for _, m := range xc.Markets {
				base, quote := convertAssetIDToAssetType(m.BaseID), convertAssetIDToAssetType(m.QuoteID)
				if base == assetTypeNoAsset || quote == assetTypeNoAsset {
					// market asset not supported by cryptopower. TODO: Should
					// we support just displaying stats for unsupported markets?
					continue
				}

				marketItem := cryptomaterial.DropDownItem{
					Text:      base.String() + "/" + quote.String(),
					DisplayFn: pg.marketDropdownListItem(base, quote),
				}

				if dexc.HasWallet(int32(m.BaseID)) && dexc.HasWallet(int32(m.QuoteID)) {
					lastSelectedItem = &marketItem
				}

				markets = append(markets, marketItem)
			}
		}
	}

	noMarketOrServerDisconnected := len(markets) == 0 || serverIsDisconnected
	pg.noMarketOrServerDisconnected.Store(noMarketOrServerDisconnected)

	if noMarketOrServerDisconnected {
		msg := values.String(values.StrNoSupportedMarket)
		if serverIsDisconnected {
			msg = values.String(values.StrDEXServerDisconnected)
		}
		markets = []cryptomaterial.DropDownItem{{
			Text:             msg,
			PreventSelection: true,
		}}
	}

	pg.marketSelector = pg.Theme.NewCommonDropDown(markets, lastSelectedItem, cryptomaterial.MatchParent, values.DEXCurrencyPairGroup, false)
	pg.fetchOrderBook()
}

func (pg *DEXMarketPage) fetchOrderBook() {
	base, quote, _ := strings.Cut(pg.marketSelector.Selected(), "/")
	baseAssetID, _ := bip(strings.ToLower(base))
	quoteAssetID, _ := bip(strings.ToLower(quote))
	pg.selectedMarketOrderBook = orderbookInfo{
		base:        baseAssetID,
		quote:       quoteAssetID,
		baseSymbol:  base,
		quoteSymbol: quote,
		marketID:    pg.formatSelectedMarketAsDEXMarketName(),
	}
	pg.closeAndResetOrderbookListener()

	if pg.noMarketOrServerDisconnected.Load() {
		return // nothing to do.
	}

	// Update order form editors.
	pg.priceEditor.ExtraText = pg.selectedMarketOrderBook.quoteSymbol + " / " + pg.selectedMarketOrderBook.baseSymbol
	pg.totalEditor.ExtraText = pg.selectedMarketOrderBook.quoteSymbol
	pg.amountEditor.ExtraText = pg.selectedMarketOrderBook.baseSymbol

	pg.showLoader = true
	go func() {
		// Fetch order book and only update if we're still on the same market.
		book, feed, err := pg.AssetsManager.DexClient().SyncBook(pg.serverSelector.Selected(), baseAssetID, quoteAssetID)
		if err == nil && pg.selectedMarketOrderBook.base == baseAssetID && pg.selectedMarketOrderBook.quote == quoteAssetID {
			pg.selectedMarketOrderBook.book = book
			pg.closeOrderBookListener = feed.Close
			pg.showLoader = false
			pg.ParentWindow().Reload()
			pg.listenForOrderbookNotifications(feed)
		} else if err != nil {
			log.Errorf("dexc.Book %v", err)
		}
		pg.showLoader = false
	}()
}

// listenForOrderbookNotifications listens for orderbook updates and MUST be
// called from a goroutine.
func (pg *DEXMarketPage) listenForOrderbookNotifications(feed core.BookFeed) {
	defer func() {
		pg.closeAndResetOrderbookListener()
	}()
	for {
		if pg.isDEXReset() {
			return
		}

		select {
		case <-pg.ctx.Done():
			return
		case bookUpdate, ok := <-feed.Next():
			if !ok {
				return // closed
			}

			sameMarket := bookUpdate.MarketID == pg.selectedMarketOrderBook.marketID
			if bookUpdate.Action == core.FreshBookAction {
				mktBook := bookUpdate.Payload.(*core.MarketOrderBook)
				sameMarket = pg.selectedMarketOrderBook.base == mktBook.Base && pg.selectedMarketOrderBook.quote == mktBook.Quote
			}

			if pg.serverSelector.Selected() == bookUpdate.Host && sameMarket {
				pg.ParentWindow().Reload()
			}
		}
	}
}

func (pg *DEXMarketPage) closeAndResetOrderbookListener() {
	if pg.closeOrderBookListener != nil {
		pg.closeOrderBookListener()
		pg.closeOrderBookListener = nil // reset
	}
}

func (pg *DEXMarketPage) marketDropdownListItem(baseAsset, quoteAsset libutils.AssetType) func(gtx C) D {
	baseIcon, quoteIcon := assetIcon(pg.Theme, baseAsset), assetIcon(pg.Theme, quoteAsset)
	return func(gtx cryptomaterial.C) cryptomaterial.D {
		return layout.Flex{Axis: horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if baseIcon == nil {
							return D{}
						}
						return baseIcon.Layout20dp(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: dp2, Left: dp2}.Layout(gtx, pg.Theme.Label(values.TextSize16, baseAsset.String()).Layout)
					}),
				)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Right: dp2, Left: dp2}.Layout(gtx, pg.Theme.Label(values.TextSize16, "/").Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if quoteIcon == nil {
							return D{}
						}
						return quoteIcon.Layout20dp(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: dp2, Left: dp2}.Layout(gtx, pg.Theme.Label(values.TextSize16, quoteAsset.String()).Layout)
					}),
				)
			}),
		)
	}
}

func assetIcon(th *cryptomaterial.Theme, assetType libutils.AssetType) *cryptomaterial.Image {
	switch assetType {
	case libutils.DCRWalletAsset:
		return th.Icons.DCR
	case libutils.BTCWalletAsset:
		return th.Icons.BTC
	case libutils.LTCWalletAsset:
		return th.Icons.LTC
	}
	return nil
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *DEXMarketPage) OnNavigatedFrom() {
	pg.cancelCtx()
	pg.closeAndResetOrderbookListener()
}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *DEXMarketPage) Layout(gtx C) D {
	if pg.isDEXReset() {
		pg.ParentNavigator().CloseCurrentPage()
		return D{}
	}

	pageContent := []layout.FlexChild{
		layout.Rigid(pg.serverAndCurrencySelection),
		layout.Rigid(pg.priceAndVolumeDetail),
		layout.Rigid(pg.orderFormAndOrderBook),
		layout.Rigid(pg.openOrdersAndHistory),
	}

	return cryptomaterial.LinearLayout{
		Width:  cryptomaterial.MatchParent,
		Height: cryptomaterial.MatchParent,
		Margin: layout.Inset{
			Bottom: values.MarginPadding30,
			Right:  dp10,
			Left:   dp10,
		},
		Direction: layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, _ int) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, pageContent...)
		})
	})
}

func (pg *DEXMarketPage) serverAndCurrencySelection(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     gtx.Dp(100),
		Margin:     layout.Inset{Top: dp5, Bottom: dp5},
		Background: pg.Theme.Color.Surface,
		Padding:    layout.UniformInset(dp16),
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
	}.Layout(gtx,
		layout.Flexed(0.5, func(gtx C) D {
			return layout.Inset{Left: values.MarginPadding10, Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: vertical}.Layout(gtx,
					layout.Rigid(pg.semiBoldLabelText(values.String(values.StrServer)).Layout),
					layout.Rigid(func(gtx C) D {
						pg.serverSelector.Background = &pg.Theme.Color.Surface
						pg.serverSelector.BorderColor = &pg.Theme.Color.Gray5
						return layout.Inset{Top: dp2}.Layout(gtx, pg.serverSelector.Layout)
					}),
				)
			})
		}),
		layout.Flexed(0.5, func(gtx C) D {
			return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: vertical}.Layout(gtx,
					layout.Rigid(pg.semiBoldLabelText(values.String(values.StrCurrencyPair)).Layout),
					layout.Rigid(func(gtx C) D {
						pg.marketSelector.Background = &pg.Theme.Color.Surface
						pg.marketSelector.BorderColor = &pg.Theme.Color.Gray5
						return layout.Inset{Top: dp2}.Layout(gtx, pg.marketSelector.Layout)
					}),
				)
			})
		}),
	)
}

func (pg *DEXMarketPage) priceAndVolumeDetail(gtx C) D {
	var change24, priceChange float64
	marketRate, low24, high24, baseVol24, quoteVol24 := "------", "------", "------", "------", "------"
	mkt, ticker := pg.selectedMarketInfo(), pg.selectedMarketUSDRateTicker()
	if mkt != nil && mkt.SpotPrice != nil {
		rate := mkt.MsgRateToConventional(mkt.SpotPrice.Rate)
		if ticker == nil {
			marketRate = pg.Printer.Sprintf("%f", rate)
		} else {
			marketRate = pg.Printer.Sprintf("%f (~ %s)", rate, pageutils.FormatAsUSDString(pg.Printer, rate*ticker.LastTradePrice))
		}

		change24 = mkt.SpotPrice.Change24
		priceChange = mkt.MsgRateToConventional(mkt.SpotPrice.High24 - mkt.SpotPrice.Low24)
		low24 = fmt.Sprintf("%f", mkt.MsgRateToConventional(mkt.SpotPrice.Low24))
		high24 = fmt.Sprintf("%f", mkt.MsgRateToConventional(mkt.SpotPrice.High24))
		if mkt.SpotPrice.Rate > 0 { // should be impossible but...
			quoteVol24 = fmt.Sprintf("%f", mkt.MsgRateToConventional(mkt.SpotPrice.Vol24/mkt.SpotPrice.Rate))
		}
		baseVol24 = fmt.Sprintf("%f", mkt.MsgRateToConventional(mkt.SpotPrice.Vol24))
	}

	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     cryptomaterial.WrapContent,
		Padding:    layout.UniformInset(16),
		Margin:     layout.Inset{Top: dp5, Bottom: dp5},
		Background: pg.Theme.Color.Surface,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
	}.Layout(gtx,
		layout.Flexed(0.33, func(gtx C) D {
			return pg.priceAndVolumeColumn(gtx,
				values.String(values.StrPrice), pg.semiBoldLabelSize14(marketRate).Layout,
				values.String(values.Str24hLow), low24,
			)
		}),
		layout.Flexed(0.33, func(gtx C) D {
			change24Layout := func(gtx C) D {
				lb := pg.semiBoldLabelSize14(pg.Printer.Sprintf(`%f (%.2f`, priceChange, change24) + "%)")
				if change24 < 0 {
					lb.Color = pg.Theme.Color.OrangeRipple
				} else if change24 > 0 {
					lb.Color = pg.Theme.Color.GreenText
				}
				return lb.Layout(gtx)
			}
			return pg.priceAndVolumeColumn(gtx,
				values.String(values.Str24HChange), change24Layout,
				values.StringF(values.Str24hVolume, convertAssetIDToAssetType(pg.selectedMarketOrderBook.base)), baseVol24,
			)
		}),
		layout.Flexed(0.33, func(gtx C) D {
			return pg.priceAndVolumeColumn(gtx,
				values.String(values.Str24hHigh), pg.semiBoldLabelSize14(high24).Layout,
				values.StringF(values.Str24hVolume, convertAssetIDToAssetType(pg.selectedMarketOrderBook.quote)), quoteVol24,
			)
		}),
	)
}

func (pg *DEXMarketPage) selectedMarketUSDRateTicker() *ext.Ticker {
	return pg.AssetsManager.RateSource.GetTicker(rateSourceMarketName(pg.marketSelector.Selected()), true)
}

func (pg *DEXMarketPage) selectedMarketInfo() (mkt *core.Market) {
	dexMarketName := pg.formatSelectedMarketAsDEXMarketName()
	if dexMarketName == "" {
		return
	}

	if pg.xc != nil {
		mkt = pg.xc.Markets[dexMarketName]
	}

	return mkt
}

// formatSelectedMarketAsDEXMarketName converts the currently selected market to
// a format recognized by the DEX client.
func (pg *DEXMarketPage) formatSelectedMarketAsDEXMarketName() string {
	dexMarketName, _ := dex.MarketName(pg.selectedMarketOrderBook.base, pg.selectedMarketOrderBook.quote)
	return dexMarketName
}

func (pg *DEXMarketPage) priceAndVolumeColumn(gtx C, title1 string, body1 func(gtx C) D, title2, body2 string) D {
	return layout.Flex{Axis: vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.WrapContent,
				Height:      cryptomaterial.WrapContent,
				Margin:      layout.Inset{Bottom: dp20},
				Orientation: vertical,
			}.Layout(gtx,
				layout.Rigid(semiBoldLabelGrey3(pg.Theme, title1).Layout),
				layout.Rigid(body1),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: vertical}.Layout(gtx,
				layout.Rigid(semiBoldLabelGrey3(pg.Theme, title2).Layout),
				layout.Rigid(pg.semiBoldLabelSize14(body2).Layout),
			)
		}),
	)
}

func (pg *DEXMarketPage) semiBoldLabelSize14(txt string) cryptomaterial.Label {
	lb := pg.Theme.Label(values.TextSize14, txt)
	lb.Font.Weight = font.SemiBold
	return lb
}

func (pg *DEXMarketPage) orderFormAndOrderBook(gtx C) D {
	elementWidth := (gtx.Constraints.Max.X - 20) / 2
	orientation := horizontal
	if pg.IsMobileView() {
		orientation = vertical
		elementWidth = gtx.Constraints.Max.X
	}
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: orientation,
		Spacing:     0,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = elementWidth
			return pg.orderForm(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: values.MarginPadding10}.Layout),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = elementWidth
			return pg.orderbook(gtx)
		}),
	)
}

func (pg *DEXMarketPage) orderForm(gtx C) D {
	sell := pg.isSellOrder()
	availableAssetBal, balStr := 0.0, "----"
	var tradeDirection, baseOrQuoteAssetSym string
	var overlaySet bool
	var overlayMsg string
	var actionBtn *cryptomaterial.Button
	xc := pg.xc
	dexClient := pg.AssetsManager.DexClient()
	hasZeroEffectiveTier := dexClient.IsLoggedIn() && xc != nil && xc.Auth.EffectiveTier == 0 && xc.Auth.PendingStrength == 0
	if !dexClient.IsLoggedIn() {
		overlaySet = true
		overlayMsg = values.String(values.StrLoginWithDEXPassword)
		actionBtn = &pg.loginBtn
	} else if pg.noMarketOrServerDisconnected.Load() {
		overlaySet = true
		if xc != nil && xc.ConnectionStatus != comms.Connected {
			overlayMsg = values.String(values.StrDEXServerDisconnected)
		} else {
			overlayMsg = values.String(values.StrNoSupportedMarketMsg)
		}
	} else if hasZeroEffectiveTier && dexClient.HasWallet(int32(xc.Auth.BondAssetID)) { // Need to post bond to trade, but be sure the wallet exists in dex client.
		overlaySet = true
		overlayMsg = values.String(values.StrPostBondMsg)
		targetTier := xc.Auth.TargetTier
		if targetTier > 0 { // Maintenance enabled
			bondAssetID := xc.Auth.BondAssetID
			setting, err := dexClient.WalletSettings(bondAssetID)
			if err != nil {
				// Wallet is said to exist in the if check, just log an error
				// here.
				log.Errorf("Error retrieving bond asset asset settings: %w", err)
			} else {
				// Wallet is being used by the dex client so it exists, can
				// ignore errors.
				walletID, _ := strconv.Atoi(setting[dexc.WalletIDConfigKey])
				accountNumber, _ := strconv.Atoi(setting[dexc.WalletAccountNumberConfigKey])
				asset := pg.AssetsManager.WalletWithID(walletID)
				accountName, _ := asset.AccountName(int32(accountNumber))
				bondAmtString := calculateBondAmount(asset, xc.BondAssets[asset.GetAssetType().ToStringLower()], int(targetTier), dexClient.BondsFeeBuffer(bondAssetID))
				overlayMsg = values.StringF(values.StrBondPostingInProgressMsg, bondAmtString, accountName, asset.GetAssetType(), asset.GetWalletName())
			}
		} else {
			actionBtn = &pg.postBondBtn
		}
	} else if missingMarketWalletType := pg.missingMarketWallet(); missingMarketWalletType != "" {
		overlaySet = true
		overlayMsg = values.StringF(values.StrMissingDEXWalletMsg, missingMarketWalletType, missingMarketWalletType)
		actionBtn = &pg.addWalletToDEX
	} else {
		if sell { // Show base asset available balance.
			tradeDirection = values.String(values.StrSell)
			availableAssetBal, baseOrQuoteAssetSym = pg.availableWalletAccountBalance(false)
		} else {
			tradeDirection = values.String(values.StrBuy)
			availableAssetBal, baseOrQuoteAssetSym = pg.availableWalletAccountBalance(true)
		}
	}

	balStr = fmt.Sprintf("%s %s", trimmedConventionalAmtString(availableAssetBal), baseOrQuoteAssetSym)
	totalSubText, lotsSubText := pg.orderFormEditorSubtext()
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     gtx.Dp(orderFormAndOrderBookHeight),
		Background: pg.Theme.Color.Surface,
		Margin:     layout.Inset{Top: dp5, Bottom: dp5},
		Padding:    layout.UniformInset(dp16),
		Direction:  layout.E,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
		Orientation: vertical,
	}.Layout2(gtx, func(gtx C) D {
		formLayout := func(gtx C) D {
			return layout.Stack{Alignment: layout.NW}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.MatchParent,
						Height:      cryptomaterial.WrapContent,
						Margin:      layout.Inset{Top: values.MarginPadding70},
						Orientation: vertical,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return orderFormRow(gtx, vertical, []layout.FlexChild{
								layout.Rigid(pg.semiBoldLabelText(values.String(values.StrPrice)).Layout),
								layout.Rigid(pg.priceEditor.Layout),
							})
						}),
						layout.Rigid(func(gtx C) D {
							return orderFormRow(gtx, vertical, []layout.FlexChild{
								layout.Rigid(func(gtx C) D {
									return layout.Inset{Bottom: dp5}.Layout(gtx, func(gtx C) D {
										var labelText string
										var lotSize string
										labelText = fmt.Sprintf("%s (%s)", values.String(values.StrLots), lotsSubText)
										if mkt := pg.selectedMarketInfo(); mkt != nil {
											lotSize = values.StringF(values.StrLotSizeFmt, fmt.Sprintf("%s %s", trimmedConventionalAmtString(mkt.MsgRateToConventional(mkt.LotSize)), convertAssetIDToAssetType(pg.selectedMarketOrderBook.base)))
										}
										return layout.Flex{Axis: horizontal}.Layout(gtx,
											layout.Rigid(pg.semiBoldLabelText(labelText).Layout),
											layout.Rigid(func(gtx C) D {
												return layout.Inset{Top: dp5, Left: dp2}.Layout(gtx, func(gtx C) D {
													return pg.lotsInfoBtn.Layout(gtx, pg.Theme.Icons.InfoAction.Layout16dp)
												})
											}),
											layout.Flexed(1, func(gtx C) D {
												if lotSize == "" {
													return D{}
												}

												return layout.E.Layout(gtx, pg.Theme.Label(values.TextSize14, lotSize).Layout)
											}),
										)
									})
								}),
								layout.Rigid(pg.lotsEditor.Layout),
								layout.Rigid(func(gtx C) D {
									return layout.Flex{Axis: horizontal}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											if !sell {
												return D{}
											}
											return layout.W.Layout(gtx, pg.Theme.Label(values.TextSize12, values.StringF(values.StrAvailableBalance, balStr)).Layout)
										}),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return layout.E.Layout(gtx, pg.Theme.Label(values.TextSize12, values.StringF(values.StrMaxDEX, tradeDirection, pg.maxBuyOrSellStr)).Layout)
										}),
									)
								}),
							})
						}),
						layout.Rigid(func(gtx C) D {
							return orderFormRow(gtx, vertical, []layout.FlexChild{
								layout.Rigid(func(gtx C) D {
									return layout.Inset{Bottom: dp5}.Layout(gtx, pg.semiBoldLabelText(fmt.Sprintf("%s (%s)", values.String(values.StrAmount), lotsSubText)).Layout)
								}),
								layout.Rigid(pg.amountEditor.Layout),
							})
						}),
						layout.Rigid(func(gtx C) D {
							return orderFormRow(gtx, vertical, []layout.FlexChild{
								layout.Rigid(func(gtx C) D {
									totalLabelTxt := fmt.Sprintf("%s (%s)", values.String(values.StrTotal), totalSubText)
									return layout.Inset{Bottom: dp5}.Layout(gtx, pg.semiBoldLabelText(totalLabelTxt).Layout)
								}),
								layout.Rigid(pg.totalEditor.Layout),
								layout.Rigid(func(gtx C) D {
									if sell {
										return D{} // Base asset available balance is shown on the sell form view
									}

									// Show quote asset balance
									return layout.Flex{Axis: horizontal}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return layout.E.Layout(gtx, pg.Theme.Label(values.TextSize12, values.StringF(values.StrAvailableBalance, balStr)).Layout)
										}),
									)
								}),
							})
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Flex{Axis: horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(semiBoldLabelGrey3(pg.Theme, values.String(values.StrEstimatedFee)).Layout),
								layout.Rigid(func(gtx C) D {
									feeEstimatedLabel := pg.Theme.Label(values.TextSize12, pg.orderFeeEstimateStr)
									feeEstimatedLabel.Alignment = text.Middle
									return feeEstimatedLabel.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx C) D {
							pg.immediateOrderCheckbox.Color = pg.Theme.Color.Text
							return orderFormRow(gtx, horizontal, []layout.FlexChild{
								layout.Rigid(pg.immediateOrderCheckbox.Layout),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{Top: dp10, Left: dp2}.Layout(gtx, func(gtx C) D {
										return pg.immediateOrderInfoBtn.Layout(gtx, pg.Theme.Icons.InfoAction.Layout16dp)
									})
								})},
							)
						}),
						layout.Rigid(func(gtx C) D {
							pg.createOrderBtn.SetEnabled(pg.hasValidOrderInfo())
							return layout.Flex{Axis: horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, pg.createOrderBtn.Layout),
							)
						}),
					)
				}),
				layout.Stacked(func(gtx C) D {
					return layout.Flex{Axis: horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.toggleBuyAndSellBtn.GroupTileLayout(gtx)
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, func(gtx C) D {
								return layout.Inset{Bottom: dp5, Top: dp5}.Layout(gtx, pg.orderTypesDropdown.Layout)
							})
						}),
					)
				}),
			)
		}
		if !overlaySet {
			return formLayout(gtx)
		}

		gtxCopy := gtx
		overlay := func(_ C) D {
			label := pg.Theme.Body1(overlayMsg)
			label.Alignment = text.Middle
			return cryptomaterial.DisableLayout(nil, gtxCopy,
				func(gtx C) D {
					return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, label.Layout)
				},
				nil, 180, pg.Theme.Color.Gray3, actionBtn)
		}

		gtx = gtx.Disabled()
		return layout.Stack{}.Layout(gtx, layout.Expanded(formLayout), layout.Stacked(overlay))
	})
}

func (pg *DEXMarketPage) missingMarketWallet() libutils.AssetType {
	dexc := pg.AssetsManager.DexClient()
	if !dexc.HasWallet(int32(pg.selectedMarketOrderBook.base)) {
		return convertAssetIDToAssetType(pg.selectedMarketOrderBook.base)
	}
	if !dexc.HasWallet(int32(pg.selectedMarketOrderBook.quote)) {
		return convertAssetIDToAssetType(pg.selectedMarketOrderBook.quote)
	}
	return ""
}

func (pg *DEXMarketPage) setMaxBuyAndMaxSell() {
	pg.maxBuyOrSellStr = "---"
	host, base, quote := pg.serverSelector.Selected(), pg.selectedMarketOrderBook.base, pg.selectedMarketOrderBook.quote

	dexc := pg.AssetsManager.DexClient()

	mkt := pg.selectedMarketInfo()
	price := pg.orderPrice(mkt)
	if price <= 0 && !pg.isSellOrder() {
		return
	}

	var est *core.MaxOrderEstimate
	var err error
	if pg.isSellOrder() {
		est, err = dexc.MaxSell(host, base, quote)
	} else {
		est, err = dexc.MaxBuy(host, base, quote, mkt.ConventionalRateToMsg(price))
	}
	if err != nil || est.Swap == nil || est.Redeem == nil {
		return
	}

	maxBuyOrSellAssetSym := convertAssetIDToAssetType(base)
	if !pg.isSellOrder() {
		maxBuyOrSellAssetSym = convertAssetIDToAssetType(quote)
	}

	/* TODO: Check reputation value i.e parcel limit - used parcel. If estimated
	lots/lots value is higher than trading limit, reduce max lots and lots value
	displayed.
	*/
	pg.maxBuyOrSellStr = fmt.Sprintf("%d %s, %s %s",
		est.Swap.Lots, values.String(values.StrLots),
		trimZeros(fmt.Sprintf("%f", conventionalAmt(est.Swap.Value))), maxBuyOrSellAssetSym,
	)
}

func (pg *DEXMarketPage) estimateOrderFee() {
	pg.orderFeeEstimateStr = values.String(values.StrNotAvailable)

	mkt := pg.selectedMarketInfo()
	price := pg.orderPrice(mkt)
	if price <= 0 && !pg.isSellOrder() {
		return
	}

	dexc := pg.AssetsManager.DexClient()

	form := pg.validatedOrderFormInfo()
	if form == nil {
		return
	}

	// Use fee estimate from pre-order.
	orderEst, err := dexc.PreOrder(form)
	if err != nil || orderEst.Swap == nil || orderEst.Swap.Estimate == nil || orderEst.Redeem == nil || orderEst.Redeem.Estimate == nil {
		return
	}

	s := orderEst.Swap.Estimate
	r := orderEst.Redeem.Estimate
	swapFee := conventionalAmt(s.MaxFees)
	redeemFee := conventionalAmt(r.RealisticBestCase)
	baseSym := convertAssetIDToAssetType(pg.selectedMarketOrderBook.base)
	quoteSym := convertAssetIDToAssetType(pg.selectedMarketOrderBook.quote)
	// Swap fees are denominated in the outgoing asset's unit, while Redeem fees
	// are denominated in the incoming asset's unit.
	if pg.isSellOrder() { // Outgoing is base asset
		pg.orderFeeEstimateStr = values.StringF(values.StrSwapAndRedeemFee, fmt.Sprintf("%f %s", swapFee, baseSym), fmt.Sprintf("%f %s", redeemFee, quoteSym))
	} else { // Outgoing is quote asset
		pg.orderFeeEstimateStr = values.StringF(values.StrSwapAndRedeemFee, fmt.Sprintf("%f %s", swapFee, quoteSym), fmt.Sprintf("%f %s", redeemFee, baseSym))
	}
}

func trimZeros(s string) string {
	return strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
}

// availableWalletAccountBalance returns the balance of the DEX wallet account
// for the quote or base asset of the selected market.
func (pg *DEXMarketPage) availableWalletAccountBalance(forQuoteAsset bool) (bal float64, assetSym string) {
	if pg.noMarketOrServerDisconnected.Load() {
		return 0, ""
	}

	var assetID uint32
	if forQuoteAsset {
		assetID = pg.selectedMarketOrderBook.quote
	} else {
		assetID = pg.selectedMarketOrderBook.base
	}

	dexc := pg.AssetsManager.DexClient()
	assetSym = convertAssetIDToAssetType(assetID).String()
	if !dexc.HasWallet(int32(assetID)) {
		return 0, assetSym
	}

	walletState := dexc.WalletState(assetID)
	if walletState != nil && walletState.Balance != nil { // better safe than sorry
		bal = conventionalAmt(walletState.Balance.Available)
	}

	return bal, assetSym
}

func orderFormRow(gtx C, orientation layout.Axis, children []layout.FlexChild) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Margin:      layout.Inset{Bottom: dp10, Top: dp10},
		Orientation: orientation,
	}.Layout(gtx, children...)
}

func (pg *DEXMarketPage) orderbook(gtx C) D {
	var buyOrders, sellOrders []*orderbook.Order
	var buyOrdersFilled, sellOrdersFilled bool
	var err error
	if pg.selectedMarketOrderBook.book != nil {
		buyOrders, buyOrdersFilled, err = pg.selectedMarketOrderBook.book.BestNOrders(maxOrderDisplayedInOrderBook, false)
		if err != nil {
			log.Errorf("orderbook.OrderBook..BestNOrders for buy side error: %v", err)
		}

		sellOrders, sellOrdersFilled, err = pg.selectedMarketOrderBook.book.BestNOrders(maxOrderDisplayedInOrderBook, true)
		if err != nil {
			log.Errorf("orderbook.OrderBook..BestNOrders for sell side error: %v", err)
		}
	}

	if !buyOrdersFilled { // Pad with empty orders
		for i := maxOrderDisplayedInOrderBook - len(buyOrders); i > 0; i-- {
			buyOrders = append(buyOrders, &orderbook.Order{})
		}
	}
	if !sellOrdersFilled { // Pad with empty orders
		nRemainingOrders := maxOrderDisplayedInOrderBook - len(sellOrders)
		emptyOrders := make([]*orderbook.Order, nRemainingOrders)
		for i := 0; i < nRemainingOrders; i++ {
			emptyOrders[i] = &orderbook.Order{}
		}
		sellOrders = append(emptyOrders, sellOrders...) // prepend for sell orders
	}

	makeOrderBookBuyOrSellFlexChildren := func(isSell bool, orders []*orderbook.Order) []layout.FlexChild {
		var orderBookFlexChildren []layout.FlexChild
		for i := range orders {
			ord := orders[i]
			orderBookFlexChildren = append(orderBookFlexChildren, layout.Rigid(func(gtx C) D {
				dummyOrder := true
				qtyStr, rateStr, epochStr := "------", "------", "------"
				if ord.Rate > 0 {
					rateStr = fmt.Sprintf("%f", conventionalAmt(ord.Rate))
				}
				if ord.Quantity > 0 {
					dummyOrder = false
					qtyStr = fmt.Sprintf("%f", conventionalAmt(ord.Quantity))
				}
				if ord.Epoch > 0 || !dummyOrder {
					epochStr = fmt.Sprintf("%d", ord.Epoch)
				}
				return pg.orderBookRow(gtx, textBuyOrSell(pg.Theme, isSell, rateStr), pg.Theme.Body2(qtyStr), pg.Theme.Body2(epochStr))
			}))
		}
		return orderBookFlexChildren
	}

	baseAsset, quoteAsset := convertAssetIDToAssetType(pg.selectedMarketOrderBook.base), convertAssetIDToAssetType(pg.selectedMarketOrderBook.quote)
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      gtx.Dp(orderFormAndOrderBookHeight),
		Background:  pg.Theme.Color.Surface,
		Margin:      layout.Inset{Top: dp5, Bottom: dp5},
		Padding:     layout.UniformInset(dp16),
		Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(8)},
		Orientation: vertical,
		Direction:   layout.Center,
	}.Layout(gtx,
		layout.Rigid(pg.semiBoldLabelText(values.String(values.StrOrderBooks)).Layout),
		// TODO: Show pg.seeFullOrderBookBtn when we have a page to view full order book.
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp10}.Layout(gtx, func(gtx C) D {
				return pg.orderBookRow(gtx,
					semiBoldGray3Size14(pg.Theme, values.StringF(values.StrAssetPrice, quoteAsset)),
					semiBoldGray3Size14(pg.Theme, values.StringF(values.StrAssetAmount, baseAsset)),
					semiBoldGray3Size14(pg.Theme, values.String(values.StrEpoch)),
				)
			})
		}),
		layout.Flexed(0.5, func(gtx C) D {
			return layout.Flex{Axis: vertical}.Layout(gtx, makeOrderBookBuyOrSellFlexChildren(true, sellOrders)...)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp5, Bottom: dp10}.Layout(gtx, func(gtx C) D {
				return layout.Stack{Alignment: layout.Center}.Layout(gtx,
					layout.Stacked(pg.Theme.Separator().Layout),
					layout.Expanded(func(gtx C) D {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Background:  pg.Theme.Color.Gray3,
							Padding:     layout.Inset{Top: dp5, Bottom: dp5, Right: dp16, Left: dp16},
							Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(16)},
							Direction:   layout.Center,
							Orientation: horizontal,
						}.Layout2(gtx, func(gtx C) D {
							marketRateStr := "------"
							mkt := pg.selectedMarketInfo()
							if mkt != nil && mkt.SpotPrice != nil {
								marketRate := mkt.MsgRateToConventional(mkt.SpotPrice.Rate)
								marketRateStr = fmt.Sprintf("%f %s", marketRate, quoteAsset)
								if ticker := pg.selectedMarketUSDRateTicker(); ticker != nil {
									marketRateStr = fmt.Sprintf("%f %s (~ %s)", marketRate, quoteAsset, pageutils.FormatAsUSDString(pg.Printer, marketRate*ticker.LastTradePrice))
								}
							}
							lb := pg.Theme.Label(values.TextSize16, marketRateStr)
							lb.Font.Weight = font.SemiBold
							return lb.Layout(gtx)
						})
					}),
				)
			})
		}),
		layout.Flexed(0.5, func(gtx C) D {
			return layout.Flex{Axis: vertical}.Layout(gtx, makeOrderBookBuyOrSellFlexChildren(false, buyOrders)...)
		}),
	)
}

func (pg *DEXMarketPage) semiBoldLabelText(title string) cryptomaterial.Label {
	lb := pg.Theme.Label(values.TextSize16, title)
	lb.Font.Weight = font.SemiBold
	lb.Color = pg.Theme.Color.Text
	return lb
}

func (pg *DEXMarketPage) orderBookRow(gtx C, priceColumn, amountColumn, epochColumn cryptomaterial.Label) D {
	return cryptomaterial.LinearLayout{
		Width:   cryptomaterial.MatchParent,
		Height:  cryptomaterial.WrapContent,
		Margin:  layout.Inset{Bottom: values.MarginPadding8},
		Spacing: layout.SpaceBetween,
	}.Layout(gtx,
		layout.Flexed(0.33, priceColumn.Layout), // Price
		layout.Flexed(0.33, func(gtx C) D {
			return layout.E.Layout(gtx, amountColumn.Layout)
		}), // Amount
		layout.Flexed(0.33, func(gtx C) D {
			return layout.E.Layout(gtx, epochColumn.Layout)
		}), // Epoch
	)
}

func textBuyOrSell(th *cryptomaterial.Theme, sell bool, txt string) cryptomaterial.Label {
	lb := th.Body2(txt)
	if sell {
		lb.Color = th.Color.OrangeRipple
	} else {
		lb.Color = th.Color.Green500
	}
	return lb
}

func (pg *DEXMarketPage) openOrdersAndHistory(gtx C) D {
	headers := []string{values.String(values.StrType), values.String(values.StrPair), values.String(values.StrAge), values.String(values.StrPrice), values.String(values.StrAmount), values.String(values.StrFilled), values.String(values.StrSettled), values.String(values.StrStatus)}

	sectionHeight := gtx.Dp(values.DP400)
	sectionWidth := values.DP850
	columnWidth := sectionWidth / unit.Dp(len(headers))
	if pg.openOrdersDisplayed && len(pg.orders) > 0 {
		sectionWidth = values.DP950
		columnWidth = sectionWidth / (unit.Dp(len(headers)) + 1 /* cancel btn column */)
		headers = append(headers, "") // cancel btn column has no header
	}
	sepWidth := sectionWidth - values.MarginPadding60

	var headersFn []layout.FlexChild
	for _, header := range headers {
		headersFn = append(headersFn, pg.orderColumn(true, header, columnWidth, 0))
	}

	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     sectionHeight,
		Background: pg.Theme.Color.Surface,
		Margin:     layout.Inset{Top: dp5, Bottom: dp5},
		Padding:    layout.UniformInset(dp10),
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
		Orientation: vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gr2 := pg.Theme.Color.Gray2
			pg.openOrdersBtn.HighlightColor, pg.orderHistoryBtn.HighlightColor = gr2, gr2
			if pg.openOrdersDisplayed {
				pg.openOrdersBtn.Background = gr2
				pg.openOrdersBtn.Color = pg.Theme.Color.GrayText1
				pg.orderHistoryBtn.Background = pg.Theme.Color.SurfaceHighlight
				pg.orderHistoryBtn.Color = pg.Theme.Color.Text
			} else {
				pg.openOrdersBtn.Background = pg.Theme.Color.SurfaceHighlight
				pg.openOrdersBtn.Color = pg.Theme.Color.Text
				pg.orderHistoryBtn.Background = gr2
				pg.orderHistoryBtn.Color = pg.Theme.Color.GrayText1
			}
			return layout.Flex{Axis: horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: dp5, Right: dp10}.Layout(gtx, pg.openOrdersBtn.Layout)
				}),
				layout.Rigid(pg.orderHistoryBtn.Layout),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.Theme.List(pg.ordersTableHorizontalScroll).Layout(gtx, 1, func(gtx C, _ int) D {
				gtx.Constraints.Max.X = gtx.Dp(sectionWidth)
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				gtx.Constraints.Max.Y = sectionHeight
				gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
				return layout.Flex{Axis: vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx, headersFn...)
					}),
					layout.Rigid(func(gtx C) D {
						if len(pg.orders) == 0 {
							var noOrderMsg string
							if pg.openOrdersDisplayed {
								noOrderMsg = values.String(values.StrNoOpenOrdersMsg)
							} else {
								noOrderMsg = values.String(values.StrNoTradeHistoryMsg)
							}
							return components.LayoutNoOrderHistoryWithMsg(gtx, pg.Load, pg.showLoader, noOrderMsg)
						}

						return pg.Theme.List(pg.openOrdersAndOrderHistoryContainer).Layout(gtx, len(pg.orders), func(gtx C, index int) D {
							ord := pg.orders[index]
							return layout.Flex{Axis: vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									if index == 0 {
										sep := pg.Theme.Separator()
										sep.Width = gtx.Dp(sepWidth)
										return layout.Center.Layout(gtx, sep.Layout)
									}
									return D{}
								}),
								layout.Rigid(func(gtx C) D {
									orderReader := pg.orderReader(ord.Order)
									return layout.Flex{Axis: horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
										pg.orderColumn(false, fmt.Sprintf("%s %s", values.String(ord.Type.String()), values.String(orderReader.SideString())), columnWidth, index),
										pg.orderColumn(false, ord.MarketID, columnWidth, index),
										pg.orderColumn(false, pageutils.TimeAgo(int64(ord.SubmitTime/1000)), columnWidth, index),
										pg.orderColumn(false, orderReader.RateString(), columnWidth, index),
										pg.orderColumn(false, fmt.Sprintf("%s %s", orderReader.BaseQtyString(), strings.ToTitle(orderReader.BaseSymbol)), columnWidth, index),
										pg.orderColumn(false, fmt.Sprintf("%s%%", orderReader.FilledPercent()), columnWidth, index),
										pg.orderColumn(false, fmt.Sprintf("%s%%", orderReader.SettledPercent()), columnWidth, index),
										pg.orderColumn(false, orderReader.StatusString(), columnWidth, index), // TODO: Add possible values to translation
										pg.orderColumn(false, "", columnWidth, index),                         // for cancel btn
									)
								}),
								layout.Rigid(func(gtx C) D {
									// No divider for last row
									if index == len(pg.orders)-1 {
										return D{}
									}
									sep := pg.Theme.Separator()
									sep.Width = gtx.Dp(sepWidth)
									return layout.Center.Layout(gtx, sep.Layout)
								}),
							)
						})
					}),
				)
			})
		}),
	)
}

func semiBoldGray3Size14(th *cryptomaterial.Theme, text string) cryptomaterial.Label {
	lb := th.Label(values.TextSize14, text)
	lb.Color = th.Color.GrayText3
	lb.Font.Weight = font.SemiBold
	return lb
}

func (pg *DEXMarketPage) orderColumn(header bool, txt string, columnWidth unit.Dp, orderIndex int) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		padding := layout.Inset{Top: dp16, Bottom: dp16}
		var showCancelBtn bool
		if !header {
			ord := pg.orders[orderIndex]
			orderIsStamped := ord.Stamp > 0
			showCancelBtn = pg.openOrdersDisplayed && !ord.Cancelling && orderIsStamped && ord.cancelBtn != nil
			if showCancelBtn {
				padding = layout.Inset{Top: dp8, Bottom: dp8}
			}
		}

		return cryptomaterial.LinearLayout{
			Width:       gtx.Dp(columnWidth),
			Height:      cryptomaterial.WrapContent,
			Orientation: horizontal,
			Alignment:   layout.Middle,
			Padding:     padding,
			Direction:   layout.Center,
		}.Layout2(gtx, func(gtx C) D {
			if header {
				return semiBoldGray3Size14(pg.Theme, txt).Layout(gtx)
			} else if txt != "" {
				lb := pg.Theme.Body2(txt)
				lb.Color = pg.Theme.Color.Text
				return lb.Layout(gtx)
			} else if showCancelBtn {
				return pg.orders[orderIndex].cancelBtn.Layout(gtx, pg.Theme.Icons.FailedIcon.Layout24dp)
			}

			return D{}
		})
	})
}

func (pg *DEXMarketPage) refreshOrderForm() {
	isSell := pg.isSellOrder()
	pg.lotsEditor.UpdateFocus(true)
	pg.lotsEditor.Editor.SetText("")
	pg.totalEditor.Editor.SetText("")
	pg.amountEditor.Editor.SetText("")
	pg.refreshPriceField()
	pg.orderFeeEstimateStr = values.String(values.StrNotAvailable)

	if !isSell { // Buy
		pg.createOrderBtn.Text = values.String(values.StrBuy)
		pg.createOrderBtn.Background = pg.Theme.Color.Green500
		pg.createOrderBtn.HighlightColor = pg.Theme.Color.Success
		return
	}

	// Sell
	pg.createOrderBtn.Text = values.String(values.StrSell)
	pg.createOrderBtn.Background = pg.Theme.Color.Orange
	pg.createOrderBtn.HighlightColor = pg.Theme.Color.OrangeRipple
}

func (pg *DEXMarketPage) orderFormEditorSubtext() (totalSubText, lotsSubText string) {
	if !pg.isSellOrder() {
		return values.String(values.StrIWillGive), values.String(values.StrIWillGet)
	}
	return values.String(values.StrIWillGet), values.String(values.StrIWillGive)
}

func (pg *DEXMarketPage) handleEditorEvents(gtx C) {
	isMktOrder := pg.isMarketOrder()
	if pg.priceEditor.Editor.Text() == "" && !isMktOrder && !pg.priceEditor.IsFocused() {
		pg.refreshPriceField()
	}

	if pg.toggleBuyAndSellBtn.Changed() {
		pg.refreshOrderForm()
		pg.setMaxBuyAndMaxSell()
	}

	if pg.orderTypesDropdown.Changed(gtx) {
		pg.refreshPriceField()
	}

	mkt := pg.selectedMarketInfo()

	var reEstimateFee bool
	for pg.priceEditor.Changed() && pg.priceEditor.IsFocused() {
		pg.priceEditor.SetError("")
		priceStr := pg.priceEditor.Editor.Text()
		if isMktOrder || priceStr == "" {
			continue
		}

		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil || price <= 0 {
			pg.priceEditor.SetError(values.String(values.StrInvalidAmount))
			continue
		}

		mktRate := mkt.ConventionalRateToMsg(price)
		if mktRate < mkt.MinimumRate {
			pg.priceEditor.SetError(values.StringF(values.StrInvalidRateFmt, trimmedConventionalAmtString(price), trimmedConventionalAmtString(mkt.MsgRateToConventional(mkt.MinimumRate))))
			continue
		}

		formattedPrice := price - mkt.MsgRateToConventional(mktRate%mkt.RateStep)
		if formattedPrice != price {
			pg.priceEditor.Editor.SetText(trimmedConventionalAmtString(formattedPrice))
		}

		if ok := pg.calculateOrderAmount(mkt); ok {
			reEstimateFee = true
		}
	}

	// Handle updates to lots Editor.
	for pg.lotsEditor.Changed() && pg.lotsEditor.IsFocused() {
		pg.lotsEditor.SetError("")
		lots := pg.lotsEditor.Editor.Text()
		if lots == "" {
			break
		}

		if ok := pg.calculateOrderAmount(mkt); ok {
			reEstimateFee = true
		}
	}

	if reEstimateFee && !pg.showLoader {
		pg.showLoader = true
		go func() {
			pg.estimateOrderFee()
			pg.showLoader = false
		}()
	}
}

func (pg *DEXMarketPage) refreshPriceField() {
	mkt := pg.selectedMarketInfo()
	isMktOrder := pg.isMarketOrder()
	pg.priceEditor.Editor.ReadOnly = isMktOrder
	if isMktOrder {
		pg.priceEditor.Editor.SetText(values.String(values.StrMarket))
	} else if mkt != nil {
		orderPrice := pg.orderPrice(mkt)
		if orderPrice == 0 {
			pg.priceEditor.Editor.SetText("")
		} else {
			pg.priceEditor.Editor.SetText(trimmedConventionalAmtString(orderPrice))
			formattedPrice := orderPrice - mkt.MsgRateToConventional(mkt.ConventionalRateToMsg(orderPrice)%mkt.RateStep)
			if formattedPrice != orderPrice {
				pg.priceEditor.Editor.SetText(trimmedConventionalAmtString(formattedPrice))
			}
		}
	} else {
		pg.priceEditor.Editor.SetText("")
	}
}

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (pg *DEXMarketPage) HandleUserInteractions(gtx C) {
	if pg.isDEXReset() {
		return
	}

	dexc := pg.AssetsManager.DexClient()
	if pg.serverSelector.Changed(gtx) {
		selectedServer := pg.serverSelector.Selected()
		xc, err := dexc.Exchange(selectedServer)
		if err != nil && xc.Auth.EffectiveTier == 0 /* need to post bond now */ {
			pg.ParentNavigator().ClearStackAndDisplay(NewDEXOnboarding(pg.Load, selectedServer, nil))
		} else {
			pg.lastSelectedDEXServer = selectedServer
			pg.setServerMarkets()
		}
	}

	if pg.addServerBtn.Clicked(gtx) {
		pg.ParentNavigator().ClearStackAndDisplay(NewDEXOnboarding(pg.Load, "", func() {
			pg.ParentNavigator().ClearStackAndDisplay(NewDEXMarketPage(pg.Load, ""))
		}))
	}

	if pg.openOrdersBtn.Clicked(gtx) {
		pg.orders = nil // clear orders
		pg.openOrdersDisplayed = true
		go pg.refreshOrders()
	}

	if pg.marketSelector.Changed(gtx) {
		pg.fetchOrderBook()
		pg.refreshOrderForm()
		pg.setMaxBuyAndMaxSell()
	}

	if pg.orderHistoryBtn.Clicked(gtx) {
		pg.orders = nil // clear orders
		pg.openOrdersDisplayed = false
		go pg.refreshOrders()
	}

	if pg.seeFullOrderBookBtn.Clicked(gtx) {
		// TODO: display full order book
		log.Info("button click listener for full order book view is not implemented")
	}

	if pg.lotsInfoBtn.Clicked(gtx) {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrLots)).
			UseCustomWidget(func(gtx layout.Context) layout.Dimensions {
				return pg.Theme.Body2(values.String(values.StrLotsExplanation)).Layout(gtx)
			}).
			SetCancelable(true).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetPositiveButtonText(values.String(values.StrOk))
		pg.ParentWindow().ShowModal(infoModal)
	}

	if pg.immediateOrderInfoBtn.Clicked(gtx) {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrImmediateOrder)).
			UseCustomWidget(func(gtx layout.Context) layout.Dimensions {
				return pg.Theme.Body2(values.String(values.StrImmediateExplanation)).Layout(gtx)
			}).
			SetCancelable(true).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetPositiveButtonText(values.String(values.StrOk))
		pg.ParentWindow().ShowModal(infoModal)
	}

	// TODO: postBondBtn should open a separate page when its design is ready.
	if pg.postBondBtn.Clicked(gtx) {
		pg.ParentNavigator().ClearStackAndDisplay(NewDEXOnboarding(pg.Load, pg.serverSelector.Selected(), nil))
	}

	if pg.loginBtn.Clicked(gtx) {
		pg.ParentWindow().ShowModal(dexLoginModal(pg.Load, dexc, func(_ string) {
			// This will reset pg.xc so we have update server details.
			pg.resetServerAndMarkets()
		}))
	}

	if pg.addWalletToDEX.Clicked(gtx) {
		pg.handleMissingMarketWallet()
	}

	for _, ord := range pg.orders {
		if ord.cancelBtn != nil && ord.cancelBtn.Clicked(gtx) {
			go func(ordID dex.Bytes) {
				err := dexc.Cancel(ordID)
				if err != nil {
					pg.notifyError(fmt.Sprintf("Error canceling order: %s", err.Error()))
				} else {
					pg.ParentWindow().Reload()
				}
			}(ord.ID)
		}
	}

	if pg.createOrderBtn.Clicked(gtx) {
		orderForm := pg.validatedOrderFormInfo()
		if orderForm == nil {
			return
		}

		pg.showLoader = true
		dexPasswordModal := modal.NewCreatePasswordModal(pg.Load).
			EnableName(false).
			EnableConfirmPassword(false).
			Title(values.String(values.StrDexPassword)).
			SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
				var err error
				defer func() {
					if err != nil {
						pm.SetError(err.Error())
					}
					pg.showLoader = false
				}()

				err = dexc.Login([]byte(password))
				if err != nil {
					return false
				}

				_, err = dexc.Trade([]byte(password), orderForm)
				if err != nil {
					return false
				}

				// Clear the trade form to allow for another trade entry
				pg.refreshOrderForm()

				return true
			})

		dexPasswordModal.SetPasswordTitleVisibility(false)
		pg.ParentWindow().ShowModal(dexPasswordModal)
	}

	if pg.walletSelector != nil {
		pg.walletSelector.Handle(gtx)
	}

	if pg.accountSelector != nil {
		pg.accountSelector.Handle(gtx)
	}

	pg.handleEditorEvents(gtx)
}

// validatedOrderFormInfo checks the the order info supplied by the user are
// valid and returns a non-nil *core.TradeForm in they are valid.
func (pg *DEXMarketPage) validatedOrderFormInfo() *core.TradeForm {
	if !pg.hasValidOrderInfo() {
		return nil
	}

	mkt := pg.selectedMarketInfo()
	lots, _ := pg.orderLots()

	orderForm := &core.TradeForm{
		Host:    pg.serverSelector.Selected(),
		IsLimit: !pg.isMarketOrder(),
		Sell:    pg.isSellOrder(),
		Qty:     mkt.ConventionalRateToMsg(lots * mkt.MsgRateToConventional(mkt.LotSize)),
		Base:    mkt.BaseID,
		Quote:   mkt.QuoteID,
		TifNow:  pg.immediateOrderCheckbox.CheckBox.Value,
	}

	if orderForm.IsLimit {
		// Set the limit order rate.
		orderForm.Rate = mkt.ConventionalRateToMsg(pg.orderPrice(mkt))
	}

	return orderForm
}

func (pg *DEXMarketPage) handleMissingMarketWallet() {
	missingMarketWalletAssetType := pg.missingMarketWallet()
	if missingMarketWalletAssetType == "" {
		return // nothing to do
	}

	showWalletModal := func() bool {
		availableWallets := pg.AssetsManager.AssetWallets(missingMarketWalletAssetType)
		if len(availableWallets) == 0 {
			return false
		}
		pg.showSelectDEXWalletModal(missingMarketWalletAssetType)
		return true
	}

	if ok := showWalletModal(); ok {
		return
	}

	callbackFn := func(_ sharedW.Asset) {
		pg.ParentNavigator().ClosePagesAfter(DEXMarketPageID)
		showWalletModal()
	}

	// No wallet exists, create it now.
	pg.ParentNavigator().Display(components.NewCreateWallet(pg.Load, callbackFn, missingMarketWalletAssetType))
}

func (pg *DEXMarketPage) showSelectDEXWalletModal(missingWallet libutils.AssetType) {
	pg.walletSelector = components.NewWalletDropdown(pg.Load, missingWallet).
		EnableWatchOnlyWallets(false).
		SetChangedCallback(func(asset sharedW.Asset) {
			_ = pg.accountSelector.Setup(asset)
		}).
		Setup()

	pg.accountSelector = components.NewAccountDropdown(pg.Load).
		AccountValidator(func(a *sharedW.Account) bool {
			return !a.IsWatchOnly && !utils.IsImportedAccount(pg.walletSelector.SelectedWallet().GetAssetType(), a)
		}).
		Setup(pg.walletSelector.SelectedWallet())

	var dexPass string
	// walletPasswordModal will request user's wallet password and bind the
	// selected wallet to core.
	walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrEnterSpendingPassword)).
		SetPositiveButtonCallback(func(_, walletPass string, pm *modal.CreatePasswordModal) bool {
			err := pg.createMissingMarketWallet(missingWallet, dexPass, walletPass)
			if err != nil {
				pm.SetError(err.Error())
				return false
			}

			return true
		}).
		SetCancelable(false)

	// Prompt user to provide DEX password then show the wallet password modal.
	dexPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrDexPassword)).
		PasswordHint(values.String(values.StrDexPassword)).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := pg.AssetsManager.DexClient().Login([]byte(password))
			if err != nil {
				pm.SetError(err.Error())
				return false
			}

			dexPass = password
			pg.ParentWindow().ShowModal(walletPasswordModal)
			return true
		}).SetCancelable(false)
	dexPasswordModal.SetPasswordTitleVisibility(false)

	// Show modal to select DEX wallet and then prompt user for DEX password.
	walletSelectModal := modal.NewCustomModal(pg.Load).
		Title(values.String(values.StrSelectWallet)).
		SetCancelable(true).
		UseCustomWidget(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: dp20}.Layout(gtx, func(gtx C) D {
						return pg.walletSelector.Layout(gtx, values.StrSelectWallet)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: dp2}.Layout(gtx, func(gtx C) D {
						return pg.accountSelector.Layout(gtx, values.String(values.StrSelectAcc))
					})
				}),
			)
		}).
		SetPositiveButtonText(values.String(values.StrAddWallet)).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			pg.ParentWindow().ShowModal(dexPasswordModal)
			return true
		})
	pg.ParentWindow().ShowModal(walletSelectModal)
}

func (pg *DEXMarketPage) createMissingMarketWallet(missingWallet libutils.AssetType, dexPass, walletPass string) error {
	// Check selected wallet and bind to core.
	asset := pg.walletSelector.SelectedWallet()
	selectedAccount := pg.accountSelector.SelectedAccount()
	if selectedAccount == nil || asset == nil {
		return errors.New("no wallet selected")
	}

	if !asset.IsSynced() { // Only fully synced wallets should connect to core.
		return errors.New(values.String(values.StrWalletNotSynced))
	}

	walletAssetID, ok := bip(missingWallet.ToStringLower())
	if !ok {
		return fmt.Errorf("no assetID for %s", missingWallet)
	}

	dexClient := pg.AssetsManager.DexClient()
	if dexClient.HasWallet(int32(walletAssetID)) {
		// TODO: For now return. We might need to allow users select
		// which wallet to use at the time of trade.
		return fmt.Errorf("%s wallet already exists in dex client", missingWallet)
	}

	// Validate wallet password.
	err := asset.UnlockWallet(walletPass)
	if err != nil {
		return err
	}

	cfg := map[string]string{
		dexc.WalletIDConfigKey:            fmt.Sprintf("%d", asset.GetWalletID()),
		dexc.WalletAccountNumberConfigKey: fmt.Sprint(selectedAccount.AccountNumber),
	}

	err = dexClient.AddWallet(walletAssetID, cfg, []byte(dexPass), []byte(walletPass))
	if err != nil {
		return fmt.Errorf("failed to add wallet to DEX client: %w", err)
	}

	return nil
}

func (pg *DEXMarketPage) refreshOrders() {
	filter := &core.OrderFilter{
		Statuses: []order.OrderStatus{order.OrderStatusBooked, order.OrderStatusEpoch, order.OrderStatusExecuted},
	}
	if !pg.openOrdersDisplayed {
		filter = &core.OrderFilter{
			Statuses: []order.OrderStatus{order.OrderStatusCanceled, order.OrderStatusExecuted, order.OrderStatusRevoked},
		}
	}

	// TODO: Paginate.
	orders, err := pg.AssetsManager.DexClient().Orders(filter)
	if err != nil {
		pg.notifyError(err.Error())
		return
	}

	pg.orders = nil
	for i := range orders {
		ord := &clickableOrder{Order: orders[i]}
		if ord.Status == order.OrderStatusExecuted && anyMatchActive(ord.Matches) != pg.openOrdersDisplayed /* display active orders on open order view */ {
			continue // skip order
		}

		if pg.openOrdersDisplayed && ord.Status.IsActive() {
			ord.cancelBtn = pg.Theme.NewClickable(false)
		}

		pg.orders = append(pg.orders, ord)
	}

	// Always sort orders.
	sort.SliceStable(pg.orders, func(i, j int) bool {
		return pg.orders[i].SubmitTime > pg.orders[j].SubmitTime
	})
}

func anyMatchActive(matches []*core.Match) bool {
	for _, m := range matches {
		if m.Active {
			return true
		}
	}
	return false
}

func (pg *DEXMarketPage) hasValidOrderInfo() bool {
	mkt := pg.selectedMarketInfo()
	lots, lotsOk := pg.orderLots()
	orderAmt, totalOk := pg.totalOrderAmt()
	if pg.isSellOrder() {
		orderAmt = lots * mkt.MsgRateToConventional(mkt.LotSize)
	}

	// TODO: Check that their tier limit has not been exceeded by this trade.
	orderPriceIsOk := pg.orderPrice(mkt) > 0 && lotsOk && totalOk
	if !orderPriceIsOk {
		return false
	}

	// Fetch wallet balance from dex and ensure wallet can fund dex order.
	walletBalance, _ := pg.availableWalletAccountBalance(!pg.isSellOrder())
	return orderPriceIsOk && orderAmt < walletBalance
}

func (pg *DEXMarketPage) orderLots() (float64, bool) {
	lotsStr := pg.lotsEditor.Editor.Text()
	lots, err := strconv.ParseFloat(lotsStr, 64)
	return lots, err == nil && lots > 0
}

func (pg *DEXMarketPage) totalOrderAmt() (float64, bool) {
	totalAmtStr := pg.totalEditor.Editor.Text()
	totalAmt, err := strconv.ParseFloat(totalAmtStr, 64)
	return totalAmt, err == nil && totalAmt > 0
}

func (pg *DEXMarketPage) orderPrice(mkt *core.Market) (price float64) {
	limitOrdPriceStr := pg.priceEditor.Editor.Text()
	if !pg.isMarketOrder() && limitOrdPriceStr != "" {
		price, _ = strconv.ParseFloat(limitOrdPriceStr, 64)
	} else if mkt != nil && !pg.isSellOrder() {
		var midGap uint64
		if pg.selectedMarketOrderBook.book != nil {
			midGap, _ = pg.selectedMarketOrderBook.book.MidGap()
		}
		if midGap != 0 {
			price = mkt.MsgRateToConventional(midGap)
		}
	} else if mkt != nil && mkt.SpotPrice != nil {
		price = mkt.MsgRateToConventional(mkt.SpotPrice.Rate)
	}

	return price
}

// calculateOrderAmount uses the amount of lots to calculate the order amount.
// Returns true if there's no invalid number.
func (pg *DEXMarketPage) calculateOrderAmount(mkt *core.Market) bool {
	orderPrice := pg.orderPrice(mkt)
	if orderPrice == 0 {
		pg.lotsEditor.Editor.SetText("")
		pg.totalEditor.Editor.SetText("")
		pg.amountEditor.Editor.SetText("")
		return false
	}

	// Use only lots editor to calculate.
	lotsStr := pg.lotsEditor.Editor.Text()
	if lotsStr == "" {
		pg.totalEditor.Editor.SetText("")
		pg.amountEditor.Editor.SetText("")
		return false
	}

	lots, err := strconv.ParseFloat(lotsStr, 64)
	if err != nil || lots <= 0 || float64(int64(lots)) != lots {
		pg.lotsEditor.SetError(values.String(values.StrInvalidLot))
		pg.totalEditor.Editor.SetText("")
		pg.amountEditor.Editor.SetText("")
		return false
	}

	amount := lots * mkt.MsgRateToConventional(mkt.LotSize)
	amtText := trimmedConventionalAmtString(amount * orderPrice)
	pg.totalEditor.Editor.SetText(amtText)
	pg.amountEditor.Editor.SetText(fmt.Sprint(trimmedConventionalAmtString(amount)))

	return true
}

func (pg *DEXMarketPage) isMarketOrder() bool {
	return pg.orderTypesDropdown.Selected() == values.String(values.StrMarket)
}

func (pg *DEXMarketPage) isSellOrder() bool {
	return pg.toggleBuyAndSellBtn.SelectedSegment() == values.String(values.StrSell)
}

func (pg *DEXMarketPage) notifyError(errMsg string) {
	errModal := modal.NewErrorModal(pg.Load, errMsg, modal.DefaultClickFunc())
	pg.ParentWindow().ShowModal(errModal)
}

func conventionalAmt(rate uint64) float64 {
	return float64(rate) / defaultConversionFactor
}

func trimmedConventionalAmtString(r float64) string {
	s := strconv.FormatFloat(r, 'f', 8, 64)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}

// rateSourceMarketName converts the provided marketPair to the expected market
// name for fiat rate fetching.
func rateSourceMarketName(marketPair string) values.Market {
	base, quote, _ := strings.Cut(marketPair, "/")
	_, baseSymOk := dex.BipSymbolID(strings.ToLower(base))
	_, quoteSymOk := dex.BipSymbolID(strings.ToLower(quote))
	if baseSymOk && quoteSymOk {
		switch quote {
		case libutils.DCRWalletAsset.String():
			return values.DCRUSDTMarket
		case libutils.BTCWalletAsset.String():
			return values.BTCUSDTMarket
		case libutils.LTCWalletAsset.String():
			return values.LTCUSDTMarket
		}
	}
	return ""
}
