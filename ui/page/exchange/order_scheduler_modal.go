package exchange

import (
	"context"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
	api "github.com/crypto-power/instantswap/instantswap"
)

type orderSchedulerModal struct {
	*load.Load
	*cryptomaterial.Modal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	pageContainer *widget.List

	orderSchedulerStarted func()
	onCancel              func()

	cancelBtn              cryptomaterial.Button
	startBtn               cryptomaterial.Button
	refreshExchangeRateBtn cryptomaterial.IconButton

	balanceToMaintain          cryptomaterial.Editor
	balanceToMaintainErrorText string
	passwordEditor             cryptomaterial.Editor
	copyRedirect               *cryptomaterial.Clickable

	exchangeSelector  *ExSelector
	frequencySelector *FrequencySelector

	materialLoader material.LoaderStyle

	isStarting   bool
	fetchingRate bool
	rateError    bool

	exchangeRate float64
	exchange     api.IDExchange
	*orderData
	instantCurrencies []api.Currency
}

func newOrderSchedulerModalModal(l *load.Load, data *orderData) *orderSchedulerModal {
	osm := &orderSchedulerModal{
		Load:              l,
		Modal:             l.Theme.ModalFloatTitle(values.String(values.StrOrderScheduler), l.IsMobileView(), nil),
		exchangeSelector:  NewExSelector(l, instantswap.FlypMe),
		frequencySelector: NewFrequencySelector(l),
		orderData:         data,
		copyRedirect:      l.Theme.NewClickable(false),
		exchangeRate:      -1,
	}

	osm.cancelBtn = l.Theme.OutlineButton(values.String(values.StrCancel))
	osm.cancelBtn.Font.Weight = font.Medium

	osm.startBtn = l.Theme.Button(values.String(values.StrStart))
	osm.startBtn.Font.Weight = font.Medium
	osm.startBtn.SetEnabled(false)

	osm.refreshExchangeRateBtn = l.Theme.IconButton(l.Theme.Icons.NavigationRefresh)
	osm.refreshExchangeRateBtn.Size = values.MarginPadding18
	osm.refreshExchangeRateBtn.Inset = layout.UniformInset(values.MarginPadding0)

	osm.balanceToMaintain = l.Theme.Editor(new(widget.Editor), values.StringF(values.StrBalanceToMaintain, osm.fromCurrency))
	osm.balanceToMaintain.Editor.SingleLine, osm.balanceToMaintain.Editor.Submit = true, true

	osm.passwordEditor = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrSpendingPassword))
	osm.passwordEditor.Editor.SetText("")
	osm.passwordEditor.Editor.SingleLine = true
	osm.passwordEditor.Editor.Submit = true

	osm.materialLoader = material.Loader(l.Theme.Base)

	osm.pageContainer = &widget.List{
		List: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
	}

	osm.exchangeSelector.ExchangeSelected(func(es *Exchange) {
		// Initialize a new exchange using the selected exchange server
		exchange, err := osm.AssetsManager.InstantSwap.NewExchangeServer(es.Server)
		if err != nil {
			log.Error(err)
			return
		}

		osm.exchange = exchange

		go func() {
			err := osm.fetchInstantExchangeCurrencies()
			if err != nil {
				log.Error(err)
				return
			}
			err = osm.getExchangeRateInfo()
			if err != nil {
				log.Error(err)
			}
		}()
	})

	return osm
}

func (osm *orderSchedulerModal) OnOrderSchedulerStarted(orderSchedulerStarted func()) *orderSchedulerModal {
	osm.orderSchedulerStarted = orderSchedulerStarted
	return osm
}

func (osm *orderSchedulerModal) OnCancel(cancel func()) *orderSchedulerModal {
	osm.onCancel = cancel
	return osm
}

func (osm *orderSchedulerModal) OnResume() {
	osm.ctx, osm.ctxCancel = context.WithCancel(context.TODO())
}

func (osm *orderSchedulerModal) setLoading(loading bool) {
	osm.isStarting = loading
}

func (osm *orderSchedulerModal) OnDismiss() {
	osm.ctxCancel()
}

func (osm *orderSchedulerModal) SetError(err string) {
	osm.passwordEditor.SetError(values.TranslateErr(err))
}

