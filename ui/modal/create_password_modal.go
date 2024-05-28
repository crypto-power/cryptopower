package modal

import (
	"strconv"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

type CreatePasswordModal struct {
	*load.Load
	*cryptomaterial.Modal

	walletName            cryptomaterial.Editor
	passwordEditor        cryptomaterial.Editor
	confirmPasswordEditor cryptomaterial.Editor
	passwordStrength      cryptomaterial.ProgressBarStyle

	isLoading              bool
	isCancelable           bool
	walletNameEnabled      bool
	showWalletWarnInfo     bool
	confirmPasswordEnabled bool

	dialogTitle string
	serverError string
	description string

	parent app.Page

	materialLoader material.LoaderStyle

	customWidget layout.Widget

	// positiveButtonText string
	btnPositive cryptomaterial.Button
	// Returns true to dismiss dialog
	positiveButtonClicked func(walletName, password string, m *CreatePasswordModal) bool

	// negativeButtonText    string
	btnNegative           cryptomaterial.Button
	negativeButtonClicked func()
}

func NewCreatePasswordModal(l *load.Load) *CreatePasswordModal {
	cm := &CreatePasswordModal{
		Load:                   l,
		Modal:                  l.Theme.ModalFloatTitle("create_password_modal", l.IsMobileView()),
		passwordStrength:       l.Theme.ProgressBar(0),
		btnPositive:            l.Theme.Button(values.String(values.StrConfirm)),
		btnNegative:            l.Theme.OutlineButton(values.String(values.StrCancel)),
		isCancelable:           true,
		confirmPasswordEnabled: true,
	}

	cm.btnPositive.Font.Weight = font.Medium

	cm.btnNegative.Font.Weight = font.Medium
	cm.btnNegative.Margin = layout.Inset{Right: values.MarginPadding8}

	cm.walletName = l.Theme.Editor(new(widget.Editor), values.String(values.StrWalletName))
	cm.walletName.Editor.SingleLine, cm.walletName.Editor.Submit = true, true

	cm.passwordEditor = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrSpendingPassword))
	cm.passwordEditor.Editor.SingleLine, cm.passwordEditor.Editor.Submit = true, true

	cm.confirmPasswordEditor = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrConfirmSpendingPassword))
	cm.confirmPasswordEditor.Editor.SingleLine, cm.confirmPasswordEditor.Editor.Submit = true, true
	cm.confirmPasswordEditor.AllowSpaceError(true)

	// Set the default click functions
	cm.negativeButtonClicked = func() {}
	cm.positiveButtonClicked = func(walletName, password string, m *CreatePasswordModal) bool { return true }

	cm.materialLoader = material.Loader(l.Theme.Base)

	return cm
}

func (cm *CreatePasswordModal) OnResume() {
	if cm.walletNameEnabled {
		cm.walletName.Editor.Focus()
	} else {
		cm.passwordEditor.Editor.Focus()
	}

	cm.btnPositive.SetEnabled(cm.validToCreate())
}

func (cm *CreatePasswordModal) OnDismiss() {}

func (cm *CreatePasswordModal) Title(title string) *CreatePasswordModal {
	cm.dialogTitle = title
	return cm
}

func (cm *CreatePasswordModal) EnableName(enable bool) *CreatePasswordModal {
	cm.walletNameEnabled = enable
	return cm
}

func (cm *CreatePasswordModal) EnableConfirmPassword(enable bool) *CreatePasswordModal {
	cm.confirmPasswordEnabled = enable
	return cm
}

func (cm *CreatePasswordModal) NameHint(hint string) *CreatePasswordModal {
	cm.walletName.Hint = hint
	return cm
}

func (cm *CreatePasswordModal) PasswordHint(hint string) *CreatePasswordModal {
	cm.passwordEditor.Hint = hint
	return cm
}

func (cm *CreatePasswordModal) ConfirmPasswordHint(hint string) *CreatePasswordModal {
	cm.confirmPasswordEditor.Hint = hint
	return cm
}

func (cm *CreatePasswordModal) ShowWalletInfoTip(show bool) *CreatePasswordModal {
	cm.showWalletWarnInfo = show
	return cm
}

func (cm *CreatePasswordModal) SetPositiveButtonText(text string) *CreatePasswordModal {
	cm.btnPositive.Text = text
	return cm
}

func (cm *CreatePasswordModal) SetPositiveButtonCallback(callback func(walletName, password string, m *CreatePasswordModal) bool) *CreatePasswordModal {
	cm.positiveButtonClicked = callback
	return cm
}

