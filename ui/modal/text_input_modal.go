package modal

import (
	"image/color"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/renderers"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

type TextInputModal struct {
	*InfoModal

	isLoading           bool
	showAccountWarnInfo bool
	isCancelable        bool

	textInput cryptomaterial.Editor
	callback  func(string, *TextInputModal) bool

	textCustomTemplate []layout.Widget
}

func NewTextInputModal(l *load.Load) *TextInputModal {
	tm := &TextInputModal{
		isCancelable: true,
	}
	tm.InfoModal = newInfoModalWithKey(l, "text_input_modal", InfoBtn, tm.firstLoad)
	tm.btnNegative = l.Theme.OutlineButton(values.String(values.StrCancel))

	tm.textInput = l.Theme.Editor(new(widget.Editor), values.String(values.StrHint))
	tm.textInput.Editor.SingleLine, tm.textInput.Editor.Submit = true, true

	// Set the default click functions
	tm.callback = func(string, *TextInputModal) bool { return true }

	return tm
}

func (tm *TextInputModal) OnResume() {
	// set the positive button state
	tm.btnPositive.SetEnabled(utils.EditorsNotEmpty(tm.textInput.Editor))
}

func (tm *TextInputModal) firstLoad(gtx C) {
	gtx.Execute(key.FocusCmd{Tag: &tm.textInput.Editor})
}

func (tm *TextInputModal) Hint(hint string) *TextInputModal {
	tm.textInput.Hint = hint
	return tm
}

func (tm *TextInputModal) setLoading(loading bool) {
	tm.isLoading = loading
	tm.Modal.SetDisabled(loading)
}

func (tm *TextInputModal) ShowAccountInfoTip(show bool) *TextInputModal {
	tm.showAccountWarnInfo = show
	return tm
}

func (tm *TextInputModal) SetPositiveButtonCallback(callback func(string, *TextInputModal) bool) *TextInputModal {
	tm.callback = callback
	return tm
}

func (tm *TextInputModal) PositiveButtonStyle(background, text color.NRGBA) *TextInputModal {
	tm.btnPositive.Background, tm.btnPositive.Color = background, text
	return tm
}

func (tm *TextInputModal) SetError(err string) {
	if err == "" {
		tm.textInput.ClearError()
	} else {
		tm.textInput.SetError(values.TranslateErr(err))
	}
}

func (tm *TextInputModal) SetCancelable(min bool) *TextInputModal {
	tm.isCancelable = min
	return tm
}

func (tm *TextInputModal) SetTextWithTemplate(template string, walletName ...string /*optional parameter*/) *TextInputModal {
	switch template {
	case AllowUnmixedSpendingTemplate:
		tm.textCustomTemplate = allowUnspendUnmixedAcct(tm.Load)
	case RemoveWalletInfoTemplate:
		var walletNameStr string
		if walletName != nil {
			walletNameStr = walletName[0]
		}
		tm.textCustomTemplate = removeWalletInfo(tm.Load, walletNameStr)
	case SetGapLimitTemplate:
		tm.textCustomTemplate = setGapLimitText(tm.Load)
	}
	return tm
}

func (tm *TextInputModal) Handle(gtx C) {
	// set the positive button state
	tm.btnPositive.SetEnabled(utils.EditorsNotEmpty(tm.textInput.Editor))

	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(gtx, &tm.textInput)
	if isChanged {
		tm.textInput.SetError("")
	}

	if tm.btnPositive.Clicked(gtx) || isSubmit {
		if tm.isLoading {
			return
		}

		tm.setLoading(true)
		tm.SetError("")
		go func() {
			if tm.callback(tm.textInput.Editor.Text(), tm) {
				tm.Dismiss()
				return
			}
			tm.setLoading(false)
		}()
	}

	if tm.btnNegative.Clicked(gtx) {
		if !tm.isLoading {
			tm.Dismiss()
			tm.negativeButtonClicked()
		}
	}

	if tm.Modal.BackdropClicked(gtx, tm.isCancelable) {
		if !tm.isLoading {
			tm.Dismiss()
			tm.negativeButtonClicked()
		}
	}
}

func (tm *TextInputModal) Layout(gtx layout.Context) D {

	var w []layout.Widget

	if tm.dialogTitle != "" {
		w = append(w, tm.titleLayout())
	}

	if tm.showAccountWarnInfo {
		l := func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					img := cryptomaterial.NewIcon(tm.Theme.Icons.ActionInfo)
					img.Color = tm.Theme.Color.Gray1
					inset := layout.Inset{Right: values.MarginPadding4}
					return inset.Layout(gtx, func(gtx C) D {
						return img.Layout(gtx, values.MarginPadding20)
					})
				}),
				layout.Rigid(func(gtx C) D {
					text := values.StringF(values.StrAddAcctWarn, `<span style="text-color: grayText1">`, `<span style="font-weight: bold">`, `</span>`, `</span>`)
					return renderers.RenderHTML(text, tm.Theme).Layout(gtx)
				}),
			)
		}
		w = append(w, l)
	}

	if tm.textCustomTemplate != nil {
		w = append(w, tm.textCustomTemplate...)
	}

	w = append(w, tm.textInput.Layout)

	if tm.btnNegative.Text != "" || tm.btnPositive.Text != "" {
		w = append(w, tm.actionButtonsLayout())
	}

	return tm.Modal.Layout(gtx, w)
}

// SetText replaces the content of this editor with the provided text.
func (tm *TextInputModal) SetText(text string) *TextInputModal {
	tm.textInput.Editor.SetText(text)
	return tm
}