func (osm *orderSchedulerModal) Handle(gtx C) {
	osm.startBtn.SetEnabled(osm.canStart())

	for osm.startBtn.Clicked(gtx) {
		osm.startOrderScheduler()
	}

	if osm.cancelBtn.Clicked(gtx) || osm.Modal.BackdropClicked(gtx, true) {
		osm.onCancel()
		osm.Dismiss()
	}

	if osm.refreshExchangeRateBtn.Button.Clicked(gtx) {
		go func() {
			err := osm.getExchangeRateInfo()
			if err != nil {
				log.Error(err)
			}
		}()
	}

	for {
		event, ok := osm.balanceToMaintain.Editor.Update(gtx)
		if !ok {
			break
		}

		if gtx.Source.Focused(osm.balanceToMaintain.Editor) {
			switch event.(type) {
			case widget.ChangeEvent:
				if components.InputsNotEmpty(osm.balanceToMaintain.Editor) {
					f, err := strconv.ParseFloat(osm.balanceToMaintain.Editor.Text(), 32)
					if err != nil {
						osm.balanceToMaintainErrorText = values.String(values.StrInvalidAmount)
						osm.balanceToMaintain.LineColor = osm.Theme.Color.Danger
						return
					}

					if f >= osm.sourceAccountSelector.SelectedAccount().Balance.Spendable.ToCoin() || f < 0 {
						osm.balanceToMaintainErrorText = values.String(values.StrInvalidAmount)
						osm.balanceToMaintain.LineColor = osm.Theme.Color.Danger
						return
					}
					osm.balanceToMaintainErrorText = ""

				}
			}
		}
	}
}

func (osm *orderSchedulerModal) canStart() bool {
	if osm.exchangeSelector.selectedExchange == nil {
		return false
	}

	if osm.frequencySelector.selectedFrequency == nil {
		return false
	}

	if osm.balanceToMaintain.Editor.Text() == "" {
		return false
	}

	if osm.balanceToMaintainErrorText != "" {
		return false
	}

	return true
}

