package cryptomaterial

import (
	"fmt"
	"image"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

type Hoverable struct {
	hovered  bool
	position *f32.Point
}

func (t *Theme) Hoverable() *Hoverable {
	return &Hoverable{}
}

func (h *Hoverable) Hovered() bool {
	return h.hovered
}

func (h *Hoverable) Position() *f32.Point {
	return h.position
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
	// // TODO07
	// for {
	// 	event, ok := gtx.Event(pointer.Filter{
	// 		Target: h,
	// 		Kinds:  pointer.Enter | pointer.Leave,
	// 	})
	// 	if !ok {
	// 		continue
	// 	}
	// 	ev, ok := event.(pointer.Event)
	// 	if !ok {
	// 		continue
	// 	}
	// 	switch ev.Kind {
	// 	case pointer.Enter:
	// 		h.hovered = true
	// 		h.position = &ev.Position
	// 	case pointer.Leave:
	// 		h.hovered = false
	// 		h.position = &f32.Point{}
	// 	}
	// }
}

func (h *Hoverable) Layout(gtx C, rect image.Rectangle) D {
	h.update(gtx)
	// fmt.Println("-----Hoverable------0000-----")
	// area := clip.Rect(rect).Push(gtx.Ops)
	// fmt.Println("-----Hoverable------1111-----")
	// event.Op(gtx.Ops, h)
	// fmt.Println("-----Hoverable------2222-----")
	// // TODO07
	// // pointer.InputOp{
	// // 	Tag:   h,
	// // 	Types: pointer.Enter | pointer.Leave,
	// // }.Add(gtx.Ops)
	// area.Pop()
	// fmt.Println("-----Hoverable------3333-----")

	// return layout.Dimensions{
	// 	Size: rect.Max,
	// }

	fmt.Println("-----Hoverable------0000-----")
	defer clip.Rect(rect).Push(gtx.Ops).Pop()
	fmt.Println("-----Hoverable------1111-----")
	event.Op(gtx.Ops, h)
	fmt.Println("-----Hoverable------2222-----")
	return D{Size: gtx.Constraints.Max}
}
