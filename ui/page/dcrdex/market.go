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
	DEXMarketPageID              = "dex_market"
	maxOrderDisplayedInOrderBook = 7
)

var (
	u5                          = values.MarginPadding5
	u300                        = unit.Dp(300)
	orderFormAndOrderBookWidth  = (values.AppWidth / 2) - 30
	orderFormAndOrderBookHeight = unit.Dp(515)

	limitOrderIndex = 0
	orderTypes      = []cryptomaterial.DropDownItem{
		{
			Text: values.String(values.StrLimit),
		},
		{
			Text: values.String(values.StrMarket),
		},
	}
)

type DEXMarketPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scrollContainer       *widget.List
	pageContainer         layout.List
	sellOrdersContainer   layout.List
	buyOrdersContainer    layout.List
	openOrdersContainer   *widget.List
	tradeHistoryContainer *widget.List

	serverSelector *cryptomaterial.DropDown
	addServerBtn   *cryptomaterial.Clickable

	marketSelector *cryptomaterial.DropDown

	toggleBuyAndSellBtn *cryptomaterial.ToggleButton
	orderTypesDropdown  *cryptomaterial.DropDown
	isMarketOrder       bool

	priceEditor        cryptomaterial.Editor
	switchLotsOrAmount *cryptomaterial.Switch
	lotsAmountEditor   cryptomaterial.Editor
	totalEditor        cryptomaterial.Editor

	seeFullOrderBookBtn cryptomaterial.Button

	buySellBtn        cryptomaterial.Button
	buyOrder          bool
	immediateOrder    cryptomaterial.CheckBoxStyle
	immediateMoreInfo *cryptomaterial.Clickable

	orders                       []*order
	toggleOpenAndHistoryOrderBtn *cryptomaterial.ToggleButton

	noOrderMsg string
	showLoader bool
}