func (osm *orderSchedulerModal) Layout(gtx layout.Context) D {
	w := []layout.Widget{
		func(gtx C) D {
			return layout.Stack{Alignment: layout.S}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return layout.Stack{Alignment: layout.NE}.Layout(gtx,
						layout.Expanded(func(gtx C) D {
							return layout.Inset{
								Bottom: values.MarginPadding16,
							}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return layout.Inset{
											Bottom: values.MarginPadding16,
										}.Layout(gtx, func(gtx C) D {
											txt := osm.Theme.Label(values.TextSize20, values.String(values.StrOrderScheduler))
											txt.Font.Weight = font.SemiBold
											return txt.Layout(gtx)
										})
									}),
									layout.Rigid(func(gtx C) D {
										return osm.Theme.List(osm.pageContainer).Layout(gtx, 1, func(gtx C, _ int) D {
											return cryptomaterial.LinearLayout{
												Width:     cryptomaterial.MatchParent,
												Height:    cryptomaterial.WrapContent,
												Direction: layout.Center,
											}.Layout2(gtx, func(gtx C) D {
												return cryptomaterial.LinearLayout{
													Width:  cryptomaterial.MatchParent,
													Height: cryptomaterial.WrapContent,
												}.Layout2(gtx, func(gtx C) D {
													return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																Bottom: values.MarginPadding16,
															}.Layout(gtx, func(gtx C) D {
																return cryptomaterial.LinearLayout{
																	Width:       cryptomaterial.MatchParent,
																	Height:      cryptomaterial.WrapContent,
																	Orientation: layout.Vertical,
																	Margin:      layout.Inset{Bottom: values.MarginPadding4},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return osm.exchangeSelector.Layout(osm.ParentWindow(), gtx)
																	}),
																	layout.Rigid(func(gtx C) D {
																		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				if osm.rateError {
																					txt := osm.Theme.Label(values.TextSize14, values.String(values.StrFetchRateError))
																					txt.Color = osm.Theme.Color.Gray1
																					txt.Font.Weight = font.SemiBold
																					return txt.Layout(gtx)
																				}

																				if osm.fetchingRate {
																					gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding16)
																					gtx.Constraints.Min.X = gtx.Constraints.Max.X
																					return osm.materialLoader.Layout(gtx)
																				}

																				fromCur := osm.fromCurrency.String()
																				toCur := osm.toCurrency.String()
																				missingAsset := fromCur == "" || toCur == ""
																				if osm.exchangeSelector.SelectedExchange() != nil && osm.exchangeRate > 0 && !missingAsset {
																					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																						layout.Rigid(func(gtx C) D {
																							exName := osm.exchangeSelector.SelectedExchange().Name
																							exchangeRate := values.StringF(values.StrServerRate, exName, fromCur, osm.exchangeRate, toCur)
																							txt := osm.Theme.Label(values.TextSize14, exchangeRate)
																							txt.Font.Weight = font.SemiBold
																							txt.Color = osm.Theme.Color.Gray1
																							return txt.Layout(gtx)
																						}),
																						layout.Rigid(func(gtx C) D {
																							market := values.NewMarket(fromCur, toCur)
																							ticker := osm.AssetsManager.RateSource.GetTicker(market, true)
																							if ticker == nil || ticker.LastTradePrice <= 0 {
																								return D{}
																							}

																							rate := ticker.LastTradePrice
																							//  Binance and Bittrex always returns
																							// ticker.LastTradePrice in's the quote
																							// asset unit e.g DCR-BTC, LTC-BTC. We will
																							// also do this when and if USDT is
																							// supported.
																							if osm.fromCurrency == libutils.BTCWalletAsset {
																								rate = 1 / ticker.LastTradePrice
																							}

																							binanceRate := values.StringF(values.StrCurrencyConverterRate, osm.AssetsManager.RateSource.Name(), fromCur, rate, toCur)
																							txt := osm.Theme.Label(values.TextSize14, binanceRate)
																							txt.Font.Weight = font.SemiBold
																							txt.Color = osm.Theme.Color.Gray1
																							return txt.Layout(gtx)
																						}),
																					)
																				}
																				return D{}
																			}),
																			layout.Rigid(func(gtx C) D {
																				if !osm.fetchingRate && osm.rateError {
																					return osm.refreshExchangeRateBtn.Layout(gtx)
																				}
																				return D{}
																			}),
																		)
																	}),
																)
															})
														}),
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																Bottom: values.MarginPadding16,
															}.Layout(gtx, func(gtx C) D {
																return cryptomaterial.LinearLayout{
																	Width:       cryptomaterial.MatchParent,
																	Height:      cryptomaterial.WrapContent,
																	Orientation: layout.Vertical,
																	Margin:      layout.Inset{Bottom: values.MarginPadding4},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return osm.frequencySelector.Layout(osm.ParentWindow(), gtx)
																	}),
																)
															})
														}),
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																Bottom: values.MarginPadding16,
															}.Layout(gtx, func(gtx C) D {
																return cryptomaterial.LinearLayout{
																	Width:       cryptomaterial.MatchParent,
																	Height:      cryptomaterial.WrapContent,
																	Orientation: layout.Vertical,
																	Margin:      layout.Inset{Bottom: values.MarginPadding4},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return osm.balanceToMaintain.Layout(gtx)
																	}),
																	layout.Rigid(func(gtx C) D {
																		if osm.balanceToMaintainErrorText != "" {
																			txt := osm.Theme.Label(values.TextSize14, osm.balanceToMaintainErrorText)
																			txt.Font.Weight = font.SemiBold
																			txt.Color = osm.Theme.Color.Danger
																			return txt.Layout(gtx)
																		}
																		return D{}
																	}),
																)
															})
														}),
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																Bottom: values.MarginPadding16,
															}.Layout(gtx, func(gtx C) D {
																return osm.passwordEditor.Layout(gtx)
															})
														}),
														layout.Rigid(func(gtx C) D {
															return cryptomaterial.LinearLayout{
																Width:      cryptomaterial.MatchParent,
																Height:     cryptomaterial.WrapContent,
																Background: osm.Theme.Color.Gray8,
																Padding: layout.Inset{
																	Top:    values.MarginPadding12,
																	Bottom: values.MarginPadding12,
																	Left:   values.MarginPadding8,
																	Right:  values.MarginPadding8,
																},
																Border:    cryptomaterial.Border{Radius: cryptomaterial.Radius(8)},
																Direction: layout.Center,
																Alignment: layout.Middle,
															}.Layout2(gtx, func(gtx C) D {
																return layout.Inset{Bottom: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
																	msg := values.String(values.StrOrderSchedulerInfo)
																	txt := osm.Theme.Label(values.TextSize14, msg)
																	txt.Alignment = text.Middle
																	txt.Color = osm.Theme.Color.GrayText3
																	if osm.AssetsManager.IsDarkModeOn() {
																		txt.Color = osm.Theme.Color.Gray3
																	}
																	return txt.Layout(gtx)
																})
															})
														}),
													)
												})
											})
										})
									}),
								)
							})
						}),
					)
				}),
				layout.Stacked(func(gtx C) D {
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y

					return layout.S.Layout(gtx, func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							c := osm.Theme.Card()
							c.Radius = cryptomaterial.Radius(0)
							return c.Layout(gtx, func(gtx C) D {
								inset := layout.Inset{
									Top: values.MarginPadding16,
								}
								return inset.Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
										layout.Flexed(1, func(gtx C) D {
											return layout.E.Layout(gtx, func(gtx C) D {
												return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
													layout.Rigid(func(gtx C) D {
														return layout.Inset{
															Right: values.MarginPadding4,
														}.Layout(gtx, osm.cancelBtn.Layout)
													}),
													layout.Rigid(func(gtx C) D {
														if osm.isStarting {
															return layout.Inset{Top: unit.Dp(7)}.Layout(gtx, func(gtx C) D {
																return material.Loader(osm.Theme.Base).Layout(gtx)
															})
														}
														return osm.startBtn.Layout(gtx)
													}),
												)
											})
										}),
									)
								})
							})
						})
					})
				}),
			)
		},
	}
	return osm.Modal.Layout(gtx, w)
}

