package exchange

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"

	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"github.com/btcsuite/btcutil"
	"github.com/decred/dcrd/dcrutil/v4"
)

type confirmOrderModal struct {
	*load.Load
	*cryptomaterial.Modal
	modal.CreatePasswordModal

	closeConfirmationModalButton cryptomaterial.Button
	confirmButton                cryptomaterial.Button
	passwordEditor               cryptomaterial.Editor

	orderCreated func()
	isCreating   bool

	*orderData
	exchangeRateSet bool
}

func newConfirmOrderModal(l *load.Load, data *orderData) *confirmOrderModal {
	com := &confirmOrderModal{
		Load:      l,
		Modal:     l.Theme.ModalFloatTitle("send_confirm_modal"),
		orderData: data,
	}

	com.closeConfirmationModalButton = l.Theme.OutlineButton(values.String(values.StrCancel))
	com.closeConfirmationModalButton.Font.Weight = text.Medium

	com.confirmButton = l.Theme.Button("")
	com.confirmButton.Font.Weight = text.Medium
	com.confirmButton.SetEnabled(false)

	com.passwordEditor = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrSpendingPassword))
	com.passwordEditor.Editor.SetText("")
	com.passwordEditor.Editor.SingleLine = true
	com.passwordEditor.Editor.Submit = true

	return com
}

func (com *confirmOrderModal) OnResume() {
	com.passwordEditor.Editor.Focus()
}

func (com *confirmOrderModal) SetError(err string) {
	com.passwordEditor.SetError(values.TranslateErr(err))
}

func (com *confirmOrderModal) SetLoading(loading bool) {
	com.isCreating = loading
	com.Modal.SetDisabled(loading)
}

func (com *confirmOrderModal) OnDismiss() {}

func (com *confirmOrderModal) confirmOrder() {
	password := com.passwordEditor.Editor.Text()
	if password == "" || com.isCreating {
		return
	}

	com.SetLoading(true)
	go func() {
		err := com.sourceWalletSelector.SelectedWallet().UnlockWallet(password)
		if err != nil {
			com.SetError(err.Error())
			com.SetLoading(false)
			return
		}

		order, err := com.createOrder()
		if err != nil {
			com.SetError(err.Error())
			com.SetLoading(false)
			return
		}

		err = com.constructTx(order.DepositAddress, order.InvoicedAmount)
		if err != nil {
			com.SetError(err.Error())
			com.SetLoading(false)
			return
		}

		// TODO: Enable to allow debit of account
		// err = com.sourceWalletSelector.SelectedWallet().Broadcast(password)
		// if err != nil {
		// 	com.SetError(err.Error())
		// 	com.SetLoading(false)
		// 	return
		// }

		successModal := modal.NewSuccessModal(com.Load, "Order created successfully!", modal.DefaultClickFunc())
		com.ParentWindow().ShowModal(successModal)

		// com.orderCreated()
		com.Dismiss()
		com.ParentNavigator().Display(NewOrderDetailsPage(com.Load, order))

	}()

}

func (com *confirmOrderModal) Handle() {
	for _, evt := range com.passwordEditor.Editor.Events() {
		if com.passwordEditor.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				com.confirmButton.SetEnabled(com.passwordEditor.Editor.Text() != "")
			case widget.SubmitEvent:
				com.confirmOrder()
			}
		}
	}

	for com.confirmButton.Clicked() {
		com.confirmOrder()
	}

	for com.closeConfirmationModalButton.Clicked() {
		if !com.isCreating {
			com.Dismiss()
		}
	}
}

