package send

import (
	"context"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

type PageModal struct {
	*load.Load
	*cryptomaterial.Modal

	ctx       context.Context // modal context
	ctxCancel context.CancelFunc

	*sharedProperties
}

func NewPageModal(l *load.Load) *PageModal {
	sm := &PageModal{
		Load:              l,
		Modal:             l.Theme.ModalFloatTitle(values.String(values.StrSend)),
		sharedProperties: newSharedProperties(l, true),
	}

	return sm
}

func (sm *PageModal) OnResume() {
	sm.ctx, sm.ctxCancel = context.WithCancel(context.TODO())
	sm.sourceAccountSelector.ListenForTxNotifications(sm.ctx, sm.ParentWindow())

	sm.onLoaded()
}

func (sm *PageModal) OnDismiss() {
	sm.ctxCancel()
}

func (sm *PageModal) Handle() {
	if sm.Modal.BackdropClicked(true) {
		sm.Dismiss()
	}

	sm.handleFunc()
}

func (sm *PageModal) Layout(gtx C) D {
	modalContent := []layout.Widget{
		func(gtx C) D {
			return sm.layoutDesktop(gtx, sm.ParentWindow())
		},
	}
	return sm.Modal.Layout(gtx, modalContent, 450)
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this page wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (sm *PageModal) KeysToHandle() key.Set {
	return cryptomaterial.AnyKeyWithOptionalModifier(key.ModShift, key.NameTab)
}
