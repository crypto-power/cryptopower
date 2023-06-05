package exchange

import (
	"context"

	"gioui.org/layout"
	"gioui.org/widget/material"

	"gitlab.com/cryptopower/cryptopower/app"
	"gitlab.com/cryptopower/cryptopower/libwallet/instantswap"
	"gitlab.com/cryptopower/cryptopower/listeners"
	"gitlab.com/cryptopower/cryptopower/ui/cryptomaterial"
	"gitlab.com/cryptopower/cryptopower/ui/load"
	"gitlab.com/cryptopower/cryptopower/ui/page/components"
	"gitlab.com/cryptopower/cryptopower/ui/values"
	"gitlab.com/cryptopower/cryptopower/wallet"

	api "gitlab.com/cryptopower/instantswap/instantswap"
)

const OrderHistoryPageID = "OrderHistory"

type OrderHistoryPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	*listeners.OrderNotificationListener

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	scroll         *components.Scroll
	previousStatus api.Status

	ordersList *cryptomaterial.ClickableList

	materialLoader material.LoaderStyle

	backButton cryptomaterial.IconButton

	refreshClickable *cryptomaterial.Clickable
	refreshIcon      *cryptomaterial.Image
	statusDropdown   *cryptomaterial.DropDown
}

func NewOrderHistoryPage(l *load.Load) *OrderHistoryPage {
	pg := &OrderHistoryPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OrderHistoryPageID),
		refreshClickable: l.Theme.NewClickable(true),
		refreshIcon:      l.Theme.Icons.Restore,
	}

	pg.backButton, _ = components.SubpageHeaderButtons(l)
	// pageSize defines the number of orders that can be fetched at ago.
	pageSize := int32(10)
	pg.scroll = components.NewScroll(l, pageSize, pg.fetchOrders)

	pg.materialLoader = material.Loader(l.Theme.Base)

	pg.ordersList = pg.Theme.NewClickableList(layout.Vertical)
	pg.ordersList.IsShadowEnabled = true

	pg.statusDropdown = l.Theme.DropDown([]cryptomaterial.DropDownItem{
		{Text: api.OrderStatusWaitingForDeposit.String()},
		{Text: api.OrderStatusDepositReceived.String()},
		{Text: api.OrderStatusNew.String()},
		{Text: api.OrderStatusCompleted.String()},
		{Text: api.OrderStatusExpired.String()},
	}, values.OrderStatusDropdownGroup, 0)

	return pg
}

func (pg *OrderHistoryPage) ID() string {
	return OrderHistoryPageID
}

func (pg *OrderHistoryPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.listenForSyncNotifications()
	go pg.scroll.FetchScrollData(false, pg.ParentWindow())
}

func (pg *OrderHistoryPage) OnNavigatedFrom() {
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}

func (pg *OrderHistoryPage) HandleUserInteractions() {
	if pg.statusDropdown.Changed() {
		pg.scroll.FetchScrollData(false, pg.ParentWindow())
	}

	if clicked, selectedItem := pg.ordersList.ItemClicked(); clicked {
		orderItems := pg.scroll.FetchedData().([]*instantswap.Order)
		pg.ParentNavigator().Display(NewOrderDetailsPage(pg.Load, orderItems[selectedItem]))
	}

	if pg.refreshClickable.Clicked() {
		go pg.WL.AssetsManager.InstantSwap.Sync(pg.ctx)
	}
}

func (pg *OrderHistoryPage) Layout(gtx C) D {
	pg.scroll.OnScrollChangeListener(pg.ParentWindow())

	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrOrderHistory),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				return layout.Stack{}.Layout(gtx, layout.Expanded(pg.layout))
			},
		}

		return cryptomaterial.LinearLayout{
			Width:     cryptomaterial.MatchParent,
			Height:    cryptomaterial.MatchParent,
			Direction: layout.Center,
		}.Layout2(gtx, func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:     gtx.Dp(values.MarginPadding550),
				Height:    cryptomaterial.MatchParent,
				Alignment: layout.Middle,
			}.Layout2(gtx, func(gtx C) D {
				return sp.Layout(pg.ParentWindow(), gtx)
			})
		})
	}

	return components.UniformPadding(gtx, container)
}

