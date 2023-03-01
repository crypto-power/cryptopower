package preference

import (
	"sort"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type ListPreferenceModal struct {
	*load.Load
	*cryptomaterial.Modal

	optionsRadioGroup *widget.Enum

	btnSave      cryptomaterial.Button
	btnCancel    cryptomaterial.Button
	customWidget layout.Widget

	items           map[string]string //[key]str-key
	itemKeys        []string
	title           string
	preferenceKey   string
	defaultValue    string // str-key
	initialValue    string
	currentValue    string
	isWalletAccount bool

	updateButtonClicked func(string)
}

func NewListPreference(l *load.Load, preferenceKey, defaultValue string, items map[string]string) *ListPreferenceModal {
	// sort keys to keep order when refreshed
	sortedKeys := make([]string, 0)
	for k := range items {
		sortedKeys = append(sortedKeys, k)
	}

	sort.Slice(sortedKeys, func(i int, j int) bool { return sortedKeys[i] < sortedKeys[j] })

	lp := ListPreferenceModal{
		Load:          l,
		preferenceKey: preferenceKey,
		defaultValue:  defaultValue,

		btnSave:   l.Theme.Button(values.String(values.StrSave)),
		btnCancel: l.Theme.OutlineButton(values.String(values.StrCancel)),

		items:    items,
		itemKeys: sortedKeys,

		optionsRadioGroup: new(widget.Enum),
		Modal:             l.Theme.ModalFloatTitle("list_preference"),
	}

	lp.btnSave.Font.Weight = text.Medium
	lp.btnCancel.Font.Weight = text.Medium

	return &lp
}

func (lp *ListPreferenceModal) ReadPreferenceKeyedValue() string {
	switch lp.preferenceKey {
	case sharedW.CurrencyConversionConfigKey:
		return lp.WL.AssetsManager.GetCurrencyConversionExchange()
	case sharedW.LanguagePreferenceKey:
		return lp.WL.AssetsManager.GetLanguagePreference()
	case sharedW.LogLevelConfigKey:
		return lp.WL.AssetsManager.GetLogLevels()
	default:
		return ""
	}
}

func (lp *ListPreferenceModal) SavePreferenceKeyedValue() {
	val := lp.optionsRadioGroup.Value
	switch lp.preferenceKey {
	case sharedW.CurrencyConversionConfigKey:
		lp.WL.AssetsManager.SetCurrencyConversionExchange(val)
	case sharedW.LanguagePreferenceKey:
		lp.WL.AssetsManager.SetLanguagePreference(val)
	case sharedW.LogLevelConfigKey:
		lp.WL.AssetsManager.SetLogLevels(val)
	}
}

func (lp *ListPreferenceModal) OnResume() {
	initialValue := lp.ReadPreferenceKeyedValue()
	if initialValue == "" {
		initialValue = lp.defaultValue
	}

	lp.initialValue = initialValue
	lp.currentValue = initialValue

	lp.optionsRadioGroup.Value = lp.currentValue
}

func (lp *ListPreferenceModal) OnDismiss() {}

func (lp *ListPreferenceModal) Title(title string) *ListPreferenceModal {
	lp.title = title
	return lp
}

func (lp *ListPreferenceModal) UseCustomWidget(layout layout.Widget) *ListPreferenceModal {
	lp.customWidget = layout
	return lp
}

func (lp *ListPreferenceModal) IsWallet(setAccount bool) *ListPreferenceModal {
	lp.isWalletAccount = setAccount
	return lp
}

func (lp *ListPreferenceModal) UpdateValues(clicked func(val string)) *ListPreferenceModal {
	lp.updateButtonClicked = clicked
	return lp
}

func (lp *ListPreferenceModal) Handle() {
	for lp.btnSave.Button.Clicked() {
		lp.currentValue = lp.optionsRadioGroup.Value
		lp.SavePreferenceKeyedValue()
		lp.updateButtonClicked(lp.optionsRadioGroup.Value)
		lp.RefreshTheme(lp.ParentWindow())
		lp.Dismiss()
	}

	for lp.btnCancel.Button.Clicked() {
		lp.Modal.Dismiss()
	}

	if lp.Modal.BackdropClicked(true) {
		lp.Modal.Dismiss()
	}
}

func (lp *ListPreferenceModal) Layout(gtx C) D {
	var w []layout.Widget

	title := func(gtx C) D {
		txt := lp.Theme.H6(values.String(lp.title))
		txt.Color = lp.Theme.Color.Text
		return txt.Layout(gtx)
	}

	items := []layout.Widget{
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lp.layoutItems()...)
		},
		func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(lp.btnCancel.Layout),
					layout.Rigid(lp.btnSave.Layout),
				)
			})
		},
	}

	if lp.title != "" {
		w = append(w, title)
	}

	if lp.customWidget != nil {
		w = append(w, lp.customWidget)
	}

	for i := 0; i < len(items); i++ {
		w = append(w, items[i])
	}

	return lp.Modal.Layout(gtx, w)
}

func (lp *ListPreferenceModal) layoutItems() []layout.FlexChild {

	items := make([]layout.FlexChild, 0)
	for _, k := range lp.itemKeys {
		text := values.String(lp.items[k])
		if lp.isWalletAccount {
			text = lp.items[k]
		}

		radioItem := layout.Rigid(lp.Theme.RadioButton(lp.optionsRadioGroup, k, text, lp.Theme.Color.DeepBlue, lp.Theme.Color.Primary).Layout)
		items = append(items, radioItem)
	}

	return items
}
