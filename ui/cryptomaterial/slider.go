// SPDX-License-Identifier: Unlicense OR MIT

package cryptomaterial

import (
	"image"
	"image/color"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

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
	clicker                  gesture.Click
	clicked                  bool
	disableButtonDirection   bool
	ControlInset             layout.Inset
}

var m4 = values.MarginPadding4

func (t *Theme) Slider() *Slider {
	sl := &Slider{
		t:          t,
		slideItems: make([]*sliderItem, 0),

		nextButton:               t.NewClickable(false),
		prevButton:               t.NewClickable(false),
		ButtonBackgroundColor:    t.Color.LightGray,
		IndicatorBackgroundColor: t.Color.LightGray,
		SelectedIndicatorColor:   t.Color.White,
		slideAction:              NewSlideAction(),
	}

	sl.ControlInset = layout.Inset{
		Right:  values.MarginPadding16,
		Left:   values.MarginPadding16,
		Bottom: values.MarginPadding16,
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

func (s *Slider) IsLastSlide() bool {
	return s.selected == len(s.slideItems)-1
}

func (s *Slider) NextSlide() {
	s.handleActionEvent(true)
}

func (s *Slider) ResetSlide() {
	s.selected = 0
}

func (s *Slider) SetDisableDirectionBtn(disable bool) {
	s.disableButtonDirection = disable
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

	s.handleClickEvent(gtx)
	var dims D
	var call op.CallOp
	{
		m := op.Record(gtx.Ops)
		dims = s.slideAction.DragLayout(gtx, func(gtx C) D {
			return layout.Stack{Alignment: layout.S}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return s.slideAction.TransformLayout(gtx, s.slideItems[s.selected].widgetItem)
				}),
				layout.Stacked(func(gtx C) D {
					if len(s.slideItems) == 1 {
						return D{}
					}
					return s.ControlInset.Layout(gtx, func(gtx C) D {
						return layout.Flex{
							Axis: layout.Horizontal,
						}.Layout(gtx,
							layout.Rigid(s.selectedItemIndicatorLayout),
							layout.Flexed(1, func(gtx C) D {
								if s.disableButtonDirection {
									return D{}
								}
								return layout.E.Layout(gtx, s.buttonLayout)
							}),
						)
					})
				}),
			)
		})
		call = m.Stop()
	}

	area := clip.Rect(image.Rect(0, 0, dims.Size.X, dims.Size.Y)).Push(gtx.Ops)
	s.clicker.Add(gtx.Ops)
	defer area.Pop()

	call.Add(gtx.Ops)
	return dims
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
					return s.prevButton.Layout(gtx, s.t.NewIcon(s.t.Icons.ChevronLeft).Layout20dp)
				}),
				layout.Rigid(func(gtx C) D {
					return s.nextButton.Layout(gtx, s.t.NewIcon(s.t.Icons.ChevronRight).Layout20dp)
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
				ic := NewIcon(s.t.Icons.DotIcon)
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

func (s *Slider) Clicked() bool {
	clicked := s.clicked
	s.clicked = false
	return clicked
}

func (s *Slider) handleClickEvent(gtx C) {
	if s.nextButton.Clicked(gtx) {
		s.handleActionEvent(true)
	}

	if s.prevButton.Clicked(gtx) {
		s.handleActionEvent(false)
	}

	for {
		e, ok := s.clicker.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			if !s.clicked {
				s.clicked = true
			}
		}
	}

	for i, item := range s.slideItems {
		if item.button.Clicked(gtx) {
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
