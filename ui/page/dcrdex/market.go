package dcrdex

import (
	"fmt"
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	DEXMarketPageID = "dex_market"
	// maxOrderDisplayedInOrderBook is the maximum number of orders that can be
	// accommodated/displayed on the order book.
	maxOrderDisplayedInOrderBook = 7
)

var (
	dp5                        = values.MarginPadding5
	dp8                        = values.MarginPadding8
	dp300                      = values.DP300
	orderFormAndOrderBookWidth = (values.AppWidth / 2) - 40 // Minus 40 px to allow for margin between the order form and order book.
	// orderFormAndOrderBookHeight is a an arbitrary height that accommodates
	// the current order form elements and maxOrderDisplayedInOrderBook. Use
	// this to ensure they (order form and orderbook) have the same height as
	// they are displayed sided by side.
	orderFormAndOrderBookHeight = values.DP515

	orderTypes = []cryptomaterial.DropDownItem{
		{
			Text: values.String(values.StrLimit),
		},
		{
			Text: values.String(values.StrMarket),
		},
	}

	buyBtnStringIndex    = 0
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

	scrollContainer                    *widget.List
	openOrdersAndOrderHistoryContainer *widget.List

	serverSelector *cryptomaterial.DropDown
	addServerBtn   *cryptomaterial.Clickable

	marketSelector *cryptomaterial.DropDown

	toggleBuyAndSellBtn *cryptomaterial.SegmentedControl
	orderTypesDropdown  *cryptomaterial.DropDown

	priceEditor        cryptomaterial.Editor
	switchLotsOrAmount *cryptomaterial.Switch
	lotsOrAmountEditor cryptomaterial.Editor
	totalEditor        cryptomaterial.Editor

	seeFullOrderBookBtn cryptomaterial.Button

	createOrderBtn         cryptomaterial.Button
	immediateOrderCheckbox cryptomaterial.CheckBoxStyle
	immediateOrderInfoBtn  *cryptomaterial.Clickable

	openOrdersBtn       cryptomaterial.Button
	orderHistoryBtn     cryptomaterial.Button
	openOrdersDisplayed bool

	ordersTableHorizontalScroll *widget.List

	showLoader bool
}