func NewDEXMarketPage(l *load.Load) *DEXMarketPage {
	th := l.Theme
	pg := &DEXMarketPage{
		Load: l,
		pageContainer: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
		buyOrdersContainer:    layout.List{Axis: layout.Vertical, Alignment: layout.Middle},
		sellOrdersContainer:   layout.List{Axis: layout.Vertical, Alignment: layout.Middle},
		openOrdersContainer:   &widget.List{List: layout.List{Axis: layout.Vertical, Alignment: layout.Middle}},
		tradeHistoryContainer: &widget.List{List: layout.List{Axis: layout.Vertical, Alignment: layout.Middle}},
		GenericPageModal:      app.NewGenericPageModal(DEXAccountOnboardingID),
		scrollContainer:       &widget.List{List: layout.List{Axis: layout.Vertical, Alignment: layout.Middle}},
		priceEditor:           newTextEditor(l.Theme, values.String(values.StrPrice), "0", false),
		totalEditor:           newTextEditor(th, values.String(values.StrTotal), "", false),
		switchLotsOrAmount:    l.Theme.Switch(), // TODO: Set last user choice, default is unchecked.
		lotsAmountEditor:      newTextEditor(l.Theme, values.String(values.StrLots), "0", false),
		buySellBtn:            th.Button(values.String(values.StrBuy)), // TODO: toggle
		buyOrder:              true,
		immediateOrder:        th.CheckBox(new(widget.Bool), values.String(values.StrImmediate)),
		orderTypesDropdown:    th.DropDown(orderTypes, values.DEXOrderTypes, 0),
		immediateMoreInfo:     th.NewClickable(false),
		addServerBtn:          th.NewClickable(false),
		seeFullOrderBookBtn:   th.Button(values.String(values.StrSeeMore)),
	}

	pg.orderTypesDropdown.Width = values.MarginPadding120
	pg.orderTypesDropdown.Color = pg.Theme.Color.Surface
	pg.orderTypesDropdown.FontWeight = font.SemiBold
	pg.orderTypesDropdown.Hoverable = false
	pg.orderTypesDropdown.NavigationIconColor = &pg.Theme.Color.Primary

	buyBtn := th.Button(values.String(values.StrBuy))
	buyBtn.Font.Weight = font.SemiBold
	sellBtn := th.Button(values.String(values.StrSell))
	sellBtn.Font.Weight = font.SemiBold

	toggleBtns := []*cryptomaterial.Button{&buyBtn, &sellBtn}
	pg.toggleBuyAndSellBtn = th.ToggleButton(toggleBtns, false)
	pg.toggleBuyAndSellBtn.SetToggleButtonCallback(func(selectedItem int) {
		if selectedItem == 0 { // Buy
			pg.buyOrder = true
			pg.buySellBtn.Text = values.String(values.StrBuy)
			pg.buySellBtn.Background = pg.Theme.Color.Green500
			pg.buySellBtn.HighlightColor = pg.Theme.Color.Success
		} else if selectedItem == 1 { // Sell
			pg.buyOrder = false
			pg.buySellBtn.Text = values.String(values.StrSell)
			pg.buySellBtn.Background = pg.Theme.Color.Orange
			pg.buySellBtn.HighlightColor = pg.Theme.Color.OrangeRipple
		}
	})
	pg.toggleBuyAndSellBtn.SelectItemAtIndex(0)

	openOrderBtn := th.Button(values.String(values.StrOpenOrders))
	openOrderBtn.Font.Weight = font.SemiBold
	historyOrderBtn := th.Button(values.String(values.StrTradeHistory))
	historyOrderBtn.Font.Weight = font.SemiBold

	toggleOpenAndHistoryOrderBtn := []*cryptomaterial.Button{&openOrderBtn, &historyOrderBtn}
	pg.toggleOpenAndHistoryOrderBtn = th.ToggleButton(toggleOpenAndHistoryOrderBtn, true)
	pg.toggleOpenAndHistoryOrderBtn.SetToggleButtonCallback(func(selectedItem int) {
		if selectedItem == 0 { // TODO: Fetch open orders
			pg.noOrderMsg = values.String(values.StrNoOpenOrdersMsg)
		} else if selectedItem == 1 { // TODO: Fetch History orders
			pg.noOrderMsg = values.String(values.StrNoTradeHistoryMsg)
		}
	})
	pg.toggleOpenAndHistoryOrderBtn.SelectItemAtIndex(0)

	pg.priceEditor.IsTitleLabel = false
	pg.lotsAmountEditor.IsTitleLabel = false
	pg.totalEditor.IsTitleLabel = false

	pg.seeFullOrderBookBtn.HighlightColor, pg.seeFullOrderBookBtn.Background = color.NRGBA{}, color.NRGBA{}
	pg.seeFullOrderBookBtn.Color = th.Color.Primary
	pg.seeFullOrderBookBtn.Font.Weight = font.SemiBold
	pg.seeFullOrderBookBtn.Inset = layout.Inset{}

	pg.immediateOrder.Font.Weight = font.SemiBold

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

	pg.serverSelector = pg.Theme.DropDown(servers, values.DEXServerDropdownGroup, 0 /* TODO: use real value */)
	pg.serverSelector.SetExtraDisplay(pg.addServerDisplay())
	pg.marketSelector = pg.Theme.DropDown([]cryptomaterial.DropDownItem{
		{
			Text:      "DCR/BTC",
			DisplayFn: pg.marketDropdownListItem(libutils.DCRWalletAsset, libutils.BTCWalletAsset),
		},
		{
			Text:      "DCR/LTC",
			DisplayFn: pg.marketDropdownListItem(libutils.DCRWalletAsset, libutils.LTCWalletAsset),
		},
	}, values.DEXCurrencyPairGroup, 0)

	pg.serverSelector.Color = pg.Theme.Color.Surface
	pg.serverSelector.BorderWidth = values.MarginPadding2
	pg.serverSelector.BorderColor = &pg.Theme.Color.Gray5
	pg.serverSelector.NavigationIconColor = &pg.Theme.Color.Primary

	pg.marketSelector.Color = pg.Theme.Color.Surface
	pg.marketSelector.BorderWidth = values.MarginPadding2
	pg.marketSelector.BorderColor = &pg.Theme.Color.Gray5
	pg.marketSelector.NavigationIconColor = &pg.Theme.Color.Primary

	pg.serverSelector.Hoverable, pg.marketSelector.Hoverable = false, false
	pg.serverSelector.Width, pg.marketSelector.Width = u300, u300
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

func (pg *DEXMarketPage) marketDropdownListItem(baseAsset, quoteAsset libutils.AssetType) func(gtx C) D {
	baseIcon, quoteIcon := assetIcon(pg.Theme, baseAsset), assetIcon(pg.Theme, quoteAsset)
	return func(gtx cryptomaterial.C) cryptomaterial.D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if baseIcon == nil {
							return D{}
						}
						return baseIcon.Layout20dp(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: u2, Left: u2}.Layout(gtx, pg.Theme.Label(values.TextSize16, baseAsset.String()).Layout)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: u2, Left: u2}.Layout(gtx, pg.Theme.Label(values.TextSize16, "/").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if quoteIcon == nil {
							return D{}
						}
						return quoteIcon.Layout20dp(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: u2, Left: u2}.Layout(gtx, pg.Theme.Label(values.TextSize16, quoteAsset.String()).Layout)
					}),
				)
			}),
		)
	}
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
	return pg.pageContentLayout(gtx)
}

