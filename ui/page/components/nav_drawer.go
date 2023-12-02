package components

import (
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/values"
)

type NavBarItem struct {
	Clickable *cryptomaterial.Clickable
	Image     *cryptomaterial.Image
	Title     string
	PageID    string
}

func LayoutNavigationBar(gtx layout.Context, theme *cryptomaterial.Theme, navItems []NavBarItem) layout.Dimensions {
	card := theme.Card()
	card.Radius = cryptomaterial.Radius(20)
	card.Color = theme.Color.Gray2
	padding8 := values.MarginPadding8
	padding20 := values.MarginPadding20
	return layout.Inset{Right: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
		return card.Layout(gtx, func(gtx C) D {
			list := layout.List{Axis: layout.Horizontal}
			return list.Layout(gtx, len(navItems), func(gtx C, i int) D {
				// The parent card has all border radius (tl, tr, bl, br) set to
				// 20. This requires setting it's children border so they
				// respects the parent border.
				radius := cryptomaterial.CornerRadius{TopLeft: 20, BottomLeft: 20}
				if i+1 == len(navItems) {
					radius = cryptomaterial.CornerRadius{TopRight: 20, BottomRight: 20}
				}
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.WrapContent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Horizontal,
					Clickable:   navItems[i].Clickable,
					Alignment:   layout.Middle,
					Border: cryptomaterial.Border{
						Radius: radius,
					},
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Top:    padding8,
							Bottom: padding8,
							Left:   padding20,
							Right:  padding8,
						}.Layout(gtx, func(gtx C) D {
							return layout.Center.Layout(gtx, navItems[i].Image.Layout24dp)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Top:    padding8,
							Bottom: padding8,
							Right:  padding20,
							Left:   values.MarginPadding0,
						}.Layout(gtx, func(gtx C) D {
							return layout.Center.Layout(gtx, theme.Body1(navItems[i].Title).Layout)
						})
					}),
					layout.Rigid(func(gtx C) D {
						if i+1 == len(navItems) {
							return D{}
						}
						verticalSeparator := theme.SeparatorVertical(int(gtx.Metric.PxPerDp*20.0), 2)
						verticalSeparator.Color = theme.Color.DeepBlue
						return verticalSeparator.Layout(gtx)
					}),
				)
			})
		})
	})
}