func NewDEXMarketPage(l *load.Load) *DEXMarketPage {
	th := l.Theme
	pg := &DEXMarketPage{
		Load:                               l,
		openOrdersAndOrderHistoryContainer: &widget.List{List: layout.List{Axis: vertical, Alignment: layout.Middle}},
		GenericPageModal:                   app.NewGenericPageModal(DEXOnboardingPageID),
		scrollContainer:                    &widget.List{List: layout.List{Axis: vertical, Alignment: layout.Middle}},
		priceEditor:                        newTextEditor(l.Theme, values.String(values.StrPrice), "0", false),
		totalEditor:                        newTextEditor(th, values.String(values.StrTotal), "", false),
		switchLotsOrAmount:                 l.Theme.Switch(), // TODO: Set last user choice, default is unchecked.
		lotsOrAmountEditor:                 newTextEditor(l.Theme, values.String(values.StrLots), "0", false),
		createOrderBtn:                     th.Button(values.String(values.StrBuy)), // TODO: toggle
		immediateOrderCheckbox:             th.CheckBox(new(widget.Bool), values.String(values.StrImmediate)),
		orderTypesDropdown:                 th.DropDown(orderTypes, values.DEXOrderTypes, true),
		immediateOrderInfoBtn:              th.NewClickable(false),
		addServerBtn:                       th.NewClickable(false),
		seeFullOrderBookBtn:                th.Button(values.String(values.StrSeeMore)),
		toggleBuyAndSellBtn:                th.SegmentedControl(buyAndSellBtnStrings, cryptomaterial.SegmentTypeGroup),
		openOrdersBtn:                      th.Button(values.String(values.StrOpenOrders)),
		orderHistoryBtn:                    th.Button(values.String(values.StrTradeHistory)),
		openOrdersDisplayed:                true,
		ordersTableHorizontalScroll:        &widget.List{List: layout.List{Axis: horizontal, Alignment: layout.Middle}},
	}

	btnPadding := layout.Inset{Top: dp8, Right: dp20, Left: dp20, Bottom: dp8}
	pg.toggleBuyAndSellBtn.Padding = btnPadding
	pg.openOrdersBtn.Inset, pg.orderHistoryBtn.Inset = btnPadding, btnPadding
	pg.openOrdersBtn.Font.Weight, pg.orderHistoryBtn.Font.Weight = font.SemiBold, font.SemiBold

	pg.orderTypesDropdown.CollapsedLayoutTextDirection = layout.E
	pg.orderTypesDropdown.Width = values.MarginPadding120
	pg.orderTypesDropdown.FontWeight = font.SemiBold
	pg.orderTypesDropdown.Hoverable = false
	pg.orderTypesDropdown.SelectedItemIconColor = &pg.Theme.Color.Primary

	pg.orderTypesDropdown.ExpandedLayoutInset = layout.Inset{Top: values.MarginPadding35}
	pg.orderTypesDropdown.MakeCollapsedLayoutVisibleWhenExpanded = true

	pg.priceEditor.IsTitleLabel, pg.lotsOrAmountEditor.IsTitleLabel, pg.totalEditor.IsTitleLabel = false, false, false

	pg.seeFullOrderBookBtn.HighlightColor, pg.seeFullOrderBookBtn.Background = color.NRGBA{}, color.NRGBA{}
	pg.seeFullOrderBookBtn.Color = th.Color.Primary
	pg.seeFullOrderBookBtn.Font.Weight = font.SemiBold
	pg.seeFullOrderBookBtn.Inset = layout.Inset{}

	pg.immediateOrderCheckbox.Font.Weight = font.SemiBold

	pg.setBuyOrSell()

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *DEXMarketPage) OnNavigatedTo() {
	pg.showLoader = false

	// TODO: Use real server values saved by user and it should be the default selected in dropdown position.
	var servers []cryptomaterial.DropDownItem
	for _, s := range knownDEXServers[pg.AssetsManager.NetType()] {
		servers = append(servers, cryptomaterial.DropDownItem{
			Text: s.Text,
			Icon: s.Icon,
		})
	}

	// Include the "Add Server" button as part of pg.serverSelector items. The
	// pg.addServerBtn should open a modal or page to add a new server to DEX
	// when clicked.
	servers = append(servers, cryptomaterial.DropDownItem{
		DisplayFn:        components.IconButton(pg.Theme.Icons.ContentAdd, values.String(values.StrAddServer), layout.Inset{}, pg.Theme, pg.addServerBtn),
		PreventSelection: true,
	})

	pg.serverSelector = pg.Theme.DropDown(servers, values.DEXServerDropdownGroup, false)
	pg.marketSelector = pg.Theme.DropDown([]cryptomaterial.DropDownItem{
		{
			Text:      "DCR/BTC",
			DisplayFn: pg.marketDropdownListItem(libutils.DCRWalletAsset, libutils.BTCWalletAsset),
		},
		{
			Text:      "DCR/LTC",
			DisplayFn: pg.marketDropdownListItem(libutils.DCRWalletAsset, libutils.LTCWalletAsset),
		},
		{
			Text:      "LTC/BTC",
			DisplayFn: pg.marketDropdownListItem(libutils.LTCWalletAsset, libutils.BTCWalletAsset),
		},
	}, values.DEXCurrencyPairGroup, false)

	pg.serverSelector.Width, pg.marketSelector.Width = dp300, dp300
	pg.serverSelector.MakeCollapsedLayoutVisibleWhenExpanded, pg.marketSelector.MakeCollapsedLayoutVisibleWhenExpanded = true, true
	inset := layout.Inset{Top: values.DP45}
	pg.serverSelector.ExpandedLayoutInset, pg.marketSelector.ExpandedLayoutInset = inset, inset
	pg.serverSelector.BorderWidth, pg.marketSelector.BorderWidth = dp2, dp2
	pg.serverSelector.Hoverable, pg.marketSelector.Hoverable = false, false
	pg.serverSelector.SelectedItemIconColor, pg.marketSelector.SelectedItemIconColor = &pg.Theme.Color.Primary, &pg.Theme.Color.Primary

	// TODO: Fetch orders or order history.
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
func (pg *DEXMarketPage) OnNavigatedFrom() {}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *DEXMarketPage) Layout(gtx C) D {
	pageContent := []func(gtx C) D{
		pg.priceAndVolumeDetail,
		pg.orderFormAndOrderBook,
		pg.openOrdersAndHistory,
	}

	return cryptomaterial.LinearLayout{
		Width:  gtx.Dp(values.AppWidth - 50 /* allow for left and right margin */),
		Height: cryptomaterial.MatchParent,
		Margin: layout.Inset{
			Bottom: values.MarginPadding30,
			Right:  dp10,
			Left:   dp10,
		},
		Direction: layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, index int) D {
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return layout.Inset{Top: 110}.Layout(gtx, func(gtx C) D {
						l := &layout.List{Axis: vertical}
						return l.Layout(gtx, len(pageContent), func(gtx C, i int) D {
							return pageContent[i](gtx)
						})
					})
				}),
				layout.Stacked(pg.serverAndCurrencySelection),
			)
		})
	})
}

