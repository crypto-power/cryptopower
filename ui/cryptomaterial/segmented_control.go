package cryptomaterial

import (
	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/values"
)

type SegmentedControl struct {
	theme *Theme
	list  *ClickableList

	selectedIndex int
	segmentItems  []string
}

func (t *Theme) SegmentedControl(segmentItems []string) *SegmentedControl {
	list := t.NewClickableList(layout.Horizontal)
	list.IsHoverable = false

	return &SegmentedControl{
		list:         list,
		theme:        t,
		segmentItems: segmentItems,
	}
}

func (sc *SegmentedControl) Layout(gtx C) D {
	sc.handleEvents()
	// var selectedTabDims D

	// sc.shadowBox.SetShadowRadius(20)
	return LinearLayout{
		Width:  WrapContent,
		Height: WrapContent,
		// Padding:    layout.UniformInset(values.MarginPadding16),
		Background: sc.theme.Color.Background,
		// Alignment:  layout.Middle,
		// Shadow:     sc.shadowBox,
		// Margin:     layout.UniformInset(values.MarginPadding5),
		Border: Border{Radius: Radius(8)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return sc.list.Layout(gtx, len(sc.segmentItems), func(gtx C, i int) D {
				isSelectedSegment := sc.SelectedIndex() == i
				// padding := values.MarginPadding24
				return layout.Stack{Alignment: layout.Center}.Layout(gtx,
					layout.Stacked(func(gtx C) D {
						return layout.Inset{
							// Right: padding,
							// Left:  padding,
							// Bottom: values.MarginPadding8,
						}.Layout(gtx, func(gtx C) D {
							return layout.Center.Layout(gtx, func(gtx C) D {
								bg := sc.theme.Color.SurfaceHighlight
								txt := sc.theme.Label(values.TextSize16, sc.segmentItems[i])
								txt.Color = sc.theme.Color.GrayText1
								txt.Font.Weight = font.SemiBold
								border := Border{Radius: Radius(0)}
								if isSelectedSegment {
									bg = sc.theme.Color.Surface
									txt.Color = sc.theme.Color.Text
									border = Border{Radius: Radius(8)}
								}
								return LinearLayout{
									Width:      WrapContent,
									Height:     WrapContent,
									Padding:    layout.UniformInset(values.MarginPadding8),
									Background: bg,
									// Alignment:  layout.Middle,
									// Shadow:     sc.shadowBox,
									Margin: layout.UniformInset(values.MarginPadding5),
									Border: border,
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return txt.Layout(gtx)
									}),
								)
								// }
								// lbl := sc.theme.Label(values.TextSize16, sc.segmentItems[i])
								// lbl.Color = sc.theme.Color.GrayText1
								// // if isSelectedSegment {
								// // 	lbl.Color = sc.theme.Color.Primary
								// // 	selectedTabDims = lbl.Layout(gtx)
								// // }

								// return lbl.Layout(gtx)
							})
						})
					}),
				)
			})
		}),
	)
}

func (sc *SegmentedControl) handleEvents() {
	if segmentItemClicked, clickedSegmentIndex := sc.list.ItemClicked(); segmentItemClicked {
		sc.selectedIndex = clickedSegmentIndex
	}
}

func (sc *SegmentedControl) SelectedIndex() int {
	return sc.selectedIndex
}

func (sc *SegmentedControl) SelectedSegment() string {
	return sc.segmentItems[sc.selectedIndex]
}

func (sc *SegmentedControl) SetSelectedSegment(segment string) {
	for i, item := range sc.segmentItems {
		if item == segment {
			sc.selectedIndex = i
		}
	}
}
