// SPDX-License-Identifier: Unlicense OR MIT

package cryptomaterial

import (
	"image/color"

	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/values"
)

type sliderItem struct {
	widgetItem layout.Widget
	button     *Clickable
}

type Slider struct {
	t *Theme

	nextButton *Clickable
	prevButton *Clickable
	card       Card
	slideItems []*sliderItem

	selected         int
	isSliderItemsSet bool
	// colors of the indicator and navigation button
	ButtonBackgroundColor    color.NRGBA
	IndicatorBackgroundColor color.NRGBA
	SelectedIndicatorColor   color.NRGBA // this is a full color no opacity
	slideAction              *SlideAction
}

var m4 = values.MarginPadding4

func (t *Theme) Slider() *Slider {
	sl := &Slider{
		t:          t,
		slideItems: make([]*sliderItem, 0),

		nextButton:               t.NewClickable(false),
		prevButton:               t.NewClickable(false),
		ButtonBackgroundColor:    values.TransparentColor(values.TransparentWhite, 0.2),
		IndicatorBackgroundColor: values.TransparentColor(values.TransparentWhite, 0.2),
		SelectedIndicatorColor:   t.Color.White,
		slideAction:              NewSliceAction(),
	}

	sl.card = sl.t.Card()
	sl.card.Radius = Radius(8)

	sl.slideAction.Draged(func(dragDirection SwipeDirection) {
		isNext := dragDirection == SwipeLeft
		sl.handleActionEvent(isNext)
	})

	return sl
}

// GetSelectedIndex returns the index of the current slider item.
func (s *Slider) GetSelectedIndex() int {
	return s.selected
}

func (s *Slider) sliderItems(items []layout.Widget) []*sliderItem {
	slideItems := make([]*sliderItem, 0)
	for _, item := range items {
		slideItems = append(slideItems, &sliderItem{
			widgetItem: item,
			button:     s.t.NewClickable(false),
		})
	}

	return slideItems
}

func (s *Slider) Layout(gtx C, items []layout.Widget) D {
	// set slider items once since layout is drawn multiple times per sec.
	if !s.isSliderItemsSet {
		s.slideItems = s.sliderItems(items)
		s.isSliderItemsSet = true
	}

	if len(s.slideItems) == 0 {
		return D{}
	}

	s.handleClickEvent()
	return s.slideAction.DragLayout(gtx, func(gtx C) D {
		return layout.Stack{Alignment: layout.S}.Layout(gtx,
			layout.Expanded(func(gtx C) D {
				return s.slideAction.TransformLayout(gtx, s.slideItems[s.selected].widgetItem)
			}),
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
	})
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
			return list.Layout(gtx, len(s.slideItems), func(gtx C, i int) D {
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
					return s.slideItems[i].button.Layout(gtx, func(gtx C) D {
						return ic.Layout(gtx, values.MarginPadding12)
					})
				})
			})
		})
	})
}

func (s *Slider) containerLayout(gtx C, content layout.Widget) D {
	return s.card.Layout(gtx, content)
}

func (s *Slider) RefreshItems() {
	s.isSliderItemsSet = false
}

func (s *Slider) handleClickEvent() {
	if s.nextButton.Clicked() {
		s.handleActionEvent(true)
	}

	if s.prevButton.Clicked() {
		s.handleActionEvent(false)
	}

	for i, item := range s.slideItems {
		if item.button.Clicked() {
			if i == s.selected {
				continue
			}
			lastSelected := s.selected
			s.selected = i
			if lastSelected < i {
				s.slideAction.PushLeft()
			} else {
				s.slideAction.PushRight()
			}
			break
		}
	}
}

func (s *Slider) handleActionEvent(isNext bool) {
	if len(s.slideItems) == 1 {
		return
	}
	l := len(s.slideItems) - 1 // index starts at 0
	if isNext {
		if s.selected == l {
			s.selected = 0
		} else {
			s.selected++
		}
		s.slideAction.PushLeft()
	} else {
		if s.selected == 0 {
			s.selected = l
		} else {
			s.selected--
		}
		s.slideAction.PushRight()
	}
}
