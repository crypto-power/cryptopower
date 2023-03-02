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
	libutils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/listeners"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"code.cryptopower.dev/group/cryptopower/wallet"

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

	*listeners.OrderNotificationListener

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	listContainer *widget.List

	exchange         api.IDExchange
	orderItems       []*instantswap.Order
	ordersList       *cryptomaterial.ClickableList
	exchangeSelector *ExchangeSelector
	selectedExchange *Exchange

	exchangeRateInfo string
	amountErrorText  string
	fetchingRate     bool
	rateError        bool

	materialLoader material.LoaderStyle

	fromAmountEditor components.SelectAssetEditor
	toAmountEditor   components.SelectAssetEditor

	backButton cryptomaterial.IconButton

	createOrderBtn         cryptomaterial.Button
	swapButton             cryptomaterial.IconButton
	refreshExchangeRateBtn cryptomaterial.IconButton
	infoButton             cryptomaterial.IconButton
	settingsButton         cryptomaterial.IconButton
	refreshClickable       *cryptomaterial.Clickable
	refreshIcon            *cryptomaterial.Image

	syncButton cryptomaterial.IconButton

	min          float64
	max          float64
	exchangeRate float64

	*orderData
}

type orderData struct {
	exchange       api.IDExchange
	exchangeServer instantswap.ExchangeServer

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

	fromCurrency libutils.AssetType
	toCurrency   libutils.AssetType

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
		refreshClickable: l.Theme.NewClickable(true),
		refreshIcon:      l.Theme.Icons.Restore,
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

	pg.toAmountEditor = *components.NewSelectAssetEditor(l)
	pg.fromAmountEditor = *components.NewSelectAssetEditor(l)

	pg.fromAmountEditor.AssetTypeSelector.AssetTypeSelected(func(ati *components.AssetTypeItem) {
		pg.fromCurrency = ati.Type
		pg.toAmountEditor.AssetTypeSelector.SelectFirstValidAssetType(&ati.Type)
		oldDesWalletType := pg.orderData.destinationWalletSelector.SelectedAsset()
		if oldDesWalletType.ToStringLower() == ati.Type.ToStringLower() {
			pg.orderData.destinationWalletSelector.SelectFirstValidAssetType(&ati.Type)
		}
		pg.orderData.sourceWalletSelector.SetSelectedAsset(ati.Type)
		pg.updateExchangeRate()
	})
	pg.toAmountEditor.AssetTypeSelector.AssetTypeSelected(func(ati *components.AssetTypeItem) {
		pg.toCurrency = ati.Type
		pg.fromAmountEditor.AssetTypeSelector.SelectFirstValidAssetType(&ati.Type)
		oldSouWalletType := pg.orderData.sourceWalletSelector.SelectedAsset()
		if oldSouWalletType.ToStringLower() == ati.Type.ToStringLower() {
			pg.orderData.sourceWalletSelector.SelectFirstValidAssetType(&ati.Type)
		}
		pg.orderData.destinationWalletSelector.SetSelectedAsset(ati.Type)
		pg.updateExchangeRate()
	})

	pg.loadOrderConfig()

	pg.createOrderBtn = pg.Theme.Button(values.String(values.StrCreateOrder))
	pg.createOrderBtn.SetEnabled(false)

	pg.exchangeSelector.ExchangeSelected(func(es *Exchange) {
		pg.selectedExchange = es

		// Initialize a new exchange using the selected exchange server
		exchange, err := pg.WL.AssetsManager.InstantSwap.NewExchanageServer(pg.selectedExchange.Server)
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
	})

	return pg
}

func (pg *CreateOrderPage) ID() string {
	return CreateOrderPageID
}

