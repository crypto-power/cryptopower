package cryptomaterial

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

const defaultDuration = 300 * time.Millisecond

type SwipeDirection int

const (
	SwipeLeft SwipeDirection = iota
	SwipeRight
)

type Dragged func(dragDirection SwipeDirection)

type SliceAction struct {
	duration time.Duration
	push     int
	next     *op.Ops
	nextCall op.CallOp
	lastCall op.CallOp

	t0     time.Time
	offset float32

	// animation state
	dragStarted f32.Point
	dragOffset  int
	drag        gesture.Drag
	draged      Dragged
	isPushing   bool
}

// PushLeft pushes the existing widget to the left.
func (s *SliceAction) PushLeft() { s.push = 1 }

// PushRight pushes the existing widget to the right.
func (s *SliceAction) PushRight() { s.push = -1 }

func (s *SliceAction) Draged(drag Dragged) {
	s.draged = drag
}

func (s *SliceAction) DragLayout(gtx C, w layout.Widget, isWrapContent bool) D {
	for _, event := range s.drag.Events(gtx.Metric, gtx.Queue, gesture.Horizontal) {
		switch event.Type {
		case pointer.Press:
			s.dragStarted = event.Position
			s.dragOffset = 0
		case pointer.Drag:
			newOffset := int(s.dragStarted.X - event.Position.X)
			if newOffset > 100 {
				if !s.isPushing && s.draged != nil {
					s.isPushing = true
					s.draged(SwipeLeft)
				}
			} else if newOffset < -100 {
				if !s.isPushing && s.draged != nil {
					s.isPushing = true
					s.draged(SwipeRight)
				}
			}
			s.dragOffset = newOffset
		case pointer.Release:
			fallthrough
		case pointer.Cancel:
			s.isPushing = false
		}
	}

	if isWrapContent {
		area := clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)).Push(gtx.Ops)
		s.drag.Add(gtx.Ops)
		defer area.Pop()
	} else {
		s.drag.Add(gtx.Ops)
	}

	return w(gtx)
}

func (s *SliceAction) TransformLayout(gtx C, w layout.Widget) D {
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
		duration := s.duration
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
