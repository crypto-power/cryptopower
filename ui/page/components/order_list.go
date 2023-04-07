package components

import (
	"fmt"
	"time"

	"gioui.org/layout"

	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	libutils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	api "code.cryptopower.dev/group/instantswap"
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
									return SetWalletLogo(l, gtx, libutils.AssetType(orderItem.FromCurrency), values.MarginPadding30)
								})
							}),
							layout.Rigid(func(gtx C) D {
								return LayoutOrderAmount(l, gtx, orderItem.FromCurrency, orderItem.InvoicedAmount)
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
												return SetWalletLogo(l, gtx, libutils.AssetType(orderItem.ToCurrency), values.MarginPadding30)
											})
										}),
										layout.Rigid(func(gtx C) D {
											return LayoutOrderAmount(l, gtx, orderItem.ToCurrency, orderItem.OrderedAmount)
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
							}.Layout(gtx, func(gtx C) D {
								return D{}
							})
						}),
						layout.Rigid(l.Theme.Label(values.TextSize16, orderItem.Status.String()).Layout),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, func(gtx C) D {
								return layout.Flex{
									Axis:      layout.Horizontal,
									Alignment: layout.Middle,
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										date := time.Unix(orderItem.CreatedAt, 0).Format("Jan 2, 2006")
										timeSplit := time.Unix(orderItem.CreatedAt, 0).Format("03:04:05 PM")
										createdAt := fmt.Sprintf("%v at %v", date, timeSplit)
										timestamp := l.Theme.Label(values.TextSize16, createdAt)
										timestamp.Color = l.Theme.Color.GrayText2
										return timestamp.Layout(gtx)
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
	text := l.Theme.Body1(values.String(values.StrNoOrders))
	text.Color = l.Theme.Color.GrayText3
	if syncing {
		text = l.Theme.Body1(values.String(values.StrFetchingOrders))
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{
			Top:    values.MarginPadding10,
			Bottom: values.MarginPadding10,
		}.Layout(gtx, text.Layout)
	})
}

func LoadOrders(l *load.Load, offset, limit int32, newestFirst bool, status ...api.Status) []*instantswap.Order {
	var orders []*instantswap.Order

	orders, err := l.WL.AssetsManager.InstantSwap.GetOrdersRaw(offset, limit, true, status...)
	if err != nil {
		log.Error(err)
	}

	return orders
}
