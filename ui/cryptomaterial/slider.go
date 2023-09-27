// SPDX-License-Identifier: Unlicense OR MIT

package cryptomaterial

import (
	"image/color"

	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/values"
)

type Slider struct {
	t *Theme

	nextButton *Clickable
	prevButton *Clickable
	card       Card
	items      []layout.Widget

	selected         int
	isSliderItemsSet bool
	// colors of the indicator and navigation button
	ButtonBackgroundColor    color.NRGBA
	IndicatorBackgroundColor color.NRGBA
	SelectedIndicatorColor   color.NRGBA // this is a full color no opacity
}

var m4 = values.MarginPadding4

func (t *Theme) Slider() *Slider {
	sl := &Slider{
		t:     t,
		items: make([]layout.Widget, 0),

		nextButton:               t.NewClickable(false),
		prevButton:               t.NewClickable(false),
		ButtonBackgroundColor:    values.TransparentColor(values.TransparentWhite, 0.2),
		IndicatorBackgroundColor: values.TransparentColor(values.TransparentWhite, 0.2),
		SelectedIndicatorColor:   t.Color.White,
	}

	sl.card = sl.t.Card()
	sl.card.Radius = Radius(8)

	return sl
}

func (s *Slider) Layout(gtx C, items []layout.Widget) D {
	// set slider items once since layout is drawn multiple times per sec.
	if !s.isSliderItemsSet {
		s.items = items
		s.isSliderItemsSet = true
	}
	s.handleClickEvent()

	gtx.Constraints.Max = s.items[s.selected](gtx).Size
	return layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Expanded(s.items[s.selected]),
		layout.Stacked(func(gtx C) D {
			return layout.Inset{
				Right:  values.MarginPadding15,
				Left:   values.MarginPadding15,
				Bottom: values.MarginPadding10,
			}.Layout(gtx, func(gtx C) D {
				return layout.Flex{
					Axis: layout.Horizontal,
				}.Layout(gtx,
					layout.Rigid(s.selectedItemIndicatorLayout),
					layout.Flexed(1, func(gtx C) D {
						return layout.E.Layout(gtx, s.buttonLayout)
					}),
				)
			})
		}),
	)
}

func (s *Slider) buttonLayout(gtx C) D {
	s.card.Radius = Radius(10)
	s.card.Color = s.ButtonBackgroundColor
	return s.containerLayout(gtx, func(gtx C) D {
		return layout.Inset{
			Right: m4,
			Left:  m4,
		}.Layout(gtx, func(gtx C) D {
			return LinearLayout{
				Width:       WrapContent,
				Height:      WrapContent,
				Orientation: layout.Horizontal,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return s.prevButton.Layout(gtx, s.t.Icons.ChevronLeft.Layout20dp)
				}),
				layout.Rigid(func(gtx C) D {
					return s.nextButton.Layout(gtx, s.t.Icons.ChevronRight.Layout20dp)
				}),
			)
		})
	})
}

func (s *Slider) selectedItemIndicatorLayout(gtx C) D {
	m4 := values.MarginPadding4
	s.card.Radius = Radius(10)
	s.card.Color = s.IndicatorBackgroundColor
	return s.containerLayout(gtx, func(gtx C) D {
		return layout.Inset{
			Right: m4,
			Left:  m4,
		}.Layout(gtx, func(gtx C) D {
			list := &layout.List{Axis: layout.Horizontal}
			return list.Layout(gtx, len(s.items), func(gtx C, i int) D {
				ic := NewIcon(s.t.Icons.ImageBrightness1)
				ic.Color = values.TransparentColor(values.TransparentBlack, 0.2)
				if i == s.selected {
					ic.Color = s.SelectedIndicatorColor
				}
				return layout.Inset{
					Top:    m4,
					Bottom: m4,
					Right:  m4,
					Left:   m4,
				}.Layout(gtx, func(gtx C) D {
					return ic.Layout(gtx, values.MarginPadding12)
				})
			})
		})
	})
}

func (s *Slider) containerLayout(gtx C, content layout.Widget) D {
	return s.card.Layout(gtx, content)
}

func (s *Slider) centerLayout(gtx C, content layout.Widget) D {
	return layout.Center.Layout(gtx, content)
}

func (s *Slider) RefreshItems() {
	s.isSliderItemsSet = false
}

func (s *Slider) handleClickEvent() {
	l := len(s.items) - 1 // index starts at 0
	if s.nextButton.Clicked() {
		if s.selected == l {
			s.selected = 0
		} else {
			s.selected++
		}
	}

	if s.prevButton.Clicked() {
		if s.selected == 0 {
			s.selected = l
		} else {
			s.selected--
		}
	}
}