func (com *confirmOrderModal) Layout(gtx layout.Context) D {

	w := []layout.Widget{
		func(gtx C) D {
			return com.Theme.Label(values.TextSize18, "Confirm your order").Layout(gtx)
		},
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return com.setWalletLogo(gtx, com.orderData.fromCurrency, values.MarginPadding30)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Left: values.MarginPadding10,
								// Right: values.MarginPadding50,
							}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return com.Theme.Label(values.TextSize16, "Sending").Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										if com.orderData.fromCurrency == utils.DCRWalletAsset {
											invoicedAmount, _ := dcrutil.NewAmount(com.orderData.invoicedAmount)
											return com.Theme.Label(values.TextSize16, invoicedAmount.String()).Layout(gtx)

										}
										invoicedAmount, _ := btcutil.NewAmount(com.orderData.invoicedAmount)
										return com.Theme.Label(values.TextSize16, invoicedAmount.String()).Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										sourceWallet := com.WL.MultiWallet.WalletWithID(com.orderData.sourceWalletID)
										sourceWalletName := sourceWallet.GetWalletName()
										sourceAccount, _ := sourceWallet.GetAccount(com.orderData.sourceAccountNumber)
										fromText := fmt.Sprintf("From: %s (%s)", sourceWalletName, sourceAccount.Name)
										return com.Theme.Label(values.TextSize16, fromText).Layout(gtx)
									}),
								)
							})
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Top:    values.MarginPadding24,
						Bottom: values.MarginPadding24,
					}.Layout(gtx, func(gtx C) D {
						return com.Theme.Icons.ArrowDownIcon.LayoutSize(gtx, values.MarginPadding60)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return com.setWalletLogo(gtx, com.orderData.toCurrency, values.MarginPadding30)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Left: values.MarginPadding10,
							}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return com.Theme.Label(values.TextSize16, "Receiving").Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										if com.orderData.toCurrency == utils.DCRWalletAsset {
											orderedAmount, _ := dcrutil.NewAmount(com.orderData.orderedAmount)
											return com.Theme.Label(values.TextSize16, orderedAmount.String()).Layout(gtx)
										}
										orderedAmount, _ := btcutil.NewAmount(com.orderData.orderedAmount)
										return com.Theme.Label(values.TextSize16, orderedAmount.String()).Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										destinationWallet := com.WL.MultiWallet.WalletWithID(com.orderData.destinationWalletID)
										destinationWalletName := destinationWallet.GetWalletName()
										destinationAccount, _ := destinationWallet.GetAccount(com.orderData.destinationAccountNumber)
										toText := fmt.Sprintf("To: %s (%s)", destinationWalletName, destinationAccount.Name)
										return com.Theme.Label(values.TextSize16, toText).Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										return com.Theme.Label(values.TextSize16, com.orderData.destinationAddress).Layout(gtx)
									}),
								)
							})
						}),
					)
				}),
			)
		},
		func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, com.passwordEditor.Layout)
		},
		func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Right: values.MarginPadding8,
							}.Layout(gtx, func(gtx C) D {
								if com.isCreating {
									return D{}
								}
								return com.closeConfirmationModalButton.Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx C) D {
							if com.isCreating {
								return layout.Inset{Top: unit.Dp(7)}.Layout(gtx, func(gtx C) D {
									return material.Loader(com.Theme.Base).Layout(gtx)
								})
							}
							com.confirmButton.Text = "Confirm Order"
							return com.confirmButton.Layout(gtx)
						}),
					)
				})
			})
		},
	}
	return com.Modal.Layout(gtx, w)
}

func (com *confirmOrderModal) setWalletLogo(gtx C, currency utils.AssetType, size unit.Dp) D {
	if currency == utils.DCRWalletAsset {
		return com.Theme.Icons.DecredSymbol2.LayoutSize(gtx, size)
	}
	return com.Theme.Icons.BTC.LayoutSize(gtx, size)
}

func (com *confirmOrderModal) createOrder() (*instantswap.Order, error) {
	data := instantswap.Order{
		Server:                   com.server,
		SourceWalletID:           com.sourceWalletSelector.SelectedWallet().GetWalletID(),
		SourceAccountNumber:      com.sourceAccountSelector.SelectedAccount().Number,
		DestinationWalletID:      com.destinationWalletSelector.SelectedWallet().GetWalletID(),
		DestinationAccountNumber: com.destinationAccountSelector.SelectedAccount().Number,

		InvoicedAmount: com.invoicedAmount,
		FromCurrency:   com.fromCurrency.String(),
		ToCurrency:     com.toCurrency.String(),

		RefundAddress:      com.refundAddress,
		DestinationAddress: com.destinationAddress,
	}

	order, err := com.WL.MultiWallet.InstantSwap.CreateOrder(com.exchange, data)
	if err != nil {
		return nil, err
	}

	return order, nil
}

func (com *confirmOrderModal) constructTx(depositAddress string, unitAmount float64) error {
	destinationAddress := depositAddress

	sourceAccount := com.sourceAccountSelector.SelectedAccount()
	err := com.sourceWalletSelector.SelectedWallet().NewUnsignedTx(sourceAccount.Number)
	if err != nil {
		return err
	}

	amount := btc.AmountSatoshi(unitAmount)
	err = com.sourceWalletSelector.SelectedWallet().AddSendDestination(destinationAddress, amount, false)
	if err != nil {
		return err
	}

	return nil
}
