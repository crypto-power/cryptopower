package exchange

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

type callbackParams struct {
	sourceAccountSelector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector
}

type orderSettingsModal struct {
	*load.Load
	*cryptomaterial.Modal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	pageContainer *widget.List

	settingsSaved func(params *callbackParams)
	onCancel      func()

	cancelBtn cryptomaterial.Button
	saveBtn   cryptomaterial.Button

	sourceInfoButton      cryptomaterial.IconButton
	destinationInfoButton cryptomaterial.IconButton

	addressEditor cryptomaterial.Editor
	copyRedirect  *cryptomaterial.Clickable

	feeRateText  string
	editRates    cryptomaterial.Button
	fetchRates   cryptomaterial.Button
	ratesEditor  cryptomaterial.Editor
	priority     string
	rateEditMode bool

	*orderData
}

func newOrderSettingsModalModal(l *load.Load, data *orderData) *orderSettingsModal {
	osm := &orderSettingsModal{
		Load:         l,
		Modal:        l.Theme.ModalFloatTitle(values.String(values.StrSettings)),
		orderData:    data,
		copyRedirect: l.Theme.NewClickable(false),
	}

	osm.cancelBtn = l.Theme.OutlineButton(values.String(values.StrCancel))
	osm.cancelBtn.Font.Weight = text.Medium

	osm.saveBtn = l.Theme.Button(values.String(values.StrSave))
	osm.saveBtn.Font.Weight = text.Medium
	osm.saveBtn.SetEnabled(false)

	osm.sourceInfoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	osm.destinationInfoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	osm.sourceInfoButton.Size, osm.destinationInfoButton.Size = values.MarginPadding14, values.MarginPadding14
	buttonInset := layout.UniformInset(values.MarginPadding0)
	osm.sourceInfoButton.Inset, osm.destinationInfoButton.Inset = buttonInset, buttonInset

	osm.addressEditor = l.Theme.IconEditor(new(widget.Editor), "", l.Theme.Icons.ContentCopy, true)
	osm.addressEditor.Editor.SingleLine = true

	osm.pageContainer = &widget.List{
		List: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
	}

	osm.feeRateText = " - "
	osm.priority = values.String(values.StrUnknown)
	osm.editRates = osm.Theme.Button(values.String(values.StrEdit))
	osm.fetchRates = osm.Theme.Button(values.String(values.StrFetchRates))

	bInset := layout.Inset{
		Top:    values.MarginPadding4,
		Right:  values.MarginPadding8,
		Bottom: values.MarginPadding4,
		Left:   values.MarginPadding8,
	}
	osm.fetchRates.TextSize, osm.editRates.TextSize = values.TextSize12, values.TextSize12
	osm.fetchRates.Inset, osm.editRates.Inset = bInset, bInset

	osm.ratesEditor = osm.Theme.Editor(new(widget.Editor), "in Sat/kvB")
	osm.ratesEditor.HasCustomButton = false
	osm.ratesEditor.Editor.SingleLine = true
	osm.ratesEditor.TextSize = values.TextSize14

	return osm
}

func (osm *orderSettingsModal) OnSettingsSaved(settingsSaved func(params *callbackParams)) *orderSettingsModal {
	osm.settingsSaved = settingsSaved
	return osm
}

func (osm *orderSettingsModal) OnCancel(cancel func()) *orderSettingsModal {
	osm.onCancel = cancel
	return osm
}

func (osm *orderSettingsModal) OnResume() {
	osm.ctx, osm.ctxCancel = context.WithCancel(context.TODO())

	osm.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		osm.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
	})

	address, err := osm.destinationWalletSelector.SelectedWallet().CurrentAddress(osm.destinationAccountSelector.SelectedAccount().Number)
	if err != nil {
		log.Error(err)
	}
	osm.addressEditor.Editor.SetText(address)

	osm.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		osm.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		address, err := osm.destinationWalletSelector.SelectedWallet().CurrentAddress(osm.destinationAccountSelector.SelectedAccount().Number)
		if err != nil {
			log.Error(err)
		}
		osm.addressEditor.Editor.SetText(address)
	})

	osm.destinationAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		address, err := osm.destinationWalletSelector.SelectedWallet().CurrentAddress(osm.destinationAccountSelector.SelectedAccount().Number)
		if err != nil {
			log.Error(err)
		}
		osm.addressEditor.Editor.SetText(address)
	})
}