func (pg *CreateOrderPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	if pg.isExchangeAPIAllowed() {
		pg.listenForSyncNotifications()
		pg.FetchOrders()
	}

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
		orderSettingsModal := newOrderSettingsModalModal(pg.Load, pg.orderData).
			OnSettingsSaved(func(params *callbackParams) {
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

	for _, evt := range pg.fromAmountEditor.Edit.Editor.Events() {
		if pg.fromAmountEditor.Edit.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				if pg.inputsNotEmpty(pg.fromAmountEditor.Edit.Editor) {
					f, err := strconv.ParseFloat(pg.fromAmountEditor.Edit.Editor.Text(), 32)
					if err != nil {
						// empty usd input
						pg.toAmountEditor.Edit.Editor.SetText("")
						pg.amountErrorText = values.String(values.StrInvalidAmount)
						pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
						pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
						return
					}
					pg.amountErrorText = ""
					if pg.exchangeRate != -1 {
						value := f / pg.exchangeRate
						v := strconv.FormatFloat(value, 'f', 8, 64)
						pg.amountErrorText = ""
						pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
						pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
						pg.toAmountEditor.Edit.Editor.SetText(v) // 2 decimal places
					}
				} else {
					pg.toAmountEditor.Edit.Editor.SetText("")
				}
			}
		}
	}

	for _, evt := range pg.toAmountEditor.Edit.Editor.Events() {
		if pg.toAmountEditor.Edit.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				if pg.inputsNotEmpty(pg.toAmountEditor.Edit.Editor) {
					f, err := strconv.ParseFloat(pg.toAmountEditor.Edit.Editor.Text(), 32)
					if err != nil {
						// empty usd input
						pg.fromAmountEditor.Edit.Editor.SetText("")
						pg.amountErrorText = values.String(values.StrInvalidAmount)
						pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
						pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
						return
					}
					pg.amountErrorText = ""
					if pg.exchangeRate != -1 {
						value := f * pg.exchangeRate
						v := strconv.FormatFloat(value, 'f', 8, 64)
						pg.amountErrorText = ""
						pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
						pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
						pg.fromAmountEditor.Edit.Editor.SetText(v)
					}
				} else {
					pg.fromAmountEditor.Edit.Editor.SetText("")
				}
			}
		}
	}

	if pg.refreshClickable.Clicked() {
		go func() {
			pg.WL.AssetsManager.InstantSwap.Sync(context.Background())
			pg.ParentWindow().Reload()
		}()
	}

}

func (pg *CreateOrderPage) updateAmount() {
	if pg.inputsNotEmpty(pg.fromAmountEditor.Edit.Editor) {
		if pg.fromCurrency.ToStringLower() == pg.toCurrency.ToStringLower() {
			pg.toAmountEditor.Edit.Editor.SetText(pg.fromAmountEditor.Edit.Editor.Text())
			return
		}
		f, err := strconv.ParseFloat(pg.fromAmountEditor.Edit.Editor.Text(), 32)
		if err != nil {
			// empty usd input
			pg.toAmountEditor.Edit.Editor.SetText("")
			pg.amountErrorText = values.String(values.StrInvalidAmount)
			pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
			pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Danger
			return
		}
		pg.amountErrorText = ""
		if pg.exchangeRate != -1 {
			value := f / pg.exchangeRate
			v := strconv.FormatFloat(value, 'f', 8, 64)
			pg.amountErrorText = ""
			pg.fromAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
			pg.toAmountEditor.Edit.LineColor = pg.Theme.Color.Gray2
			pg.toAmountEditor.Edit.Editor.SetText(v) // 2 decimal places
		}
	} else {
		pg.toAmountEditor.Edit.Editor.SetText("")
	}
}

