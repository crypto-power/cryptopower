package modal

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

type InfoModal struct {
	*load.Load
	*cryptomaterial.Modal

	dialogIcon *cryptomaterial.Image

	dialogTitle    string
	subtitle       string
	customTemplate []layout.Widget
	customWidget   layout.Widget

	// positiveButtonText    string
	positiveButtonClicked ClickFunc
	btnPositive           cryptomaterial.Button
	btnPositiveWidth      unit.Dp

	// negativeButtonText    string
	negativeButtonClicked func()
	btnNegative           cryptomaterial.Button

	checkbox      cryptomaterial.CheckBoxStyle
	mustBeChecked bool

	titleAlignment, subtitleAlignment, btnAlignment layout.Direction
	materialLoader                                  material.LoaderStyle
	titleTextAlignment, subTileTextAlignment        text.Alignment

	isCancelable bool
	isLoading    bool
}

// ButtonType is the type of button in modal.
type ButtonType uint8

// ClickFunc defines the positive button click method signature.
// Adding the InfoModal parameter allow reference of the parent info modal
// qualities inside the positive button function call.
type ClickFunc func(isChecked bool, im *InfoModal) bool

const (
	// CustomBtn defines the bare metal custom modal button type.
	CustomBtn ButtonType = iota
	// DangerBtn defines the default danger modal button type
	DangerBtn
	// InfoBtn defines the default info modal button type
	InfoBtn
)

// NewCustomModal returns a modal that can be customized.
func NewCustomModal(l *load.Load) *InfoModal {
	return newInfoModalWithKey(l, "info_modal", InfoBtn, nil)
}

// NewSuccessModal returns the default success modal UI component.
func NewSuccessModal(l *load.Load, title string, clicked ClickFunc) *InfoModal {
	icon := l.Theme.Icons.SuccessIcon
	md := newModal(l, title, icon, clicked)
	md.SetContentAlignment(layout.Center, layout.Center, layout.Center)
	return md
}

// NewErrorModal returns the default error modal UI component.
func NewErrorModal(l *load.Load, title string, clicked ClickFunc) *InfoModal {
	icon := l.Theme.Icons.FailedIcon
	md := newModal(l, title, icon, clicked)
	md.SetContentAlignment(layout.Center, layout.Center, layout.Center)
	return md
}

// DefaultClickFunc returns the default click function satisfying the positive
// btn click function.
func DefaultClickFunc() ClickFunc {
	return func(_ bool, _ *InfoModal) bool {
		return true
	}
}

func newModal(l *load.Load, title string, icon *cryptomaterial.Image, clicked ClickFunc) *InfoModal {
	info := newInfoModalWithKey(l, "info_modal", InfoBtn, nil)
	info.positiveButtonClicked = clicked
	info.btnPositiveWidth = values.MarginPadding100
	info.dialogIcon = icon
	info.dialogTitle = title
	info.titleAlignment = layout.Center
	info.btnAlignment = layout.Center
	return info
}

func newInfoModalWithKey(l *load.Load, key string, btnPositiveType ButtonType, firstLoad func(gtx C)) *InfoModal {
	in := &InfoModal{
		Load:             l,
		Modal:            l.Theme.ModalFloatTitle(key, l.IsMobileView(), firstLoad),
		btnNegative:      l.Theme.OutlineButton(""),
		isCancelable:     true,
		isLoading:        false,
		btnAlignment:     layout.E,
		btnPositiveWidth: 0,
	}

	in.btnPositive = getPositiveButtonType(l, btnPositiveType)
	in.btnPositive.Font.Weight = font.Medium
	in.btnNegative.Font.Weight = font.Medium

	// Set the default click functions
	in.positiveButtonClicked = DefaultClickFunc()
	in.negativeButtonClicked = func() {}

	in.materialLoader = material.Loader(l.Theme.Base)

	return in
}

func getPositiveButtonType(l *load.Load, btnType ButtonType) cryptomaterial.Button {
	switch btnType {
	case InfoBtn:
		return l.Theme.Button(values.String(values.StrOk))
	case DangerBtn:
		return l.Theme.DangerButton(values.String(values.StrOk))
	default:
		return l.Theme.OutlineButton(values.String(values.StrOk))
	}
}

func (in *InfoModal) OnResume() {}

func (in *InfoModal) OnDismiss() {}

func (in *InfoModal) SetCancelable(min bool) *InfoModal {
	in.isCancelable = min
	return in
}