func (pg *DEXMarketPage) serverAndCurrencySelection(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     gtx.Dp(100),
		Background: pg.Theme.Color.Surface,
		Padding:    layout.UniformInset(dp16),
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
	}.Layout(gtx,
		layout.Flexed(0.5, func(gtx C) D {
			return layout.Flex{Axis: vertical}.Layout(gtx,
				layout.Rigid(pg.semiBoldLabelText(values.String(values.StrServer)).Layout),
				layout.Rigid(func(gtx C) D {
					pg.serverSelector.Background = &pg.Theme.Color.Surface
					pg.serverSelector.BorderColor = &pg.Theme.Color.Gray5
					return layout.Inset{Top: dp2}.Layout(gtx, pg.serverSelector.Layout)
				}),
			)
		}),
		layout.Flexed(0.5, func(gtx C) D {
			return layout.Inset{Left: values.MarginPadding60}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: vertical, Alignment: layout.End}.Layout(gtx,
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
	// TODO: Fetch rate for the base asset from DEX server and exchange rate from rate source!
	rate := 200.0
	tradeRate := 0.0034353
	var rateStr string
	if rate == 0 {
		rateStr = pg.Printer.Sprintf("%f", tradeRate)
	} else {
		rateStr = pg.Printer.Sprintf("%f (~ %s)", tradeRate, utils.FormatAsUSDString(pg.Printer, tradeRate*rate))
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
			return pg.priceAndVolumeColume(gtx,
				values.String(values.StrPrice),
				pg.semiBoldLabelSize14(rateStr).Layout,
				values.String(values.Str24hLow),
				pg.Printer.Sprintf("%f", 0.0034353 /* TODO: use DEX server value */),
			)
		}),
		layout.Flexed(0.33, func(gtx C) D {
			return pg.priceAndVolumeColume(gtx,
				values.String(values.Str24HChange),
				func(gtx C) D {
					// TODO: Use real values.
					priceChange := 0.0010353
					priceChangePercent := 0.18
					lb := pg.semiBoldLabelSize14(pg.Printer.Sprintf(`%f (%.2f`, priceChange, priceChangePercent) + "%)")
					if priceChangePercent < 0 {
						lb.Color = pg.Theme.Color.OrangeRipple
					} else if priceChangePercent > 0 {
						lb.Color = pg.Theme.Color.GreenText
					}
					return lb.Layout(gtx)
				},
				values.StringF(values.Str24hVolume, "DCR" /* TODO: use market base asset symbol */),
				pg.Printer.Sprintf("%f", 4400.0477380 /* TODO: use DEX server value */),
			)
		}),
		layout.Flexed(0.33, func(gtx C) D {
			return pg.priceAndVolumeColume(gtx,
				values.String(values.Str24hHigh),
				pg.semiBoldLabelSize14(pg.Printer.Sprintf("%f", 0.0034353 /* TODO: USe DEX server price */)).Layout,
				values.StringF(values.Str24hVolume, "BTC" /* TODO: use market quote asset*/),
				pg.Printer.Sprintf("%f", 2.3445532 /* TODO: use DEX server value */),
			)
		}),
	)
}

