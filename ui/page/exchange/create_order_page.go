package exchange

import (
	"context"
	"fmt"
	"strconv"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"gitlab.com/raedah/cryptopower/app"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/values"

	"code.cryptopower.dev/exchange/instantswap"
	_ "code.cryptopower.dev/exchange/instantswap/exchange/flypme" //register flyp.me
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

	scrollContainer *widget.List

	fromAmountEditor cryptomaterial.Editor
	toAmountEditor   cryptomaterial.Editor

	addressEditor cryptomaterial.Editor

	sourceAccountSelector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	backButton cryptomaterial.IconButton
	infoButton cryptomaterial.IconButton

	createOrderBtn cryptomaterial.Button
}

func NewCreateOrderPage(l *load.Load) *CreateOrderPage {
	pg := &CreateOrderPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(CreateOrderPageID),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
	}

	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)

	pg.fromAmountEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrAmount)+" (DCR)")
	pg.fromAmountEditor.Editor.SetText("")
	pg.fromAmountEditor.HasCustomButton = true
	pg.fromAmountEditor.Editor.SingleLine = true

	pg.fromAmountEditor.CustomButton.Inset = layout.UniformInset(values.MarginPadding2)
	pg.fromAmountEditor.CustomButton.Text = values.String(values.StrMax)
	pg.fromAmountEditor.CustomButton.CornerRadius = values.MarginPadding0

	pg.toAmountEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrAmount)+" (BTC)")
	pg.toAmountEditor.Editor.SetText("")
	pg.toAmountEditor.HasCustomButton = false
	pg.toAmountEditor.Editor.SingleLine = true

	pg.toAmountEditor.CustomButton.Inset = layout.UniformInset(values.MarginPadding2)
	pg.toAmountEditor.CustomButton.Text = values.String(values.StrMax)
	pg.toAmountEditor.CustomButton.CornerRadius = values.MarginPadding0

	pg.addressEditor = l.Theme.Editor(new(widget.Editor), "")
	pg.addressEditor.Editor.SetText("")
	pg.addressEditor.Editor.SingleLine = true

	// Source wallet picker
	pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrTo))

	// Source account picker
	pg.sourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrAccount))
	pg.sourceAccountSelector.SelectFirstValidAccount(pg.sourceWalletSelector.SelectedWallet())

	pg.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
	})

	// Destination wallet picker
	pg.destinationWalletSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrTo))

	// Destination account picker
	pg.destinationAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
		Title(values.String(values.StrAccount))
	pg.destinationAccountSelector.SelectFirstValidAccount(pg.destinationWalletSelector.SelectedWallet())

	pg.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		address, _ := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
		pg.addressEditor.Editor.SetText(address)
	})

	pg.destinationAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		address, _ := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
		pg.addressEditor.Editor.SetText(address)
	})

	address, _ := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
	pg.addressEditor.Editor.SetText(address)

	pg.createOrderBtn = pg.Theme.Button("Create Order")

	return pg
}

func (pg *CreateOrderPage) ID() string {
	return CreateOrderPageID
}

func (pg *CreateOrderPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
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
}

func (pg *CreateOrderPage) Layout(gtx C) D {
	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      "Create Order",
			SubTitle:   "flypme",
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
	// return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {

	// })
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
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
								txt := pg.Theme.Label(values.TextSize14, "Min: 0.12982833 . Max: 329.40848571")
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
								txt := pg.Theme.Label(values.TextSize16, "Source")
								txt.Font.Weight = text.SemiBold
								return txt.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								pg.infoButton.Inset = layout.UniformInset(values.MarginPadding0)
								pg.infoButton.Size = values.MarginPadding20
								return pg.infoButton.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Bottom: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							return pg.sourceWalletSelector.Layout(pg.ParentWindow(), gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return pg.sourceAccountSelector.Layout(pg.ParentWindow(), gtx)
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
								txt := pg.Theme.Label(values.TextSize16, "Destination")
								txt.Font.Weight = text.SemiBold
								return txt.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								pg.infoButton.Inset = layout.UniformInset(values.MarginPadding0)
								pg.infoButton.Size = values.MarginPadding20
								return pg.infoButton.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Bottom: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							return pg.destinationWalletSelector.Layout(pg.ParentWindow(), gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Bottom: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							return pg.destinationAccountSelector.Layout(pg.ParentWindow(), gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return pg.addressEditor.Layout(gtx)
					}),
				)

			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top:   values.MarginPadding24,
				Right: values.MarginPadding16,
			}.Layout(gtx, pg.createOrderBtn.Layout)
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
			exchange, orderUUID, err := pg.createOrder()
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			pg.ParentNavigator().Display(NewOrderDetailsPage(pg.Load, exchange, orderUUID))
			return true
		})
	pg.ParentWindow().ShowModal(walletPasswordModal)

	// spendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
	// 	EnableName(false).
	// 	EnableConfirmPassword(false).
	// 	Title(values.String(values.StrResumeAccountDiscoveryTitle)).
	// 	PasswordHint(values.String(values.StrSpendingPassword)).
	// 	SetPositiveButtonText(values.String(values.StrUnlock)).
	// 	SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
	// 		err := pg.WL.SelectedWallet.Wallet.UnlockWallet(password)
	// 		if err != nil {
	// 			pm.SetError(err.Error())
	// 			pm.SetLoading(false)
	// 			return false
	// 		}
	// 		pm.Dismiss()
	// 		// pg.startSyncing(wal)
	// 		return true
	// 	})
	// pg.ParentWindow().ShowModal(spendingPasswordModal)
}

func (pg *CreateOrderPage) createOrder() (instantswap.IDExchange, string, error) {
	// Initialize a new wxchange using the selected exchange server
	exchange, err := instantswap.NewExchange("flypme", instantswap.ExchangeConfig{
		Debug:     false,
		ApiKey:    "", // Optional
		ApiSecret: "", // Optional
	})
	if err != nil {
		return nil, "", err
	}

	res, err := exchange.GetExchangeRateInfo(instantswap.ExchangeRateRequest{
		From:   "DCR",
		To:     "BTC",
		Amount: 5,
	})
	if err != nil {
		return nil, "", err
	}

	refundAddress, err := pg.sourceWalletSelector.SelectedWallet().CurrentAddress(pg.sourceAccountSelector.SelectedAccount().Number)
	if err != nil {
		return nil, "", err
	}

	destinationAddress, err := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
	if err != nil {
		return nil, "", err
	}

	fmt.Println("[][][][] ", pg.fromAmountEditor.Editor.Text())
	invoicedAmount, err := strconv.ParseFloat(pg.fromAmountEditor.Editor.Text(), 8)
	if err != nil {
		return nil, "", err
	}
	fmt.Println("[][][][] ", invoicedAmount)

	order, err := exchange.CreateOrder(instantswap.CreateOrder{
		RefundAddress:   refundAddress,      // if the trading fail, the exchange will refund here
		Destination:     destinationAddress, // your received dcr address
		FromCurrency:    "DCR",
		OrderedAmount:   0, // use OrderedAmount or InvoicedAmount
		InvoicedAmount:  invoicedAmount,
		ToCurrency:      "BTC",
		ExtraID:         "",
		Signature:       res.Signature,
		UserReferenceID: "",
		RefundExtraID:   "",
	})
	if err != nil {
		return nil, "", err
	}

	return exchange, order.UUID, nil
}