func (cm *CreatePasswordModal) SetNegativeButtonText(text string) *CreatePasswordModal {
	cm.btnNegative.Text = text
	return cm
}

func (cm *CreatePasswordModal) SetNegativeButtonCallback(callback func()) *CreatePasswordModal {
	cm.negativeButtonClicked = callback
	return cm
}

func (cm *CreatePasswordModal) setLoading(loading bool) {
	cm.isLoading = loading
	cm.Modal.SetDisabled(loading)
}

func (cm *CreatePasswordModal) SetCancelable(min bool) *CreatePasswordModal {
	cm.isCancelable = min
	return cm
}

func (cm *CreatePasswordModal) SetDescription(description string) *CreatePasswordModal {
	cm.description = description
	return cm
}

func (cm *CreatePasswordModal) SetError(err string) {
	cm.serverError = values.TranslateErr(err)
}

func (cm *CreatePasswordModal) SetPasswordTitleVisibility(show bool) {
	cm.passwordEditor.IsTitleLabel = show
}

func (cm *CreatePasswordModal) UseCustomWidget(layout layout.Widget) *CreatePasswordModal {
	cm.customWidget = layout
	return cm
}

func (cm *CreatePasswordModal) validToCreate() bool {
	nameValid := true
	if cm.walletNameEnabled {
		nameValid = utils.EditorsNotEmpty(cm.walletName.Editor)
	}

	validPassword, passwordsMatch := true, true
	if cm.confirmPasswordEnabled {
		validPassword = utils.EditorsNotEmpty(cm.confirmPasswordEditor.Editor)
		if len(cm.confirmPasswordEditor.Editor.Text()) > 0 {
			passwordsMatch = cm.passwordsMatch(cm.passwordEditor.Editor, cm.confirmPasswordEditor.Editor)
		}
	}

	return nameValid && utils.EditorsNotEmpty(cm.passwordEditor.Editor) && validPassword && passwordsMatch
}

// SetParent sets the page that created PasswordModal as it's parent.
func (cm *CreatePasswordModal) SetParent(parent app.Page) *CreatePasswordModal {
	cm.parent = parent
	return cm
}

