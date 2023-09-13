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

	AppBarNavItems    []NavHandler
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
