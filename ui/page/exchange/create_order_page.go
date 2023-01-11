package exchange

import (
	"context"
	"fmt"
	"strconv"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"code.cryptopower.dev/group/cryptopower/app"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"

	api "code.cryptopower.dev/group/instantswap"
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
	ordersList       *cryptomaterial.ClickableList
	exchangeSelector *ExchangeSelector
	selectedExchange *Exchange

	fromCurrencyType utils.AssetType
	toCurrencyType   utils.AssetType
	exchangeRateInfo string
	amountErrorText  string
	fetchingRate     bool
	rateError        bool

	materialLoader material.LoaderStyle

	fromAmountEditor cryptomaterial.Editor
	toAmountEditor   cryptomaterial.Editor
	addressEditor    cryptomaterial.Editor

	sourceAccountSelector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	backButton cryptomaterial.IconButton

	createOrderBtn         cryptomaterial.Button
	swapButton             cryptomaterial.IconButton
	refreshExchangeRateBtn cryptomaterial.IconButton
	infoButton             cryptomaterial.IconButton
	settingsButton         cryptomaterial.IconButton

	min          float64
	max          float64
	exchangeRate float64

	*orderData
}

type orderData struct {
	exchange api.IDExchange
	server   instantswap.ExchangeServer

	sourceAccountSelector *components.WalletAndAccountSelector
	sourceWalletSelector  *components.WalletAndAccountSelector

	destinationAccountSelector *components.WalletAndAccountSelector
	destinationWalletSelector  *components.WalletAndAccountSelector

	sourceWalletID           int
	sourceAccountNumber      int32
	destinationWalletID      int
	destinationAccountNumber int32

	invoicedAmount float64
	orderedAmount  float64

	fromCurrency utils.AssetType
	toCurrency   utils.AssetType

	refundAddress      string
	destinationAddress string
}

func NewCreateOrderPage(l *load.Load) *CreateOrderPage {
	pg := &CreateOrderPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(CreateOrderPageID),
		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		exchangeSelector: NewExchangeSelector(l),
		orderData:        &orderData{},
		exchangeRate:     -1,
	}

	pg.backButton, _ = components.SubpageHeaderButtons(l)

	pg.swapButton = l.Theme.IconButton(l.Theme.Icons.ActionSwapHoriz)
	pg.refreshExchangeRateBtn = l.Theme.IconButton(l.Theme.Icons.NavigationRefresh)
	pg.refreshExchangeRateBtn.Size = values.MarginPadding18

	pg.settingsButton = l.Theme.IconButton(l.Theme.Icons.ActionSettings)
	pg.infoButton = l.Theme.IconButton(l.Theme.Icons.ActionInfo)
	pg.infoButton.Size = values.MarginPadding18
	buttonInset := layout.UniformInset(values.MarginPadding0)
	pg.settingsButton.Inset, pg.infoButton.Inset,
		pg.swapButton.Inset, pg.refreshExchangeRateBtn.Inset = buttonInset, buttonInset, buttonInset, buttonInset

	pg.exchangeRateInfo = fmt.Sprintf(values.String(values.StrMinMax), pg.min, pg.max)
	pg.materialLoader = material.Loader(l.Theme.Base)

	pg.ordersList = pg.Theme.NewClickableList(layout.Vertical)
	pg.ordersList.IsShadowEnabled = true

	pg.fromAmountEditor = l.Theme.Editor(new(widget.Editor), "")
	pg.fromAmountEditor.Editor.SetText("")
	pg.fromAmountEditor.HasCustomButton = true
	pg.fromAmountEditor.Editor.SingleLine = true

	pg.fromAmountEditor.CustomButton.Inset = layout.UniformInset(values.MarginPadding2)
	pg.fromAmountEditor.CustomButton.Text = utils.DCRWalletAsset.String()
	pg.fromAmountEditor.CustomButton.Background = l.Theme.Color.Primary
	pg.fromAmountEditor.CustomButton.CornerRadius = values.MarginPadding0

	pg.toAmountEditor = l.Theme.Editor(new(widget.Editor), "")
	pg.toAmountEditor.Editor.SetText("")
	pg.toAmountEditor.HasCustomButton = true
	pg.toAmountEditor.Editor.SingleLine = true

	pg.toAmountEditor.CustomButton.Inset = layout.UniformInset(values.MarginPadding2)
	pg.toAmountEditor.CustomButton.Text = utils.BTCWalletAsset.String()
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
	address, err := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
	if err != nil {
		log.Error(err)
	}
	pg.addressEditor.Editor.SetText(address)

	pg.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		address, err := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
		if err != nil {
			log.Error(err)
		}
		pg.addressEditor.Editor.SetText(address)
	})

	pg.destinationAccountSelector.AccountSelected(func(selectedAccount *sharedW.Account) {
		address, err := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)
		if err != nil {
			log.Error(err)
		}
		pg.addressEditor.Editor.SetText(address)
	})

	pg.fromCurrencyType = pg.sourceWalletSelector.SelectedWallet().GetAssetType()
	pg.toCurrencyType = pg.destinationWalletSelector.SelectedWallet().GetAssetType()

	pg.createOrderBtn = pg.Theme.Button(values.String(values.StrCreateOrder))
	pg.createOrderBtn.SetEnabled(false)

	pg.exchangeSelector.ExchangeSelected(func(es *Exchange) {
		pg.selectedExchange = es

		// Initialize a new exchange using the selected exchange server
		exchange, err := pg.WL.MultiWallet.InstantSwap.NewExchanageServer(pg.selectedExchange.Server)
		if err != nil {
			log.Error(err)
			return
		}
		pg.exchange = exchange

		go func() {
			err := pg.getExchangeRateInfo()
			if err != nil {
				log.Error(err)
			}
		}()

		pg.createOrderBtn.SetEnabled(true)
	})

	return pg
}

