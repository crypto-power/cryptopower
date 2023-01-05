package exchange

import (
	"context"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
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

	settingsSaved func(params *callbackParams)
	onCancel      func()

	cancelBtn cryptomaterial.Button
	saveBtn   cryptomaterial.Button

	sourceInfoButton      cryptomaterial.IconButton
	destinationInfoButton cryptomaterial.IconButton

	sourceAccountSelector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	addressEditor cryptomaterial.Editor

	*orderData
}

func newOrderSettingsModalModal(l *load.Load, data *orderData) *orderSettingsModal {
	osm := &orderSettingsModal{
		Load:      l,
		Modal:     l.Theme.ModalFloatTitle(values.String(values.StrSettings)),
		orderData: data,
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

	osm.initializeWalletAndAccountSelector()

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
	// osm.WL.MultiWallet.ClearExchangeConfig()
	if osm.WL.MultiWallet.ExchangeConfigIsSet() {
		exchangeConfig := osm.WL.MultiWallet.ExchangeConfig()
		sourceWallet := osm.WL.MultiWallet.WalletWithID(int(exchangeConfig.SourceWalletID))
		destinationWallet := osm.WL.MultiWallet.WalletWithID(int(exchangeConfig.DestinationWalletID))

		sourceCurrency := exchangeConfig.SourceAsset
		toCurrency := exchangeConfig.DestinationAsset

		if sourceCurrency != osm.orderData.fromCurrency || toCurrency != osm.orderData.toCurrency {
			// if currencies have changed, reset the exchange config
			osm.initializeWalletAndAccountSelector()
		} else {
			if sourceWallet != nil {
				_, err := sourceWallet.GetAccount(exchangeConfig.SourceAccountNumber)
				if err != nil {
					log.Error(err)
				}

				// Source wallet picker
				osm.sourceWalletSelector = components.NewWalletAndAccountSelector(osm.Load, osm.orderData.fromCurrency).
					Title(values.String(values.StrFrom))

				sourceW := &load.WalletMapping{
					Asset: sourceWallet,
				}
				osm.sourceWalletSelector.SelectWallet(sourceW)

				// Source account picker
				osm.sourceAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
					Title(values.String(values.StrAccount)).
					AccountValidator(func(account *sharedW.Account) bool {
						accountIsValid := account.Number != load.MaxInt32 && !osm.sourceWalletSelector.SelectedWallet().IsWatchingOnlyWallet()

						return accountIsValid
					})
				osm.sourceAccountSelector.SelectAccount(osm.sourceWalletSelector.SelectedWallet(), exchangeConfig.SourceAccountNumber)

				osm.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
					osm.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
				})
			}

			if destinationWallet != nil {
				_, err := destinationWallet.GetAccount(exchangeConfig.DestinationAccountNumber)
				if err != nil {
					log.Error(err)
				}

				// Destination wallet picker
				osm.destinationWalletSelector = components.NewWalletAndAccountSelector(osm.Load, osm.orderData.toCurrency).
					Title(values.String(values.StrTo))

				// Destination account picker
				osm.destinationAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
					Title(values.String(values.StrAccount)).
					AccountValidator(func(account *sharedW.Account) bool {
						// Imported accounts and watch only accounts are imvalid
						accountIsValid := account.Number != load.MaxInt32 && !osm.sourceWalletSelector.SelectedWallet().IsWatchingOnlyWallet()

						return accountIsValid
					})
				osm.destinationAccountSelector.SelectAccount(osm.destinationWalletSelector.SelectedWallet(), exchangeConfig.DestinationAccountNumber)
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
		}

	}
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

		osm.WL.MultiWallet.SetExchangeConfig(osm.orderData.fromCurrency, int32(params.sourceWalletSelector.SelectedWallet().GetWalletID()), osm.orderData.toCurrency, int32(params.destinationWalletSelector.SelectedWallet().GetWalletID()), params.sourceAccountSelector.SelectedAccount().Number, params.destinationAccountSelector.SelectedAccount().Number)
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

	return true
}

func (osm *orderSettingsModal) Layout(gtx layout.Context) D {
	osm.handleCopyEvent(gtx)
	w := []layout.Widget{
		func(gtx C) D {
			return layout.Inset{
				Bottom: values.MarginPadding8,
			}.Layout(gtx, func(gtx C) D {
				txt := osm.Theme.Label(values.TextSize20, values.String(values.StrSettings))
				txt.Font.Weight = text.SemiBold
				return txt.Layout(gtx)
			})
		},
		func(gtx C) D {
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
										gtx = gtx.Disabled() // since this is disabled, the copy icon doesn't work
										osm.addressEditor.SelectionColor = osm.Theme.Color.Gray5
										return osm.addressEditor.Layout(gtx)
									}),
								)

							})
						}),
					)
				})
			})
		},
		func(gtx C) D {
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
		},
	}
	return osm.Modal.Layout(gtx, w)
}

func (osm *orderSettingsModal) initializeWalletAndAccountSelector() {
	// Source wallet picker
	osm.sourceWalletSelector = components.NewWalletAndAccountSelector(osm.Load, osm.orderData.fromCurrency).
		Title(values.String(values.StrFrom))

	// Source account picker
	osm.sourceAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
		Title(values.String(values.StrAccount)).
		AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32 && !osm.sourceWalletSelector.SelectedWallet().IsWatchingOnlyWallet()

			return accountIsValid
		})
	osm.sourceAccountSelector.SelectFirstValidAccount(osm.sourceWalletSelector.SelectedWallet())

	osm.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		osm.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
	})

	// Destination wallet picker
	osm.destinationWalletSelector = components.NewWalletAndAccountSelector(osm.Load, osm.orderData.toCurrency).
		Title(values.String(values.StrTo))

	// Destination account picker
	osm.destinationAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
		Title(values.String(values.StrAccount)).
		AccountValidator(func(account *sharedW.Account) bool {
			// Imported accounts and watch only accounts are imvalid
			accountIsValid := account.Number != load.MaxInt32 && !osm.sourceWalletSelector.SelectedWallet().IsWatchingOnlyWallet()

			return accountIsValid
		})
	osm.destinationAccountSelector.SelectFirstValidAccount(osm.destinationWalletSelector.SelectedWallet())
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