func (cm *CreatePasswordModal) Handle() {
	cm.btnPositive.SetEnabled(cm.validToCreate())

	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(cm.passwordEditor.Editor, cm.confirmPasswordEditor.Editor, cm.walletName.Editor)
	if isChanged {
		// reset all modal errors when any editor is modified
		cm.serverError = ""
		cm.walletName.SetError("")
		cm.passwordEditor.SetError("")
		cm.confirmPasswordEditor.SetError("")
	}

	if cm.btnPositive.Clicked() || isSubmit {
		if cm.walletNameEnabled {
			if !utils.EditorsNotEmpty(cm.walletName.Editor) {
				cm.walletName.SetError(values.String(values.StrEnterWalletName))
				return
			}
		}

		if !utils.EditorsNotEmpty(cm.passwordEditor.Editor) {
			cm.passwordEditor.SetError(values.String(values.StrEnterSpendingPassword))
			return
		}

		if cm.confirmPasswordEnabled {
			if !utils.EditorsNotEmpty(cm.confirmPasswordEditor.Editor) {
				cm.confirmPasswordEditor.SetError(values.String(values.StrConfirmSpendingPassword))
				return
			}
		}
		cm.setLoading(true)
		go func() {
			if cm.positiveButtonClicked(cm.walletName.Editor.Text(), cm.passwordEditor.Editor.Text(), cm) {
				cm.Dismiss()
				return
			}
			cm.setLoading(false)
		}()
	}

	cm.btnNegative.SetEnabled(!cm.isLoading)
	if cm.btnNegative.Clicked() {
		if !cm.isLoading {
			if cm.parent != nil {
				cm.parent.OnNavigatedTo()
			}
			cm.negativeButtonClicked()
			cm.Dismiss()
		}
	}

	if cm.Modal.BackdropClicked(cm.isCancelable) {
		if !cm.isLoading {
			cm.Dismiss()
		}
	}

	if cm.confirmPasswordEnabled {
		utils.ComputePasswordStrength(&cm.passwordStrength, cm.Theme, cm.passwordEditor.Editor)
	}
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this modal wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (cm *CreatePasswordModal) KeysToHandle() key.Set {
	return cryptomaterial.AnyKeyWithOptionalModifier(key.ModShift, key.NameTab)
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (cm *CreatePasswordModal) HandleKeyPress(evt *key.Event) {
	if cm.walletNameEnabled {
		if cm.confirmPasswordEnabled {
			cryptomaterial.SwitchEditors(evt, cm.walletName.Editor, cm.passwordEditor.Editor, cm.confirmPasswordEditor.Editor)
		} else {
			cryptomaterial.SwitchEditors(evt, cm.walletName.Editor, cm.passwordEditor.Editor)
		}
	} else {
		cryptomaterial.SwitchEditors(evt, cm.passwordEditor.Editor, cm.confirmPasswordEditor.Editor)
	}
}

func (cm *CreatePasswordModal) passwordsMatch(editors ...*widget.Editor) bool {
	if len(editors) < 2 {
		return false
	}

	password := editors[0]
	matching := editors[1]

	if password.Text() != matching.Text() {
		cm.confirmPasswordEditor.SetError(values.String(values.StrPasswordNotMatch))
		return false
	}

	cm.confirmPasswordEditor.SetError("")
	return true
}

func (cm *CreatePasswordModal) titleLayout() layout.Widget {
	return func(gtx C) D {
		t := cm.Theme.H6(cm.dialogTitle)
		if cm.IsMobileView() {
			t.TextSize = values.TextSize16
		}
		t.Font.Weight = font.SemiBold
		return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, t.Layout)
	}
}

func (cm *CreatePasswordModal) descriptionLayout() layout.Widget {
	return func(gtx C) D {
		desc := cm.Theme.Label(values.TextSizeTransform(cm.IsMobileView(), values.TextSize16), cm.description)
		return layout.Inset{Bottom: values.MarginPadding5}.Layout(gtx, desc.Layout)
	}
}

func (cm *CreatePasswordModal) Layout(gtx C) D {
	return cm.Modal.Layout(gtx, cm.LayoutComponents())
}

func (cm *CreatePasswordModal) LayoutComponents() []layout.Widget {
	btnTextSize := values.TextSize16
	if cm.IsMobileView() {
		btnTextSize = values.TextSize14
	}
	cm.btnNegative.TextSize = btnTextSize
	cm.btnPositive.TextSize = btnTextSize

	w := []layout.Widget{}

	if cm.dialogTitle != "" {
		w = append(w, cm.titleLayout())
	}

	if cm.description != "" {
		w = append(w, cm.descriptionLayout())
	}

	if cm.customWidget != nil {
		w = append(w, cm.customWidget)
	}

	if cm.serverError != "" {
		// set wallet name editor error if wallet name already exist
		if cm.serverError == libutils.ErrExist && cm.walletNameEnabled {
			cm.walletName.SetError(values.StringF(values.StrWalletExist, cm.walletName.Editor.Text()))
		} else if !utils.ValidateLengthName(cm.walletName.Editor.Text()) && cm.walletNameEnabled {
			cm.walletName.SetError(values.String(values.StrWalletNameLengthError))
		} else {
			t := cm.Theme.Body2(cm.serverError)
			t.Color = cm.Theme.Color.Danger
			w = append(w, t.Layout)
		}
	}

	if cm.walletNameEnabled {
		w = append(w, cm.walletName.Layout)
	}

	w = append(w, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(cm.passwordEditor.Layout),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding20, Right: values.MarginPadding20}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							if cm.showWalletWarnInfo {
								txt := cm.Theme.Label(values.TextSize12, values.String(values.StrSpendingPasswordInfo2))
								txt.Color = cm.Theme.Color.GrayText1
								return txt.Layout(gtx)
							}
							return D{}
						}),
						layout.Rigid(func(gtx C) D {
							txt := cm.Theme.Label(values.TextSize12, strconv.Itoa(cm.passwordEditor.Editor.Len()))
							txt.Color = cm.Theme.Color.GrayText1

							if txt.Text != "0" {
								return layout.E.Layout(gtx, txt.Layout)
							}
							return D{}
						}),
					)
				})
			}),
		)
	})

	if cm.confirmPasswordEnabled {
		w = append(w, cm.passwordStrength.Layout)
		w = append(w, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(cm.confirmPasswordEditor.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Right: values.MarginPadding20}.Layout(gtx, func(gtx C) D {
						txt := cm.Theme.Label(values.TextSize12, strconv.Itoa(cm.confirmPasswordEditor.Editor.Len()))
						txt.Color = cm.Theme.Color.GrayText1
						if txt.Text != "0" {
							return layout.E.Layout(gtx, txt.Layout)
						}

						return D{}
					})
				}),
			)
		})
	}

	w = append(w, func(gtx C) D {
		return layout.E.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if cm.isLoading {
						return D{}
					}

					return cm.btnNegative.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					if cm.isLoading {
						return cm.materialLoader.Layout(gtx)
					}

					return cm.btnPositive.Layout(gtx)
				}),
			)
		})
	})

	return w
}