func (pg *DEXMarketPage) pageContentLayout(gtx C) D {
	pageContent := []func(gtx C) D{
		pg.priceAndVolumeDetail(),
		func(gtx layout.Context) layout.Dimensions {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Horizontal,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.W.Layout(gtx, pg.orderForm())
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, pg.orderbook())
				}),
			)
		},
		pg.openOrdersAndHistory,
	}

	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.MatchParent,
		Direction: layout.Center,
		Padding:   layout.UniformInset(values.MarginPadding20),
	}.Layout2(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:  gtx.Dp(values.AppWidth - 40),
			Height: cryptomaterial.MatchParent,
			Margin: layout.Inset{
				Bottom: values.MarginPadding30,
				Right:  u10,
				Left:   u10,
			},
		}.Layout2(gtx, func(gtx C) D {
			return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 110}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return pg.pageContainer.Layout(gtx, len(pageContent), func(gtx C, i int) D {
								return pageContent[i](gtx)
							})
						})
					}),
					layout.Expanded(pg.serverAndCurrencySelection()),
				)
			})
		})
	})
}

func (pg DEXMarketPage) addServerDisplay() func(gtx C) D {
	return func(gtx cryptomaterial.C) cryptomaterial.D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return pg.Theme.Separator().Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						color := pg.Theme.Color.Primary
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Orientation: layout.Horizontal,
							Padding:     layout.UniformInset(u10),
							Direction:   layout.W,
							Alignment:   layout.Middle,
							Clickable:   pg.addServerBtn,
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								icon := pg.Theme.Icons.ContentAdd
								return icon.Layout(gtx, color)
							}),
							layout.Rigid(func(gtx C) D {
								label := pg.Theme.Label(values.TextSize16, values.String(values.StrAddServer))
								label.Color = color
								label.Font.Weight = font.SemiBold
								return layout.Inset{Left: values.MarginPadding5}.Layout(gtx, label.Layout)
							}),
						)
					}),
				)
			}),
		)
	}
}
func (pg *DEXMarketPage) serverAndCurrencySelection() func(gtx C) D {
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:      cryptomaterial.MatchParent,
			Height:     gtx.Dp(100),
			Background: pg.Theme.Color.Surface,
			Padding:    layout.UniformInset(u16),
			Border: cryptomaterial.Border{
				Radius: cryptomaterial.Radius(8),
			},
		}.Layout(gtx,
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return pg.semiBoldLabelText(gtx, values.String(values.StrServer))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Stack{}.Layout(gtx,
							layout.Expanded(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: u2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return pg.serverSelector.Layout(gtx, 0, false)
								})
							}),
						)
					}),
				)
			}),
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: values.MarginPadding60}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return pg.semiBoldLabelText(gtx, values.String(values.StrMarket))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: u2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return pg.marketSelector.Layout(gtx, 0, false)
							})
						}),
					)
				})
			}),
		)
	}
}

