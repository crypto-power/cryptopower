package cryptomaterial

import (
	"gioui.org/layout"
)

type ToggleButton struct {
	btns         []*Button
	theme        *Theme
	radius       CornerRadius // this radius is used by the clickable
	selectedItem int          // defaults to first btn

	colorInverted bool
	callback      func(selectedItem int)
}

func (t *Theme) ToggleButton(btns []*Button, colorInverted bool) *ToggleButton {
	tb := &ToggleButton{
		theme:         t,
		btns:          btns,
		selectedItem:  -1,
		colorInverted: colorInverted,
	}

	for i := range tb.btns {
		b := tb.btns[i]
		if tb.colorInverted {
			b.HighlightColor = tb.theme.Color.Gray5
		} else {
			b.HighlightColor = tb.theme.Color.Surface
		}
		b.Color = tb.theme.Color.Text
	}

	return tb
}

func (tb *ToggleButton) SelectItemAtIndex(index int) {
	itemsLen := len(tb.btns)
	if index > itemsLen || index < 0 {
		return // no-op
	}

	tb.selectedItem = index
	if tb.callback != nil {
		tb.callback(index)
	}
}

func (tb *ToggleButton) SetToggleButtonCallback(callback func(selectedItem int)) {
	tb.callback = callback
}

func (tb *ToggleButton) handleClickables() {
	for index := range tb.btns {
		b := tb.btns[index]
		for b.Clicked() {
			tb.selectedItem = index
			if tb.callback != nil {
				tb.callback(tb.selectedItem)
			}
		}
	}
}

func (tb *ToggleButton) Layout(gtx layout.Context) layout.Dimensions {
	tb.handleClickables()
	var btns []layout.FlexChild
	for index := range tb.btns {
		b := tb.btns[index]
		if index == tb.selectedItem {
			if tb.colorInverted {
				b.Background = tb.theme.Color.Gray5
			} else {
				b.Background = tb.theme.Color.Surface
			}
		} else {
			if tb.colorInverted {
				b.Background = tb.theme.Color.Surface
			} else {
				b.Background = tb.theme.Color.Gray5
			}
		}
		btns = append(btns, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(5).Layout(gtx, b.Layout)
		}))
	}

	bg := tb.theme.Color.Gray5
	if tb.colorInverted {
		bg = tb.theme.Color.Surface
	}
	return LinearLayout{
		Width:      WrapContent,
		Height:     WrapContent,
		Background: bg,
		Border: Border{
			Radius: Radius(8),
		},
	}.Layout(gtx, btns...)
}
