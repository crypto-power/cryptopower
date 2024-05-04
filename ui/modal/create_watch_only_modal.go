package modal

import (
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

type CreateWatchOnlyModal struct {
	*load.Load
	*cryptomaterial.Modal

	materialLoader material.LoaderStyle

	walletName     cryptomaterial.Editor
	extendedPubKey cryptomaterial.Editor

	btnPositve  cryptomaterial.Button
	btnNegative cryptomaterial.Button

	serverError string

	isLoading         bool
	isCancelable      bool
	walletNameEnabled bool
	isEnabled         bool

	callback func(walletName, extPubKey string, m *CreateWatchOnlyModal) bool // return true to dismiss dialog
}

func NewCreateWatchOnlyModal(l *load.Load) *CreateWatchOnlyModal {
	cm := &CreateWatchOnlyModal{
		Load:         l,
		Modal:        l.Theme.ModalFloatTitle("create_watch_only_modal", l.IsMobileView()),
		btnPositve:   l.Theme.Button(values.String(values.StrImport)),
		btnNegative:  l.Theme.OutlineButton(values.String(values.StrCancel)),
		isCancelable: true,
	}

	cm.btnPositve.Font.Weight = font.Medium

	cm.btnNegative.Font.Weight = font.Medium
	cm.btnNegative.Margin = layout.Inset{Right: values.MarginPadding8}

	cm.walletName = l.Theme.Editor(new(widget.Editor), values.String(values.StrWalletName))
	cm.walletName.Editor.SingleLine, cm.walletName.Editor.Submit = true, true

	cm.extendedPubKey = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrExtendedPubKey))
	cm.extendedPubKey.Editor.Submit = true

	cm.materialLoader = material.Loader(l.Theme.Base)

	return cm
}

func (cm *CreateWatchOnlyModal) OnResume() {
	if cm.walletNameEnabled {
		cm.walletName.Editor.Focus()
	} else {
		cm.extendedPubKey.Editor.Focus()
	}
}

func (cm *CreateWatchOnlyModal) OnDismiss() {}

func (cm *CreateWatchOnlyModal) EnableName(enable bool) *CreateWatchOnlyModal {
	cm.walletNameEnabled = enable
	return cm
}

func (cm *CreateWatchOnlyModal) setLoading(loading bool) {
	cm.isLoading = loading
	cm.Modal.SetDisabled(loading)
}

func (cm *CreateWatchOnlyModal) SetCancelable(min bool) *CreateWatchOnlyModal {
	cm.isCancelable = min
	return cm
}

func (cm *CreateWatchOnlyModal) SetError(err string) {
	cm.serverError = values.TranslateErr(err)
}

func (cm *CreateWatchOnlyModal) WatchOnlyCreated(callback func(walletName, extPubKey string, m *CreateWatchOnlyModal) bool) *CreateWatchOnlyModal {
	cm.callback = callback
	return cm
}

func (cm *CreateWatchOnlyModal) Handle() {
	if utils.EditorsNotEmpty(cm.walletName.Editor) ||
		utils.EditorsNotEmpty(cm.extendedPubKey.Editor) {
		cm.btnPositve.Background = cm.Theme.Color.Primary
		cm.isEnabled = true
	} else {
		cm.btnPositve.Background = cm.Theme.Color.Gray3
		cm.isEnabled = false
	}

	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(cm.walletName.Editor, cm.extendedPubKey.Editor)
	if isChanged {
		// reset editor errors
		cm.serverError = ""
		cm.walletName.SetError("")
		cm.extendedPubKey.SetError("")
	}

	for (cm.btnPositve.Clicked() || isSubmit) && cm.isEnabled {
		if cm.walletNameEnabled {
			if !utils.EditorsNotEmpty(cm.walletName.Editor) {
				cm.walletName.SetError(values.String(values.StrEnterWalletName))
				return
			}
		}

		if !utils.EditorsNotEmpty(cm.extendedPubKey.Editor) {
			cm.extendedPubKey.SetError(values.String(values.StrEnterExtendedPubKey))
			return
		}

		// Check if there are existing wallets with identical Xpub.
		// matchedWalletID == ID of the wallet whose xpub is identical to provided xpub.
		matchedWalletID, err := cm.AssetsManager.DCRWalletWithXPub(cm.extendedPubKey.Editor.Text())
		if err != nil {
			log.Errorf("Error checking xpub: %v", err)
			cm.SetError(values.StringF(values.StrXpubKeyErr, err))
			return
		}

		if matchedWalletID != -1 {
			cm.SetError(values.String(values.StrXpubWalletExist))
			return
		}

		cm.setLoading(true)
		go func() {
			if cm.callback(cm.walletName.Editor.Text(), cm.extendedPubKey.Editor.Text(), cm) {
				cm.Dismiss()
				return
			}
			cm.setLoading(false)
		}()
	}

	cm.btnNegative.SetEnabled(!cm.isLoading)
	if cm.btnNegative.Clicked() {
		if !cm.isLoading {
			cm.Dismiss()
		}
	}

	if cm.Modal.BackdropClicked(cm.isCancelable) {
		if !cm.isLoading {
			cm.Dismiss()
		}
	}
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this modal wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (cm *CreateWatchOnlyModal) KeysToHandle() key.Set {
	if !cm.walletNameEnabled {
		return ""
	}
	return cryptomaterial.AnyKeyWithOptionalModifier(key.ModShift, key.NameTab)
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (cm *CreateWatchOnlyModal) HandleKeyPress(evt *key.Event) {
	if cm.walletNameEnabled {
		cryptomaterial.SwitchEditors(evt, cm.walletName.Editor, cm.extendedPubKey.Editor)
	}
}

func (cm *CreateWatchOnlyModal) Layout(gtx layout.Context) D {
	w := []layout.Widget{
		func(gtx C) D {
			t := cm.Theme.H6(values.String(values.StrImportWatchingOnlyWallet))
			t.Font.Weight = font.SemiBold
			return t.Layout(gtx)
		},
		func(gtx C) D {
			if cm.serverError != "" {
				// set wallet name editor error if wallet name already exist
				if cm.serverError == libutils.ErrExist && cm.walletNameEnabled {
					cm.walletName.SetError(values.StringF(values.StrWalletExist, cm.walletName.Editor.Text()))
				} else {
					cm.extendedPubKey.SetError(cm.serverError)
				}
			}
			if cm.walletNameEnabled {
				return cm.walletName.Layout(gtx)
			}
			return D{}
		},
		func(gtx C) D {
			return cm.extendedPubKey.Layout(gtx)
		},
		func(gtx C) D {
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
						return cm.btnPositve.Layout(gtx)
					}),
				)
			})
		},
	}

	return cm.Modal.Layout(gtx, w)
}