func (pg *CreateOrderPage) canCreateOrder() bool {
	if pg.selectedExchange == nil {
		return false
	}

	if pg.exchangeRate == 0 {
		return false
	}

	if pg.fromAmountEditor.Edit.Editor.Text() == "" {
		return false
	}

	if pg.toAmountEditor.Edit.Editor.Text() == "" {
		return false
	}

	if pg.amountErrorText != "" {
		return false
	}

	if pg.fromCurrency.ToStringLower() == pg.toCurrency.ToStringLower() {
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
	tempSourceWalletSelector := pg.orderData.sourceWalletSelector
	tempSourceAccountSelector := pg.orderData.sourceAccountSelector
	tempFromCurrencyType := pg.fromCurrency
	tempFromCurrencyValue := pg.fromAmountEditor.Edit.Editor.Text()

	// Swap values
	pg.orderData.sourceWalletSelector = pg.orderData.destinationWalletSelector
	pg.orderData.sourceAccountSelector = pg.orderData.destinationAccountSelector
	pg.fromCurrency = pg.toCurrency
	pg.fromAmountEditor.Edit.Editor.SetText(pg.toAmountEditor.Edit.Editor.Text())
	pg.fromAmountEditor.AssetTypeSelector.SetSelectedAssetType(&pg.fromCurrency)

	pg.orderData.destinationWalletSelector = tempSourceWalletSelector
	pg.orderData.destinationAccountSelector = tempSourceAccountSelector
	pg.toCurrency = tempFromCurrencyType
	pg.toAmountEditor.Edit.Editor.SetText(tempFromCurrencyValue)
	pg.toAmountEditor.AssetTypeSelector.SetSelectedAssetType(&pg.toCurrency)

	// check the watch only wallet on destination
	if pg.orderData.sourceWalletSelector.SelectedWallet().IsWatchingOnlyWallet() {
		pg.orderData.sourceWalletSelector.SetSelectedAsset(pg.orderData.fromCurrency)
	}

	// update title of wallet selector
	pg.orderData.sourceWalletSelector.Title(values.String(values.StrSource)).EnableWatchOnlyWallets(false)
	pg.orderData.destinationWalletSelector.Title(values.String(values.StrDestination)).EnableWatchOnlyWallets(true)
}

func (pg *CreateOrderPage) isExchangeAPIAllowed() bool {
	return pg.WL.AssetsManager.IsHttpAPIPrivacyModeOff(libutils.ExchangeHttpAPI)
}

func (pg *CreateOrderPage) Layout(gtx C) D {
	overlay := layout.Stacked(func(gtx C) D { return D{} })
	if !pg.isExchangeAPIAllowed() {
		overlay = layout.Stacked(func(gtx C) D {
			str := values.StringF(values.StrNotAllowed, values.String(values.StrExchange))
			return components.DisablePageWithOverlay(pg.Load, nil, gtx.Disabled(), str)
		})
	}

	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrCreateOrder),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				gtxCopy := gtx
				if !pg.isExchangeAPIAllowed() {
					gtxCopy = gtx.Disabled()
				}
				return layout.Stack{}.Layout(gtxCopy, layout.Expanded(pg.layout), overlay)
			},
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
											}.Layout(gtx, pg.infoButton.Layout)
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
									walletName := "----"
									if pg.orderData.sourceWalletSelector.SelectedWallet() != nil {
										walletName = pg.orderData.sourceWalletSelector.SelectedWallet().GetWalletName()
									}
									accountName := "----"
									if pg.orderData.sourceAccountSelector.SelectedAccount() != nil {
										accountName = pg.orderData.sourceAccountSelector.SelectedAccount().Name
									}
									txt := fmt.Sprintf("%s: %s[%s]", values.String(values.StrSource), walletName, accountName)
									lb := pg.Theme.Label(values.TextSize16, txt)
									lb.Font.Weight = text.SemiBold
									return lb.Layout(gtx)
								}),
								// layout.Rigid(pg.fromAmountEditor.Layout),
								layout.Rigid(func(gtx C) D {
									return pg.fromAmountEditor.Layout(pg.ParentWindow(), gtx)
								}),
							)
						}),
						layout.Flexed(0.1, func(gtx C) D {
							return layout.Center.Layout(gtx, pg.swapButton.Layout)
						}),
						layout.Flexed(0.45, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									walletName := "----"
									if pg.orderData.destinationWalletSelector.SelectedWallet() != nil {
										walletName = pg.orderData.destinationWalletSelector.SelectedWallet().GetWalletName()
									}
									accountName := "----"
									if pg.orderData.destinationAccountSelector.SelectedAccount() != nil {
										accountName = pg.orderData.destinationAccountSelector.SelectedAccount().Name
									}
									txt := fmt.Sprintf("%s: %s[%s]", values.String(values.StrDestination), walletName, accountName)
									lb := pg.Theme.Label(values.TextSize16, txt)
									lb.Font.Weight = text.SemiBold
									return lb.Layout(gtx)
								}),
								// layout.Rigid(pg.toAmountEditor.Layout),
								layout.Rigid(func(gtx C) D {
									return pg.toAmountEditor.Layout(pg.ParentWindow(), gtx)
								}),
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
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										txt := pg.Theme.Label(values.TextSize18, values.String(values.StrHistory))
										txt.Font.Weight = text.SemiBold
										return txt.Layout(gtx)
									}),
									layout.Flexed(1, func(gtx C) D {
										body := func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													var text string
													if pg.WL.AssetsManager.InstantSwap.IsSyncing() {
														text = values.String(values.StrSyncingState)
													} else {
														text = values.String(values.StrUpdated) + " " + components.TimeAgo(pg.WL.AssetsManager.InstantSwap.GetLastSyncedTimeStamp())
													}

													lastUpdatedInfo := pg.Theme.Label(values.TextSize10, text)
													lastUpdatedInfo.Color = pg.Theme.Color.GrayText2
													return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, lastUpdatedInfo.Layout)
												}),
												layout.Rigid(func(gtx C) D {
													return cryptomaterial.LinearLayout{
														Width:     cryptomaterial.WrapContent,
														Height:    cryptomaterial.WrapContent,
														Clickable: pg.refreshClickable,
														Direction: layout.Center,
														Alignment: layout.Middle,
														Margin:    layout.Inset{Left: values.MarginPadding10},
													}.Layout(gtx,
														layout.Rigid(func(gtx C) D {
															return layout.Inset{Right: values.MarginPadding16}.Layout(gtx, pg.refreshIcon.Layout16dp)
														}),
													)
												}),
											)
										}
										return layout.E.Layout(gtx, body)
									}),
								)
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
							Margin:      layout.Inset{Bottom: values.MarginPadding4, Top: values.MarginPadding4},
						}.
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
	invoicedAmount, _ := strconv.ParseFloat(pg.fromAmountEditor.Edit.Editor.Text(), 32)
	orderedAmount, _ := strconv.ParseFloat(pg.toAmountEditor.Edit.Editor.Text(), 32)

	refundAddress, _ := pg.orderData.sourceWalletSelector.SelectedWallet().CurrentAddress(pg.orderData.sourceAccountSelector.SelectedAccount().Number)
	destinationAddress, _ := pg.orderData.destinationWalletSelector.SelectedWallet().CurrentAddress(pg.orderData.destinationAccountSelector.SelectedAccount().Number)

	pg.orderData.exchange = pg.exchange
	pg.orderData.exchangeServer = pg.selectedExchange.Server
	pg.orderData.sourceWalletSelector = pg.sourceWalletSelector
	pg.orderData.sourceAccountSelector = pg.sourceAccountSelector
	pg.orderData.destinationWalletSelector = pg.sourceWalletSelector
	pg.orderData.destinationAccountSelector = pg.destinationAccountSelector

	pg.sourceWalletID = pg.orderData.sourceWalletSelector.SelectedWallet().GetWalletID()
	pg.sourceAccountNumber = pg.orderData.sourceAccountSelector.SelectedAccount().Number
	pg.destinationWalletID = pg.orderData.destinationWalletSelector.SelectedWallet().GetWalletID()
	pg.destinationAccountNumber = pg.orderData.destinationAccountSelector.SelectedAccount().Number

	pg.invoicedAmount = invoicedAmount
	pg.orderedAmount = orderedAmount

	pg.refundAddress = refundAddress
	pg.destinationAddress = destinationAddress

	confirmOrderModal := newConfirmOrderModal(pg.Load, pg.orderData).
		OnOrderCompleted(func(order *instantswap.Order) {
			pg.FetchOrders()
			successModal := modal.NewCustomModal(pg.Load).
				Title(values.String(values.StrOrderSubmitted)).
				SetCancelable(true).
				SetContentAlignment(layout.Center, layout.Center, layout.Center).
				SetNegativeButtonText(values.String(values.StrOK)).
				SetNegativeButtonCallback(func() {
				}).
				PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
				SetPositiveButtonText(values.String(values.StrOrderDetails)).
				SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
					pg.ParentNavigator().Display(NewOrderDetailsPage(pg.Load, order))
					return true
				})
			pg.ParentWindow().ShowModal(successModal)
		})

	pg.ParentWindow().ShowModal(confirmOrderModal)
}

