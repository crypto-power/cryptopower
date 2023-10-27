package components

import (
	"fmt"
	"time"

	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
	api "github.com/crypto-power/instantswap/instantswap"
)

func OrderItemWidget(gtx C, l *load.Load, orderItem *instantswap.Order) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     cryptomaterial.WrapContent,
		Background: l.Theme.Color.Surface,
		Alignment:  layout.Middle,
		Border:     cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
		Margin: layout.Inset{
			Top:    values.MarginPadding8,
			Bottom: values.MarginPadding10,
		},
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
								Right: values.MarginPadding50,
							}.Layout(gtx, func(gtx C) D {
								return D{}
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(l.Theme.Label(values.TextSize16, orderItem.Status.String()).Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									statusLayout := statusIcon(l, orderItem.Status)
									if statusLayout == nil {
										return layout.Dimensions{}
									}
									return layout.Inset{Left: values.MarginPadding6}.Layout(gtx, statusLayout)
								}),
							)
						}),
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

func statusIcon(l *load.Load, status api.Status) func(gtx C) layout.Dimensions {
	switch status {
	case api.OrderStatusCompleted:
		return l.Theme.Icons.ConfirmIcon.Layout16dp
	case api.OrderStatusCanceled, api.OrderStatusExpired, api.OrderStatusFailed:
		return l.Theme.Icons.FailedIcon.Layout16dp
	}
	return nil
}

func LayoutNoOrderHistory(gtx C, l *load.Load, syncing bool) D {
	return LayoutNoOrderHistoryWithMsg(gtx, l, syncing, values.String(values.StrNoOrders))
}

func LayoutNoOrderHistoryWithMsg(gtx C, l *load.Load, syncing bool, msg string) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	text := l.Theme.Body1(msg)
	text.Color = l.Theme.Color.GrayText3
	if syncing {
		text = l.Theme.Body1(values.String(values.StrFetchingOrders))
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{
			Top:    values.MarginPadding30,
			Bottom: values.MarginPadding30,
		}.Layout(gtx, text.Layout)
	})
}

func LoadOrders(l *load.Load, offset, limit int32, newestFirst bool, status ...api.Status) []*instantswap.Order {
	var orders []*instantswap.Order

	orders, err := l.AssetsManager.InstantSwap.GetOrdersRaw(offset, limit, newestFirst, status...)
	if err != nil {
		log.Error(err)
	}

	return orders
}