func (pg *DEXMarketPage) priceAndVolumeDetail() func(gtx C) D {
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:      cryptomaterial.MatchParent,
			Height:     cryptomaterial.WrapContent,
			Padding:    layout.UniformInset(16),
			Margin:     layout.Inset{Top: u5, Bottom: u5},
			Background: pg.Theme.Color.Surface,
			Border: cryptomaterial.Border{
				Radius: cryptomaterial.Radius(8),
			},
		}.Layout(gtx,
			layout.Flexed(0.33, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Margin:      layout.Inset{Bottom: u20},
							Orientation: layout.Vertical,
						}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return semiBoldLabelGrey3(pg.Theme, gtx, values.String(values.StrPrice))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								// TODO: Fetch rate for the base asset from DEX server and exchange rate from rate source!
								rate := 200.0
								tradeRate := 0.0034353
								var lb cryptomaterial.Label
								if rate == 0 {
									lb = pg.Theme.Label(values.TextSize14, pg.Printer.Sprintf("%f", tradeRate))
								} else {
									lb = pg.Theme.Label(values.TextSize14, pg.Printer.Sprintf("%f (~ %s)", tradeRate, utils.FormatAsUSDString(pg.Printer, tradeRate*rate)))
								}
								lb.Font.Weight = font.SemiBold
								return lb.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return semiBoldLabelGrey3(pg.Theme, gtx, values.String(values.Str24hLow))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lb := pg.Theme.Label(values.TextSize14, pg.Printer.Sprintf("%f", 0.0034353 /* TODO: use DEX server value */))
								lb.Font.Weight = font.SemiBold
								return lb.Layout(gtx)
							}),
						)
					}),
				)
			}),
			layout.Flexed(0.33, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Margin:      layout.Inset{Bottom: u20},
							Orientation: layout.Vertical,
						}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return semiBoldLabelGrey3(pg.Theme, gtx, values.String(values.Str24hChange))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								priceChange := 0.0010353
								priceChangePercent := 0.18
								lb := pg.Theme.Label(values.TextSize14, pg.Printer.Sprintf(`%f (%.2f`, priceChange, priceChangePercent)+"%)")
								lb.Font.Weight = font.SemiBold
								if priceChangePercent < 0 {
									lb.Color = pg.Theme.Color.OrangeRipple
								} else if priceChangePercent > 0 {
									lb.Color = pg.Theme.Color.GreenText
								}
								return lb.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return semiBoldLabelGrey3(pg.Theme, gtx, values.StringF(values.Str24hVolume, "DCR" /* TODO: use market base asset symbol */))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lb := pg.Theme.Label(values.TextSize14, pg.Printer.Sprintf("%f", 4400.0477380 /* TODO: use DEX server value */))
								lb.Font.Weight = font.SemiBold
								return lb.Layout(gtx)
							}),
						)
					}),
				)
			}),
			layout.Flexed(0.33, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Margin:      layout.Inset{Bottom: u20},
							Orientation: layout.Vertical,
						}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return semiBoldLabelGrey3(pg.Theme, gtx, values.String(values.Str24hHigh))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lb := pg.Theme.Label(values.TextSize14, pg.Printer.Sprintf("%f", 0.0034353 /* TODO: USe DEX server price */))
								lb.Font.Weight = font.SemiBold
								return lb.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return semiBoldLabelGrey3(pg.Theme, gtx, values.StringF(values.Str24hVolume, "BTC" /* TODO: use market quote asset*/))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lb := pg.Theme.Label(values.TextSize14, pg.Printer.Sprintf("%f", 2.3445532 /* TODO: use DEX server value */))
								lb.Font.Weight = font.SemiBold
								return lb.Layout(gtx)
							}),
						)
					}),
				)
			}),
		)
	}
}

