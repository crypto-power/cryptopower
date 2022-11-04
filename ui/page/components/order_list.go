package components

import (
	"fmt"
	"strconv"

	"gioui.org/layout"

	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

func OrderItemWidget(gtx C, l *load.Load, orderItem *instantswap.Order) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     cryptomaterial.WrapContent,
		Background: l.Theme.Color.Surface,
		Alignment:  layout.Middle,
		Border:     cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Bottom: values.MarginPadding10,
					}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Right: values.MarginPadding10,
									Left:  values.MarginPadding10,
								}.Layout(gtx, func(gtx C) D {
									if orderItem.FromCurrency == "DCR" {
										return l.Theme.Icons.DecredSymbol2.LayoutSize(gtx, values.MarginPadding24)
									}
									return l.Theme.Icons.BTC.LayoutSize(gtx, values.MarginPadding24)
								})
							}),
							layout.Rigid(func(gtx C) D {
								fmt.Println("[][][][] amount", orderItem.OrderedAmount)
								orderedAmount := strconv.FormatFloat(orderItem.OrderedAmount, 'f', -1, 64)
								return l.Theme.Label(values.TextSize16, orderedAmount).Layout(gtx)
							}),
							layout.Flexed(1, func(gtx C) D {
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
												if orderItem.ToCurrency == "DCR" {
													return l.Theme.Icons.DecredSymbol2.LayoutSize(gtx, values.MarginPadding24)
												}
												return l.Theme.Icons.BTC.LayoutSize(gtx, values.MarginPadding24)
											})
										}),
										layout.Rigid(func(gtx C) D {
											receiveAmount := strconv.FormatFloat(orderItem.ReceiveAmount, 'f', -1, 64)
											return l.Theme.Label(values.TextSize16, receiveAmount).Layout(gtx)
										}),
									)
								})
							}),
						)
					})

				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Right: values.MarginPadding10,
								Left:  values.MarginPadding10,
							}.Layout(gtx, func(gtx C) D {
								return D{}
							})
						}),
						layout.Rigid(l.Theme.Label(values.TextSize16, orderItem.Status).Layout),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, func(gtx C) D {
								return layout.Flex{
									Axis:      layout.Horizontal,
									Alignment: layout.Middle,
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										timestamp := strconv.FormatFloat(orderItem.ReceiveAmount, 'f', -1, 64)
										return l.Theme.Label(values.TextSize16, timestamp).Layout(gtx)
									}),
								)
							})
						}),
					)
				}),
			)
		}),
	)
}

func LayoutNoOrderHistory(gtx C, l *load.Load, syncing bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	text := l.Theme.Body1("Orders you create will be shown here.")
	text.Color = l.Theme.Color.GrayText3
	if syncing {
		text = l.Theme.Body1("Fetching Orders")
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{
			Top:    values.MarginPadding10,
			Bottom: values.MarginPadding10,
		}.Layout(gtx, text.Layout)
	})
}

func LoadOrders(l *load.Load, newestFirst bool) []*instantswap.Order {
	orderItems := make([]*instantswap.Order, 0)

	orders, err := l.WL.MultiWallet.InstantSwap.GetOrdersRaw(0, 0, true)
	if err == nil {
		for i := 0; i < len(orders); i++ {
			orderItems = append(orderItems, &orders[i])
		}
	}

	return orderItems
}
