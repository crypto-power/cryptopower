package device

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import "device_ios.h"
*/
import "C"
import (
	"gioui.org/app"
	"gioui.org/io/event"
)

type device struct {
	window *app.Window
	view   uintptr
}

func newDevice(w *app.Window) *device {
	return &device{window: w}
}

func (d *Device) setScreenAwake(isOn bool) error {
	d.window.Run(func() {
		C.setScreenAwake(C.bool(isOn))
	})
	return nil
}

func (d *Device) listenEvents(evt event.Event) {
	if evt, ok := evt.(app.UIKitViewEvent); ok {
		d.view = evt.ViewController
	}
}
