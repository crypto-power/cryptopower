package exchange

import (
	"context"
	"fmt"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"

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

	exchange   api.IDExchange
	order      *instantswap.Order
	UUID       string
	orderInfo  *api.OrderInfoResult
	status     string
	backButton cryptomaterial.IconButton
	infoButton cryptomaterial.IconButton

	refreshBtn     cryptomaterial.Button
	createOrderBtn cryptomaterial.Button
}

func NewOrderDetailsPage(l *load.Load, exchange api.IDExchange, order *instantswap.Order) *OrderDetailsPage {
	pg := &OrderDetailsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OrderDetailsPageID),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		exchange: exchange,
		order:    order,
	}

	pg.backButton, _ = components.SubpageHeaderButtons(l)

	_, pg.infoButton = components.SubpageHeaderButtons(pg.Load)

	pg.createOrderBtn = pg.Theme.Button("Create Order")
	pg.refreshBtn = pg.Theme.Button("Refresh")

	fmt.Println("[][][][] UUID", pg.order.UUID)
	pg.orderInfo, _ = pg.getOrderInfo(pg.order.UUID)
	fmt.Println("[][][][] status", pg.orderInfo.Status)

	pg.status = pg.orderInfo.Status

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
		pg.orderInfo, _ = pg.getOrderInfo(pg.UUID)
		fmt.Println("[][][][] status new", pg.orderInfo.Status)
		pg.status = pg.orderInfo.Status
	}
}

func (pg *OrderDetailsPage) Layout(gtx C) D {
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

func (pg *OrderDetailsPage) layout(gtx C) D {
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
								txt := pg.Theme.Label(values.TextSize16, pg.status)
								txt.Font.Weight = text.SemiBold
								return txt.Layout(gtx)
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
				)

			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top:   values.MarginPadding24,
				Right: values.MarginPadding16,
			}.Layout(gtx, pg.createOrderBtn.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top:   values.MarginPadding24,
				Right: values.MarginPadding16,
			}.Layout(gtx, pg.refreshBtn.Layout)
		}),
	)
}

func (pg *OrderDetailsPage) getOrderInfo(UUID string) (*api.OrderInfoResult, error) {
	orderInfo, err := pg.exchange.OrderInfo(UUID)
	if err != nil {
		return nil, err
	}

	fmt.Println(orderInfo)

	fmt.Println(orderInfo.InternalStatus.String())

	return &orderInfo, nil
}