func (in *InfoModal) SetContentAlignment(title, subTitle, btn layout.Direction) *InfoModal {
	in.titleAlignment = title
	in.subtitleAlignment = subTitle
	switch title {
	case layout.Center:
		in.titleTextAlignment = text.Middle
	case layout.E:
		in.titleTextAlignment = text.End
	default:
		in.titleTextAlignment = text.Start
	}
	switch subTitle {
	case layout.Center:
		in.subTileTextAlignment = text.Middle
	case layout.E:
		in.subTileTextAlignment = text.End
	default:
		in.subTileTextAlignment = text.Start
	}
	in.btnAlignment = btn
	return in
}

func (in *InfoModal) Icon(icon *cryptomaterial.Image) *InfoModal {
	in.dialogIcon = icon
	return in
}

func (in *InfoModal) CheckBox(checkbox cryptomaterial.CheckBoxStyle, mustBeChecked bool) *InfoModal {
	in.checkbox = checkbox
	in.mustBeChecked = mustBeChecked // determine if the checkbox must be selected to proceed
	return in
}

func (in *InfoModal) setLoading(loading bool) {
	in.isLoading = loading
}

func (in *InfoModal) Title(title string) *InfoModal {
	in.dialogTitle = title
	return in
}

func (in *InfoModal) Body(subtitle string) *InfoModal {
	in.subtitle = subtitle
	return in
}

func (in *InfoModal) SetPositiveButtonText(text string) *InfoModal {
	in.btnPositive.Text = text
	return in
}

func (in *InfoModal) SetPositiveButtonCallback(clicked ClickFunc) *InfoModal {
	in.positiveButtonClicked = clicked
	return in
}

func (in *InfoModal) PositiveButtonStyle(background, text color.NRGBA) *InfoModal {
	in.btnPositive.Background, in.btnPositive.Color = background, text
	return in
}

func (in *InfoModal) PositiveButtonWidth(width unit.Dp) *InfoModal {
	in.btnPositiveWidth = width
	return in
}

func (in *InfoModal) SetNegativeButtonText(text string) *InfoModal {
	in.btnNegative.Text = text
	return in
}

func (in *InfoModal) SetNegativeButtonCallback(clicked func()) *InfoModal {
	in.negativeButtonClicked = clicked
	return in
}

func (in *InfoModal) NegativeButtonStyle(background, text color.NRGBA) *InfoModal {
	in.btnNegative.Background, in.btnNegative.Color = background, text
	return in
}

// for backwards compatibility.
func (in *InfoModal) SetupWithTemplate(template string) *InfoModal {
	title := in.dialogTitle
	subtitle := in.subtitle
	var customTemplate []layout.Widget
	switch template {
	case TransactionDetailsInfoTemplate:
		title = values.String(values.StrHowToCopy)
		customTemplate = transactionDetailsInfo(in.Theme)
	case SignMessageInfoTemplate:
		customTemplate = signMessageInfo(in.Theme)
	case VerifyMessageInfoTemplate:
		customTemplate = verifyMessageInfo(in.Theme)
	case PrivacyInfoTemplate:
		title = values.String(values.StrUseMixer)
		customTemplate = privacyInfo(in.Load)
	case SetupMixerInfoTemplate:
		customTemplate = setupMixerInfo(in.Theme)
	case WalletBackupInfoTemplate:
		customTemplate = backupInfo(in.Theme)
	case SecurityToolsInfoTemplate:
		customTemplate = securityToolsInfo(in.Theme)
	case SourceModalInfoTemplate:
		customTemplate = sourceModalInfo(in.Theme)
	case TotalValueInfoTemplate:
		customTemplate = totalValueInfo(in.Theme)
	case BondStrengthInfoTemplate:
		customTemplate = bondStrengthInfo(in.Theme)
	}

	in.dialogTitle = title
	in.subtitle = subtitle
	in.customTemplate = customTemplate
	return in
}

func (in *InfoModal) UseCustomWidget(layout layout.Widget) *InfoModal {
	in.customWidget = layout
	return in
}

// KeysToHandle returns a Filter's slice that describes a set of key combinations
// that this modal wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (in *InfoModal) KeysToHandle() []event.Filter {
	return []event.Filter{key.FocusFilter{Target: in},
		key.Filter{Focus: in, Name: key.NameReturn},
		key.Filter{Focus: in, Name: key.NameEnter},
		key.Filter{Focus: in, Name: key.NameEscape},
	}
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (in *InfoModal) HandleKeyPress(_ *key.Event) {
	in.btnPositive.Click()
	in.ParentWindow().Reload()
}

