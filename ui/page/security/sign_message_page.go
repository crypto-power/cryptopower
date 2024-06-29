package security

import (
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	uiUtils "github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const SignMessagePageID = "SignMessage"

type (
	C = layout.Context
	D = layout.Dimensions
)

type SignMessagePage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	container layout.List
	wallet    sharedW.Asset

	isSigningMessage bool
	addressIsValid   bool
	messageIsValid   bool
	isEnabled        bool

	titleLabel, errorLabel, signedMessageLabel cryptomaterial.Label
	addressEditor, messageEditor               *cryptomaterial.Editor
	clearButton, signButton, copyButton        cryptomaterial.Button
	copySignature                              *cryptomaterial.Clickable

	backButton cryptomaterial.IconButton
	infoButton cryptomaterial.IconButton
}

func NewSignMessagePage(l *load.Load, wallet sharedW.Asset) *SignMessagePage {
	addressEditor := l.Theme.Editor(new(widget.Editor), values.String(values.StrAddress))
	addressEditor.Editor.SingleLine, addressEditor.Editor.Submit = true, true
	messageEditor := l.Theme.Editor(new(widget.Editor), values.String(values.StrMessage))
	messageEditor.Editor.SingleLine, messageEditor.Editor.Submit = true, true

	clearButton := l.Theme.OutlineButton(values.String(values.StrClearAll))
	signButton := l.Theme.Button(values.String(values.StrSignMessage))
	clearButton.Font.Weight, signButton.Font.Weight = font.Medium, font.Medium
	signButton.SetEnabled(false)
	clearButton.SetEnabled(false)

	errorLabel := l.Theme.Caption("")
	errorLabel.Color = l.Theme.Color.Danger

	pg := &SignMessagePage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(SignMessagePageID),
		wallet:           wallet,
		container: layout.List{
			Axis: layout.Vertical,
		},

		titleLabel:         l.Theme.H5(values.String(values.StrSignMessage)),
		signedMessageLabel: l.Theme.Body1(""),
		errorLabel:         errorLabel,
		addressEditor:      &addressEditor,
		messageEditor:      &messageEditor,
		clearButton:        clearButton,
		signButton:         signButton,
		copyButton:         l.Theme.Button(values.String(values.StrCopy)),
		copySignature:      l.Theme.NewClickable(false),
	}

	pg.signedMessageLabel.Color = l.Theme.Color.GrayText2
	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.backButton = components.GetBackButton(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *SignMessagePage) OnNavigatedTo() {
	pg.addressEditor.Focus()
}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *SignMessagePage) Layout(gtx C) D {
	pg.handleEditorEvents(gtx)
	body := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrSignMessage),
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
							layout.Rigid(pg.editors(pg.addressEditor)),
							layout.Rigid(pg.editors(pg.messageEditor)),
							layout.Rigid(pg.drawButtonsRow()),
							layout.Rigid(pg.drawResult()),
						)
					})
				})
			},
			InfoTemplate: modal.SignMessageInfoTemplate,
		}
		return sp.Layout(pg.ParentWindow(), gtx)
	}

	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx, body)
	}
	return pg.layoutDesktop(gtx, body)
}

func (pg *SignMessagePage) layoutDesktop(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return body(gtx)
}

func (pg *SignMessagePage) layoutMobile(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return components.UniformMobile(gtx, false, false, body)
}

func (pg *SignMessagePage) description() layout.Widget {
	return func(gtx C) D {
		desc := pg.Theme.Caption(values.String(values.StrEnterAddressToSign))
		desc.Color = pg.Theme.Color.GrayText2
		return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, desc.Layout)
	}
}

func (pg *SignMessagePage) editors(editor *cryptomaterial.Editor) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Bottom: values.MarginPadding15}.Layout(gtx, editor.Layout)
	}
}

func (pg *SignMessagePage) drawButtonsRow() layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							inset := layout.Inset{
								Right: values.MarginPadding5,
							}
							return inset.Layout(gtx, pg.clearButton.Layout)
						}),
						layout.Rigid(pg.signButton.Layout),
					)
				})
			}),
		)
	}
}