func (osm *orderSettingsModal) SetLoading(loading bool) {
	osm.Modal.SetDisabled(loading)
}

func (osm *orderSettingsModal) OnDismiss() {
	osm.ctxCancel()
}

func (osm *orderSettingsModal) Handle() {
	osm.saveBtn.SetEnabled(osm.canSave())

	for osm.saveBtn.Clicked() {
		params := &callbackParams{
			sourceAccountSelector: osm.sourceAccountSelector,
			sourceWalletSelector:  osm.sourceWalletSelector,

			destinationAccountSelector: osm.destinationAccountSelector,
			destinationWalletSelector:  osm.destinationWalletSelector,
		}

		osm.WL.AssetsManager.SetExchangeConfig(osm.orderData.fromCurrency, int32(params.sourceWalletSelector.SelectedWallet().GetWalletID()), osm.orderData.toCurrency, int32(params.destinationWalletSelector.SelectedWallet().GetWalletID()), params.sourceAccountSelector.SelectedAccount().Number, params.destinationAccountSelector.SelectedAccount().Number)
		osm.settingsSaved(params)
		osm.Dismiss()
	}

	if osm.cancelBtn.Clicked() || osm.Modal.BackdropClicked(true) {
		osm.onCancel()
		osm.Dismiss()
	}

	if osm.sourceInfoButton.Button.Clicked() {
		info := modal.NewCustomModal(osm.Load).
			PositiveButtonStyle(osm.Theme.Color.Primary, osm.Theme.Color.Surface).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetupWithTemplate(modal.SourceModalInfoTemplate).
			Title(values.String(values.StrSource))
		osm.ParentWindow().ShowModal(info)
	}

	if osm.destinationInfoButton.Button.Clicked() {
		info := modal.NewCustomModal(osm.Load).
			PositiveButtonStyle(osm.Theme.Color.Primary, osm.Theme.Color.Surface).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			Body(values.String(values.StrDestinationModalInfo)).
			Title(values.String(values.StrDestination))
		osm.ParentWindow().ShowModal(info)
	}

	if osm.fetchRates.Clicked() {
		go osm.feeRateAPIHandler()
	}

	if osm.editRates.Clicked() {
		osm.rateEditMode = !osm.rateEditMode
		if osm.rateEditMode {
			osm.editRates.Text = values.String(values.StrSave)
		} else {
			rateStr := osm.ratesEditor.Editor.Text()
			rateInt, err := osm.sourceWalletSelector.SelectedWallet().SetAPIFeeRate(rateStr)
			if err != nil {
				osm.feeRateText = " - "
			}

			osm.feeRateText = osm.addRatesUnits(rateInt)
		}
	}
}

func (osm *orderSettingsModal) handleCopyEvent(gtx C) {
	osm.addressEditor.EditorIconButtonEvent = func() {
		clipboard.WriteOp{Text: osm.addressEditor.Editor.Text()}.Add(gtx.Ops)
		osm.Toast.Notify(values.String(values.StrCopied))
	}
}

func (osm *orderSettingsModal) canSave() bool {
	if osm.sourceWalletSelector.SelectedWallet() == nil {
		return false
	}

	if osm.sourceAccountSelector.SelectedAccount() == nil {
		return false
	}

	if osm.destinationWalletSelector.SelectedWallet() == nil {
		return false
	}

	if osm.destinationAccountSelector.SelectedAccount() == nil {
		return false
	}

	if osm.addressEditor.Editor.Text() == "" {
		return false
	}

	if !osm.sourceWalletSelector.SelectedWallet().IsSynced() {
		return false
	}

	return true
}

