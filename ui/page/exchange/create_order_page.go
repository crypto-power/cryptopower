package exchange

import (
	"context"
	"fmt"
	"strconv"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"

	api "code.cryptopower.dev/exchange/instantswap"
)

const CreateOrderPageID = "CreateOrder"

type (
	C = layout.Context
	D = layout.Dimensions
)

type CreateOrderPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	listContainer *widget.List

	exchange         api.IDExchange
	orderItems       []*instantswap.Order
	exchangeSelector *ExchangeSelector
	selectedExchange *Exchange

	fromAmountEditor cryptomaterial.Editor
	toAmountEditor   cryptomaterial.Editor

	addressEditor cryptomaterial.Editor

	sourceAccountSelector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	backButton cryptomaterial.IconButton

	createOrderBtn cryptomaterial.Button
	infoButton     cryptomaterial.IconButton
	settingsButton cryptomaterial.IconButton

	min float64
	max float64
}

func NewCreateOrderPage(l *load.Load) *CreateOrderPage {
	pg := &CreateOrderPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(CreateOrderPageID),
		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		exchangeSelector: NewExchangeSelector(l),
	}

	pg.backButton, _ = components.SubpageHeaderButtons(l)

	pg.settingsButton = l.Theme.IconButton(l.Theme.Icons.ActionSettings)
	pg.infoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	pg.infoButton.Size = values.MarginPadding18
	buttonInset := layout.UniformInset(values.MarginPadding0)
	pg.settingsButton.Inset, pg.infoButton.Inset = buttonInset, buttonInset

	pg.fromAmountEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrAmount)+" (DCR)")
	pg.fromAmountEditor.Editor.SetText("")
	pg.fromAmountEditor.HasCustomButton = true
	pg.fromAmountEditor.Editor.SingleLine = true

	pg.fromAmountEditor.CustomButton.Inset = layout.UniformInset(values.MarginPadding2)
	pg.fromAmountEditor.CustomButton.Text = "DCR"
	pg.toAmountEditor.CustomButton.Background = l.Theme.Color.Primary
	pg.fromAmountEditor.CustomButton.CornerRadius = values.MarginPadding0

	pg.toAmountEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrAmount)+" (BTC)")
	pg.toAmountEditor.Editor.SetText("")
	pg.toAmountEditor.HasCustomButton = true
	pg.toAmountEditor.Editor.SingleLine = true

	pg.toAmountEditor.CustomButton.Inset = layout.UniformInset(values.MarginPadding2)
	pg.toAmountEditor.CustomButton.Text = "BTC"
	pg.toAmountEditor.CustomButton.Background = l.Theme.Color.Danger
	pg.toAmountEditor.CustomButton.CornerRadius = values.MarginPadding0

	pg.addressEditor = l.Theme.Editor(new(widget.Editor), "")
	pg.addressEditor.Editor.SetText("")
	pg.addressEditor.Editor.SingleLine = true

	// Source wallet picker
	pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, utils.DCRWalletAsset).
		Title(values.String(values.StrFrom))

	// Source account picker
	pg.sourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrAccount)).
		AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32 && !pg.sourceWalletSelector.SelectedWallet().IsWatchingOnlyWallet()

			return accountIsValid
		})
	pg.sourceAccountSelector.SelectFirstValidAccount(pg.sourceWalletSelector.SelectedWallet())

	pg.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
	})

	// Destination wallet picker
	pg.destinationWalletSelector = components.NewWalletAndAccountSelector(pg.Load, utils.BTCWalletAsset).
		Title(values.String(values.StrTo))

	// Destination account picker
	pg.destinationAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrAccount)).
		AccountValidator(func(account *sharedW.Account) bool {
			accountIsValid := account.Number != load.MaxInt32 && !pg.sourceWalletSelector.SelectedWallet().IsWatchingOnlyWallet()

			return accountIsValid
		})
	pg.destinationAccountSelector.SelectFirstValidAccount(pg.destinationWalletSelector.SelectedWallet())
	address, _ := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
	pg.addressEditor.Editor.SetText(address)

	pg.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		address, _ := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
		pg.addressEditor.Editor.SetText(address)
	})

	pg.destinationAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		address, _ := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
		pg.addressEditor.Editor.SetText(address)
	})

	pg.createOrderBtn = pg.Theme.Button("Create Order")
	pg.createOrderBtn.SetEnabled(false)

	pg.exchangeSelector.ExchangeSelected(func(es *Exchange) {
		pg.selectedExchange = es

		// Initialize a new exchange using the selected exchange server
		exchange, err := pg.WL.MultiWallet.InstantSwap.NewExchanageServer(pg.selectedExchange.Server, "", "")
		if err != nil {
			fmt.Println(err)
		}

		pg.exchange = exchange

		pg.createOrderBtn.SetEnabled(true)
	})

	return pg
}

