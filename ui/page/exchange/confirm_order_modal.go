package exchange

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gitlab.com/cryptopower/cryptopower/libwallet/assets/btc"
	"gitlab.com/cryptopower/cryptopower/libwallet/assets/dcr"
	"gitlab.com/cryptopower/cryptopower/libwallet/assets/ltc"
	"gitlab.com/cryptopower/cryptopower/libwallet/instantswap"
	"gitlab.com/cryptopower/cryptopower/libwallet/utils"
	"gitlab.com/cryptopower/cryptopower/ui/cryptomaterial"
	"gitlab.com/cryptopower/cryptopower/ui/load"
	"gitlab.com/cryptopower/cryptopower/ui/modal"
	"gitlab.com/cryptopower/cryptopower/ui/page/components"
	"gitlab.com/cryptopower/cryptopower/ui/values"
)

type confirmOrderModal struct {
	*load.Load
	*cryptomaterial.Modal
	modal.CreatePasswordModal

	closeConfirmationModalButton cryptomaterial.Button
	confirmButton                cryptomaterial.Button
	passwordEditor               cryptomaterial.Editor

	onOrderCompleted func(order *instantswap.Order)
	onCancel         func()

	pageContainer *widget.List

	isCreating bool

	*orderData
}

func newConfirmOrderModal(l *load.Load, data *orderData) *confirmOrderModal {
	com := &confirmOrderModal{
		Load:      l,
		Modal:     l.Theme.ModalFloatTitle(values.String(values.StrConfirmYourOrder)),
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

	com.pageContainer = &widget.List{
		List: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
	}

	return com
}

func (com *confirmOrderModal) OnResume() {
	com.passwordEditor.Editor.Focus()
}

func (com *confirmOrderModal) OnOrderCompleted(orderCompleted func(order *instantswap.Order)) *confirmOrderModal {
	com.onOrderCompleted = orderCompleted
	return com
}

func (com *confirmOrderModal) OnCancel(cancel func()) *confirmOrderModal {
	com.onCancel = cancel
	return com
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
	com.passwordEditor.SetError("")
	password := com.passwordEditor.Editor.Text()
	if password == "" || com.isCreating {
		return
	}

	com.SetLoading(true)
	go func() {
		if !com.sourceWalletSelector.SelectedWallet().IsSynced() {
			com.SetError(values.String(values.StrSourceWalletNotSynced))
			com.SetLoading(false)
			return
		}

		if !com.destinationWalletSelector.SelectedWallet().IsSynced() {
			com.SetError(values.String(values.StrDestinationWalletNotSynced))
			com.SetLoading(false)
			return
		}

		err := com.sourceWalletSelector.SelectedWallet().UnlockWallet(password)
		if err != nil {
			com.SetError(err.Error())
			com.SetLoading(false)
			return
		}

		order, err := com.createOrder()
		if err != nil {
			log.Error(err)
			com.SetError(err.Error())
			com.SetLoading(false)
			return
		}

		err = com.constructTx(order.DepositAddress, order.InvoicedAmount)
		if err != nil {
			com.WL.AssetsManager.InstantSwap.DeleteOrder(order)
			com.SetError(err.Error())
			com.SetLoading(false)
			return
		}

		// FOR DEVELOPMENT: Comment this block to prevent debit of account
		_, err = com.sourceWalletSelector.SelectedWallet().Broadcast(password, "")
		if err != nil {
			com.WL.AssetsManager.InstantSwap.DeleteOrder(order)
			com.SetError(err.Error())
			com.SetLoading(false)
			return
		}

		com.onOrderCompleted(order)
		com.Dismiss()
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
			return layout.Stack{Alignment: layout.S}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return layout.Stack{Alignment: layout.NE}.Layout(gtx,
						layout.Expanded(func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
										txt := com.Theme.Label(values.TextSize20, values.String(values.StrConfirmYourOrder))
										txt.Font.Weight = text.SemiBold
										return txt.Layout(gtx)
									})
								}),
								layout.Rigid(func(gtx C) D {
									return com.Theme.List(com.pageContainer).Layout(gtx, 1, func(gtx C, i int) D {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
													layout.Rigid(func(gtx C) D {
														return components.SetWalletLogo(com.Load, gtx, com.orderData.fromCurrency, values.MarginPadding30)
													}),
													layout.Rigid(func(gtx C) D {
														return layout.Inset{
															Left: values.MarginPadding10,
														}.Layout(gtx, func(gtx C) D {
															return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																layout.Rigid(func(gtx C) D {
																	return com.Theme.Label(values.TextSize16, values.String(values.StrSending)).Layout(gtx)
																}),
																layout.Rigid(func(gtx C) D {
																	return components.LayoutOrderAmount(com.Load, gtx, com.orderData.fromCurrency.String(), com.orderData.invoicedAmount)
																}),
																layout.Rigid(func(gtx C) D {
																	sourceWallet := com.WL.AssetsManager.WalletWithID(com.orderData.sourceWalletID)
																	sourceWalletName := sourceWallet.GetWalletName()
																	sourceAccount, _ := sourceWallet.GetAccount(com.orderData.sourceAccountNumber)
																	fromText := fmt.Sprintf(values.String(values.StrOrderSendingFrom), sourceWalletName, sourceAccount.Name)
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
													return com.Theme.Icons.ArrowDownIcon.LayoutSize(gtx, values.MarginPadding20)
												})
											}),
											layout.Rigid(func(gtx C) D {
												return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
													layout.Rigid(func(gtx C) D {
														return components.SetWalletLogo(com.Load, gtx, com.orderData.toCurrency, values.MarginPadding30)
													}),
													layout.Rigid(func(gtx C) D {
														return layout.Inset{
															Left: values.MarginPadding10,
														}.Layout(gtx, func(gtx C) D {
															return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																layout.Rigid(func(gtx C) D {
																	return com.Theme.Label(values.TextSize16, values.String(values.StrReceiving)).Layout(gtx)
																}),
																layout.Rigid(func(gtx C) D {
																	return components.LayoutOrderAmount(com.Load, gtx, com.orderData.toCurrency.String(), com.orderData.orderedAmount)
																}),
																layout.Rigid(func(gtx C) D {
																	destinationWallet := com.WL.AssetsManager.WalletWithID(com.orderData.destinationWalletID)
																	destinationWalletName := destinationWallet.GetWalletName()
																	destinationAccount, _ := destinationWallet.GetAccount(com.orderData.destinationAccountNumber)
																	toText := fmt.Sprintf(values.String(values.StrOrderReceivingTo), destinationWalletName, destinationAccount.Name)
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
											layout.Rigid(func(gtx C) D {
												return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, com.passwordEditor.Layout)
											}),
										)
									})
								}),
							)
						}),
					)
				}),
				layout.Stacked(func(gtx C) D {
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y

					return layout.S.Layout(gtx, func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							c := com.Theme.Card()
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
														com.confirmButton.Text = values.String(values.StrConfirmOrder)
														return com.confirmButton.Layout(gtx)
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
	return com.Modal.Layout(gtx, w)
}

func (com *confirmOrderModal) createOrder() (*instantswap.Order, error) {
	data := instantswap.Order{
		ExchangeServer:           com.exchangeServer,
		SourceWalletID:           com.sourceWalletID,
		SourceAccountNumber:      com.sourceAccountNumber,
		DestinationWalletID:      com.destinationWalletID,
		DestinationAccountNumber: com.destinationAccountNumber,

		InvoicedAmount: com.invoicedAmount,
		FromCurrency:   com.fromCurrency.String(),
		ToCurrency:     com.toCurrency.String(),

		RefundAddress:      com.refundAddress,
		DestinationAddress: com.destinationAddress,
	}

	order, err := com.WL.AssetsManager.InstantSwap.CreateOrder(com.exchange, data)
	if err != nil {
		return nil, err
	}

	return order, nil
}

func (com *confirmOrderModal) constructTx(depositAddress string, unitAmount float64) error {
	destinationAddress := depositAddress

	sourceAccount := com.sourceAccountSelector.SelectedAccount()
	err := com.sourceWalletSelector.SelectedWallet().NewUnsignedTx(sourceAccount.Number, nil)
	if err != nil {
		return err
	}

	var amount int64
	switch com.sourceWalletSelector.SelectedWallet().GetAssetType() {
	case utils.BTCWalletAsset:
		amount = btc.AmountSatoshi(unitAmount)
	case utils.DCRWalletAsset:
		amount = dcr.AmountAtom(unitAmount)
	case utils.LTCWalletAsset:
		amount = ltc.AmountLitoshi(unitAmount)
	}
	err = com.sourceWalletSelector.SelectedWallet().AddSendDestination(destinationAddress, amount, false)
	if err != nil {
		return err
	}

	return nil
}
