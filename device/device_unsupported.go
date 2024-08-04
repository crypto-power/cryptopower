//go:build !android && !ios
// +build !android,!ios

package device

import (
	"gioui.org/app"
	"gioui.org/io/event"
)

type device struct{}

func newDevice(_ *app.Window) *device {
	return new(device)
}

func (d *Device) setScreenAwake(_ bool) error {
	return ErrNotAvailable
}

func (d *Device) listenEvents(_ event.Event) {}
