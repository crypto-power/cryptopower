// SPDX-License-Identifier: Unlicense OR MIT

package cryptomaterial

import (
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

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

	Duration time.Duration
	push     int
	next     *op.Ops
	nextCall op.CallOp
	lastCall op.CallOp
	t0       time.Time
	offset   float32

	// animation state
	dragging    bool
	dragStarted f32.Point
	dragOffset  int

	drag    gesture.Drag
	Draged  Dragged
	canPush bool

	IsShowCarousel bool
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
		canPush:                  true,
	}

	sl.card = sl.t.Card()
	sl.card.Radius = Radius(8)

	return sl
}

// GetSelectedIndex returns the index of the current slider item.
func (s *Slider) GetSelectedIndex() int {
	return s.selected
}

func (s *Slider) Layout(gtx C, items []layout.Widget) D {
	for _, event := range s.drag.Events(gtx.Metric, gtx.Queue, gesture.Horizontal) {
		switch event.Type {
		case pointer.Press:
			s.dragStarted = event.Position
			s.dragOffset = 0
			s.dragging = true
		case pointer.Drag:
			newOffset := int(s.dragStarted.X - event.Position.X)
			if newOffset > 100 && s.canPush {
				s.handleActionEvent(true)
				s.canPush = false
			} else if newOffset < -100 && s.canPush {
				s.handleActionEvent(false)
				s.canPush = false
			}
			s.dragOffset = newOffset
		case pointer.Release:
			fallthrough
		case pointer.Cancel:
			s.dragging = false
			s.canPush = true
		}
	}

	if s.push != 0 {
		s.next = nil
		s.lastCall = s.nextCall
		s.offset = float32(s.push)
		s.t0 = gtx.Now
		s.push = 0
	}

	var delta time.Duration
	if !s.t0.IsZero() {
		now := gtx.Now
		delta = now.Sub(s.t0)
		s.t0 = now
	}

	if s.offset != 0 {
		duration := s.Duration
		if duration == 0 {
			duration = defaultDuration
		}
		movement := float32(delta.Seconds()) / float32(duration.Seconds())
		if s.offset < 0 {
			s.offset += movement
			if s.offset >= 0 {
				s.offset = 0
			}
		} else {
			s.offset -= movement
			if s.offset <= 0 {
				s.offset = 0
			}
		}

		op.InvalidateOp{}.Add(gtx.Ops)
	}

	var dims layout.Dimensions
	{
		if s.next == nil {
			s.next = new(op.Ops)
		}
		gtx := gtx
		gtx.Ops = s.next
		gtx.Ops.Reset()
		m := op.Record(gtx.Ops)
		dims = s.layout(gtx, items)
		s.nextCall = m.Stop()
	}

	s.drag.Add(gtx.Ops)

	if s.offset == 0 {
		s.nextCall.Add(gtx.Ops)
		return dims
	}

	offset := smooth(s.offset)

	if s.offset > 0 {
		defer op.Offset(image.Point{
			X: int(float32(dims.Size.X) * (offset - 1)),
		}).Push(gtx.Ops).Pop()
		s.lastCall.Add(gtx.Ops)

		defer op.Offset(image.Point{
			X: dims.Size.X,
		}).Push(gtx.Ops).Pop()
		s.nextCall.Add(gtx.Ops)
	} else {
		defer op.Offset(image.Point{
			X: int(float32(dims.Size.X) * (offset + 1)),
		}).Push(gtx.Ops).Pop()
		s.lastCall.Add(gtx.Ops)

		defer op.Offset(image.Point{
			X: -dims.Size.X,
		}).Push(gtx.Ops).Pop()
		s.nextCall.Add(gtx.Ops)
	}
	return dims
}

// smooth handles -1 to 1 with ease-in-out cubic easing func.
func smooth(t float32) float32 {
	if t < 0 {
		return -easeInOutCubic(-t)
	}
	return easeInOutCubic(t)
}

// easeInOutCubic maps a linear value to a ease-in-out-cubic easing function.
func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return (t-1)*(2*t-2)*(2*t-2) + 1
}

func (s *Slider) layout(gtx C, items []layout.Widget) D {
	// set slider items once since layout is drawn multiple times per sec.
	if !s.isSliderItemsSet {
		s.items = items
		s.isSliderItemsSet = true
	}

	if len(s.items) == 0 {
		return D{}
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
}

func (s *Slider) handleActionEvent(isNext bool) {
	l := len(s.items) - 1 // index starts at 0
	if isNext {
		if s.selected == l {
			s.selected = 0
		} else {
			s.selected++
		}
		s.push = 1
	} else {
		if s.selected == 0 {
			s.selected = l
		} else {
			s.selected--
		}
		s.push = -1
	}
}
