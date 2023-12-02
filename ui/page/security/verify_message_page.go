package security

import (
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const VerifyMessagePageID = "VerifyMessage"

type VerifyMessagePage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	wallet sharedW.Asset

	addressEditor          cryptomaterial.Editor
	messageEditor          cryptomaterial.Editor
	signatureEditor        cryptomaterial.Editor
	clearBtn, verifyButton cryptomaterial.Button
	backButton             cryptomaterial.IconButton
	infoButton             cryptomaterial.IconButton

	addressIsValid bool
}

func NewVerifyMessagePage(l *load.Load, wallet sharedW.Asset) *VerifyMessagePage {
	pg := &VerifyMessagePage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(VerifyMessagePageID),
		wallet:           wallet,
	}

	pg.addressEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrAddress))
	pg.addressEditor.Editor.SingleLine = true
	pg.addressEditor.Editor.Submit = true

	pg.messageEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrMessage))
	pg.messageEditor.Editor.SingleLine = true
	pg.messageEditor.Editor.Submit = true

	pg.signatureEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrSignature))
	pg.signatureEditor.Editor.Submit = true

	pg.verifyButton = l.Theme.Button(values.String(values.StrVerifyMessage))
	pg.verifyButton.Font.Weight = font.Medium

	pg.clearBtn = l.Theme.OutlineButton(values.String(values.StrClearAll))
	pg.clearBtn.Font.Weight = font.Medium

	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *VerifyMessagePage) OnNavigatedTo() {
	pg.addressEditor.Editor.Focus()

	pg.verifyButton.SetEnabled(pg.updateBtn())
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *VerifyMessagePage) Layout(gtx C) D {
	body := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrVerifyMessage),
			BackButton: pg.backButton,
			InfoButton: pg.infoButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				return pg.Theme.Card().Layout(gtx, func(gtx C) D {
					return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(pg.description()),
							layout.Rigid(pg.inputRow(pg.addressEditor)),
							layout.Rigid(pg.inputRow(pg.signatureEditor)),
							layout.Rigid(pg.inputRow(pg.messageEditor)),
							layout.Rigid(pg.verifyAndClearButtons()),
						)
					})
				})
			},
			InfoTemplate: modal.VerifyMessageInfoTemplate,
		}
		return sp.Layout(pg.ParentWindow(), gtx)
	}
	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx, body)
	}
	return pg.layoutDesktop(gtx, body)
}

func (pg *VerifyMessagePage) layoutDesktop(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return body(gtx)
}

func (pg *VerifyMessagePage) layoutMobile(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return components.UniformMobile(gtx, false, false, body)
}

func (pg *VerifyMessagePage) inputRow(editor cryptomaterial.Editor) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Bottom: values.MarginPadding15}.Layout(gtx, editor.Layout)
	}
}

func (pg *VerifyMessagePage) description() layout.Widget {
	return func(gtx C) D {
		desc := pg.Theme.Caption(values.String(values.StrVerifyMsgNote))
		desc.Color = pg.Theme.Color.GrayText2
		return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, desc.Layout)
	}
}

func (pg *VerifyMessagePage) verifyAndClearButtons() layout.Widget {
	return func(gtx C) D {
		dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.clearBtn.Layout)
						}),
						layout.Rigid(pg.verifyButton.Layout),
					)
				})
			}),
		)
		return dims
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *VerifyMessagePage) HandleUserInteractions() {
	pg.verifyButton.SetEnabled(pg.updateBtn())

	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(pg.addressEditor.Editor, pg.messageEditor.Editor, pg.signatureEditor.Editor)
	if isChanged {
		if pg.addressEditor.Editor.Focused() {
			pg.validateAddress()
		}
	}

	if (pg.verifyButton.Clicked() || isSubmit) && pg.validateAllInputs() {
		var verifyMessageText string
		var info *modal.InfoModal

		valid, err := pg.wallet.VerifyMessage(pg.addressEditor.Editor.Text(), pg.messageEditor.Editor.Text(), pg.signatureEditor.Editor.Text())
		if err != nil || !valid {
			verifyMessageText = values.String(values.StrInvalidAddress)
			if !valid {
				verifyMessageText = values.String(values.StrInvalidSignature)
			}
			info = modal.NewErrorModal(pg.Load, verifyMessageText, modal.DefaultClickFunc())
		} else {
			verifyMessageText = values.String(values.StrValidSignature)
			info = modal.NewSuccessModal(pg.Load, verifyMessageText, modal.DefaultClickFunc())
		}
		pg.ParentWindow().ShowModal(info)
	}

	if pg.clearBtn.Clicked() {
		pg.clearInputs()
	}
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this page wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *VerifyMessagePage) KeysToHandle() key.Set {
	return cryptomaterial.AnyKeyWithOptionalModifier(key.ModShift, key.NameTab)
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *VerifyMessagePage) HandleKeyEvent(evt *key.Event) {
	// Switch editors on tab press.
	cryptomaterial.SwitchEditors(evt, pg.addressEditor.Editor, pg.signatureEditor.Editor, pg.messageEditor.Editor)
}

func (pg *VerifyMessagePage) validateAllInputs() bool {
	if !pg.validateAddress() {
		return false
	}

	if !utils.StringNotEmpty(pg.signatureEditor.Editor.Text()) {
		pg.signatureEditor.SetError(values.String(values.StrEmptySign))
		return false
	}

	if !utils.StringNotEmpty(pg.messageEditor.Editor.Text()) {
		pg.messageEditor.SetError(values.String(values.StrEmptyMsg))
		return false
	}

	return true
}

func (pg *VerifyMessagePage) updateBtn() bool {
	if pg.addressIsValid || utils.StringNotEmpty(pg.signatureEditor.Editor.Text()) || utils.StringNotEmpty(pg.messageEditor.Editor.Text()) {
		return true
	}
	return false
}

func (pg *VerifyMessagePage) clearInputs() {
	pg.signatureEditor.Editor.SetText("")
	pg.messageEditor.Editor.SetText("")
	pg.addressEditor.SetError("")
}

func (pg *VerifyMessagePage) validateAddress() bool {
	address := pg.addressEditor.Editor.Text()
	pg.addressEditor.SetError("")

	var valid bool
	var errorMessage string

	switch {
	case !utils.StringNotEmpty(address):
		errorMessage = values.String(values.StrEnterValidAddress)
	case !pg.wallet.IsAddressValid(address):
		errorMessage = values.String(values.StrInvalidAddress)
	default:
		valid = true
	}
	if !valid {
		pg.addressEditor.SetError(errorMessage)
	}

	pg.addressIsValid = valid
	return valid
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *VerifyMessagePage) OnNavigatedFrom() {}