func (in *InfoModal) Handle(gtx C) {
	if in.btnPositive.Clicked(gtx) {
		if in.isLoading {
			return
		}
		isChecked := false
		if in.checkbox.CheckBox != nil {
			isChecked = in.checkbox.CheckBox.Value
		}

		in.setLoading(true)
		go func() {
			if in.positiveButtonClicked(isChecked, in) {
				in.Dismiss()
				return
			}
			in.setLoading(false)
		}()
	}

	if in.btnNegative.Clicked(gtx) {
		if !in.isLoading {
			in.Dismiss()
			in.negativeButtonClicked()
		}
	}

	if in.Modal.BackdropClicked(gtx, in.isCancelable) {
		if !in.isLoading {
			in.Dismiss()
			in.negativeButtonClicked()
		}
	}

	if in.checkbox.CheckBox != nil {
		if in.mustBeChecked {
			in.btnNegative.SetEnabled(in.checkbox.CheckBox.Value)
		}
	}
}

func (in *InfoModal) Layout(gtx layout.Context) D {
	icon := func(gtx C) D {
		if in.dialogIcon == nil {
			return D{}
		}

		return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D {
				return in.dialogIcon.LayoutSize(gtx, values.MarginPadding50)
			})
		})
	}

	checkbox := func(gtx C) D {
		if in.checkbox.CheckBox == nil {
			return D{}
		}

		return layout.Inset{Top: values.MarginPaddingMinus5, Left: values.MarginPaddingMinus5}.Layout(gtx, func(gtx C) D {
			in.checkbox.TextSize = values.TextSize14
			if in.IsMobileView() {
				in.checkbox.TextSize = values.TextSize12
			}
			in.checkbox.Color = in.Theme.Color.GrayText1
			in.checkbox.IconColor = in.Theme.Color.Gray2
			if in.checkbox.CheckBox.Value {
				in.checkbox.IconColor = in.Theme.Color.Primary
			}
			return in.checkbox.Layout(gtx)
		})
	}

	subtitle := func(gtx C) D {
		text := in.Theme.Body1(in.subtitle)
		text.Alignment = in.subTileTextAlignment
		text.Color = in.Theme.Color.GrayText2
		return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, text.Layout)
	}

	var w []layout.Widget

	// Every section of the dialog is optional
	if in.dialogIcon != nil {
		w = append(w, icon)
	}

	if in.dialogTitle != "" {
		w = append(w, in.titleLayout())
	}

	if in.subtitle != "" {
		w = append(w, subtitle)
	}

	if in.customTemplate != nil {
		w = append(w, in.customTemplate...)
	}

	if in.checkbox.CheckBox != nil {
		w = append(w, checkbox)
	}

	if in.customWidget != nil {
		w = append(w, in.customWidget)
	}

	if in.btnNegative.Text != "" || in.btnPositive.Text != "" {
		w = append(w, in.actionButtonsLayout())
	}

	return in.Modal.Layout(gtx, w)
}

func (in *InfoModal) titleLayout() layout.Widget {
	return func(gtx C) D {
		t := in.Theme.H6(in.dialogTitle)
		if in.IsMobileView() {
			t.TextSize = values.TextSize16
		}
		t.Alignment = in.titleTextAlignment
		t.Font.Weight = font.SemiBold
		return in.titleAlignment.Layout(gtx, t.Layout)
	}
}

func (in *InfoModal) actionButtonsLayout() layout.Widget {
	btnTextSize := values.TextSize16
	if in.IsMobileView() {
		btnTextSize = values.TextSize14
	}
	in.btnNegative.TextSize = btnTextSize
	in.btnPositive.TextSize = btnTextSize

	return func(gtx C) D {
		return in.btnAlignment.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if in.btnNegative.Text == "" || in.isLoading {
						return D{}
					}

					gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding250)
					return layout.Inset{Right: values.MarginPadding5}.Layout(gtx, in.btnNegative.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					if in.isLoading {
						return in.materialLoader.Layout(gtx)
					}

					if in.btnPositive.Text == "" {
						return D{}
					}

					gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding250)
					if in.btnPositiveWidth > 0 {
						gtx.Constraints.Min.X = gtx.Dp(in.btnPositiveWidth)
					}
					return in.btnPositive.Layout(gtx)
				}),
			)
		})
	}
}
