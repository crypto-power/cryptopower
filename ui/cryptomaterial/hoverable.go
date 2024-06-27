package cryptomaterial

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

type Hoverable struct {
	hovered bool
}

func (t *Theme) Hoverable() *Hoverable {
	return &Hoverable{}
}

func (h *Hoverable) Hovered() bool {
	return h.hovered
}

func (h *Hoverable) update(gtx C) {
	start := h.hovered
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: h,
			Kinds:  pointer.Enter | pointer.Leave,
		})
		if !ok {
			break
		}
		switch ev := ev.(type) {
		case pointer.Event:
			switch ev.Kind {
			case pointer.Enter:
				h.hovered = true
			case pointer.Leave:
				h.hovered = false
			case pointer.Cancel:
				h.hovered = false
			}
		}
	}
	if h.hovered != start {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (h *Hoverable) Layout(gtx C, rect image.Rectangle) D {
	h.update(gtx)
	defer clip.Rect(rect).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, h)
	return D{Size: rect.Max}
}
