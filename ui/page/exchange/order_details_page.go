package exchange

import (
	"context"
	"fmt"
	"time"

	"gioui.org/layout"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"

	api "github.com/crypto-power/instantswap/instantswap"
)

const OrderDetailsPageID = "OrderDetailsPage"

type OrderDetailsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	exchange  api.IDExchange
	orderInfo *instantswap.Order

	backButton     cryptomaterial.IconButton
	refreshBtn     cryptomaterial.Button
	createOrderBtn cryptomaterial.Button

	isRefreshing bool
}

func NewOrderDetailsPage(l *load.Load, order *instantswap.Order) *OrderDetailsPage {
	pg := &OrderDetailsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OrderDetailsPageID),
		orderInfo:        order,
	}

	// if the order was created before the ExchangeServer field was added
	// to the Order struct update it here. This prevents a crash when
	// attempting to open legacy orders
	nilExchangeServer := instantswap.ExchangeServer{}
	if order.ExchangeServer == nilExchangeServer {
		switch order.Server {
		case instantswap.ChangeNow:
			order.ExchangeServer.Server = order.Server
			order.ExchangeServer.Config = instantswap.ExchangeConfig{
				APIKey: instantswap.API_KEY_CHANGENOW,
			}
		case instantswap.GoDex:
			order.ExchangeServer.Server = order.Server
			order.ExchangeServer.Config = instantswap.ExchangeConfig{
				APIKey: instantswap.API_KEY_GODEX,
			}
		default:
			order.ExchangeServer.Server = order.Server
			order.ExchangeServer.Config = instantswap.ExchangeConfig{}
		}

		err := pg.AssetsManager.InstantSwap.UpdateOrder(order)
		if err != nil {
			log.Errorf("Error updating legacy order: %v", err)
		}
	}

	exchange, err := pg.AssetsManager.InstantSwap.NewExchangeServer(order.ExchangeServer)
	if err != nil {
		log.Error(err)
	}
	pg.exchange = exchange

	pg.backButton, _ = components.SubpageHeaderButtons(l)

	pg.createOrderBtn = pg.Theme.Button(values.String(values.StrCreateNewOrder))
	pg.refreshBtn = pg.Theme.Button(values.String(values.StrRefresh))

	go func() {
		pg.isRefreshing = true
		pg.orderInfo, err = pg.getOrderInfo(pg.orderInfo.UUID)
		if err != nil {
			pg.isRefreshing = false
			log.Error(err)
		}

		time.Sleep(1 * time.Second)
		pg.isRefreshing = false
	}()

	return pg
}

func (pg *OrderDetailsPage) ID() string {
	return OrderDetailsPageID
}

func (pg *OrderDetailsPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
}

func (pg *OrderDetailsPage) OnNavigatedFrom() {
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}

func (pg *OrderDetailsPage) HandleUserInteractions() {
	if pg.refreshBtn.Clicked() {
		go func() {
			pg.isRefreshing = true
			pg.orderInfo, _ = pg.getOrderInfo(pg.orderInfo.UUID)

			time.Sleep(1 * time.Second)
			pg.isRefreshing = false
		}()
	}

	if pg.createOrderBtn.Clicked() {
		pg.ParentNavigator().CloseCurrentPage()
	}
}

func (pg *OrderDetailsPage) Layout(gtx C) D {
	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrOrderDetails),
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

