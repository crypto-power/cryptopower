package components

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

var (
	navDrawerMaximizedWidth = values.Size180
	navDrawerMinimizedWidth = values.MarginPadding100
)

type NavHandler struct {
	Clickable     *cryptomaterial.Clickable
	Image         *cryptomaterial.Image
	ImageInactive *cryptomaterial.Image
	Title         string
	PageID        string
}

type NavDrawer struct {
	*load.Load

	AppNavBarItems    []NavHandler
	DCRDrawerNavItems []NavHandler
	BTCDrawerNavItems []NavHandler
	CurrentPage       string

	axis      layout.Axis
	textSize  unit.Sp
	leftInset unit.Dp
	width     unit.Dp
	alignment layout.Alignment
	direction layout.Direction

	MinimizeNavDrawerButton cryptomaterial.IconButton
	MaximizeNavDrawerButton cryptomaterial.IconButton
	activeDrawerBtn         cryptomaterial.IconButton
	IsNavExpanded           bool
}

func (nd *NavDrawer) LayoutNavDrawer(gtx layout.Context, navItems []NavHandler) layout.Dimensions {
	return cryptomaterial.LinearLayout{
		Width:       gtx.Dp(nd.width),
		Height:      cryptomaterial.MatchParent,
		Orientation: layout.Vertical,
		Background:  nd.Theme.Color.Surface,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			list := layout.List{Axis: layout.Vertical}
			return list.Layout(gtx, len(navItems), func(gtx C, i int) D {
				mGtx := gtx
				background := nd.Theme.Color.Surface

				if nd.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() && (navItems[i].PageID == values.String(values.StrSend) ||
					navItems[i].PageID == values.String(values.StrAccountMixer)) {
					return D{}
				}

				if navItems[i].PageID == nd.CurrentPage {
					background = nd.Theme.Color.Gray5
				}
				return cryptomaterial.LinearLayout{
					Orientation: nd.axis,
					Width:       cryptomaterial.MatchParent,
					Height:      cryptomaterial.WrapContent,
					Padding:     layout.UniformInset(values.MarginPadding15),
					Alignment:   nd.alignment,
					Direction:   nd.direction,
					Background:  background,
					Clickable:   navItems[i].Clickable,
				}.Layout(mGtx,
					layout.Rigid(func(gtx C) D {
						img := navItems[i].ImageInactive
						if navItems[i].PageID == nd.CurrentPage {
							img = navItems[i].Image
						}
						return img.Layout24dp(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if !nd.IsNavExpanded {
							return layout.Inset{
								Left: nd.leftInset,
							}.Layout(gtx, func(gtx C) D {
								textColor := nd.Theme.Color.GrayText1
								if navItems[i].PageID == nd.CurrentPage {
									textColor = nd.Theme.Color.DeepBlue
								}
								txt := nd.Theme.Label(nd.textSize, navItems[i].Title)
								txt.Color = textColor
								return txt.Layout(gtx)
							})
						}

						return D{}
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.SE.Layout(gtx, func(gtx C) D {
				return nd.activeDrawerBtn.Layout(gtx)
			})
		}),
	)
}

func (nd *NavDrawer) LayoutTopBar(gtx layout.Context) layout.Dimensions {
	card := nd.Theme.Card()
	card.Radius = cryptomaterial.Radius(20)
	card.Color = nd.Theme.Color.Gray2
	padding8 := values.MarginPadding8
	padding20 := values.MarginPadding20
	return layout.Inset{Right: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
		return card.Layout(gtx, func(gtx C) D {
			list := layout.List{Axis: layout.Horizontal}
			return list.Layout(gtx, len(nd.AppNavBarItems), func(gtx C, i int) D {
				// The parent card has all border radius (tl, tr, bl, br) set to
				// 20. This requires setting it's children border so they
				// respects the parent border.
				radius := cryptomaterial.CornerRadius{TopLeft: 20, BottomLeft: 20}
				if i+1 == len(nd.AppNavBarItems) {
					radius = cryptomaterial.CornerRadius{TopRight: 20, BottomRight: 20}
				}
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.WrapContent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Horizontal,
					Clickable:   nd.AppNavBarItems[i].Clickable,
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
							return layout.Center.Layout(gtx, nd.AppNavBarItems[i].Image.Layout24dp)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Top:    padding8,
							Bottom: padding8,
							Right:  padding20,
							Left:   values.MarginPadding0,
						}.Layout(gtx, func(gtx C) D {
							return layout.Center.Layout(gtx, nd.Theme.Body1(nd.AppNavBarItems[i].Title).Layout)
						})
					}),
					layout.Rigid(func(gtx C) D {
						if i+1 == len(nd.AppNavBarItems) {
							return D{}
						}
						verticalSeparator := nd.Theme.SeparatorVertical(int(gtx.Metric.PxPerDp*20.0), 2)
						verticalSeparator.Color = nd.Theme.Color.DeepBlue
						return verticalSeparator.Layout(gtx)
					}),
				)
			})
		})
	})
}

func (nd *NavDrawer) DrawerToggled(min bool) {
	if min {
		nd.axis = layout.Vertical
		nd.leftInset = values.MarginPadding0
		nd.width = navDrawerMinimizedWidth
		nd.activeDrawerBtn = nd.MaximizeNavDrawerButton
		nd.alignment = layout.Middle
		nd.direction = layout.Center
	} else {
		nd.axis = layout.Horizontal
		nd.textSize = values.TextSize16
		nd.leftInset = values.MarginPadding15
		nd.width = navDrawerMaximizedWidth
		nd.activeDrawerBtn = nd.MinimizeNavDrawerButton
		nd.alignment = layout.Start
		nd.direction = layout.W
	}
}
