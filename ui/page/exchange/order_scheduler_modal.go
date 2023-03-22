package exchange

import (
	"context"
	"strconv"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"code.cryptopower.dev/group/cryptopower/libwallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/ext"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	api "code.cryptopower.dev/group/instantswap"
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

	exchangeSelector  *ExchangeSelector
	frequencySelector *FrequencySelector

	materialLoader material.LoaderStyle

	isStarting   bool
	fetchingRate bool
	rateError    bool

	exchangeRate, binanceRate float64
	exchange                  api.IDExchange
	*orderData
}

func newOrderSchedulerModalModal(l *load.Load, data *orderData) *orderSchedulerModal {
	osm := &orderSchedulerModal{
		Load:              l,
		Modal:             l.Theme.ModalFloatTitle(values.String(values.StrOrderScheduler)),
		exchangeSelector:  NewExchangeSelector(l, instantswap.FlypMe),
		frequencySelector: NewFrequencySelector(l),
		orderData:         data,
		copyRedirect:      l.Theme.NewClickable(false),
	}

	osm.cancelBtn = l.Theme.OutlineButton(values.String(values.StrCancel))
	osm.cancelBtn.Font.Weight = text.Medium

	osm.startBtn = l.Theme.Button(values.String(values.StrStart))
	osm.startBtn.Font.Weight = text.Medium
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
		exchange, err := osm.WL.AssetsManager.InstantSwap.NewExchangeServer(es.Server)
		if err != nil {
			log.Error(err)
			return
		}

		osm.exchange = exchange

		go func() {
			err := osm.getExchangeRateInfo()
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

func (osm *orderSchedulerModal) SetLoading(loading bool) {
	osm.isStarting = loading
	osm.Modal.SetDisabled(loading)
}

func (osm *orderSchedulerModal) OnDismiss() {
	osm.ctxCancel()
}

func (osm *orderSchedulerModal) SetError(err string) {
	osm.passwordEditor.SetError(values.TranslateErr(err))
}

func (osm *orderSchedulerModal) Handle() {
	osm.startBtn.SetEnabled(osm.canStart())

	for osm.startBtn.Clicked() {
		osm.startOrderScheduler()
	}

	if osm.cancelBtn.Clicked() || osm.Modal.BackdropClicked(true) {
		osm.onCancel()
		osm.Dismiss()
	}

	if osm.refreshExchangeRateBtn.Button.Clicked() {
		go func() {
			err := osm.getExchangeRateInfo()
			if err != nil {
				log.Error(err)
			}
		}()
	}

	for _, evt := range osm.balanceToMaintain.Editor.Events() {
		if osm.balanceToMaintain.Editor.Focused() {
			switch evt.(type) {
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
											txt.Font.Weight = text.SemiBold
											return txt.Layout(gtx)
										})
									}),
									layout.Rigid(func(gtx C) D {
										return osm.Theme.List(osm.pageContainer).Layout(gtx, 1, func(gtx C, i int) D {
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
																					txt.Font.Weight = text.SemiBold
																					return txt.Layout(gtx)
																				}

																				if osm.fetchingRate {
																					gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding16)
																					gtx.Constraints.Min.X = gtx.Constraints.Max.X
																					return osm.materialLoader.Layout(gtx)
																				}

																				if osm.exchangeSelector.SelectedExchange() != nil && osm.exchangeRate != -1 {
																					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																						layout.Rigid(func(gtx C) D {
																							exchangeRate := values.StringF(values.StrServerRate, osm.exchangeSelector.SelectedExchange().Name, osm.exchangeRate)
																							txt := osm.Theme.Label(values.TextSize14, exchangeRate)
																							txt.Font.Weight = text.SemiBold
																							txt.Color = osm.Theme.Color.Gray1
																							return txt.Layout(gtx)
																						}),
																						layout.Rigid(func(gtx C) D {
																							binanceRate := values.StringF(values.StrBinanceRate, osm.binanceRate)
																							txt := osm.Theme.Label(values.TextSize14, binanceRate)
																							txt.Font.Weight = text.SemiBold
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
																			txt.Font.Weight = text.SemiBold
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
																	if osm.WL.AssetsManager.IsDarkModeOn() {
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
	// osm.SetLoading(true)

	go func() {
		osm.SetLoading(true)
		err := osm.sourceWalletSelector.SelectedWallet().UnlockWallet(osm.passwordEditor.Editor.Text())
		if err != nil {
			osm.SetError(err.Error())
			osm.SetLoading(false)
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

				DestinationAddress: osm.orderData.destinationAddress,
				RefundAddress:      osm.orderData.refundAddress,
			},

			Frequency:          osm.frequencySelector.selectedFrequency.item,
			BalanceToMaintain:  balanceToMaintain,
			SpendingPassphrase: osm.passwordEditor.Editor.Text(),
		}

		go osm.WL.AssetsManager.StartScheduler(context.Background(), params)

		osm.Dismiss()
		osm.orderSchedulerStarted()

	}()
}

func (osm *orderSchedulerModal) getExchangeRateInfo() error {
	osm.fetchingRate = true
	osm.rateError = false
	params := api.ExchangeRateRequest{
		From:   osm.fromCurrency.String(),
		To:     osm.toCurrency.String(),
		Amount: libwallet.DefaultRateRequestAmount, // amount needs to be greater than 0 to get the exchange rate
	}
	res, err := osm.WL.AssetsManager.InstantSwap.GetExchangeRateInfo(osm.exchange, params)
	if err != nil {
		osm.rateError = true
		osm.fetchingRate = false
		return err
	}

	ticker, err := osm.WL.AssetsManager.ExternalService.GetTicker(ext.Binance, values.String(values.StrDcrBtcPair))
	if err != nil {
		osm.rateError = true
		osm.fetchingRate = false
		return err
	}

	var binanceRate float64
	switch osm.fromCurrency {
	case utils.DCRWalletAsset:
		binanceRate = ticker.LastTradePrice
	case utils.BTCWalletAsset:
		binanceRate = 1 / ticker.LastTradePrice
	}

	osm.exchangeRate = 1 / res.ExchangeRate
	osm.binanceRate = binanceRate

	osm.fetchingRate = false
	osm.rateError = false

	return nil
}