func (osm *orderSchedulerModal) startOrderScheduler() {
	go func() {
		osm.setLoading(true)
		err := osm.sourceWalletSelector.SelectedWallet().UnlockWallet(osm.passwordEditor.Editor.Text())
		if err != nil {
			osm.SetError(err.Error())
			osm.setLoading(false)
			return
		}

		balanceToMaintain, _ := strconv.ParseFloat(osm.balanceToMaintain.Editor.Text(), 32)
		params := instantswap.SchedulerParams{
			Order: instantswap.Order{
				ExchangeServer:           osm.exchangeSelector.selectedExchange.Server,
				SourceWalletID:           osm.orderData.sourceWalletID,
				SourceAccountNumber:      osm.orderData.sourceAccountNumber,
				DestinationWalletID:      osm.orderData.destinationWalletID,
				DestinationAccountNumber: osm.orderData.destinationAccountNumber,

				FromCurrency: osm.orderData.fromCurrency.String(),
				ToCurrency:   osm.orderData.toCurrency.String(),
				FromNetwork:  osm.orderData.fromNetwork,
				ToNetwork:    osm.orderData.toNetwork,
				Provider:     osm.orderData.provider,
				Signature:    osm.orderData.signature,

				DestinationAddress: osm.orderData.destinationAddress,
				RefundAddress:      osm.orderData.refundAddress,
			},

			Frequency:          osm.frequencySelector.selectedFrequency.item,
			BalanceToMaintain:  balanceToMaintain,
			SpendingPassphrase: osm.passwordEditor.Editor.Text(),
		}

		go func() {
			_ = osm.AssetsManager.StartScheduler(context.Background(), params)
		}()

		osm.Dismiss()
		osm.orderSchedulerStarted()
	}()
}

func (osm *orderSchedulerModal) fetchInstantExchangeCurrencies() error {
	osm.fetchingRate = true
	currencies, err := osm.exchange.GetCurrencies()
	osm.instantCurrencies = currencies
	osm.fetchingRate = false
	osm.rateError = err != nil
	return err
}

func getNetwork(coinName string, currencies []api.Currency) string {
	var lowerName = strings.ToLower(coinName)
	var currency *api.Currency
	for _, c := range currencies {
		if strings.ToLower(c.Symbol) == lowerName {
			currency = &c
			break
		}
	}
	if currency == nil || len(currency.Networks) == 0 {
		return ""
	}
	for _, network := range currency.Networks {
		var lowerNetwork = strings.ToLower(network)
		if lowerNetwork == string(libutils.Mainnet) {
			return network
		}
		if lowerNetwork == lowerName {
			return network
		}
	}
	return currency.Networks[0]
}

func (osm *orderSchedulerModal) getExchangeRateInfo() error {
	osm.exchangeRate = -1
	osm.fetchingRate = true
	osm.rateError = false
	fromCur := osm.fromCurrency.String()
	toCur := osm.toCurrency.String()
	params := api.ExchangeRateRequest{
		From:        fromCur,
		FromNetwork: getNetwork(fromCur, osm.instantCurrencies),
		To:          toCur,
		ToNetwork:   getNetwork(toCur, osm.instantCurrencies),
		Amount:      libwallet.DefaultRateRequestAmt(fromCur), // amount needs to be greater than 0 to get the exchange rate
	}
	res, err := osm.AssetsManager.InstantSwap.GetExchangeRateInfo(osm.exchange, params)
	osm.fetchingRate = false
	if err != nil {
		osm.rateError = true
		return err
	}
	osm.orderData.fromNetwork = params.FromNetwork
	osm.orderData.toNetwork = params.ToNetwork
	osm.orderData.provider = res.Provider
	osm.orderData.signature = res.Signature

	osm.exchangeRate = res.ExchangeRate // estimated receivable value for libwallet.DefaultRateRequestAmount (1)
	osm.rateError = false
	return nil
}
