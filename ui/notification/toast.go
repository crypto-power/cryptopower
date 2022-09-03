// This file contains the toast UI component implementation which should be used
// sparingly unless explicity defined and used in a given the UI design.
// TODO: Add Toast UI Components usage policy URL.

package notification

import (
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/values"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Toast struct {
	sync.Mutex
	theme   *cryptomaterial.Theme
	success bool
	message string
	timer   *time.Timer
}

const (
	// shortDelay s limited to 2 seconds.
	shortDelay = 2 * time.Second
	// longDelay is limited to 5 seconds
	longDelay = 5 * time.Second
)

// NewToast returns an initialized instance of the toast UI component.
// To avoid poor user experience on the UI, this component should sparingly used.
func NewToast(th *cryptomaterial.Theme) *Toast {
	return &Toast{
		theme: th,
	}
}

func checkDelay(isLong bool) time.Duration {
	if isLong {
		return longDelay
	}
	return shortDelay
}

// Notify is called to display a message indicating a successful action.
// It updates the toast object with the toast message and duration.
// isLongDelay parameter is optional.
func (t *Toast) Notify(message string, isLongDelay ...bool) {
	t.notify(message, true, isLongDelay...)
}

// Notify is called to display a message indicating a failed action.
// It updates the toast object with the toast message and duration.
// isLongDelay parameter is optional.
func (t *Toast) NotifyError(message string, isLongDelay ...bool) {
	t.notify(message, false, isLongDelay...)
}

// notify updates notification parameters on the toast object.
// It takes the message, success and duration parameters.
// It defaults to 5 secs if long delay set to true and 2 sec if otherwise.
func (t *Toast) notify(message string, success bool, d ...bool) {
	var isLong bool
	if len(d) > 0 {
		isLong = d[0]
	}

	t.Lock()
	defer t.Unlock()
	t.message = message
	t.success = success
	t.timer = time.NewTimer(checkDelay(isLong))
}

// Layout uses the provided dimensions and constraints to construct a toast UI
// component.
func (t *Toast) Layout(gtx layout.Context) layout.Dimensions {
	if t.timer == nil {
		return layout.Dimensions{}
	}

	go t.handleToastDisplay(gtx)

	color := t.theme.Color.Success
	if !t.success {
		color = t.theme.Color.Danger
	}

	card := t.theme.Card()
	card.Color = color
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: values.MarginPadding65}.Layout(gtx, func(gtx C) D {
			return card.Layout(gtx, func(gtx C) D {
				return layout.Inset{
					Top: values.MarginPadding7, Bottom: values.MarginPadding7,
					Left: values.MarginPadding15, Right: values.MarginPadding15,
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					msg := t.theme.Body1(t.message)
					msg.Color = t.theme.Color.Surface
					return msg.Layout(gtx)
				})
			})
		})
	})
}

// handleToastDisplay removes the toast UI component after the timer expires.
func (t *Toast) handleToastDisplay(gtx layout.Context) {
	select {
	case <-t.timer.C:
		t.timer = nil
		op.InvalidateOp{}.Add(gtx.Ops)
	default:
	}
}
