package cryptomaterial

import (
	"fmt"
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

const defaultDuration = 300 * time.Millisecond

type DragDirection int

const (
	SlideLeft DragDirection = iota
	SlideRight
)

type Dragged func(dragDirection DragDirection)

type SliceShow struct {
	Duration time.Duration

	push int

	next *op.Ops

	nextCall op.CallOp
	lastCall op.CallOp

	t0     time.Time
	offset float32

	// animation state
	dragging    bool
	dragStarted f32.Point
	dragOffset  int

	drag    gesture.Drag
	Draged  Dragged
	canPush bool

	IsShowCarousel bool
}

func (t *Theme) SliceShow() *SliceShow {
	return &SliceShow{
		canPush:        true,
		IsShowCarousel: true,
	}
}

// PushLeft pushes the existing widget to the left.
func (s *SliceShow) PushLeft() { s.push = 1 }

// PushRight pushes the existing widget to the right.
func (s *SliceShow) PushRight() { s.push = -1 }

func (s *SliceShow) Layout(gtx C, w layout.Widget) D {
	for _, event := range s.drag.Events(gtx.Metric, gtx.Queue, gesture.Horizontal) {
		switch event.Type {
		case pointer.Press:
			fmt.Println("----drag.Events---------Press----")
			s.dragStarted = event.Position
			s.dragOffset = 0
			s.dragging = true
		case pointer.Drag:
			newOffset := int(s.dragStarted.X - event.Position.X)
			fmt.Println("----drag.Events---------Drag----", newOffset)
			if newOffset > 100 {
				if s.canPush && s.Draged != nil {
					s.canPush = false
					s.Draged(SlideRight)
				}
			} else if newOffset < -100 {
				if s.canPush && s.Draged != nil {
					s.canPush = false
					s.Draged(SlideLeft)
				}
			}
			s.dragOffset = newOffset
		case pointer.Release:
			fmt.Println("----drag.Events---------Release----")
			fallthrough
		case pointer.Cancel:
			fmt.Println("----drag.Events---------Cancel----")
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
		dims = w(gtx)
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
// func smooth(t float32) float32 {
// 	if t < 0 {
// 		return -easeInOutCubic(-t)
// 	}
// 	return easeInOutCubic(t)
// }

// // easeInOutCubic maps a linear value to a ease-in-out-cubic easing function.
// func easeInOutCubic(t float32) float32 {
// 	if t < 0.5 {
// 		return 4 * t * t * t
// 	}
// 	return (t-1)*(2*t-2)*(2*t-2) + 1
// }
