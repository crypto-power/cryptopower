package cryptomaterial

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/ui/values"
)

type SegmentedControl struct {
	theme *Theme
	btns  []*widget.Clickable

	selectedIndex int
	items         []string
}

func (t *Theme) SegmentedControl(items []string) *SegmentedControl {
	btns := make([]*widget.Clickable, len(items))
	for i := range btns {
		btns[i] = new(widget.Clickable)
	}
	return &SegmentedControl{
		theme: t,
		btns:  btns,
		items: items,
	}
}

func (sc *SegmentedControl) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			// Draw background here if needed
			return layout.Dimensions{}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dims := layout.Dimensions{}
					for i := range sc.btns {
						btn := sc.btns[i]
						item := sc.items[i]
						inset := layout.Inset{Top: values.MarginPadding8, Bottom: values.MarginPadding8, Left: values.MarginPadding16, Right: values.MarginPadding16}
						dims = inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							for btn.Clicked() {
								sc.selectedIndex = i
							}
							lbl := sc.theme.Label(values.TextSize16, item)
							if i == sc.selectedIndex {
								lbl.Color = sc.theme.Color.Primary
								paint.FillShape(gtx.Ops, sc.theme.Color.Primary, clip.Rect{Min: image.Point{}, Max: gtx.Constraints.Max}.Op())
							}
							return lbl.Layout(gtx)
						})
					}
					return dims
				}),
			)
		}),
	)
}

func (sc *SegmentedControl) SelectedIndex() int {
	return sc.selectedIndex
}

func (sc *SegmentedControl) SetSelectedIndex(index int) {
	if index >= 0 && index < len(sc.items) {
		sc.selectedIndex = index
	}
}

func (sc *SegmentedControl) SelectedItem() string {
	return sc.items[sc.selectedIndex]
}