func (pg *CreateOrderPage) ID() string {
	return CreateOrderPageID
}

func (pg *CreateOrderPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	// go func() {
	// 	err := pg.getExchangeRateInfo()
	// 	if err != nil {
	// 		fmt.Println(err)
	// 	}
	// }()

	pg.FetchOrders()
}

func (pg *CreateOrderPage) OnNavigatedFrom() {
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}

func (pg *CreateOrderPage) HandleUserInteractions() {
	if pg.createOrderBtn.Clicked() {
		pg.confirmSourcePassword()
	}

	if pg.settingsButton.Button.Clicked() {
		orderSettingsModal := newOrderSettingsModalModal(pg.Load).OnSettingsSaved(func(params *callbackParams) {
			pg.sourceAccountSelector = params.sourceAccountSelector
			pg.sourceWalletSelector = params.sourceWalletSelector
			pg.destinationAccountSelector = params.destinationAccountSelector
			pg.destinationWalletSelector = params.destinationWalletSelector

			infoModal := modal.NewSuccessModal(pg.Load, "Order settings saved", modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(infoModal)
		}).
			OnCancel(func() {
			})
		pg.ParentWindow().ShowModal(orderSettingsModal)
	}
}

func (pg *CreateOrderPage) Layout(gtx C) D {
	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      "Create Order",
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: pg.layout,
		}
		return sp.Layout(pg.ParentWindow(), gtx)
	}

	return components.UniformPadding(gtx, container)
}

func (pg *CreateOrderPage) layout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.MatchParent,
		Direction: layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:  gtx.Dp(values.MarginPadding550),
			Height: cryptomaterial.MatchParent,
			Margin: layout.Inset{
				Bottom: values.MarginPadding30,
			},
		}.Layout2(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Bottom: values.MarginPadding16,
					}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(0.65, func(gtx C) D {
								return layout.E.Layout(gtx, func(gtx C) D {
									return layout.Flex{
										Axis:      layout.Horizontal,
										Alignment: layout.Middle,
									}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											return layout.Inset{
												Right: values.MarginPadding10,
											}.Layout(gtx, func(gtx C) D {
												return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
													layout.Rigid(func(gtx C) D {
														txt := pg.Theme.Label(values.TextSize16, "Select the exchange server you would like to use.")
														return txt.Layout(gtx)
													}),
													layout.Rigid(func(gtx C) D {
														return pg.exchangeSelector.Layout(pg.ParentWindow(), gtx)
													}),
												)
											})
										}),
									)
								})
							}),
							layout.Flexed(0.35, func(gtx C) D {
								return layout.E.Layout(gtx, func(gtx C) D {
									return layout.Flex{
										Axis:      layout.Horizontal,
										Alignment: layout.Middle,
									}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											return layout.Inset{
												Right: values.MarginPadding10,
												Left:  values.MarginPadding10,
											}.Layout(gtx, func(gtx C) D {
												return pg.infoButton.Layout(gtx)
											})
										}),
										layout.Rigid(pg.settingsButton.Layout),
									)
								})
							}),
						)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Bottom: values.MarginPadding16,
					}.Layout(gtx, func(gtx C) D {
						return layout.Flex{
							Axis:      layout.Horizontal,
							Alignment: layout.Middle,
						}.Layout(gtx,
							layout.Flexed(0.45, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										txt := pg.Theme.Label(values.TextSize16, "From")
										txt.Font.Weight = text.SemiBold
										return txt.Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										return pg.fromAmountEditor.Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										t := fmt.Sprintf("Min: %f . Max: %f", pg.min, pg.max)
										txt := pg.Theme.Label(values.TextSize14, t)
										// txt.Font.Weight = text.SemiBold
										return txt.Layout(gtx)
									}),
								)
							}),
							layout.Flexed(0.1, func(gtx C) D {
								return layout.Center.Layout(gtx, func(gtx C) D {
									icon := pg.Theme.Icons.CurrencySwapIcon
									return icon.Layout12dp(gtx)
								})
							}),
							layout.Flexed(0.45, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										txt := pg.Theme.Label(values.TextSize16, "To")
										txt.Font.Weight = text.SemiBold
										return txt.Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										return pg.toAmountEditor.Layout(gtx)
									}),
								)
							}),
						)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding16,
						}.Layout(gtx, pg.createOrderBtn.Layout)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Top: values.MarginPadding24,
					}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								txt := pg.Theme.Label(values.TextSize18, "History")
								txt.Font.Weight = text.SemiBold
								return txt.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Top: values.MarginPadding10,
								}.Layout(gtx, pg.layoutHistory)
							}),
						)
					})
				}),
			)
		})
	})
}

