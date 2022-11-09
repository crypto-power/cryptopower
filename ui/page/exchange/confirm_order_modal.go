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

	// *authoredTxData
	// asset           load.WalletMapping
	exchangeRateSet bool
}

func newConfirmOrderModal(l *load.Load, data *orderData) *confirmOrderModal {
	com := &confirmOrderModal{
		Load:  l,
		Modal: l.Theme.ModalFloatTitle("send_confirm_modal"),
		// authoredTxData: data,
		// asset:          asset,
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

		fmt.Println("[][][][]] ORDER", order)

		err = com.constructTx(order.DepositAddress, order.InvoicedAmount)
		if err != nil {
			com.SetError(err.Error())
			com.SetLoading(false)
			return
		}

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
							if com.orderData.fromCurrency == utils.DCRWalletAsset.String() {
								return com.Theme.Icons.DecredSymbol2.LayoutSize(gtx, values.MarginPadding60)
							}
							return com.Theme.Icons.BTC.LayoutSize(gtx, values.MarginPadding60)
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
										if com.orderData.fromCurrency == utils.DCRWalletAsset.String() {
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
							if com.orderData.toCurrency == utils.DCRWalletAsset.String() {
								return com.Theme.Icons.DecredSymbol2.LayoutSize(gtx, values.MarginPadding60)
							}
							return com.Theme.Icons.BTC.LayoutSize(gtx, values.MarginPadding60)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Left: values.MarginPadding10,
								// Right: values.MarginPadding50,
							}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return com.Theme.Label(values.TextSize16, "Receiving").Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										if com.orderData.toCurrency == utils.DCRWalletAsset.String() {
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

// func (com *confirmOrderModal) setWalletLogo(gtx C) D {
// 	walletIcon := com.Theme.Icons.DecredLogo
// 	if com.asset.GetAssetType() == utils.BTCWalletAsset {
// 		walletIcon = com.Theme.Icons.BTC
// 	}
// 	inset := layout.Inset{
// 		Right: values.MarginPadding8, Left: values.MarginPadding25,
// 	}
// 	return inset.Layout(gtx, walletIcon.Layout16dp)
// }

func (com *confirmOrderModal) createOrder() (*instantswap.Order, error) {
	// fmt.Println("[][][][] ", com.fromAmountEditor.Editor.Text())
	// invoicedAmount, err := strconv.ParseFloat(com.fromAmountEditor.Editor.Text(), 8)
	// if err != nil {
	// 	return nil, err
	// }
	fmt.Println("[][][][] ", com.invoicedAmount)

	// refundAddress, err := com.sourceWalletSelector.SelectedWallet().CurrentAddress(com.sourceAccountSelector.SelectedAccount().Number)
	// if err != nil {
	// 	return nil, err
	// }

	// destinationAddress, err := com.destinationWalletSelector.SelectedWallet().CurrentAddress(com.destinationAccountSelector.SelectedAccount().Number)
	// if err != nil {
	// 	return nil, err
	// }

	data := instantswap.Order{
		Server:                   com.server,
		SourceWalletID:           com.sourceWalletSelector.SelectedWallet().GetWalletID(),
		SourceAccountNumber:      com.sourceAccountSelector.SelectedAccount().Number,
		DestinationWalletID:      com.destinationWalletSelector.SelectedWallet().GetWalletID(),
		DestinationAccountNumber: com.destinationAccountSelector.SelectedAccount().Number,

		InvoicedAmount: com.invoicedAmount,
		FromCurrency:   com.fromCurrency,
		ToCurrency:     com.toCurrency,

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

	// destinationAccount := com.destinationAccountSelector.SelectedAccount()

	// amountAtom, SendMax, err := com.amount.validAmount()
	// if err != nil {
	// 	com.feeEstimationError(err.Error())
	// 	return
	// }

	// dcrImpl := com.WL.SelectedWallet.Wallet.(*dcr.DCRAsset)
	// if dcrImpl == nil {
	// 	com.feeEstimationError("Only DCR implementation is supported")
	// 	// Only DCR implementation is supported past here.
	// 	return
	// }

	sourceAccount := com.sourceAccountSelector.SelectedAccount()
	err := com.sourceWalletSelector.SelectedWallet().NewUnsignedTx(sourceAccount.Number)
	if err != nil {
		// com.feeEstimationError(err.Error())
		return err
	}

	amount := btc.AmountSatoshi(unitAmount)
	err = com.sourceWalletSelector.SelectedWallet().AddSendDestination(destinationAddress, amount, false)
	if err != nil {
		// com.feeEstimationError(err.Error())
		return err
	}

	_, err = com.sourceWalletSelector.SelectedWallet().EstimateFeeAndSize()
	if err != nil {
		// com.feeEstimationError(err.Error())
		return err
	}

	// feeAtom := feeAndSize.Fee.UnitValue
	// if SendMax {
	// 	amountAtom = sourceAccount.Balance.Spendable.ToInt() - feeAtom
	// }

	// wal := com.sourceWalletSelector.SelectedWallet()
	// totalSendingAmount := wal.ToAmount(unitAmount + feeAtom)
	// balanceAfterSend := wal.ToAmount(sourceAccount.Balance.Spendable.ToInt() - totalSendingAmount.ToInt())

	// populate display data
	// com.txFee = wal.ToAmount(feeAtom).String()
	// com.estSignedSize = fmt.Sprintf("%d bytes", feeAndSize.EstimatedSignedSize)
	// com.totalCost = totalSendingAmount.String()
	// com.balanceAfterSend = balanceAfterSend.String()
	// com.sendAmount = wal.ToAmount(amountAtom).String()
	// com.destinationAddress = destinationAddress
	// com.destinationAccount = destinationAccount
	// com.sourceAccount = sourceAccount

	// if SendMax {
	// 	// TODO: this workaround ignores the change events from the
	// 	// amount input to avoid construct tx cycle.
	// 	com.amount.setAmount(amountAtom)
	// }

	// if com.exchangeRate != -1 && com.usdExchangeSet {
	// 	com.txFeeUSD = fmt.Sprintf("$%.4f", utils.DCRToUSD(com.exchangeRate, feeAndSize.Fee.CoinValue))
	// 	com.totalCostUSD = utils.FormatUSDBalance(com.Printer, utils.DCRToUSD(com.exchangeRate, totalSendingAmount.ToCoin()))
	// 	com.balanceAfterSendUSD = utils.FormatUSDBalance(com.Printer, utils.DCRToUSD(com.exchangeRate, balanceAfterSend.ToCoin()))

	// 	usdAmount := utils.DCRToUSD(com.exchangeRate, wal.ToAmount(amountAtom).ToCoin())
	// 	com.sendAmountUSD = utils.FormatUSDBalance(com.Printer, usdAmount)
	// }

	// com.txAuthor = com.sourceWalletSelector.SelectedWallet().GetUnsignedTx()

	return nil
}