func (osm *orderSettingsModal) Layout(gtx layout.Context) D {
	osm.handleCopyEvent(gtx)
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
											Bottom: values.MarginPadding8,
										}.Layout(gtx, func(gtx C) D {
											txt := osm.Theme.Label(values.TextSize20, values.String(values.StrSettings))
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
																	Margin:      layout.Inset{Bottom: values.MarginPadding16},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				txt := osm.Theme.Label(values.TextSize16, values.String(values.StrSource))
																				txt.Font.Weight = text.SemiBold
																				return txt.Layout(gtx)
																			}),
																			layout.Rigid(func(gtx C) D {
																				return layout.Inset{
																					Top:  values.MarginPadding4,
																					Left: values.MarginPadding4,
																				}.Layout(gtx, osm.sourceInfoButton.Layout)
																			}),
																		)
																	}),
																	layout.Rigid(func(gtx C) D {
																		return layout.Inset{
																			Bottom: values.MarginPadding16,
																		}.Layout(gtx, func(gtx C) D {
																			return osm.sourceWalletSelector.Layout(osm.ParentWindow(), gtx)
																		})
																	}),
																	layout.Rigid(func(gtx C) D {
																		return osm.sourceAccountSelector.Layout(osm.ParentWindow(), gtx)
																	}),
																	layout.Rigid(func(gtx C) D {
																		if !osm.sourceWalletSelector.SelectedWallet().IsSynced() {
																			txt := osm.Theme.Label(values.TextSize14, values.String(values.StrSourceWalletNotSynced))
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
																return cryptomaterial.LinearLayout{
																	Width:       cryptomaterial.MatchParent,
																	Height:      cryptomaterial.WrapContent,
																	Orientation: layout.Vertical,
																	Margin:      layout.Inset{Bottom: values.MarginPadding16},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				txt := osm.Theme.Label(values.TextSize16, values.String(values.StrDestination))
																				txt.Font.Weight = text.SemiBold
																				return txt.Layout(gtx)
																			}),
																			layout.Rigid(func(gtx C) D {
																				return layout.Inset{
																					Top:  values.MarginPadding4,
																					Left: values.MarginPadding4,
																				}.Layout(gtx, osm.destinationInfoButton.Layout)
																			}),
																		)
																	}),
																	layout.Rigid(func(gtx C) D {
																		return layout.Inset{
																			Bottom: values.MarginPadding16,
																		}.Layout(gtx, func(gtx C) D {
																			return osm.destinationWalletSelector.Layout(osm.ParentWindow(), gtx)
																		})
																	}),
																	layout.Rigid(func(gtx C) D {
																		return layout.Inset{
																			Bottom: values.MarginPadding16,
																		}.Layout(gtx, func(gtx C) D {
																			return osm.destinationAccountSelector.Layout(osm.ParentWindow(), gtx)
																		})
																	}),
																	layout.Rigid(func(gtx C) D {
																		border := widget.Border{Color: osm.Load.Theme.Color.Gray4, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
																		wrapper := osm.Load.Theme.Card()
																		wrapper.Color = osm.Load.Theme.Color.Gray4
																		return border.Layout(gtx, func(gtx C) D {
																			return wrapper.Layout(gtx, func(gtx C) D {
																				return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
																					return layout.Flex{}.Layout(gtx,
																						layout.Flexed(0.9, osm.Load.Theme.Body1(osm.addressEditor.Editor.Text()).Layout),
																						layout.Flexed(0.1, func(gtx C) D {
																							return layout.E.Layout(gtx, func(gtx C) D {
																								if osm.copyRedirect.Clicked() {
																									clipboard.WriteOp{Text: osm.addressEditor.Editor.Text()}.Add(gtx.Ops)
																									osm.Load.Toast.Notify(values.String(values.StrCopied))
																								}
																								return osm.copyRedirect.Layout(gtx, osm.Load.Theme.Icons.CopyIcon.Layout24dp)
																							})
																						}),
																					)
																				})
																			})
																		})
																	}),
																	layout.Rigid(func(gtx C) D {
																		return layout.Inset{
																			Bottom: values.MarginPadding16,
																		}.Layout(gtx, func(gtx C) D {
																			if !osm.destinationWalletSelector.SelectedWallet().IsSynced() {
																				txt := osm.Theme.Label(values.TextSize14, values.String(values.StrDestinationWalletNotSynced))
																				txt.Font.Weight = text.SemiBold
																				txt.Color = osm.Theme.Color.Danger
																				return txt.Layout(gtx)
																			}
																			return D{}
																		})
																	}),
																	layout.Rigid(func(gtx C) D {
																		if osm.sourceWalletSelector.SelectedWallet().GetAssetType() != utils.BTCWalletAsset {
																			return D{}
																		}
																		return osm.txFeeSection(gtx)
																	}),
																)
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
													layout.Rigid(osm.saveBtn.Layout),
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