func (pg *CreateOrderPage) updateExchangeRate() {
	if pg.fromCurrency.ToStringLower() == pg.toCurrency.ToStringLower() {
		return
	}
	if pg.exchange != nil {
		go func() {
			err := pg.getExchangeRateInfo()
			if err != nil {
				log.Error(err)
			}
		}()
	}
}

func (pg *CreateOrderPage) getExchangeRateInfo() error {
	pg.fetchingRate = true
	params := api.ExchangeRateRequest{
		From:   pg.fromCurrency.String(),
		To:     pg.toCurrency.String(),
		Amount: 1,
	}
	res, err := pg.WL.AssetsManager.InstantSwap.GetExchangeRateInfo(pg.exchange, params)
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
	pg.updateAmount()

	pg.fetchingRate = false
	pg.rateError = false

	return nil
}

func (pg *CreateOrderPage) loadOrderConfig() {
	if pg.WL.AssetsManager.ExchangeConfigIsSet() {
		exchangeConfig := pg.WL.AssetsManager.ExchangeConfig()
		sourceWallet := pg.WL.AssetsManager.WalletWithID(int(exchangeConfig.SourceWalletID))
		destinationWallet := pg.WL.AssetsManager.WalletWithID(int(exchangeConfig.DestinationWalletID))

		sourceCurrency := exchangeConfig.SourceAsset
		toCurrency := exchangeConfig.DestinationAsset

		if sourceWallet != nil {
			_, err := sourceWallet.GetAccount(exchangeConfig.SourceAccountNumber)
			if err != nil {
				log.Error(err)
			}

			// Source wallet picker
			pg.orderData.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, sourceCurrency).
				Title(values.String(values.StrSource))

			sourceW := &load.WalletMapping{
				Asset: sourceWallet,
			}
			pg.orderData.sourceWalletSelector.SetSelectedWallet(sourceW)

			// Source account picker
			pg.orderData.sourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
				Title(values.String(values.StrAccount)).
				AccountValidator(func(account *sharedW.Account) bool {
					accountIsValid := account.Number != load.MaxInt32
					return accountIsValid
				})
			pg.orderData.sourceAccountSelector.SelectAccount(pg.orderData.sourceWalletSelector.SelectedWallet(), exchangeConfig.SourceAccountNumber)

			pg.orderData.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
				pg.orderData.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
			})
		}

		if destinationWallet != nil {
			_, err := destinationWallet.GetAccount(exchangeConfig.DestinationAccountNumber)
			if err != nil {
				log.Error(err)
			}

			// Destination wallet picker
			pg.orderData.destinationWalletSelector = components.NewWalletAndAccountSelector(pg.Load, toCurrency).
				Title(values.String(values.StrDestination)).
				EnableWatchOnlyWallets(true)

			// Destination account picker
			pg.orderData.destinationAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
				Title(values.String(values.StrAccount)).
				AccountValidator(func(account *sharedW.Account) bool {
					// Imported accounts and watch only accounts are imvalid
					accountIsValid := account.Number != load.MaxInt32

					return accountIsValid
				})
			pg.orderData.destinationAccountSelector.SelectAccount(pg.orderData.destinationWalletSelector.SelectedWallet(), exchangeConfig.DestinationAccountNumber)

			pg.orderData.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
				pg.orderData.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
			})
		}
	} else {
		// Source wallet picker
		pg.orderData.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, libutils.DCRWalletAsset).
			Title(values.String(values.StrFrom))

		// Source account picker
		pg.orderData.sourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
			Title(values.String(values.StrAccount)).
			AccountValidator(func(account *sharedW.Account) bool {
				accountIsValid := account.Number != load.MaxInt32

				return accountIsValid
			})
		pg.orderData.sourceAccountSelector.SelectFirstValidAccount(pg.orderData.sourceWalletSelector.SelectedWallet())

		pg.orderData.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
			pg.orderData.sourceAccountSelector.SelectFirstValidAccount(selectedWallet)
		})

		// Destination wallet picker
		pg.orderData.destinationWalletSelector = components.NewWalletAndAccountSelector(pg.Load, libutils.BTCWalletAsset).
			Title(values.String(values.StrTo)).
			EnableWatchOnlyWallets(true)

		// Destination account picker
		pg.orderData.destinationAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
			Title(values.String(values.StrAccount)).
			AccountValidator(func(account *sharedW.Account) bool {
				accountIsValid := account.Number != load.MaxInt32

				return accountIsValid
			})
		pg.orderData.destinationAccountSelector.SelectFirstValidAccount(pg.orderData.destinationWalletSelector.SelectedWallet())

		pg.orderData.destinationWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
			pg.orderData.destinationAccountSelector.SelectFirstValidAccount(selectedWallet)
		})
	}
	pg.fromCurrency = pg.orderData.sourceWalletSelector.SelectedWallet().GetAssetType()
	pg.toCurrency = pg.orderData.destinationWalletSelector.SelectedWallet().GetAssetType()
	pg.fromAmountEditor.AssetTypeSelector.SetSelectedAssetType(&pg.fromCurrency)
	pg.toAmountEditor.AssetTypeSelector.SetSelectedAssetType(&pg.toCurrency)
}

func (pg *CreateOrderPage) listenForSyncNotifications() {
	if pg.OrderNotificationListener != nil {
		return
	}
	pg.OrderNotificationListener = listeners.NewOrderNotificationListener()
	err := pg.WL.AssetsManager.InstantSwap.AddNotificationListener(pg.OrderNotificationListener, CreateOrderPageID)
	if err != nil {
		log.Errorf("Error adding instanswap notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-pg.OrderNotifChan:
				if n.OrderStatus == wallet.OrderStatusSynced {
					pg.FetchOrders()
					pg.ParentWindow().Reload()
				}
			case <-pg.ctx.Done():
				pg.WL.AssetsManager.InstantSwap.RemoveNotificationListener(CreateOrderPageID)
				close(pg.OrderNotifChan)
				pg.OrderNotificationListener = nil

				return
			}
		}
	}()
}
