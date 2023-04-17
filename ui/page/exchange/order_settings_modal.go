package exchange

import (
	"context"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	libutils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
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

	addressEditor   cryptomaterial.Editor
	copyRedirect    *cryptomaterial.Clickable
	feeRateSelector *components.FeeRateSelector

	*orderData

	sourceAccountSelector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector
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

	osm.feeRateSelector = components.NewFeeRateSelector(l)
	osm.feeRateSelector.TitleFontWeight = text.SemiBold
	osm.initWalletSelectors()

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

	if osm.saveBtn.Clicked() {
		params := &callbackParams{
			sourceAccountSelector: osm.sourceAccountSelector,
			sourceWalletSelector:  osm.sourceWalletSelector,

			destinationAccountSelector: osm.destinationAccountSelector,
			destinationWalletSelector:  osm.destinationWalletSelector,
		}

		configInfo := sharedW.ExchangeConfig{
			SourceAsset:              osm.orderData.fromCurrency,
			DestinationAsset:         osm.orderData.toCurrency,
			SourceWalletID:           int32(params.sourceWalletSelector.SelectedWallet().GetWalletID()),
			DestinationWalletID:      int32(params.destinationWalletSelector.SelectedWallet().GetWalletID()),
			SourceAccountNumber:      params.sourceAccountSelector.SelectedAccount().Number,
			DestinationAccountNumber: params.destinationAccountSelector.SelectedAccount().Number,
		}

		osm.WL.AssetsManager.SetExchangeConfig(configInfo)
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

	if osm.feeRateSelector.FetchRates.Clicked() {
		go osm.feeRateSelector.FetchFeeRate(osm.ParentWindow(), osm.sourceWalletSelector.SelectedWallet())
	}

	if osm.feeRateSelector.EditRates.Clicked() {
		osm.feeRateSelector.OnEditRateCliked(osm.sourceWalletSelector.SelectedWallet())
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
																		border := widget.Border{Color: osm.Load.Theme.Color.Gray2, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
																		wrapper := osm.Load.Theme.Card()
																		wrapper.Color = osm.Load.Theme.Color.Background
																		return border.Layout(gtx, func(gtx C) D {
																			return wrapper.Layout(gtx, func(gtx C) D {
																				return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
																					return layout.Flex{}.Layout(gtx,
																						layout.Flexed(0.9, osm.Load.Theme.Body1(osm.addressEditor.Editor.Text()).Layout),
																						layout.Flexed(0.1, func(gtx C) D {
																							return layout.E.Layout(gtx, func(gtx C) D {
																								mGtx := gtx
																								if osm.addressEditor.Editor.Text() == "" {
																									mGtx = gtx.Disabled()
																								}
																								if osm.copyRedirect.Clicked() {
																									clipboard.WriteOp{Text: osm.addressEditor.Editor.Text()}.Add(mGtx.Ops)
																									osm.Load.Toast.Notify(values.String(values.StrCopied))
																								}
																								return osm.copyRedirect.Layout(mGtx, osm.Load.Theme.Icons.CopyIcon.Layout24dp)
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
																		if osm.sourceWalletSelector.SelectedWallet().GetAssetType() != libutils.BTCWalletAsset {
																			return D{}
																		}
																		return osm.feeRateSelector.Layout(gtx)
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

func (pg *orderSettingsModal) initWalletSelectors() {
	if pg.WL.AssetsManager.IsExchangeConfigSet() {
		exchangeConfig := pg.WL.AssetsManager.GetExchangeConfig()
		sourceWallet := pg.WL.AssetsManager.WalletWithID(int(exchangeConfig.SourceWalletID))
		destinationWallet := pg.WL.AssetsManager.WalletWithID(int(exchangeConfig.DestinationWalletID))

		sourceCurrency := exchangeConfig.SourceAsset
		toCurrency := exchangeConfig.DestinationAsset

		if sourceWallet != nil {
			_, err := sourceWallet.GetAccount(exchangeConfig.SourceAccountNumber)
			if err != nil {
				log.Error(err)
			}

			// Source wallet picker
			pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, sourceCurrency).
				Title(values.String(values.StrSource))

			sourceW := &load.WalletMapping{
				Asset: sourceWallet,
			}
			pg.sourceWalletSelector.SetSelectedWallet(sourceW)

			// Source account picker
			pg.sourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
				Title(values.String(values.StrAccount)).
				AccountValidator(func(account *sharedW.Account) bool {
					accountIsValid := account.Number != load.MaxInt32
					return accountIsValid
				})
			pg.sourceAccountSelector.SelectAccount(pg.sourceWalletSelector.SelectedWallet(), exchangeConfig.SourceAccountNumber)

			pg.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
				pg.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
			})
		}

		if destinationWallet != nil {
			_, err := destinationWallet.GetAccount(exchangeConfig.DestinationAccountNumber)
			if err != nil {
				log.Error(err)
			}

			// Destination wallet picker
			pg.destinationWalletSelector = components.NewWalletAndAccountSelector(pg.Load, toCurrency).
				Title(values.String(values.StrDestination)).
				EnableWatchOnlyWallets(true)

			destW := &load.WalletMapping{
				Asset: destinationWallet,
			}
			pg.destinationWalletSelector.SetSelectedWallet(destW)

			// Destination account picker
			pg.destinationAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
				Title(values.String(values.StrAccount)).
				AccountValidator(func(account *sharedW.Account) bool {
					// Imported accounts and watch only accounts are imvalid
					accountIsValid := account.Number != load.MaxInt32

					return accountIsValid
				})
			pg.destinationAccountSelector.SelectAccount(pg.destinationWalletSelector.SelectedWallet(), exchangeConfig.DestinationAccountNumber)

			pg.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
				pg.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
			})
		}
	} else {
		// Source wallet picker
		pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, libutils.DCRWalletAsset).
			Title(values.String(values.StrFrom))

		// Source account picker
		pg.sourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
			Title(values.String(values.StrAccount)).
			AccountValidator(func(account *sharedW.Account) bool {
				accountIsValid := account.Number != load.MaxInt32

				return accountIsValid
			})
		pg.sourceAccountSelector.SelectFirstValidAccount(pg.sourceWalletSelector.SelectedWallet())

		pg.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
			pg.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
		})

		// Destination wallet picker
		pg.destinationWalletSelector = components.NewWalletAndAccountSelector(pg.Load, libutils.BTCWalletAsset).
			Title(values.String(values.StrTo)).
			EnableWatchOnlyWallets(true)

		// Destination account picker
		pg.destinationAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
			Title(values.String(values.StrAccount)).
			AccountValidator(func(account *sharedW.Account) bool {
				accountIsValid := account.Number != load.MaxInt32

				return accountIsValid
			})
		pg.destinationAccountSelector.SelectFirstValidAccount(pg.destinationWalletSelector.SelectedWallet())

		pg.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
			pg.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		})
	}
}
