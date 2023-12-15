package exchange

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"

	api "github.com/crypto-power/instantswap/instantswap"
)

const OrderHistoryPageID = "OrderHistory"

type OrderHistoryPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scroll         *components.Scroll[*instantswap.Order]
	previousStatus api.Status

	ordersList *cryptomaterial.ClickableList

	materialLoader material.LoaderStyle

	backButton   cryptomaterial.IconButton
	searchEditor cryptomaterial.Editor

	exchangeServers []instantswap.ExchangeServer

	refreshClickable *cryptomaterial.Clickable
	refreshIcon      *cryptomaterial.Image
	statusDropdown   *cryptomaterial.DropDown
	orderDropdown    *cryptomaterial.DropDown
	serverDropdown   *cryptomaterial.DropDown
	filterBtn        *cryptomaterial.Clickable
	isFilterOpen     bool
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

	pg.searchEditor = l.Theme.SearchEditor(new(widget.Editor), values.String(values.StrSearch), l.Theme.Icons.SearchIcon)
	pg.searchEditor.Editor.SingleLine = true
	pg.searchEditor.TextSize = pg.ConvertTextSize(l.Theme.TextSize)

	pg.materialLoader = material.Loader(l.Theme.Base)

	pg.ordersList = pg.Theme.NewClickableList(layout.Vertical)
	pg.ordersList.IsShadowEnabled = true

	pg.filterBtn = l.Theme.NewClickable(false)

	pg.statusDropdown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: api.OrderStatusWaitingForDeposit.String()},
		{Text: api.OrderStatusDepositReceived.String()},
		{Text: api.OrderStatusNew.String()},
		{Text: api.OrderStatusCompleted.String()},
		{Text: api.OrderStatusExpired.String()},
	}, values.OrderStatusDropdownGroup, 1, 0, true)
	// pg.statusDropdown.Width = values.MarginPadding221

	pg.orderDropdown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrNewest)},
		{Text: values.String(values.StrOldest)},
	}, values.ProposalDropdownGroup, 1, 0, true)

	if pg.statusDropdown.Reversed() {
		pg.statusDropdown.ExpandedLayoutInset.Right = values.MarginPadding10
	} else {
		pg.statusDropdown.ExpandedLayoutInset.Left = values.MarginPadding10
	}

	pg.statusDropdown.CollapsedLayoutTextDirection = layout.E
	pg.orderDropdown.CollapsedLayoutTextDirection = layout.E
	pg.orderDropdown.Width = values.MarginPadding100
	if l.IsMobileView() {
		pg.orderDropdown.Width = values.MarginPadding85
		pg.statusDropdown.Width = values.DP118
	}
	settingCommonDropdown(pg.Theme, pg.statusDropdown)
	settingCommonDropdown(pg.Theme, pg.orderDropdown)
	pg.statusDropdown.SetConvertTextSize(pg.ConvertTextSize)
	pg.orderDropdown.SetConvertTextSize(pg.ConvertTextSize)

	pg.initServerSelector()
	return pg
}

func (pg *OrderHistoryPage) ID() string {
	return OrderHistoryPageID
}

func (pg *OrderHistoryPage) OnNavigatedTo() {
	pg.listenForSyncNotifications() // listener stopped in OnNavigatedFrom().
	go pg.scroll.FetchScrollData(false, pg.ParentWindow(), false)
}

func (pg *OrderHistoryPage) OnNavigatedFrom() {
	pg.stopSyncNtfnListener()
}

func (pg *OrderHistoryPage) HandleUserInteractions() {
	if pg.statusDropdown.Changed() {
		pg.scroll.FetchScrollData(false, pg.ParentWindow(), false)
	}

	if clicked, selectedItem := pg.ordersList.ItemClicked(); clicked {
		orderItems := pg.scroll.FetchedData()
		pg.ParentNavigator().Display(NewOrderDetailsPage(pg.Load, orderItems[selectedItem]))
	}

	if pg.refreshClickable.Clicked() {
		go pg.AssetsManager.InstantSwap.Sync() // does nothing if already syncing
	}

	for _, evt := range pg.searchEditor.Editor.Events() {
		if pg.searchEditor.Editor.Focused() {
			switch evt.(type) {
			case widget.ChangeEvent:
				pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
			}
		}
	}

	for pg.filterBtn.Clicked() {
		pg.isFilterOpen = !pg.isFilterOpen
	}
}

// initWalletSelector initializes the wallet selector dropdown to enable
// filtering proposals
func (pg *OrderHistoryPage) initServerSelector() {
	pg.exchangeServers = pg.AssetsManager.InstantSwap.ExchangeServers()

	items := []cryptomaterial.DropDownItem{}
	for _, server := range pg.exchangeServers {
		item := cryptomaterial.DropDownItem{
			Text: server.Server.CapFirstLetter(),
			Icon: components.GetServerIcon(pg.Theme, server.Server.ToString()),
		}
		items = append(items, item)
	}

	pg.serverDropdown = pg.Theme.DropdownWithCustomPos(items, values.WalletsDropdownGroup, 2, 0, false)
	pg.serverDropdown.Width = values.MarginPadding150
	settingCommonDropdown(pg.Theme, pg.serverDropdown)
	pg.serverDropdown.SetConvertTextSize(pg.ConvertTextSize)
}

