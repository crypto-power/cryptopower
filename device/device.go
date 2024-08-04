package device

import (
	"errors"

	"gioui.org/app"
	"gioui.org/io/event"
)

var ErrNotAvailable = errors.New("current OS not supported")

type Device struct {
	*device
}

func NewDevice(w *app.Window) *Device {
	return &Device{
		device: newDevice(w),
	}
}

func (d *Device) SetScreenAwake(isOn bool) error {
	return d.setScreenAwake(isOn)
}

func (d *Device) ProcessEvent(w *app.Window) event.Event {
	evt := w.Event()
	switch e := evt.(type) {
	case app.FrameEvent:
		return e
	case app.ViewEvent:
		d.listenEvents(evt)
		return e
	default:
		return evt
	}
}
