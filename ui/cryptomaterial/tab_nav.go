package cryptomaterial

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/crypto-power/cryptopower/ui/values"
)

type TabNavigation struct {
	theme *Theme
	list  *ClickableList

	selectedIndex int
	changed       bool
	tabItems      []string
}

func (t *Theme) TabNavigation(axis layout.Axis, isHoverable bool, tabItems []string) *TabNavigation {
	list := t.NewClickableList(axis)
	list.IsHoverable = isHoverable
	return &TabNavigation{
		list:     list,
		theme:    t,
		tabItems: tabItems,
	}
}

func (tn *TabNavigation) Layout(gtx C) D {
	tn.handleEvents()
	var selectedTabDims D

	return tn.list.Layout(gtx, len(tn.tabItems), func(gtx C, i int) D {
		isSelectedTab := tn.SelectedIndex() == i
		padding := values.MarginPadding24
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx C) D {
				return layout.Inset{
					Right:  padding,
					Left:   padding,
					Bottom: values.MarginPadding8,
				}.Layout(gtx, func(gtx C) D {
					return layout.Center.Layout(gtx, func(gtx C) D {
						lbl := tn.theme.Label(values.TextSize16, tn.tabItems[i])
						lbl.Color = tn.theme.Color.GrayText1
						if isSelectedTab {
							lbl.Color = tn.theme.Color.Primary
							selectedTabDims = lbl.Layout(gtx)
						}

						return lbl.Layout(gtx)
					})
				})
			}),
			layout.Stacked(func(gtx C) D {
				if !isSelectedTab {
					return D{}
				}

				tabHeight := gtx.Dp(values.MarginPadding4)
				selectedTabDimsWidth := gtx.Dp(values.MarginPadding50)
				tabRect := image.Rect(0, 0, selectedTabDims.Size.X+selectedTabDimsWidth, tabHeight)
				defer clip.RRect{Rect: tabRect, SE: 0, SW: 0, NW: 10, NE: 10}.Push(gtx.Ops).Pop()

				return layout.Inset{
					Bottom: values.MarginPaddingMinus22,
				}.Layout(gtx, func(gtx C) D {
					paint.FillShape(gtx.Ops, tn.theme.Color.Primary, clip.Rect(tabRect).Op())
					return layout.Dimensions{
						Size: image.Point{X: selectedTabDims.Size.X + selectedTabDimsWidth, Y: tabHeight},
					}
				})
			}),
		)
	})
}

func (tn *TabNavigation) handleEvents() {
	if tabItemClicked, clickedTabIndex := tn.list.ItemClicked(); tabItemClicked {
		if tn.selectedIndex != clickedTabIndex {
			tn.changed = true
		}
		tn.selectedIndex = clickedTabIndex
	}
}

func (tn *TabNavigation) SelectedIndex() int {
	return tn.selectedIndex
}

func (tn *TabNavigation) SelectedTab() string {
	return tn.tabItems[tn.selectedIndex]
}

func (tn *TabNavigation) Changed() bool {
	changed := tn.changed
	tn.changed = false
	return changed
}
