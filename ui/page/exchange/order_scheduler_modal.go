package exchange

import (
	"context"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	// sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

type orderSchedulerModal struct {
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

	addressEditor     cryptomaterial.Editor
	balanceToMaintain cryptomaterial.Editor
	maxRateDeviation  cryptomaterial.Editor
	copyRedirect      *cryptomaterial.Clickable

	exchangeSelector  *ExchangeSelector
	frequencySelector *FrequencySelector
}

func newOrderSchedulerModalModal(l *load.Load) *orderSchedulerModal {
	osm := &orderSchedulerModal{
		Load:              l,
		Modal:             l.Theme.ModalFloatTitle(values.String(values.StrSettings)),
		exchangeSelector:  NewExchangeSelector(l),
		frequencySelector: NewFrequencySelector(l),
		copyRedirect:      l.Theme.NewClickable(false),
	}

	osm.cancelBtn = l.Theme.OutlineButton(values.String(values.StrCancel))
	osm.cancelBtn.Font.Weight = text.Medium

	osm.saveBtn = l.Theme.Button("Start")
	osm.saveBtn.Font.Weight = text.Medium
	osm.saveBtn.SetEnabled(false)

	osm.balanceToMaintain = l.Theme.Editor(new(widget.Editor), "Balance to maintain")
	osm.balanceToMaintain.Editor.SingleLine, osm.balanceToMaintain.Editor.Submit = true, true

	osm.maxRateDeviation = l.Theme.Editor(new(widget.Editor), "Max rate deviation")
	osm.maxRateDeviation.Editor.SingleLine, osm.maxRateDeviation.Editor.Submit = true, true

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

	return osm
}

func (osm *orderSchedulerModal) OnSettingsSaved(settingsSaved func(params *callbackParams)) *orderSchedulerModal {
	osm.settingsSaved = settingsSaved
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
	osm.Modal.SetDisabled(loading)
}

func (osm *orderSchedulerModal) OnDismiss() {
	osm.ctxCancel()
}

func (osm *orderSchedulerModal) Handle() {
	osm.saveBtn.SetEnabled(osm.canSave())

	for osm.saveBtn.Clicked() {

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
}

func (osm *orderSchedulerModal) handleCopyEvent(gtx C) {
	osm.addressEditor.EditorIconButtonEvent = func() {
		clipboard.WriteOp{Text: osm.addressEditor.Editor.Text()}.Add(gtx.Ops)
		osm.Toast.Notify(values.String(values.StrCopied))
	}
}

func (osm *orderSchedulerModal) canSave() bool {
	return true
}

func (osm *orderSchedulerModal) Layout(gtx layout.Context) D {
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
																Bottom: values.MarginPadding8,
															}.Layout(gtx, func(gtx C) D {
																return cryptomaterial.LinearLayout{
																	Width:       cryptomaterial.MatchParent,
																	Height:      cryptomaterial.WrapContent,
																	Orientation: layout.Vertical,
																	Margin:      layout.Inset{Bottom: values.MarginPadding4},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				txt := osm.Theme.Label(values.TextSize16, "Select server")
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
																		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				return osm.exchangeSelector.Layout(osm.ParentWindow(), gtx)
																			}),
																		)
																	}),
																	layout.Rigid(func(gtx C) D {
																		// if !osm.sourceWalletSelector.SelectedWallet().IsSynced() {
																		// txt := osm.Theme.Label(values.TextSize14, "select a server")
																		// txt.Font.Weight = text.SemiBold
																		// txt.Color = osm.Theme.Color.Danger
																		// return txt.Layout(gtx)
																		// }
																		return D{}
																	}),
																)
															})
														}),
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																// Bottom: values.MarginPadding16,
															}.Layout(gtx, func(gtx C) D {
																return cryptomaterial.LinearLayout{
																	Width:       cryptomaterial.MatchParent,
																	Height:      cryptomaterial.WrapContent,
																	Orientation: layout.Vertical,
																	Margin:      layout.Inset{Bottom: values.MarginPadding4},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				txt := osm.Theme.Label(values.TextSize16, "Select frequency")
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
																			return osm.frequencySelector.Layout(osm.ParentWindow(), gtx)
																		})
																	}),
																	layout.Rigid(func(gtx C) D {
																		// if !osm.sourceWalletSelector.SelectedWallet().IsSynced() {
																		// txt := osm.Theme.Label(values.TextSize14, values.String(values.StrSourceWalletNotSynced))
																		// txt.Font.Weight = text.SemiBold
																		// txt.Color = osm.Theme.Color.Danger
																		// return txt.Layout(gtx)
																		// }
																		return D{}
																	}),
																)
															})
														}),
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																// Bottom: values.MarginPadding16,
															}.Layout(gtx, func(gtx C) D {
																return cryptomaterial.LinearLayout{
																	Width:       cryptomaterial.MatchParent,
																	Height:      cryptomaterial.WrapContent,
																	Orientation: layout.Vertical,
																	Margin:      layout.Inset{Bottom: values.MarginPadding4},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				txt := osm.Theme.Label(values.TextSize16, "Balance to maintain")
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
																			return osm.balanceToMaintain.Layout(gtx)
																		})
																	}),
																	layout.Rigid(func(gtx C) D {
																		// if !osm.sourceWalletSelector.SelectedWallet().IsSynced() {
																		// txt := osm.Theme.Label(values.TextSize14, values.String(values.StrSourceWalletNotSynced))
																		// txt.Font.Weight = text.SemiBold
																		// txt.Color = osm.Theme.Color.Danger
																		// return txt.Layout(gtx)
																		// }
																		return D{}
																	}),
																)
															})
														}),

														// layout.Rigid(func(gtx C) D {
														// 	return layout.Inset{
														// 		// Bottom: values.MarginPadding16,
														// 	}.Layout(gtx, func(gtx C) D {
														// 		return cryptomaterial.LinearLayout{
														// 			Width:       cryptomaterial.MatchParent,
														// 			Height:      cryptomaterial.WrapContent,
														// 			Orientation: layout.Vertical,
														// 			Margin:      layout.Inset{Bottom: values.MarginPadding4},
														// 		}.Layout(gtx,
														// 			layout.Rigid(func(gtx C) D {
														// 				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
														// 					layout.Rigid(func(gtx C) D {
														// 						txt := osm.Theme.Label(values.TextSize16, "Maximum rate deviation")
														// 						txt.Font.Weight = text.SemiBold
														// 						return txt.Layout(gtx)
														// 					}),
														// 					layout.Rigid(func(gtx C) D {
														// 						return layout.Inset{
														// 							Top:  values.MarginPadding4,
														// 							Left: values.MarginPadding4,
														// 						}.Layout(gtx, osm.sourceInfoButton.Layout)
														// 					}),
														// 				)
														// 			}),
														// 			layout.Rigid(func(gtx C) D {
														// 				return layout.Inset{
														// 					// Right: values.MarginPadding10,
														// 				}.Layout(gtx, func(gtx C) D {
														// 					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
														// 						// layout.Rigid(func(gtx C) D {
														// 						// 	txt := osm.Theme.Label(values.TextSize16, values.String(values.StrSelectServerTitle))
														// 						// 	return txt.Layout(gtx)
														// 						// }),
														// 						layout.Rigid(func(gtx C) D {
														// 							return layout.Inset{
														// 								Bottom: values.MarginPadding16,
														// 							}.Layout(gtx, func(gtx C) D {
														// 								return osm.maxRateDeviation.Layout(gtx)
														// 							})
														// 						}),
														// 					)
														// 				})
														// 			}),
														// 			layout.Rigid(func(gtx C) D {
														// 				return layout.Inset{
														// 					Bottom: values.MarginPadding16,
														// 				}.Layout(gtx, func(gtx C) D {
														// 					// if !osm.destinationWalletSelector.SelectedWallet().IsSynced() {
														// 					// txt := osm.Theme.Label(values.TextSize14, values.String(values.StrDestinationWalletNotSynced))
														// 					// txt.Font.Weight = text.SemiBold
														// 					// txt.Color = osm.Theme.Color.Danger
														// 					// return txt.Layout(gtx)
														// 					// }
														// 					return D{}
														// 				})
														// 			}),
														// 		)
														// 	})
														// }),
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