func (pg *CreateOrderPage) ID() string {
	return CreateOrderPageID
}

func (pg *CreateOrderPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.FetchOrders()
}

func (pg *CreateOrderPage) OnNavigatedFrom() {
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}

func (pg *CreateOrderPage) HandleUserInteractions() {
	pg.createOrderBtn.SetEnabled(pg.canCreateOrder())

	if pg.swapButton.Button.Clicked() {
		pg.swapCurrency()
		if pg.exchange != nil {
			go func() {
				err := pg.getExchangeRateInfo()
				if err != nil {
					log.Error(err)
				}
			}()
		}
	}

	if clicked, selectedItem := pg.ordersList.ItemClicked(); clicked {
		selectedOrder := pg.orderItems[selectedItem]
		pg.ParentNavigator().Display(NewOrderDetailsPage(pg.Load, selectedOrder))
	}

	if pg.refreshExchangeRateBtn.Button.Clicked() {
		go func() {
			err := pg.getExchangeRateInfo()
			if err != nil {
				log.Error(err)
			}
		}()
	}

	if pg.createOrderBtn.Clicked() {
		pg.showConfirmOrderModal()
	}

	if pg.settingsButton.Button.Clicked() {
		pg.fromCurrency = pg.fromCurrencyType
		pg.toCurrency = pg.toCurrencyType
		orderSettingsModal := newOrderSettingsModalModal(pg.Load, pg.orderData).
			OnSettingsSaved(func(params *callbackParams) {
				pg.sourceAccountSelector = params.sourceAccountSelector
				pg.sourceWalletSelector = params.sourceWalletSelector
				pg.destinationAccountSelector = params.destinationAccountSelector
				pg.destinationWalletSelector = params.destinationWalletSelector

				infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrOrderSettingsSaved), modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(infoModal)
			}).
			OnCancel(func() { // needed to satisfy the modal instance
			})
		pg.ParentWindow().ShowModal(orderSettingsModal)
	}

	if pg.infoButton.Button.Clicked() {
		info := modal.NewCustomModal(pg.Load).
			SetContentAlignment(layout.Center, layout.Center, layout.Center).
			Body(values.String(values.StrCreateOrderPageInfo)).
			PositiveButtonWidth(values.MarginPadding100)
		pg.ParentWindow().ShowModal(info)
	}

	for _, evt := range pg.fromAmountEditor.Editor.Events() {
		if pg.fromAmountEditor.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				if pg.inputsNotEmpty(pg.fromAmountEditor.Editor) {
					f, err := strconv.ParseFloat(pg.fromAmountEditor.Editor.Text(), 8)
					if err != nil {
						// empty usd input
						pg.toAmountEditor.Editor.SetText("")
						pg.amountErrorText = values.String(values.StrInvalidAmount)
						pg.fromAmountEditor.LineColor = pg.Theme.Color.Danger
						pg.toAmountEditor.LineColor = pg.Theme.Color.Danger
						return
					}
					pg.amountErrorText = ""
					if pg.exchangeRate != -1 {
						value := f / pg.exchangeRate
						v := strconv.FormatFloat(value, 'f', -1, 64)
						pg.amountErrorText = ""
						pg.fromAmountEditor.LineColor = pg.Theme.Color.Gray2
						pg.toAmountEditor.LineColor = pg.Theme.Color.Gray2
						pg.toAmountEditor.Editor.SetText(v) // 2 decimal places
					}
				}

			}
		}
	}

	for _, evt := range pg.toAmountEditor.Editor.Events() {
		if pg.toAmountEditor.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				if pg.inputsNotEmpty(pg.toAmountEditor.Editor) {
					f, err := strconv.ParseFloat(pg.toAmountEditor.Editor.Text(), 8)
					if err != nil {
						// empty usd input
						pg.fromAmountEditor.Editor.SetText("")
						pg.amountErrorText = values.String(values.StrInvalidAmount)
						pg.fromAmountEditor.LineColor = pg.Theme.Color.Danger
						pg.toAmountEditor.LineColor = pg.Theme.Color.Danger
						return
					}
					pg.amountErrorText = ""
					if pg.exchangeRate != -1 {
						value := f * pg.exchangeRate
						v := strconv.FormatFloat(value, 'f', -1, 64)
						pg.amountErrorText = ""
						pg.fromAmountEditor.LineColor = pg.Theme.Color.Gray2
						pg.toAmountEditor.LineColor = pg.Theme.Color.Gray2
						pg.fromAmountEditor.Editor.SetText(v)
					}
				}

			}
		}
	}

}