func (pg *SignMessagePage) drawResult() layout.Widget {
	return func(gtx C) D {
		if !uiUtils.StringNotEmpty(pg.signedMessageLabel.Text) {
			return D{}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				m := values.MarginPadding30
				return layout.Inset{Top: m, Bottom: m}.Layout(gtx, pg.Theme.Separator().Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx C) D {
						border := widget.Border{Color: pg.Theme.Color.Gray4, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
						wrapper := pg.Theme.Card()
						wrapper.Color = pg.Theme.Color.Gray4
						return border.Layout(gtx, func(gtx C) D {
							return wrapper.Layout(gtx, func(gtx C) D {
								return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
									return layout.Flex{}.Layout(gtx,
										layout.Flexed(0.9, pg.signedMessageLabel.Layout),
										layout.Flexed(0.1, func(gtx C) D {
											return layout.E.Layout(gtx, func(gtx C) D {
												return layout.Inset{Top: values.MarginPadding7}.Layout(gtx, func(gtx C) D {
													if pg.copySignature.Clicked(gtx) {
														gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(pg.signedMessageLabel.Text))})
														// clipboard.WriteOp{Text: pg.signedMessageLabel.Text}.Add(gtx.Ops)
														pg.Toast.Notify(values.String(values.StrSignCopied))
													}
													return pg.copySignature.Layout(gtx, pg.Theme.NewIcon(pg.Theme.Icons.CopyIcon).Layout24dp)
												})
											})
										}),
									)
								})
							})
						})
					}),
					layout.Stacked(func(gtx C) D {
						return layout.Inset{
							Top:  values.MarginPaddingMinus10,
							Left: values.MarginPadding10,
						}.Layout(gtx, func(gtx C) D {
							return pg.Theme.Card().Layout(gtx, func(gtx C) D {
								label := pg.Theme.Body1(values.String(values.StrSignature))
								label.Color = pg.Theme.Color.GrayText2
								return label.Layout(gtx)
							})
						})
					}),
				)
			}),
		)
	}
}

func (pg *SignMessagePage) updateButtonColors() {
	pg.isEnabled = false
	if uiUtils.StringNotEmpty(pg.addressEditor.Editor.Text()) ||
		uiUtils.StringNotEmpty(pg.messageEditor.Editor.Text()) {
		pg.clearButton.SetEnabled(true)
	} else {
		pg.clearButton.SetEnabled(false)
	}

	if !pg.isSigningMessage && pg.messageIsValid && pg.addressIsValid {
		pg.isEnabled = true
	}

	pg.signButton.SetEnabled(pg.isEnabled)
}

func (pg *SignMessagePage) handleEditorEvents(gtx C) {
	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(gtx, pg.addressEditor, pg.messageEditor)
	if isChanged {
		if gtx.Source.Focused(pg.addressEditor.Editor) {
			pg.validateAddress()
		}

		if gtx.Source.Focused(pg.messageEditor.Editor) {
			pg.validateMessage()
		}
	}

	for pg.clearButton.Clicked(gtx) {
		pg.clearForm()
	}

	if (pg.signButton.Clicked(gtx) || isSubmit) && pg.isEnabled {
		if !pg.isSigningMessage && pg.validate() {
			address := pg.addressEditor.Editor.Text()
			message := pg.messageEditor.Editor.Text()

			walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
				EnableName(false).
				EnableConfirmPassword(false).
				Title(values.String(values.StrConfirmToSign)).
				SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
					sig, err := pg.wallet.SignMessage(password, address, message)
					if err != nil {
						pm.SetError(err.Error())
						return false
					}

					pm.Dismiss()
					pg.signedMessageLabel.Text = utils.EncodeBase64(sig)
					return true
				})
			pg.ParentWindow().ShowModal(walletPasswordModal)
		}
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *SignMessagePage) HandleUserInteractions(_ C) {
	pg.updateButtonColors()
}

// KeysToHandle returns a Filter's slice that describes a set of key combinations
// that this modal wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *SignMessagePage) KeysToHandle() []event.Filter {
	return []event.Filter{key.FocusFilter{Target: pg}, key.Filter{Focus: pg, Name: key.NameTab, Optional: key.ModShift}}
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *SignMessagePage) HandleKeyPress(gtx C, evt *key.Event) {
	// Switch editors when tab key is pressed.
	cryptomaterial.SwitchEditors(gtx, evt, pg.addressEditor.Editor, pg.messageEditor.Editor)
}

func (pg *SignMessagePage) validate() bool {
	if !pg.validateAddress() || !pg.validateMessage() {
		return false
	}
	return true
}

func (pg *SignMessagePage) validateAddress() bool {
	address := pg.addressEditor.Editor.Text()
	pg.addressEditor.SetError("")

	var valid bool
	var errorMessage string

	switch {
	case !uiUtils.StringNotEmpty(address):
		errorMessage = values.String(values.StrEnterValidAddress)
	case !pg.wallet.IsAddressValid(address):
		errorMessage = values.String(values.StrInvalidAddress)
	case !pg.wallet.HaveAddress(address):
		errorMessage = values.String(values.StrAddrNotOwned)
	default:
		valid = true
	}
	if !valid {
		pg.addressEditor.SetError(errorMessage)
	}

	pg.addressIsValid = valid
	return valid
}

func (pg *SignMessagePage) validateMessage() bool {
	message := pg.messageEditor.Editor.Text()
	pg.messageEditor.SetError("")

	if !uiUtils.StringNotEmpty(message) {
		pg.messageEditor.SetError(values.String(values.StrEnterValidMsg))
		pg.messageIsValid = false
		return false
	}
	pg.messageIsValid = true
	return true
}

func (pg *SignMessagePage) clearForm() {
	pg.addressEditor.Editor.SetText("")
	pg.messageEditor.Editor.SetText("")
	pg.addressEditor.SetError("")
	pg.messageEditor.SetError("")
	pg.signedMessageLabel.Text = ""
	pg.errorLabel.Text = ""
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *SignMessagePage) OnNavigatedFrom() {}
