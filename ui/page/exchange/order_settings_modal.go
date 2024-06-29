package exchange

import (
	"context"
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
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
		Modal:        l.Theme.ModalFloatTitle(values.String(values.StrSettings), l.IsMobileView()),
		orderData:    data,
		copyRedirect: l.Theme.NewClickable(false),
	}

	osm.cancelBtn = l.Theme.OutlineButton(values.String(values.StrCancel))
	osm.cancelBtn.Font.Weight = font.Medium

	osm.saveBtn = l.Theme.Button(values.String(values.StrSave))
	osm.saveBtn.Font.Weight = font.Medium
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

	callbackFunc := func() libutils.AssetType {
		return osm.orderData.fromCurrency
	}

	osm.feeRateSelector = components.NewFeeRateSelector(l, callbackFunc)
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

	osm.sourceWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
		_ = osm.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
	})

	address, err := osm.destinationWalletSelector.SelectedWallet().CurrentAddress(osm.destinationAccountSelector.SelectedAccount().Number)
	if err != nil {
		log.Error(err)
	}
	osm.addressEditor.Editor.SetText(address)

	osm.destinationWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
		_ = osm.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		address, err := osm.destinationWalletSelector.SelectedWallet().CurrentAddress(osm.destinationAccountSelector.SelectedAccount().Number)
		if err != nil {
			log.Error(err)
		}
		osm.addressEditor.Editor.SetText(address)
	})

	osm.destinationAccountSelector.AccountSelected(func(_ *sharedW.Account) {
		address, err := osm.destinationWalletSelector.SelectedWallet().CurrentAddress(osm.destinationAccountSelector.SelectedAccount().Number)
		if err != nil {
			log.Error(err)
		}
		osm.addressEditor.Editor.SetText(address)
	})
	go osm.feeRateSelector.UpdatedFeeRate(osm.sourceWalletSelector.SelectedWallet())
}

func (osm *orderSettingsModal) setLoading(loading bool) {
	osm.Modal.SetDisabled(loading)
}

func (osm *orderSettingsModal) OnDismiss() {
	osm.ctxCancel()
}

func (osm *orderSettingsModal) Handle(gtx C) {
	osm.saveBtn.SetEnabled(osm.canSave())

	if osm.saveBtn.Clicked(gtx) {
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

		osm.AssetsManager.SetExchangeConfig(configInfo)
		osm.settingsSaved(params)
		osm.Dismiss()
	}

	if osm.cancelBtn.Clicked(gtx) || osm.Modal.BackdropClicked(gtx, true) {
		osm.onCancel()
		osm.Dismiss()
	}

	if osm.sourceInfoButton.Button.Clicked(gtx) {
		info := modal.NewCustomModal(osm.Load).
			PositiveButtonStyle(osm.Theme.Color.Primary, osm.Theme.Color.Surface).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetupWithTemplate(modal.SourceModalInfoTemplate).
			Title(values.String(values.StrSource))
		osm.ParentWindow().ShowModal(info)
	}

	if osm.destinationInfoButton.Button.Clicked(gtx) {
		info := modal.NewCustomModal(osm.Load).
			PositiveButtonStyle(osm.Theme.Color.Primary, osm.Theme.Color.Surface).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			Body(values.String(values.StrDestinationModalInfo)).
			Title(values.String(values.StrDestination))
		osm.ParentWindow().ShowModal(info)
	}

	if osm.feeRateSelector.SaveRate.Clicked(gtx) {
		osm.feeRateSelector.OnEditRateClicked(osm.sourceWalletSelector.SelectedWallet())
	}
}

func (osm *orderSettingsModal) handleCopyEvent(gtx C) {
	osm.addressEditor.EditorIconButtonEvent = func() {
		gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(osm.addressEditor.Editor.Text()))})
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
																	Margin:      layout.Inset{Bottom: values.MarginPadding16},
																}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				txt := osm.Theme.Label(values.TextSize16, values.String(values.StrSource))
																				txt.Font.Weight = font.SemiBold
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
																				txt.Font.Weight = font.SemiBold
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
																								if osm.copyRedirect.Clicked(gtx) {
																									// clipboard.WriteOp{Text: osm.addressEditor.Editor.Text()}.Add(mGtx.Ops)
																									gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(osm.addressEditor.Editor.Text()))})
																									osm.Load.Toast.Notify(values.String(values.StrCopied))
																								}
																								return osm.copyRedirect.Layout(mGtx, osm.Theme.NewIcon(osm.Theme.Icons.CopyIcon).Layout24dp)
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
																				txt.Font.Weight = font.SemiBold
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