func (pg *CreateOrderPage) canCreateOrder() bool {
	if pg.selectedExchange == nil {
		return false
	}

	if pg.exchangeRate == 0 {
		return false
	}

	if pg.fromAmountEditor.Editor.Text() == "" {
		return false
	}

	if pg.toAmountEditor.Editor.Text() == "" {
		return false
	}

	if pg.amountErrorText != "" {
		return false
	}

	return true
}

func (pg *CreateOrderPage) inputsNotEmpty(editors ...*widget.Editor) bool {
	for _, e := range editors {
		if e.Text() == "" {
			pg.amountErrorText = ""
			return false
		}
	}
	return true
}

// swapCurrency swaps the values of the from and to currency fields.
func (pg *CreateOrderPage) swapCurrency() {
	// store the current value of the from currency in a temp variable
	tempSourceWalletSelector := pg.sourceWalletSelector
	tempSourceAccountSelector := pg.sourceAccountSelector
	tempFromCurrencyType := pg.fromCurrencyType
	tempFromCurrencyValue := pg.fromAmountEditor.Editor.Text()
	tempFromButtonText := pg.fromAmountEditor.CustomButton.Text
	tempFromButtonBackground := pg.fromAmountEditor.CustomButton.Background

	// Swap values
	pg.sourceWalletSelector = pg.destinationWalletSelector
	pg.sourceAccountSelector = pg.destinationAccountSelector
	pg.fromCurrencyType = pg.toCurrencyType
	pg.fromAmountEditor.Editor.SetText(pg.toAmountEditor.Editor.Text())
	pg.fromAmountEditor.CustomButton.Text = pg.toAmountEditor.CustomButton.Text
	pg.fromAmountEditor.CustomButton.Background = pg.toAmountEditor.CustomButton.Background

	pg.destinationWalletSelector = tempSourceWalletSelector
	pg.destinationAccountSelector = tempSourceAccountSelector
	pg.toCurrencyType = tempFromCurrencyType
	pg.toAmountEditor.Editor.SetText(tempFromCurrencyValue)
	pg.toAmountEditor.CustomButton.Text = tempFromButtonText
	pg.toAmountEditor.CustomButton.Background = tempFromButtonBackground
}