func (pg *OrderHistoryPage) layout(gtx C) D {
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
					return layout.Inset{}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(1, func(gtx C) D {
										body := func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													var text string
													if pg.WL.AssetsManager.InstantSwap.IsSyncing() {
														text = values.String(values.StrSyncingState)
													} else {
														text = values.String(values.StrUpdated) + " " + components.TimeAgo(pg.WL.AssetsManager.InstantSwap.GetLastSyncedTimeStamp())

														if pg.WL.AssetsManager.InstantSwap.GetLastSyncedTimeStamp() == 0 {
															text = values.String(values.StrNeverSynced)
														}
													}

													lastUpdatedInfo := pg.Theme.Label(values.TextSize12, text)
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
															if pg.WL.AssetsManager.InstantSwap.IsSyncing() {
																gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding8)
																gtx.Constraints.Min.X = gtx.Constraints.Max.X
																return layout.Inset{Bottom: values.MarginPadding1}.Layout(gtx, pg.materialLoader.Layout)
															}
															return layout.Inset{Right: values.MarginPadding16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
																return pg.refreshIcon.LayoutSize(gtx, values.MarginPadding18)
															})
														}),
													)
												}),
											)
										}
										return layout.E.Layout(gtx, body)
									}),
								)
							}),
							layout.Flexed(1, func(gtx C) D {
								return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
									return layout.Stack{}.Layout(gtx,
										layout.Expanded(func(gtx C) D {
											return layout.Inset{
												Top: values.MarginPadding60,
											}.Layout(gtx, pg.layoutHistory)
										}),
										layout.Expanded(func(gtx C) D {
											return pg.statusDropdown.Layout(gtx, 10, true)
										}),
									)
								})
							}),
						)
					})
				}),
			)
		})
	})
}

func (pg *OrderHistoryPage) fetchOrders(offset, pageSize int32) (interface{}, int, bool, error) {
	selectedStatus := pg.statusDropdown.Selected()
	var statusFilter api.Status
	switch selectedStatus {
	case api.OrderStatusWaitingForDeposit.String():
		statusFilter = api.OrderStatusWaitingForDeposit
	case api.OrderStatusDepositReceived.String():
		statusFilter = api.OrderStatusDepositReceived
	case api.OrderStatusNew.String():
		statusFilter = api.OrderStatusNew
	case api.OrderStatusRefunded.String():
		statusFilter = api.OrderStatusRefunded
	case api.OrderStatusExpired.String():
		statusFilter = api.OrderStatusExpired
	case api.OrderStatusCompleted.String():
		statusFilter = api.OrderStatusCompleted
	default:
		statusFilter = api.OrderStatusUnknown
	}

	isReset := pg.previousStatus != statusFilter
	if isReset {
		// Since the status has changed we need to reset the offset.
		offset = 0
		pg.previousStatus = statusFilter
	}

	orders := components.LoadOrders(pg.Load, offset, pageSize, true, statusFilter)
	return orders, len(orders), isReset, nil
}

func (pg *OrderHistoryPage) layoutHistory(gtx C) D {
	if pg.scroll.ItemsCount() <= 0 {
		return components.LayoutNoOrderHistory(gtx, pg.Load, false)
	}

	orderItems := pg.scroll.FetchedData().([]*instantswap.Order)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return pg.scroll.List().Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return pg.ordersList.Layout(gtx, len(orderItems), func(gtx C, i int) D {
						return cryptomaterial.LinearLayout{
							Orientation: layout.Vertical,
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Background:  pg.Theme.Color.Surface,
							Direction:   layout.W,
							Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
							Padding:     layout.UniformInset(values.MarginPadding15),
							Margin:      layout.Inset{Bottom: values.MarginPadding4, Top: values.MarginPadding4},
						}.Layout2(gtx, func(gtx C) D {
							return components.OrderItemWidget(gtx, pg.Load, orderItems[i])
						})
					})
				})
			})
		}),
	)
}

func (pg *OrderHistoryPage) listenForSyncNotifications() {
	if pg.OrderNotificationListener != nil {
		return
	}
	pg.OrderNotificationListener = listeners.NewOrderNotificationListener()
	err := pg.WL.AssetsManager.InstantSwap.AddNotificationListener(pg.OrderNotificationListener, OrderHistoryPageID)
	if err != nil {
		log.Errorf("Error adding instanswap notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-pg.OrderNotifChan:
				if n.OrderStatus == wallet.OrderStatusSynced {
					pg.scroll.FetchScrollData(false, pg.ParentWindow())
					pg.ParentWindow().Reload()
				}
			case <-pg.ctx.Done():
				pg.WL.AssetsManager.InstantSwap.RemoveNotificationListener(OrderHistoryPageID)
				close(pg.OrderNotifChan)
				pg.OrderNotificationListener = nil

				return
			}
		}
	}()
}