func (pg *OrderHistoryPage) Layout(gtx C) D {
	pg.scroll.OnScrollChangeListener(pg.ParentWindow())

	container := func(gtx C) D {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(pg.layout), // Assuming pg.layout is a valid function that matches layout.Widget
		)
	}

	// return cryptomaterial.LinearLayout{
	// 	Width:     cryptomaterial.MatchParent,
	// 	Height:    cryptomaterial.MatchParent,
	// 	Direction: layout.Center,
	// }.Layout2(gtx, func(gtx C) D {
	// 	return cryptomaterial.LinearLayout{
	// 		Width:     gtx.Dp(values.MarginPadding550),
	// 		Height:    cryptomaterial.MatchParent,
	// 		Alignment: layout.Middle,
	// 	}.Layout2(gtx, func(gtx C) D {
	// 		return cryptomaterial.UniformPadding(gtx, container)
	// 	})
	// })

	padding := values.MarginPadding24
	if pg.IsMobileView() {
		padding = values.MarginPadding12
	}
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		inset := layout.Inset{
			// Top:    values.MarginPadding16,
			Right:  padding,
			Left:   padding,
			Bottom: values.MarginPadding16,
		}
		return inset.Layout(gtx, func(gtx C) D {
			return cryptomaterial.UniformPadding(gtx, container)
		})
	})

}

func (pg *OrderHistoryPage) layout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.MatchParent,
		Direction: layout.Center,
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
												if pg.AssetsManager.InstantSwap.IsSyncing() {
													text = values.String(values.StrSyncingState)
												} else {
													text = values.String(values.StrUpdated) + " " + components.TimeAgo(pg.AssetsManager.InstantSwap.GetLastSyncedTimeStamp())

													if pg.AssetsManager.InstantSwap.GetLastSyncedTimeStamp() == 0 {
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
														if pg.AssetsManager.InstantSwap.IsSyncing() {
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
											// Top: values.MarginPadding16,
										}.Layout(gtx, func(gtx C) D {
											return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													topInset := values.MarginPadding50
													if !pg.isFilterOpen && pg.IsMobileView() {
														return layout.Spacer{Height: topInset}.Layout(gtx)
													}
													if pg.IsMobileView() && pg.isFilterOpen {
														topInset = values.MarginPadding80
													}
													return layout.Inset{
														Top: topInset,
													}.Layout(gtx, pg.searchEditor.Layout)
												}),
												layout.Rigid(pg.layoutHistory),
											)
										})
									}),
									layout.Stacked(pg.dropdownLayout),
								)
							})
						}),
					)
				})
			}),
		)
	})
}

func (pg *OrderHistoryPage) dropdownLayout(gtx C) D {
	if pg.IsMobileView() {
		return layout.Stack{}.Layout(gtx,
			layout.Stacked(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Inset{Top: values.MarginPadding40}.Layout(gtx, pg.rightDropdown)
			}),
			layout.Expanded(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return pg.leftDropdown(gtx)
			}),
		)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(pg.leftDropdown),
		layout.Rigid(pg.rightDropdown),
	)
}

func (pg *OrderHistoryPage) leftDropdown(gtx C) D {
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if pg.serverDropdown == nil {
				return D{}
			}
			return layout.W.Layout(gtx, pg.serverDropdown.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			if !pg.IsMobileView() {
				return D{}
			}
			icon := pg.Theme.Icons.FilterOffImgIcon
			if pg.isFilterOpen {
				icon = pg.Theme.Icons.FilterImgIcon
			}
			return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return pg.filterBtn.Layout(gtx, icon.Layout16dp)
			})
		}),
	)
}

func (pg *OrderHistoryPage) rightDropdown(gtx C) D {
	if !pg.isFilterOpen && pg.IsMobileView() {
		return D{}
	}
	return layout.E.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(pg.statusDropdown.Layout),
			layout.Rigid(pg.orderDropdown.Layout),
		)
	})
}

func settingCommonDropdown(t *cryptomaterial.Theme, drodown *cryptomaterial.DropDown) {
	drodown.FontWeight = font.SemiBold
	drodown.Hoverable = false
	drodown.SelectedItemIconColor = &t.Color.Primary
	drodown.ExpandedLayoutInset = layout.Inset{Top: values.MarginPadding35}
	drodown.MakeCollapsedLayoutVisibleWhenExpanded = true
	drodown.Background = &t.Color.Surface
}

func (pg *OrderHistoryPage) fetchOrders(offset, pageSize int32) ([]*instantswap.Order, int, bool, error) {
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

	// searchKey := pg.searchEditor.Editor.Text()
	orders := components.LoadOrders(pg.Load, offset, pageSize, true, statusFilter)
	return orders, len(orders), isReset, nil
}

func (pg *OrderHistoryPage) layoutHistory(gtx C) D {
	if pg.scroll.ItemsCount() <= 0 {
		return components.LayoutNoOrderHistoryWithMsg(gtx, pg.Load, false, values.String(values.StrNoOrders))
	}

	orderItems := pg.scroll.FetchedData()
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
	orderNotificationListener := &instantswap.OrderNotificationListener{
		OnExchangeOrdersSynced: func() {
			pg.scroll.FetchScrollData(false, pg.ParentWindow(), false)
			pg.ParentWindow().Reload()
		},
	}
	err := pg.AssetsManager.InstantSwap.AddNotificationListener(orderNotificationListener, OrderHistoryPageID)
	if err != nil {
		log.Errorf("Error adding instanswap notification listener: %v", err)
		return
	}
}

func (pg *OrderHistoryPage) stopSyncNtfnListener() {
	pg.AssetsManager.InstantSwap.RemoveNotificationListener(OrderHistoryPageID)
}
