package exchange

import (
	"context"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	// "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	// "code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

type orderSettingsModal struct {
	*load.Load
	*cryptomaterial.Modal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	settingsSaved func()
	onCancel      func()

	cancelBtn      cryptomaterial.Button
	confirmButton  cryptomaterial.Button
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

	exchangeRateSet bool
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
		// authoredTxData: data,
		// asset:          asset,
	}

	osm.cancelBtn = l.Theme.OutlineButton(values.String(values.StrCancel))
	osm.cancelBtn.Font.Weight = text.Medium

	osm.confirmButton = l.Theme.Button("")
	osm.confirmButton.Font.Weight = text.Medium
	osm.confirmButton.SetEnabled(false)

	osm.passwordEditor = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrSpendingPassword))
	osm.passwordEditor.Editor.SetText("")
	osm.passwordEditor.Editor.SingleLine = true
	osm.passwordEditor.Editor.Submit = true

	osm.sourceInfoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	osm.destinationInfoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	osm.sourceInfoButton.Size, osm.destinationInfoButton.Size = values.MarginPadding14, values.MarginPadding14
	buttonInset := layout.UniformInset(values.MarginPadding0)
	osm.sourceInfoButton.Inset, osm.destinationInfoButton.Inset = buttonInset, buttonInset

	osm.addressEditor = l.Theme.Editor(new(widget.Editor), "")
	osm.addressEditor.Editor.SetText("")
	osm.addressEditor.Editor.SingleLine = true

	// Source wallet picker
	osm.sourceWalletSelector = components.NewWalletAndAccountSelector(osm.Load).
		Title(values.String(values.StrTo))

	// Source account picker
	osm.sourceAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
		Title(values.String(values.StrAccount))
	osm.sourceAccountSelector.SelectFirstValidAccount(osm.sourceWalletSelector.SelectedWallet())

	osm.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		osm.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
	})

	// Destination wallet picker
	osm.destinationWalletSelector = components.NewWalletAndAccountSelector(osm.Load).
		Title(values.String(values.StrTo))

	// Destination account picker
	osm.destinationAccountSelector = components.NewWalletAndAccountSelector(osm.Load).
		Title(values.String(values.StrAccount))
	osm.destinationAccountSelector.SelectFirstValidAccount(osm.destinationWalletSelector.SelectedWallet())

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

func (osm *orderSettingsModal) OnSettingsSaved(settingsSaved func()) *orderSettingsModal {
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
	for osm.confirmButton.Clicked() {
		osm.settingsSaved()
		osm.Dismiss()
	}

	if osm.cancelBtn.Clicked() || osm.Modal.BackdropClicked(true) {
		osm.onCancel()
		osm.Dismiss()
	}
}

func (osm *orderSettingsModal) Layout(gtx layout.Context) D {

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
			return layout.Inset{Left: values.MarginPadding16, Right: values.MarginPadding16, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Right: values.MarginPadding8,
							}.Layout(gtx, func(gtx C) D {
								if osm.isSending {
									return D{}
								}
								return osm.cancelBtn.Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx C) D {
							if osm.isSending {
								return layout.Inset{Top: unit.Dp(7)}.Layout(gtx, func(gtx C) D {
									return material.Loader(osm.Theme.Base).Layout(gtx)
								})
							}
							osm.confirmButton.Text = values.StrSave
							return osm.confirmButton.Layout(gtx)
						}),
					)
				})
			})
		},
	}
	return osm.Modal.Layout(gtx, w)
}