func (osm *orderSettingsModal) txFeeSection(gtx layout.Context) D {
	return layout.Inset{
		Bottom: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
			Margin:      layout.Inset{Bottom: values.MarginPadding16},
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				title := osm.Theme.Label(values.TextSize16, values.String(values.StrTxFee))
				title.Font.Weight = text.SemiBold
				return title.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				border := widget.Border{Color: osm.Load.Theme.Color.Gray4, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
				wrapper := osm.Load.Theme.Card()
				wrapper.Color = osm.Load.Theme.Color.Gray4
				return border.Layout(gtx, func(gtx C) D {
					return wrapper.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X // Wrapper should fill available width
						return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return layout.Inset{Bottom: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
										return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												if osm.rateEditMode {
													gtx.Constraints.Max.X = gtx.Constraints.Max.X / 3
													return osm.ratesEditor.Layout(gtx)
												}
												feerateLabel := osm.Theme.Label(values.TextSize14, osm.feeRateText)
												feerateLabel.Font.Weight = text.SemiBold
												return feerateLabel.Layout(gtx)
											}),
											layout.Rigid(func(gtx C) D {
												return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, osm.editRates.Layout)
											}),
											layout.Rigid(func(gtx C) D {
												return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, osm.fetchRates.Layout)
											}),
										)
									})
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											priorityLabel := osm.Theme.Label(values.TextSize14, values.StringF(values.StrPriority, " : "))
											priorityLabel.Font.Weight = text.SemiBold
											return priorityLabel.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											priorityVal := osm.Theme.Label(values.TextSize14, osm.priority)
											priorityVal.Font.Style = text.Italic
											return priorityVal.Layout(gtx)
										}),
									)
								}),
							)
						})
					})
				})

			}),
		)
	})
}

func (osm *orderSettingsModal) feeRateAPIHandler() {
	feeRates, err := osm.sourceWalletSelector.SelectedWallet().GetAPIFeeRate()
	if err != nil {
		return
	}

	blocksStr := func(b int32) string {
		val := strconv.Itoa(int(b)) + " block"
		if b == 1 {
			return val
		}
		return val + "s"
	}

	radiogroupbtns := new(widget.Enum)
	items := make([]layout.FlexChild, 0)
	for index, feerate := range feeRates {
		key := strconv.Itoa(index)
		value := osm.addRatesUnits(feerate.Feerate.ToInt()) + " - " + blocksStr(feerate.ConfirmedBlocks)
		radioBtn := osm.Load.Theme.RadioButton(radiogroupbtns, key, value,
			osm.Load.Theme.Color.DeepBlue, osm.Load.Theme.Color.Primary)
		items = append(items, layout.Rigid(radioBtn.Layout))
	}

	info := modal.NewCustomModal(osm.Load).
		Title(values.String(values.StrFeeRates)).
		UseCustomWidget(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		}).
		SetCancelable(true).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetPositiveButtonText(values.String(values.StrSave)).
		SetPositiveButtonCallback(func(isChecked bool, im *modal.InfoModal) bool {
			fields := strings.Fields(radiogroupbtns.Value)
			index, _ := strconv.Atoi(fields[0])
			rate := strconv.Itoa(int(feeRates[index].Feerate.ToInt()))
			rateInt, err := osm.sourceWalletSelector.SelectedWallet().SetAPIFeeRate(rate)
			if err != nil {
				//pg.feeEstimationError(err.Error())
				return false
			}

			osm.feeRateText = osm.addRatesUnits(rateInt)
			blocks := feeRates[index].ConfirmedBlocks
			timeBefore := time.Now().Add(time.Duration(-10*blocks) * time.Minute)
			osm.priority = fmt.Sprintf("%v (~%v)", blocksStr(blocks), components.TimeAgo(timeBefore.Unix()))
			im.Dismiss()
			return true
		})

	osm.ParentWindow().ShowModal((info))
	osm.editRates.SetEnabled(true)
}

func (osm *orderSettingsModal) addRatesUnits(rates int64) string {
	return osm.Load.Printer.Sprintf("%d Sat/kvB", rates)
}