func (pg *DEXMarketPage) priceAndVolumeColume(gtx C, title1 string, body1 func(gtx C) D, title2, body2 string) D {
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
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: horizontal,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.W.Layout(gtx, pg.orderForm)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, pg.orderbook)
		}),
	)
}

func (pg *DEXMarketPage) orderForm(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:      gtx.Dp(orderFormAndOrderBookWidth),
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
									if pg.switchLotsOrAmount.IsChecked() {
										labelText = values.String(values.StrAmount)
									} else {
										labelText = values.String(values.StrLots)
									}
									return layout.Flex{Axis: horizontal}.Layout(gtx,
										layout.Rigid(pg.semiBoldLabelText(labelText).Layout),
										layout.Flexed(1, func(gtx C) D {
											return layout.E.Layout(gtx, pg.switchLotsOrAmount.Layout)
										}),
									)
								})
							}),
							layout.Rigid(pg.lotsOrAmountEditor.Layout),
							layout.Rigid(func(gtx C) D {
								// TODO: Calculate max buy or max lot
								// depending on user balance of buy or sell
								// asset and use real values below.
								var maxStr string
								if pg.switchLotsOrAmount.IsChecked() { // Amount
									if pg.toggleBuyAndSellBtn.SelectedIndex() == buyBtnStringIndex {
										maxStr = values.StringF(values.StrMaxBuy, 10.089382, "DCR")
									} else {
										maxStr = values.StringF(values.StrMaxSell, 10.008245, "DCR")
									}
								} else {
									maxStr = values.StringF(values.StrMaxLots, 10)
								}
								return layout.Flex{Axis: horizontal}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.E.Layout(gtx, pg.Theme.Label(values.TextSize12, maxStr).Layout)
									}),
								)
							}),
						})
					}),
					layout.Rigid(func(gtx C) D {
						return orderFormRow(gtx, vertical, []layout.FlexChild{
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: dp5}.Layout(gtx, pg.semiBoldLabelText(values.String(values.StrTotal)).Layout)
							}),
							layout.Rigid(pg.totalEditor.Layout),
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: horizontal}.Layout(gtx,
							layout.Rigid(semiBoldLabelGrey3(pg.Theme, values.String(values.StrEstimatedFee)).Layout),
							layout.Rigid(pg.Theme.Label(values.TextSize16, pg.Printer.Sprintf("%f %s", 0.0023434, "DCR" /* TODO: use real value */)).Layout),
						)
					}),
					layout.Rigid(func(gtx C) D {
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
						pg.orderTypesDropdown.Background = &pg.Theme.Color.Surface
						return layout.Inset{Bottom: dp5, Top: dp5}.Layout(gtx, pg.orderTypesDropdown.Layout)
					}),
				)
			}),
		)
	})
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
	// TODO: Use real values
	makeOrderBookBuyOrSell := func(sell bool) []layout.FlexChild {
		var mockOrderBook []layout.FlexChild
		for i := 0; i < maxOrderDisplayedInOrderBook; i++ {
			ord := &order{
				isSell: sell,
				price:  0.001268,
				amount: 1.003422,
				epoch:  34534566,
			}

			mockOrderBook = append(mockOrderBook, layout.Rigid(func(gtx C) D {
				return pg.orderBookRow(gtx,
					textBuyOrSell(pg.Theme, ord.isSell, pg.Printer.Sprintf("%f", ord.price)),
					pg.Theme.Body2(pg.Printer.Sprintf("%f", ord.amount)),
					pg.Theme.Body2(fmt.Sprintf("%d", ord.epoch)),
				)
			}))
		}
		return mockOrderBook
	}

	return cryptomaterial.LinearLayout{
		Width:       gtx.Dp(orderFormAndOrderBookWidth),
		Height:      gtx.Dp(orderFormAndOrderBookHeight),
		Background:  pg.Theme.Color.Surface,
		Margin:      layout.Inset{Top: dp5, Bottom: dp5},
		Padding:     layout.UniformInset(dp16),
		Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(8)},
		Orientation: vertical,
		Direction:   layout.Center,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: horizontal}.Layout(gtx,
				layout.Rigid(pg.semiBoldLabelText(values.String(values.StrOrderBooks)).Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, pg.seeFullOrderBookBtn.Layout)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp10}.Layout(gtx, func(gtx C) D {
				return pg.orderBookRow(gtx,
					semiBoldGray3Size14(pg.Theme, values.StringF(values.StrAssetPrice, "BTC")),
					semiBoldGray3Size14(pg.Theme, values.StringF(values.StrAssetAmount, "DCR")),
					semiBoldGray3Size14(pg.Theme, values.String(values.StrEpoch)),
				)
			})
		}),
		layout.Flexed(0.5, func(gtx C) D {
			return layout.Flex{Axis: vertical}.Layout(gtx, makeOrderBookBuyOrSell(true)...)
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
							// TODO: Fetch rate for the base asset from DEX server and exchange rate from rate source!
							fiatRate := 34000.00
							price := 0.0222445
							lb := pg.Theme.Label(values.TextSize16, pg.Printer.Sprintf("%f %s", price, "DCR"))
							if fiatRate > 0 {
								lb = pg.Theme.Label(values.TextSize16, pg.Printer.Sprintf("%f %s (~ %s)", price, "BTC", utils.FormatAsUSDString(pg.Printer, fiatRate*price)))
							}
							lb.Font.Weight = font.SemiBold
							return lb.Layout(gtx)
						})
					}),
				)
			})
		}),
		layout.Flexed(0.5, func(gtx C) D {
			return layout.Flex{Axis: vertical}.Layout(gtx, makeOrderBookBuyOrSell(false)...)
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
		Margin:  layout.Inset{Bottom: values.MarginPadding9},
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

type order struct {
	ordType string
	isSell  bool
	market  string
	age     string
	price   float64
	amount  float64
	filled  float64
	settled float64
	epoch   uint64
	status  string
}

func (pg *DEXMarketPage) openOrdersAndHistory(gtx C) D {
	headers := []string{values.String(values.StrType), values.String(values.StrPair), values.String(values.StrAge), values.StringF(values.StrAssetPrice, "BTC"), values.StringF(values.StrAssetAmount, "DCR"), values.String(values.StrFilled), values.String(values.StrSettled), values.String(values.StrStatus)}

	sectionWidth := values.AppWidth
	columnWidth := sectionWidth / unit.Dp(len(headers))
	sepWidth := sectionWidth - values.MarginPadding60

	// TODO: Use real values
	var headersFn []layout.FlexChild
	for _, header := range headers {
		headersFn = append(headersFn, pg.orderColumn(true, header, columnWidth))
	}

	// TODO: Use real values
	var orders []*order
	for i := 0; i < 10; i++ {
		ord := &order{
			ordType: "Sell",
			isSell:  true,
			market:  "DCR/BTC",
			age:     "23h 11m",
			price:   0.0023456,
			amount:  23.00457,
			filled:  100.0,
			settled: 70.5,
			status:  "booked",
		}

		orders = append(orders, ord)
	}

	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     gtx.Dp(400),
		Background: pg.Theme.Color.Surface,
		Margin:     layout.Inset{Top: dp5, Bottom: 30},
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
			return pg.Theme.List(pg.ordersTableHorizontalScroll).Layout(gtx, 1, func(gtx C, index int) D {
				gtx.Constraints.Max.X = gtx.Dp(sectionWidth)
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx, headersFn...)
					}),
					layout.Rigid(func(gtx C) D {
						if len(orders) == 0 {
							var noOrderMsg string
							if pg.openOrdersDisplayed { // TODO: Fetch open orders
								noOrderMsg = values.String(values.StrNoOpenOrdersMsg)
							} else { // TODO: Fetch History orders
								noOrderMsg = values.String(values.StrNoTradeHistoryMsg)
							}
							return components.LayoutNoOrderHistoryWithMsg(gtx, pg.Load, false, noOrderMsg)
						}

						return pg.Theme.List(pg.openOrdersAndOrderHistoryContainer).Layout(gtx, len(orders), func(gtx C, index int) D {
							ord := orders[index]
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
									return layout.Flex{Axis: horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
										pg.orderColumn(false, ord.ordType, columnWidth),
										pg.orderColumn(false, ord.market, columnWidth),
										pg.orderColumn(false, ord.age, columnWidth),
										pg.orderColumn(false, pg.Printer.Sprintf("%f", ord.price), columnWidth),
										pg.orderColumn(false, pg.Printer.Sprintf("%f", ord.amount), columnWidth),
										pg.orderColumn(false, pg.Printer.Sprintf("%.1f", ord.filled), columnWidth),
										pg.orderColumn(false, pg.Printer.Sprintf("%.1f", ord.price), columnWidth),
										pg.orderColumn(false, values.String(ord.status), columnWidth),
									)
								}),
								layout.Rigid(func(gtx C) D {
									// No divider for last row
									if index == len(orders)-1 {
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

func (pg *DEXMarketPage) orderColumn(header bool, txt string, columnWidth unit.Dp) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       gtx.Dp(columnWidth),
			Height:      cryptomaterial.WrapContent,
			Orientation: horizontal,
			Alignment:   layout.Middle,
			Padding:     layout.Inset{Top: dp16, Bottom: dp16},
			Direction:   layout.Center,
		}.Layout2(gtx, func(gtx C) D {
			if header {
				return semiBoldGray3Size14(pg.Theme, txt).Layout(gtx)
			}

			lb := pg.Theme.Body2(txt)
			lb.Color = pg.Theme.Color.Text
			return lb.Layout(gtx)
		})
	})
}

func (pg *DEXMarketPage) setBuyOrSell() {
	if pg.toggleBuyAndSellBtn.SelectedIndex() == buyBtnStringIndex { // Buy
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

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (pg *DEXMarketPage) HandleUserInteractions() {
	if pg.serverSelector.Changed() {
		// TODO: Update the order form with required lots.
		log.Info("New sever selected: ", pg.serverSelector.Selected())
	}

	for pg.addServerBtn.Clicked() {
		// TODO: Display modal to add server
		log.Info("Add server clicked")
	}

	for pg.openOrdersBtn.Clicked() {
		pg.openOrdersDisplayed = true
		// TODO: Fetch orders and set pg.orders?
	}

	for pg.orderHistoryBtn.Clicked() {
		pg.openOrdersDisplayed = false
		// TODO: Fetch orders and set pg.orders?
	}

	if pg.orderTypesDropdown.Changed() {
		// TODO: handle order type change
		log.Info("Order type has been changed")
	}

	if pg.toggleBuyAndSellBtn.Changed() {
		pg.setBuyOrSell()
	}

	for pg.seeFullOrderBookBtn.Clicked() {
		// TODO: display full order book
		log.Info("Display full order book")
	}

	for pg.immediateOrderInfoBtn.Clicked() {
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

	// editor event listener
	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(pg.priceEditor.Editor, pg.lotsOrAmountEditor.Editor)
	if isChanged {
		// reset error when any editor is modified
		pg.priceEditor.SetError("")
		pg.lotsOrAmountEditor.SetError("")

		price := pg.priceEditor.Editor.Text()
		if price != "" {
			if price, err := strconv.ParseFloat(price, 64); err != nil || price <= 0 {
				pg.priceEditor.SetError(values.String(values.StrInvalidAmount))
			}
			// TODO: calculate and update total
		}

		lotsOrAmt := pg.lotsOrAmountEditor.Editor.Text()
		if lotsOrAmt != "" {
			if pg.switchLotsOrAmount.IsChecked() { // Amount
				if amt, err := strconv.ParseFloat(lotsOrAmt, 64); err != nil || amt <= 0 {
					pg.lotsOrAmountEditor.SetError(values.String(values.StrInvalidAmount))
				}
				// TODO: calculate and update total
			} else {
				if lot, err := strconv.Atoi(lotsOrAmt); err != nil || lot <= 0 {
					pg.lotsOrAmountEditor.SetError(values.String(values.StrInvalidLot))
				}
				// TODO: calculate and update total
			}
		}
	}

	if isSubmit {
		// TODO: Validate form
		log.Infof("Order form has been submitted..")
	}
}