func (osm *orderSettingsModal) initWalletSelectors() {
	if osm.AssetsManager.IsExchangeConfigSet() {
		exchangeConfig := osm.AssetsManager.GetExchangeConfig()
		sourceWallet := osm.AssetsManager.WalletWithID(int(exchangeConfig.SourceWalletID))
		destinationWallet := osm.AssetsManager.WalletWithID(int(exchangeConfig.DestinationWalletID))

		sourceCurrency := exchangeConfig.SourceAsset
		toCurrency := exchangeConfig.DestinationAsset

		if sourceWallet != nil {
			_, err := sourceWallet.GetAccount(exchangeConfig.SourceAccountNumber)
			if err != nil {
				log.Error(err)
			}

			// Source wallet picker
			osm.sourceWalletSelector = components.NewWalletAndAccountSelector(osm.Load, sourceCurrency).
				Title(values.String(values.StrSource))

			osm.sourceWalletSelector.SetSelectedWallet(sourceWallet)

			// Source account picker
			osm.sourceAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
				Title(values.String(values.StrAccount)).
				AccountValidator(func(account *sharedW.Account) bool {
					accountIsValid := account.Number != load.MaxInt32
					return accountIsValid
				})
			_ = osm.sourceAccountSelector.SelectAccount(osm.sourceWalletSelector.SelectedWallet(), exchangeConfig.SourceAccountNumber)

			osm.sourceWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
				_ = osm.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
			})
		}

		if destinationWallet != nil {
			_, err := destinationWallet.GetAccount(exchangeConfig.DestinationAccountNumber)
			if err != nil {
				log.Error(err)
			}

			// Destination wallet picker
			osm.destinationWalletSelector = components.NewWalletAndAccountSelector(osm.Load, toCurrency).
				Title(values.String(values.StrDestination)).
				EnableWatchOnlyWallets(true)

			osm.destinationWalletSelector.SetSelectedWallet(destinationWallet)

			// Destination account picker
			osm.destinationAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
				Title(values.String(values.StrAccount)).
				AccountValidator(func(account *sharedW.Account) bool {
					// Imported accounts and watch only accounts are imvalid
					accountIsValid := account.Number != load.MaxInt32

					return accountIsValid
				})
			_ = osm.destinationAccountSelector.SelectAccount(osm.destinationWalletSelector.SelectedWallet(), exchangeConfig.DestinationAccountNumber)

			osm.destinationWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
				_ = osm.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
			})
		}
	} else {
		// Source wallet picker
		osm.sourceWalletSelector = components.NewWalletAndAccountSelector(osm.Load, libutils.DCRWalletAsset).
			Title(values.String(values.StrFrom))

		// Source account picker
		osm.sourceAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
			Title(values.String(values.StrAccount)).
			AccountValidator(func(account *sharedW.Account) bool {
				accountIsValid := account.Number != load.MaxInt32

				return accountIsValid
			})
		_ = osm.sourceAccountSelector.SelectFirstValidAccount(osm.sourceWalletSelector.SelectedWallet())

		osm.sourceWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
			_ = osm.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
		})

		// Destination wallet picker
		osm.destinationWalletSelector = components.NewWalletAndAccountSelector(osm.Load, libutils.BTCWalletAsset).
			Title(values.String(values.StrTo)).
			EnableWatchOnlyWallets(true)

		// Destination account picker
		osm.destinationAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
			Title(values.String(values.StrAccount)).
			AccountValidator(func(account *sharedW.Account) bool {
				accountIsValid := account.Number != load.MaxInt32

				return accountIsValid
			})
		_ = osm.destinationAccountSelector.SelectFirstValidAccount(osm.destinationWalletSelector.SelectedWallet())

		osm.destinationWalletSelector.WalletSelected(func(selectedWallet sharedW.Asset) {
			_ = osm.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		})
	}
}