func (pg *DEXMarketPage) orderForm() func(gtx C) D {
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:      gtx.Dp(orderFormAndOrderBookWidth),
			Height:     gtx.Dp(orderFormAndOrderBookHeight),
			Background: pg.Theme.Color.Surface,
			Margin:     layout.Inset{Top: u5, Bottom: u5},
			Padding:    layout.UniformInset(u16),
			Direction:  layout.E,
			Border: cryptomaterial.Border{
				Radius: cryptomaterial.Radius(8),
			},
			Orientation: layout.Vertical,
		}.Layout2(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{Alignment: layout.NW}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.MatchParent,
						Height:      cryptomaterial.WrapContent,
						Margin:      layout.Inset{Top: 70},
						Orientation: layout.Vertical,
					}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return cryptomaterial.LinearLayout{
								Width:       cryptomaterial.MatchParent,
								Height:      cryptomaterial.WrapContent,
								Margin:      layout.Inset{Bottom: u10, Top: u10},
								Orientation: layout.Vertical,
							}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return pg.semiBoldLabelText(gtx, values.String(values.StrPrice))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return pg.priceEditor.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return cryptomaterial.LinearLayout{
								Width:       cryptomaterial.MatchParent,
								Height:      cryptomaterial.WrapContent,
								Margin:      layout.Inset{Bottom: u10, Top: u10},
								Orientation: layout.Vertical,
							}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Bottom: u5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												var labelText string
												if pg.switchLotsOrAmount.IsChecked() {
													labelText = values.String(values.StrAmount)
												} else {
													labelText = values.String(values.StrLots)
												}
												return pg.semiBoldLabelText(gtx, labelText)
											}),
											layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
												return layout.E.Layout(gtx, pg.switchLotsOrAmount.Layout)
											}),
										)
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return pg.lotsAmountEditor.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											// TODO: Calculate max buy or max lot
											// depending on user balance of buy or sell
											// asset and use real values below.
											var maxStr string
											if pg.switchLotsOrAmount.IsChecked() { // Amount
												if pg.buyOrder {
													maxStr = values.StringF(values.StrMaxBuy, 10.089382, "DCR")
												} else {
													maxStr = values.StringF(values.StrMaxSell, 10.008245, "DCR")
												}
											} else {
												maxStr = values.StringF(values.StrMaxLots, 10)
											}
											return layout.E.Layout(gtx, pg.Theme.Label(values.TextSize12, maxStr).Layout)
										}),
									)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return cryptomaterial.LinearLayout{
								Width:       cryptomaterial.MatchParent,
								Height:      cryptomaterial.WrapContent,
								Margin:      layout.Inset{Bottom: u10, Top: u10},
								Orientation: layout.Vertical,
							}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Bottom: u5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return pg.semiBoldLabelText(gtx, values.String(values.StrTotal))
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return pg.totalEditor.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return semiBoldLabelGrey3(pg.Theme, gtx, values.String(values.StrEstimatedFee))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return pg.Theme.Label(values.TextSize16, pg.Printer.Sprintf("%f %s", 0.0023434, "DCR" /* TODO: use real value */)).Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return cryptomaterial.LinearLayout{
								Width:       cryptomaterial.MatchParent,
								Height:      cryptomaterial.WrapContent,
								Margin:      layout.Inset{Bottom: u10, Top: u10},
								Orientation: layout.Horizontal,
							}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return pg.immediateOrder.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return cryptomaterial.LinearLayout{
										Width:     cryptomaterial.WrapContent,
										Height:    cryptomaterial.WrapContent,
										Clickable: pg.immediateMoreInfo,
										Padding:   layout.Inset{Top: u10, Left: u2},
									}.Layout2(gtx, pg.Theme.Icons.InfoAction.Layout16dp)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, pg.buySellBtn.Layout),
							)
						}),
					)
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return pg.toggleBuyAndSellBtn.Layout(gtx)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Bottom: u10, Top: u10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return pg.orderTypesDropdown.Layout(gtx, 0, true)
							})
						}),
					)
				}),
			)
		})
	}
}

