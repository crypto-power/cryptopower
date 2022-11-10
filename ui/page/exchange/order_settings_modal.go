package exchange

import (
	"context"

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

	settingsSaved func(params *callbackParams)
	onCancel      func()

	cancelBtn      cryptomaterial.Button
	saveBtn        cryptomaterial.Button
	passwordEditor cryptomaterial.Editor

	scrollContainer *widget.List

	isSending bool

	sourceInfoButton      cryptomaterial.IconButton
	destinationInfoButton cryptomaterial.IconButton

	sourceAccountSelector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	addressEditor cryptomaterial.Editor
}

func newOrderSettingsModalModal(l *load.Load) *orderSettingsModal {
	osm := &orderSettingsModal{
		Load:  l,
		Modal: l.Theme.ModalFloatTitle("Settings"),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
	}

	osm.cancelBtn = l.Theme.OutlineButton(values.String(values.StrCancel))
	osm.cancelBtn.Font.Weight = text.Medium

	osm.saveBtn = l.Theme.Button(values.String(values.StrSave))
	osm.saveBtn.Font.Weight = text.Medium
	osm.saveBtn.SetEnabled(false)

	osm.passwordEditor = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrSpendingPassword))
	osm.passwordEditor.Editor.SetText("")
	osm.passwordEditor.Editor.SingleLine = true
	osm.passwordEditor.Editor.Submit = true

	osm.sourceInfoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	osm.destinationInfoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	osm.sourceInfoButton.Size, osm.destinationInfoButton.Size = values.MarginPadding14, values.MarginPadding14
	buttonInset := layout.UniformInset(values.MarginPadding0)
	osm.sourceInfoButton.Inset, osm.destinationInfoButton.Inset = buttonInset, buttonInset

	osm.addressEditor = l.Theme.IconEditor(new(widget.Editor), "", l.Theme.Icons.ContentCopy, true)
	osm.addressEditor.Editor.SingleLine = true

	// Source wallet picker
	osm.sourceWalletSelector = components.NewWalletAndAccountSelector(osm.Load, utils.DCRWalletAsset).
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
	osm.destinationWalletSelector = components.NewWalletAndAccountSelector(osm.Load, utils.BTCWalletAsset).
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
	address, _ := osm.destinationWalletSelector.SelectedWallet().CurrentAddress(osm.destinationAccountSelector.SelectedAccount().Number)
	osm.addressEditor.Editor.SetText(address)

	osm.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		osm.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		address, _ := osm.destinationWalletSelector.SelectedWallet().CurrentAddress(osm.destinationAccountSelector.SelectedAccount().Number)
		osm.addressEditor.Editor.SetText(address)
	})

	osm.destinationAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		address, _ := osm.destinationWalletSelector.SelectedWallet().CurrentAddress(osm.destinationAccountSelector.SelectedAccount().Number)
		osm.addressEditor.Editor.SetText(address)
	})

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

	osm.passwordEditor.Editor.Focus()
}

func (osm *orderSettingsModal) SetError(err string) {
	osm.passwordEditor.SetError(values.TranslateErr(err))
}

func (osm *orderSettingsModal) SetLoading(loading bool) {
	osm.isSending = loading
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
			Body("Wallets that have not completed sync will be hidden from the list Refunds and leftover change will be returned to the selected source account").
			Title("Source")
		osm.ParentWindow().ShowModal(info)
	}

	if osm.destinationInfoButton.Button.Clicked() {
		info := modal.NewCustomModal(osm.Load).
			PositiveButtonStyle(osm.Theme.Color.Primary, osm.Theme.Color.Surface).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetupWithTemplate(modal.SecurityToolsInfoTemplate).
			Title("Destination")
		osm.ParentWindow().ShowModal(info)
	}
}

func (osm *orderSettingsModal) handleCopyEvent(gtx C) {
	osm.addressEditor.EditorIconButtonEvent = func() {
		clipboard.WriteOp{Text: osm.addressEditor.Editor.Text()}.Add(gtx.Ops)
		osm.Toast.Notify("Copied")
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
				txt := osm.Theme.Label(values.TextSize20, "Settings")
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
					return osm.Theme.List(osm.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
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
													txt := osm.Theme.Label(values.TextSize16, "Source")
													txt.Font.Weight = text.SemiBold
													return txt.Layout(gtx)
												}),
												layout.Rigid(func(gtx C) D {
													return osm.sourceInfoButton.Layout(gtx)
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
													txt := osm.Theme.Label(values.TextSize16, "Destination")
													txt.Font.Weight = text.SemiBold
													return txt.Layout(gtx)
												}),
												layout.Rigid(func(gtx C) D {
													return osm.destinationInfoButton.Layout(gtx)
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
											gtx = gtx.Disabled()
											osm.addressEditor.SelectionColor = osm.Theme.Color.Gray5
											return osm.addressEditor.Layout(gtx)
										}),
									)

								})
							}),
						)
					})
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
					layout.Rigid(func(gtx C) D {
						return osm.saveBtn.Layout(gtx)
					}),
				)
			})
		},
	}
	return osm.Modal.Layout(gtx, w)
}