func (pg *CreateOrderPage) FetchOrders() {
	items := components.LoadOrders(pg.Load, true)
	pg.orderItems = items

	pg.ParentWindow().Reload()
}

func (pg *CreateOrderPage) layoutHistory(gtx C) D {
	if len(pg.orderItems) == 0 {
		return components.LayoutNoOrderHistory(gtx, pg.Load, false)
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			list := layout.List{Axis: layout.Vertical}
			return pg.Theme.List(pg.listContainer).Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return list.Layout(gtx, len(pg.orderItems), func(gtx C, i int) D {
						return cryptomaterial.LinearLayout{
							Orientation: layout.Vertical,
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Background:  pg.Theme.Color.Surface,
							Direction:   layout.W,
							Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
							Padding:     layout.UniformInset(values.MarginPadding15),
							Margin:      layout.Inset{Bottom: values.MarginPadding4, Top: values.MarginPadding4}}.
							Layout2(gtx, func(gtx C) D {
								return components.OrderItemWidget(gtx, pg.Load, pg.orderItems[i])
							})
					})
				})
			})
		}),
	)
}

func (pg *CreateOrderPage) confirmSourcePassword() {
	walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title("Unlock to create order").
		PasswordHint(values.String(values.StrSpendingPassword)).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := pg.sourceWalletSelector.SelectedWallet().UnlockWallet(password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}

			// pm.Dismiss() // calls RefreshWindow.
			order, err := pg.createOrder()
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}

			err = pg.constructTx(order.DepositAddress, order.InvoicedAmount)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}

			// err = pg.sourceWalletSelector.SelectedWallet().Broadcast(password)
			// if err != nil {
			// 	pm.SetError(err.Error())
			// 	pm.SetLoading(false)
			// 	return false
			// }
			pm.Dismiss()
			pg.ParentNavigator().Display(NewOrderDetailsPage(pg.Load, pg.exchange, order))
			return true
		})
	pg.ParentWindow().ShowModal(walletPasswordModal)

}

func (pg *CreateOrderPage) createOrder() (*instantswap.Order, error) {
	fmt.Println("[][][][] ", pg.fromAmountEditor.Editor.Text())
	invoicedAmount, err := strconv.ParseFloat(pg.fromAmountEditor.Editor.Text(), 8)
	if err != nil {
		return nil, err
	}
	fmt.Println("[][][][] ", invoicedAmount)

	params := api.ExchangeRateRequest{
		From:   "DCR",
		To:     "BTC",
		Amount: invoicedAmount,
	}
	res, err := pg.WL.MultiWallet.InstantSwap.GetExchangeRateInfo(pg.exchange, params)
	if err != nil {
		return nil, err
	}

	refundAddress, err := pg.sourceWalletSelector.SelectedWallet().CurrentAddress(pg.sourceAccountSelector.SelectedAccount().Number)
	if err != nil {
		return nil, err
	}

	destinationAddress, err := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
	if err != nil {
		return nil, err
	}

	data := api.CreateOrder{
		RefundAddress:   refundAddress,      // if the trading fails, the exchange will refund coins here
		Destination:     destinationAddress, // your exchanged coins will be sent here
		FromCurrency:    "DCR",
		InvoicedAmount:  invoicedAmount, // use InvoicedAmount or InvoicedAmount
		ToCurrency:      "BTC",
		ExtraID:         "",
		Signature:       res.Signature,
		UserReferenceID: "",
		RefundExtraID:   "",
	}
	order, err := pg.WL.MultiWallet.InstantSwap.CreateOrder(pg.exchange, data)
	if err != nil {
		return nil, err
	}

	return order, nil
}