type orderBook struct {
	price  float64
	amount float64
	epoch  uint64
}

func (pg *DEXMarketPage) orderbook() func(gtx C) D {
	// TODO: Use real values
	var mockOrderBook []*orderBook
	for i := 0; i < maxOrderDisplayedInOrderBook; i++ {
		mockOrderBook = append(mockOrderBook, &orderBook{
			price:  0.001268,
			amount: 1.003422,
			epoch:  34534566,
		})
	}
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:      gtx.Dp(orderFormAndOrderBookWidth),
			Height:     gtx.Dp(orderFormAndOrderBookHeight), // TODO...
			Background: pg.Theme.Color.Surface,
			Margin:     layout.Inset{Top: u5, Bottom: u5},
			Padding:    layout.UniformInset(u16),
			Border: cryptomaterial.Border{
				Radius: cryptomaterial.Radius(8),
			},
			Orientation: layout.Vertical,
			Direction:   layout.Center,
		}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return pg.semiBoldLabelText(gtx, values.String(values.StrOrderBooks))
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, pg.seeFullOrderBookBtn.Layout)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.MatchParent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Horizontal,
					Direction:   layout.Center,
					Spacing:     layout.SpaceBetween,
					Margin:      layout.Inset{Top: u10, Bottom: u10},
				}.Layout(gtx,
					layout.Flexed(0.33, func(gtx layout.Context) layout.Dimensions {
						return semiBoldGray3Size14(pg.Theme, gtx, values.StringF(values.StrAssetPrice, "BTC"))
					}), // Price
					layout.Flexed(0.33, func(gtx layout.Context) layout.Dimensions {
						return semiBoldGray3Size14(pg.Theme, gtx, values.StringF(values.StrAssetAmount, "DCR"))
					}), // Amount
					layout.Flexed(0.33, func(gtx layout.Context) layout.Dimensions {
						return semiBoldGray3Size14(pg.Theme, gtx, values.String(values.StrEpoch))
					}), // Epoch
				)
			}),
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return pg.sellOrdersContainer.Layout(gtx, len(mockOrderBook), func(gtx layout.Context, index int) layout.Dimensions {
					sell := true
					ord := mockOrderBook[index]
					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.MatchParent,
						Height:      cryptomaterial.WrapContent,
						Orientation: layout.Horizontal,
						Spacing:     layout.SpaceBetween,
						Alignment:   layout.Middle,
						Direction:   layout.Center,
					}.Layout(gtx,
						layout.Flexed(0.33, colorOrderBookText(pg.Theme, &sell, pg.Printer.Sprintf("%f", ord.price))), // Price
						layout.Flexed(0.33, colorOrderBookText(pg.Theme, nil, pg.Printer.Sprintf("%f", ord.amount))),  // Amount
						layout.Flexed(0.33, colorOrderBookText(pg.Theme, nil, fmt.Sprintf("%d", ord.epoch))),          // Epoch
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.MatchParent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Horizontal,
					Margin: layout.Inset{
						Bottom: u5,
						Top:    u5,
					},
					Direction: layout.Center,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Stack{Alignment: layout.Center}.Layout(gtx,
							layout.Stacked(func(gtx C) D {
								sep := pg.Theme.Separator()
								return layout.Inset{Top: u5}.Layout(gtx, sep.Layout)
							}),
							layout.Expanded(func(gtx C) D {
								return cryptomaterial.LinearLayout{
									Width:      cryptomaterial.WrapContent,
									Height:     cryptomaterial.WrapContent,
									Background: pg.Theme.Color.Gray3,
									Padding: layout.Inset{
										Top:    u5,
										Bottom: u5,
										Right:  u16,
										Left:   u16,
									},
									Border: cryptomaterial.Border{
										Radius: cryptomaterial.Radius(16),
									},
									Direction:   layout.Center,
									Orientation: layout.Horizontal,
								}.Layout2(gtx, func(gtx layout.Context) layout.Dimensions {
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
					}),
				)
			}),
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return pg.buyOrdersContainer.Layout(gtx, len(mockOrderBook), func(gtx layout.Context, index int) layout.Dimensions {
					ord := mockOrderBook[index]
					sell := false
					return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Flexed(0.33, colorOrderBookText(pg.Theme, &sell, pg.Printer.Sprintf("%f", ord.price))), // Price
						layout.Flexed(0.33, colorOrderBookText(pg.Theme, nil, pg.Printer.Sprintf("%f", ord.amount))),  // Amount
						layout.Flexed(0.33, colorOrderBookText(pg.Theme, nil, fmt.Sprintf("%d", ord.epoch))),          // Epoch
					)
				})
			}),
		)
	}
}

