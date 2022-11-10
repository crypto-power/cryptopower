package exchange

import (
	"context"
	"fmt"

	"gioui.org/layout"
	// "gioui.org/text"
	"gioui.org/widget"
	// "strconv"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"github.com/btcsuite/btcutil"
	"github.com/decred/dcrd/dcrutil/v4"

	api "code.cryptopower.dev/exchange/instantswap"
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

	scrollContainer *widget.List
	container       *widget.List

	exchange   api.IDExchange
	UUID       string
	orderInfo  *instantswap.Order
	backButton cryptomaterial.IconButton
	infoButton cryptomaterial.IconButton

	refreshBtn     cryptomaterial.Button
	createOrderBtn cryptomaterial.Button
}

func NewOrderDetailsPage(l *load.Load, order *instantswap.Order) *OrderDetailsPage {
	pg := &OrderDetailsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OrderDetailsPageID),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		container: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		orderInfo: order,
	}

	exchange, err := pg.WL.MultiWallet.InstantSwap.NewExchanageServer(order.Server, "", "")
	if err != nil {
		fmt.Println(err)
	}
	pg.exchange = exchange

	pg.backButton, _ = components.SubpageHeaderButtons(l)

	_, pg.infoButton = components.SubpageHeaderButtons(pg.Load)

	pg.createOrderBtn = pg.Theme.Button("Create New Order")
	pg.refreshBtn = pg.Theme.Button("Refresh")

	pg.orderInfo, err = pg.getOrderInfo(pg.orderInfo.UUID)

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
		pg.orderInfo, _ = pg.getOrderInfo(pg.orderInfo.UUID)
		// fmt.Println("[][][][] status new", pg.orderInfo.Status)
		// pg.status = pg.orderInfo.Status.String()
	}
}

func (pg *OrderDetailsPage) Layout(gtx C) D {
	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      "Order Details",
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
							layout.Flexed(1, func(gtx C) D {
								return layout.E.Layout(gtx, func(gtx C) D {
									return layout.Flex{
										Axis:      layout.Horizontal,
										Alignment: layout.Middle,
									}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											return layout.Inset{
												// Right: values.MarginPadding10,
											}.Layout(gtx, func(gtx C) D {
												// return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												// 	layout.Rigid(func(gtx C) D {
												// return pg.Theme.List(pg.container).Layout(gtx, 1, func(gtx C, i int) D {
												return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
													return pg.Theme.Card().Layout(gtx, func(gtx C) D {
														return layout.UniformInset(values.MarginPadding20).Layout(gtx, func(gtx C) D {
															return layout.Center.Layout(gtx, func(gtx C) D {
																return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																	layout.Rigid(func(gtx C) D {
																		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
																			layout.Rigid(func(gtx C) D {
																				if pg.orderInfo.FromCurrency == utils.DCRWalletAsset.String() {
																					return pg.Theme.Icons.DecredSymbol2.LayoutSize(gtx, values.MarginPadding40)
																				}
																				return pg.Theme.Icons.BTC.LayoutSize(gtx, values.MarginPadding40)
																			}),
																			layout.Rigid(func(gtx C) D {
																				return layout.Inset{
																					Left: values.MarginPadding10,
																					// Right: values.MarginPadding50,
																				}.Layout(gtx, func(gtx C) D {
																					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																						layout.Rigid(func(gtx C) D {
																							return pg.Theme.Label(values.TextSize16, "Sending").Layout(gtx)
																						}),
																						layout.Rigid(func(gtx C) D {
																							if pg.orderInfo.FromCurrency == utils.DCRWalletAsset.String() {
																								invoicedAmount, _ := dcrutil.NewAmount(pg.orderInfo.InvoicedAmount)
																								return pg.Theme.Label(values.TextSize16, invoicedAmount.String()).Layout(gtx)

																							}
																							invoicedAmount, _ := btcutil.NewAmount(pg.orderInfo.InvoicedAmount)
																							return pg.Theme.Label(values.TextSize16, invoicedAmount.String()).Layout(gtx)
																						}),
																						layout.Rigid(func(gtx C) D {
																							sourceWallet := pg.WL.MultiWallet.WalletWithID(pg.orderInfo.SourceWalletID)
																							sourceWalletName := sourceWallet.GetWalletName()
																							sourceAccount, _ := sourceWallet.GetAccount(pg.orderInfo.SourceAccountNumber)
																							fromText := fmt.Sprintf("From: %s (%s)", sourceWalletName, sourceAccount.Name)
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
																				if pg.orderInfo.ToCurrency == utils.DCRWalletAsset.String() {
																					return pg.Theme.Icons.DecredSymbol2.LayoutSize(gtx, values.MarginPadding40)
																				}
																				return pg.Theme.Icons.BTC.LayoutSize(gtx, values.MarginPadding40)
																			}),
																			layout.Rigid(func(gtx C) D {
																				return layout.Inset{
																					Left: values.MarginPadding10,
																					// Right: values.MarginPadding50,
																				}.Layout(gtx, func(gtx C) D {
																					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
																						layout.Rigid(func(gtx C) D {
																							return pg.Theme.Label(values.TextSize16, "Receiving").Layout(gtx)
																						}),
																						layout.Rigid(func(gtx C) D {
																							if pg.orderInfo.ToCurrency == utils.DCRWalletAsset.String() {
																								orderedAmount, _ := dcrutil.NewAmount(pg.orderInfo.OrderedAmount)
																								return pg.Theme.Label(values.TextSize16, orderedAmount.String()).Layout(gtx)
																							}
																							orderedAmount, _ := btcutil.NewAmount(pg.orderInfo.OrderedAmount)
																							return pg.Theme.Label(values.TextSize16, orderedAmount.String()).Layout(gtx)
																						}),
																						layout.Rigid(func(gtx C) D {
																							destinationWallet := pg.WL.MultiWallet.WalletWithID(pg.orderInfo.DestinationWalletID)
																							destinationWalletName := destinationWallet.GetWalletName()
																							destinationAccount, _ := destinationWallet.GetAccount(pg.orderInfo.DestinationAccountNumber)
																							toText := fmt.Sprintf("To: %s (%s)", destinationWalletName, destinationAccount.Name)
																							return pg.Theme.Label(values.TextSize16, toText).Layout(gtx)
																						}),
																						layout.Rigid(func(gtx C) D {
																							return pg.Theme.Label(values.TextSize16, pg.orderInfo.DestinationAddress).Layout(gtx)
																						}),
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
												// })
												// 	}),
												// )
											})
										}),
									)
								})
							}),
							layout.Flexed(0.35, func(gtx C) D {
								return layout.E.Layout(gtx, func(gtx C) D {
									return D{}
								})
							}),
						)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return pg.Theme.Label(values.TextSize28, pg.orderInfo.Status.String()).Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return pg.refreshBtn.Layout(gtx)
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Left: values.MarginPadding10,
									}.Layout(gtx, func(gtx C) D {
										return pg.createOrderBtn.Layout(gtx)
									})
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
	orderInfo, err := pg.WL.MultiWallet.InstantSwap.GetOrderInfo(pg.exchange, UUID)
	if err != nil {
		return nil, err
	}

	return orderInfo, nil
}