func (pg *CreateOrderPage) Layout(gtx C) D {
	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrCreateOrder),
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
														txt := pg.Theme.Label(values.TextSize16, values.String(values.StrSelectServerTitle))
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
					return layout.Flex{
						Axis:      layout.Horizontal,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Flexed(0.45, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									txt := pg.Theme.Label(values.TextSize16, values.String(values.StrFrom))
									txt.Font.Weight = text.SemiBold
									return txt.Layout(gtx)
								}),
								layout.Rigid(pg.fromAmountEditor.Layout),
							)
						}),
						layout.Flexed(0.1, func(gtx C) D {
							return layout.Center.Layout(gtx, pg.swapButton.Layout)
						}),
						layout.Flexed(0.45, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									txt := pg.Theme.Label(values.TextSize16, values.String(values.StrTo))
									txt.Font.Weight = text.SemiBold
									return txt.Layout(gtx)
								}),
								layout.Rigid(pg.toAmountEditor.Layout),
							)
						}),
					)
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
										if pg.amountErrorText != "" {
											txt := pg.Theme.Label(values.TextSize14, pg.amountErrorText)
											txt.Font.Weight = text.SemiBold
											txt.Color = pg.Theme.Color.Danger
											return txt.Layout(gtx)
										}

										return D{}
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												if pg.fetchingRate {
													gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding16)
													gtx.Constraints.Min.X = gtx.Constraints.Max.X
													return pg.materialLoader.Layout(gtx)
												}
												txt := pg.Theme.Label(values.TextSize14, pg.exchangeRateInfo)
												txt.Color = pg.Theme.Color.Gray1
												txt.Font.Weight = text.SemiBold
												return txt.Layout(gtx)
											}),
											layout.Rigid(func(gtx C) D {
												if !pg.fetchingRate && pg.rateError {
													return pg.refreshExchangeRateBtn.Layout(gtx)
												}
												return D{}
											}),
										)
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
								txt := pg.Theme.Label(values.TextSize18, values.String(values.StrHistory))
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
			return pg.Theme.List(pg.listContainer).Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return pg.ordersList.Layout(gtx, len(pg.orderItems), func(gtx C, i int) D {
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

func (pg *CreateOrderPage) showConfirmOrderModal() {
	invoicedAmount, _ := strconv.ParseFloat(pg.fromAmountEditor.Editor.Text(), 8)
	orderedAmount, _ := strconv.ParseFloat(pg.toAmountEditor.Editor.Text(), 8)

	refundAddress, _ := pg.sourceWalletSelector.SelectedWallet().CurrentAddress(pg.sourceAccountSelector.SelectedAccount().Number)
	destinationAddress, _ := pg.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.destinationAccountSelector.SelectedAccount().Number)

	pg.orderData.exchange = pg.exchange
	pg.orderData.server = pg.selectedExchange.Server
	pg.orderData.sourceWalletSelector = pg.sourceWalletSelector
	pg.orderData.sourceAccountSelector = pg.sourceAccountSelector
	pg.orderData.destinationWalletSelector = pg.sourceWalletSelector
	pg.orderData.destinationAccountSelector = pg.destinationAccountSelector

	pg.sourceWalletID = pg.sourceWalletSelector.SelectedWallet().GetWalletID()
	pg.sourceAccountNumber = pg.sourceAccountSelector.SelectedAccount().Number
	pg.destinationWalletID = pg.destinationWalletSelector.SelectedWallet().GetWalletID()
	pg.destinationAccountNumber = pg.destinationAccountSelector.SelectedAccount().Number

	pg.invoicedAmount = invoicedAmount
	pg.orderedAmount = orderedAmount

	pg.fromCurrency = pg.fromCurrencyType
	pg.toCurrency = pg.toCurrencyType

	pg.refundAddress = refundAddress
	pg.destinationAddress = destinationAddress

	confirmOrderModal := newConfirmOrderModal(pg.Load, pg.orderData)

	pg.ParentWindow().ShowModal(confirmOrderModal)

}

func (pg *CreateOrderPage) getExchangeRateInfo() error {
	pg.fetchingRate = true
	params := api.ExchangeRateRequest{
		From:   pg.fromCurrencyType.String(),
		To:     pg.toCurrencyType.String(),
		Amount: 0,
	}
	res, err := pg.WL.MultiWallet.InstantSwap.GetExchangeRateInfo(pg.exchange, params)
	if err != nil {
		pg.exchangeRateInfo = values.String(values.StrFetchRateError)
		pg.rateError = true
		pg.fetchingRate = false
		return err
	}

	pg.min = res.Min
	pg.max = res.Max
	pg.exchangeRate = res.ExchangeRate

	pg.exchangeRateInfo = fmt.Sprintf(values.String(values.StrMinMax), pg.min, pg.max)

	pg.fetchingRate = false
	pg.rateError = false

	return nil
}