func colorOrderBookText(th *cryptomaterial.Theme, sell *bool, txt string) func(gtx C) D {
	lb := th.Body2(txt)
	if sell != nil {
		if *sell {
			lb.Color = th.Color.OrangeRipple
		} else {
			lb.Color = th.Color.Green500
		}
	}
	return func(gtx C) D {
		return layout.Inset{Bottom: u10}.Layout(gtx, lb.Layout)
	}
}

type order struct {
	ordType string
	market  string
	age     string
	price   float64
	amount  float64
	filled  float64
	settled float64
}

func (pg *DEXMarketPage) openOrdersAndHistory(gtx C) D {
	sectionWidth := values.AppWidth - values.MarginPadding80
	columnWidth := sectionWidth / 7
	sepWidth := sectionWidth - values.MarginPadding60

	headers := []string{values.String(values.StrType), values.String(values.StrPair), values.String(values.StrAge), values.StringF(values.StrAssetPrice, "BTC"), values.StringF(values.StrAssetAmount, "DCR"), values.String(values.StrFilled), values.String(values.StrSettled)}
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
			market:  "DCR/BTC",
			age:     "23h 11m",
			price:   0.0023456,
			amount:  23.00457,
			filled:  100.0,
			settled: 70.5,
		}

		orders = append(orders, ord)
	}

	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     gtx.Dp(400), // TODO...
		Background: pg.Theme.Color.Surface,
		Margin:     layout.Inset{Top: u5, Bottom: 30},
		Padding:    layout.UniformInset(u10),
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
		Orientation: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return pg.toggleOpenAndHistoryOrderBtn.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx, headersFn...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(orders) == 0 {
				return components.LayoutNoOrderHistoryWithMsg(gtx, pg.Load, false, pg.noOrderMsg)
			}

			return pg.Theme.List(pg.openOrdersContainer).Layout(gtx, len(orders), func(gtx layout.Context, index int) layout.Dimensions {
				ord := orders[index]
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if index == 0 {
							sep := pg.Theme.Separator()
							sep.Width = gtx.Dp(sepWidth)
							return layout.Center.Layout(gtx, sep.Layout)
						}
						return D{}
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
							pg.orderColumn(false, ord.ordType, columnWidth),
							pg.orderColumn(false, ord.market, columnWidth),
							pg.orderColumn(false, ord.age, columnWidth),
							pg.orderColumn(false, pg.Printer.Sprintf("%f", ord.price), columnWidth),
							pg.orderColumn(false, pg.Printer.Sprintf("%f", ord.amount), columnWidth),
							pg.orderColumn(false, pg.Printer.Sprintf("%.1f", ord.filled), columnWidth),
							pg.orderColumn(false, pg.Printer.Sprintf("%.1f", ord.price), columnWidth),
						)
					}),
					layout.Rigid(func(gtx C) D {
						// No divider for last row
						if index == len(orders)-1 {
							return layout.Dimensions{}
						}
						sep := pg.Theme.Separator()
						sep.Width = gtx.Dp(sepWidth)
						return layout.Center.Layout(gtx, sep.Layout)
					}),
				)
			})
		}),
	)
}