func (pg *OrderDetailsPage) layout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.MatchParent,
		Direction: layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:     gtx.Dp(values.MarginPadding550),
			Height:    cryptomaterial.MatchParent,
			Direction: layout.W,
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
							layout.Rigid(func(gtx C) D {
								return layout.E.Layout(gtx, func(gtx C) D {
									return layout.Flex{
										Axis:      layout.Horizontal,
										Alignment: layout.Middle,
									}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
												return pg.Theme.Card().Layout(gtx, func(gtx C) D {
													return layout.UniformInset(values.MarginPadding20).Layout(gtx, func(gtx C) D {
														return layout.Center.Layout(gtx, func(gtx C) D {
															return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																layout.Rigid(func(gtx C) D {
																	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																		layout.Rigid(func(gtx C) D {
																			frmCurrency := libutils.AssetType(pg.orderInfo.FromCurrency)
																			return components.SetWalletLogo(pg.Load, gtx, frmCurrency, values.MarginPadding30)
																		}),
																		layout.Rigid(func(gtx C) D {
																			return layout.Inset{
																				Left: values.MarginPadding10,
																			}.Layout(gtx, func(gtx C) D {
																				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																					layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrSending)).Layout),
																					layout.Rigid(func(gtx C) D {
																						return components.LayoutOrderAmount(pg.Load, gtx, pg.orderInfo.FromCurrency, pg.orderInfo.InvoicedAmount)
																					}),
																					layout.Rigid(func(gtx C) D {
																						sourceWallet := pg.AssetsManager.WalletWithID(pg.orderInfo.SourceWalletID)
																						sourceWalletName := sourceWallet.GetWalletName()
																						sourceAccount, _ := sourceWallet.GetAccount(pg.orderInfo.SourceAccountNumber)
																						fromText := fmt.Sprintf(values.String(values.StrOrderSendingFrom), sourceWalletName, sourceAccount.Name)
																						return pg.Theme.Label(values.TextSize16, fromText).Layout(gtx)
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
																		return pg.Theme.Icons.ArrowDownIcon.LayoutSize(gtx, values.MarginPadding60)
																	})
																}),
																layout.Rigid(func(gtx C) D {
																	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																		layout.Rigid(func(gtx C) D {
																			toCurrency := libutils.AssetType(pg.orderInfo.ToCurrency)
																			return components.SetWalletLogo(pg.Load, gtx, toCurrency, values.MarginPadding30)
																		}),
																		layout.Rigid(func(gtx C) D {
																			return layout.Inset{
																				Left: values.MarginPadding10,
																			}.Layout(gtx, func(gtx C) D {
																				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																					layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrReceiving)).Layout),
																					layout.Rigid(func(gtx C) D {
																						return components.LayoutOrderAmount(pg.Load, gtx, pg.orderInfo.ToCurrency, pg.orderInfo.OrderedAmount)
																					}),
																					layout.Rigid(func(gtx C) D {
																						destinationWallet := pg.AssetsManager.WalletWithID(pg.orderInfo.DestinationWalletID)
																						destinationWalletName := destinationWallet.GetWalletName()
																						destinationAccount, _ := destinationWallet.GetAccount(pg.orderInfo.DestinationAccountNumber)
																						toText := fmt.Sprintf(values.String(values.StrOrderReceivingTo), destinationWalletName, destinationAccount.Name)
																						return pg.Theme.Label(values.TextSize16, toText).Layout(gtx)
																					}),
																					layout.Rigid(pg.Theme.Label(values.TextSize16, pg.orderInfo.DestinationAddress).Layout),
																				)
																			})
																		}),
																	)
																}),
															)
														})
													})
												})
											})
										}),
									)
								})
							}),
						)
					})
				}),
				layout.Rigid(pg.Theme.Label(values.TextSize28, pg.orderInfo.Status.String()).Layout),
				layout.Rigid(func(gtx C) D {
					if pg.orderInfo.Status == api.OrderStatusWaitingForDeposit && pg.orderInfo.ExchangeServer.Server == instantswap.FlypMe {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(pg.Theme.Label(values.TextSize18, values.String(values.StrExpiresIn)).Layout),
							layout.Rigid(pg.Theme.Label(values.TextSize18, fmt.Sprint(pg.orderInfo.ExpiryTime)).Layout),
						)
					}
					return D{}
				}),
				layout.Rigid(func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									if pg.isRefreshing {
										gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding24)
										gtx.Constraints.Min.X = gtx.Constraints.Max.X
										loader := material.Loader(pg.Theme.Base)
										loader.Color = pg.Theme.Color.Gray1
										return loader.Layout(gtx)
									}
									return layout.Inset{
										Left: values.MarginPadding10,
									}.Layout(gtx, pg.refreshBtn.Layout)
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Left: values.MarginPadding10,
									}.Layout(gtx, pg.createOrderBtn.Layout)
								}),
							)
						})
					})
				}),
			)
		})
	})
}

func (pg *OrderDetailsPage) getOrderInfo(UUID string) (*instantswap.Order, error) {
	orderInfo, err := pg.AssetsManager.InstantSwap.GetOrderInfo(pg.exchange, UUID)
	if err != nil {
		return nil, err
	}

	return orderInfo, nil
}