func (pg *CreateOrderPage) constructTx(depositAddress string, unitAmount float64) error {
	destinationAddress := depositAddress

	// destinationAccount := pg.destinationAccountSelector.SelectedAccount()

	// amountAtom, SendMax, err := pg.amount.validAmount()
	// if err != nil {
	// 	pg.feeEstimationError(err.Error())
	// 	return
	// }

	// dcrImpl := pg.WL.SelectedWallet.Wallet.(*dcr.DCRAsset)
	// if dcrImpl == nil {
	// 	pg.feeEstimationError("Only DCR implementation is supported")
	// 	// Only DCR implementation is supported past here.
	// 	return
	// }

	sourceAccount := pg.sourceAccountSelector.SelectedAccount()
	err := pg.sourceWalletSelector.SelectedWallet().NewUnsignedTx(sourceAccount.Number)
	if err != nil {
		// pg.feeEstimationError(err.Error())
		return err
	}

	amount := btc.AmountSatoshi(unitAmount)
	err = pg.sourceWalletSelector.SelectedWallet().AddSendDestination(destinationAddress, amount, false)
	if err != nil {
		// pg.feeEstimationError(err.Error())
		return err
	}

	_, err = pg.sourceWalletSelector.SelectedWallet().EstimateFeeAndSize()
	if err != nil {
		// pg.feeEstimationError(err.Error())
		return err
	}

	// feeAtom := feeAndSize.Fee.UnitValue
	// if SendMax {
	// 	amountAtom = sourceAccount.Balance.Spendable.ToInt() - feeAtom
	// }

	// wal := pg.sourceWalletSelector.SelectedWallet()
	// totalSendingAmount := wal.ToAmount(unitAmount + feeAtom)
	// balanceAfterSend := wal.ToAmount(sourceAccount.Balance.Spendable.ToInt() - totalSendingAmount.ToInt())

	// populate display data
	// pg.txFee = wal.ToAmount(feeAtom).String()
	// pg.estSignedSize = fmt.Sprintf("%d bytes", feeAndSize.EstimatedSignedSize)
	// pg.totalCost = totalSendingAmount.String()
	// pg.balanceAfterSend = balanceAfterSend.String()
	// pg.sendAmount = wal.ToAmount(amountAtom).String()
	// pg.destinationAddress = destinationAddress
	// pg.destinationAccount = destinationAccount
	// pg.sourceAccount = sourceAccount

	// if SendMax {
	// 	// TODO: this workaround ignores the change events from the
	// 	// amount input to avoid construct tx cycle.
	// 	pg.amount.setAmount(amountAtom)
	// }

	// if pg.exchangeRate != -1 && pg.usdExchangeSet {
	// 	pg.txFeeUSD = fmt.Sprintf("$%.4f", utils.DCRToUSD(pg.exchangeRate, feeAndSize.Fee.CoinValue))
	// 	pg.totalCostUSD = utils.FormatUSDBalance(pg.Printer, utils.DCRToUSD(pg.exchangeRate, totalSendingAmount.ToCoin()))
	// 	pg.balanceAfterSendUSD = utils.FormatUSDBalance(pg.Printer, utils.DCRToUSD(pg.exchangeRate, balanceAfterSend.ToCoin()))

	// 	usdAmount := utils.DCRToUSD(pg.exchangeRate, wal.ToAmount(amountAtom).ToCoin())
	// 	pg.sendAmountUSD = utils.FormatUSDBalance(pg.Printer, usdAmount)
	// }

	// pg.txAuthor = pg.sourceWalletSelector.SelectedWallet().GetUnsignedTx()

	return nil
}

func (pg *CreateOrderPage) getExchangeRateInfo() error {
	params := api.ExchangeRateRequest{
		From:   "DCR",
		To:     "BTC",
		Amount: 1,
	}
	res, err := pg.WL.MultiWallet.InstantSwap.GetExchangeRateInfo(pg.exchange, params)
	if err != nil {
		return err
	}

	pg.min = res.Min
	pg.max = res.Max

	return nil
}