func semiBoldGray3Size14(th *cryptomaterial.Theme, gtx C, text string) D {
	lb := th.Label(values.TextSize14, text)
	lb.Color = th.Color.GrayText3
	lb.Font.Weight = font.SemiBold
	return lb.Layout(gtx)
}

func (pg *DEXMarketPage) orderColumn(header bool, txt string, columnWidth unit.Dp) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return cryptomaterial.LinearLayout{
			Width:       gtx.Dp(columnWidth),
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Horizontal,
			Alignment:   layout.Middle,
			Padding:     layout.Inset{Top: u16, Bottom: u16},
			Direction:   layout.Center,
		}.Layout2(gtx, func(gtx layout.Context) layout.Dimensions {
			if header {
				return semiBoldGray3Size14(pg.Theme, gtx, txt)
			}

			lb := pg.Theme.Body2(txt)
			lb.Color = pg.Theme.Color.Text
			return lb.Layout(gtx)
		})
	})
}

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (pg *DEXMarketPage) HandleUserInteractions() {
	if pg.serverSelector.Changed() {
		// TODO: Update the order form with required lots.
		fmt.Println("New sever selected: ", pg.serverSelector.Selected())
	}

	for pg.addServerBtn.Clicked() {
		// TODO: Display modal to add server
		fmt.Println("Add server clicked")
	}

	if pg.orderTypesDropdown.Changed() {
		pg.isMarketOrder = pg.orderTypesDropdown.SelectedIndex() != limitOrderIndex
	}

	for pg.seeFullOrderBookBtn.Clicked() {
		// TODO: display full order book
		fmt.Println("Display full order book")
	}

	for pg.immediateMoreInfo.Clicked() {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrImmediateOrder)).
			SetupWithTemplate(values.String(values.StrImmediateExplanation)).
			SetCancelable(true).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetPositiveButtonText(values.String(values.StrOk))
		pg.ParentWindow().ShowModal(infoModal)
	}

	// editor event listener
	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(pg.priceEditor.Editor, pg.lotsAmountEditor.Editor)
	if isChanged {
		// reset error when any editor is modified
		pg.priceEditor.SetError("")
		pg.lotsAmountEditor.SetError("")

		price := pg.priceEditor.Editor.Text()
		if price != "" {
			if price, err := strconv.ParseFloat(price, 64); err != nil || price <= 0 {
				pg.priceEditor.SetError(values.String(values.StrInvalidAmount))
			} else {
				// TODO: calculate and update total
			}
		}

		lotsOrAmt := pg.lotsAmountEditor.Editor.Text()
		if lotsOrAmt != "" {
			if pg.switchLotsOrAmount.IsChecked() { // Amount
				if amt, err := strconv.ParseFloat(lotsOrAmt, 64); err != nil || amt <= 0 {
					pg.lotsAmountEditor.SetError(values.String(values.StrInvalidAmount))
				} else {
					// TODO: calculate and update total
				}
			} else {
				if lot, err := strconv.Atoi(lotsOrAmt); err != nil || lot <= 0 {
					pg.lotsAmountEditor.SetError(values.String(values.StrInvalidLot))
				} else {
					// TODO: calculate and update total
				}
			}
		}
	}

	if isSubmit {
		// TODO: Validate form
	}
}

func (pg *DEXMarketPage) semiBoldLabelText(gtx C, title string) D {
	lb := pg.Theme.Label(values.TextSize16, title)
	lb.Font.Weight = font.SemiBold
	lb.Color = pg.Theme.Color.Text
	return lb.Layout(gtx)
}
