package preference

import (
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/crypto-power/cryptopower/ui/values/localizable"
)

const (
	binanceProhibitedCountries = "https://www.binance.com/en/legal/list-of-prohibited-countries"
	bittrexProhibitedCountries = "https://bittrex.zendesk.com/hc/en-us/articles/360034965072-Important-information-for-Bittrex-customers"
	kucoinProhibitedCountries  = "https://www.kucoin.com/vi/legal/terms-of-use"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var (
	NetworkTypes = []ItemPreference{
		{Key: string(libutils.Mainnet), Value: libutils.Mainnet.Display()},
		{Key: string(libutils.Testnet), Value: libutils.Testnet.Display()},
	}

	// ExchOptions holds the configurable options for exchange servers.
	ExchOptions = []ItemPreference{
		{Key: values.BinanceExchange, Value: values.StrUsdBinance, Warning: values.String(values.StrRateBinanceWarning), WarningLink: binanceProhibitedCountries},
		{Key: values.BinanceUSExchange, Value: values.StrUsdBinanceUS},
		{Key: values.BittrexExchange, Value: values.StrUsdBittrex, Warning: values.String(values.StrRateBittrexWarning), WarningLink: bittrexProhibitedCountries},
		{Key: values.Coinpaprika, Value: values.StrUsdCoinpaprika},
		{Key: values.Messari, Value: values.StrUsdMessari},
		{Key: values.KucoinExchange, Value: values.StrUsdKucoin, Warning: values.String(values.StrRateKucoinWarning), WarningLink: kucoinProhibitedCountries},
		{Key: values.DefaultExchangeValue, Value: values.StrNone},
	}

	// LangOptions stores the configurable language options.
	LangOptions = []ItemPreference{
		{Key: localizable.ENGLISH, Value: values.StrEnglish},
		{Key: localizable.FRENCH, Value: values.StrFrench},
		{Key: localizable.SPANISH, Value: values.StrSpanish},
	}

	// LogOptions are the selectable debug levels.
	LogOptions = []ItemPreference{
		{Key: libutils.LogLevelTrace, Value: values.StrLogLevelTrace},
		{Key: libutils.LogLevelDebug, Value: values.StrLogLevelDebug},
		{Key: libutils.LogLevelInfo, Value: values.StrLogLevelInfo},
		{Key: libutils.LogLevelWarn, Value: values.StrLogLevelWarn},
		{Key: libutils.LogLevelError, Value: values.StrLogLevelError},
		{Key: libutils.LogLevelCritical, Value: values.StrLogLevelCritical},
	}
)

type ListPreferenceModal struct {
	*load.Load
	*cryptomaterial.Modal

	optionsRadioGroup *widget.Enum

	btnSave      cryptomaterial.Button
	btnCancel    cryptomaterial.Button
	customWidget layout.Widget

	title           string
	preferenceKey   string
	defaultValue    string // str-key
	initialValue    string
	currentValue    string
	isWalletAccount bool
	preferenceItems []ItemPreference

	updateButtonClicked func(string)

	// use for warning link
	viewWarningAction *cryptomaterial.Clickable
	copyRedirectURL   *cryptomaterial.Clickable
	redirectIcon      *cryptomaterial.Image
}

// ItemPreference models the options shown by the list
// preference modal.
type ItemPreference struct {
	Key   string // option's key
	Value string // option's value

	// use when need to show warning for option
	Warning     string // option's value
	WarningLink string // option's value
}

func NewListPreference(l *load.Load, preferenceKey, defaultValue string, items []ItemPreference) *ListPreferenceModal {
	lp := ListPreferenceModal{
		Load:          l,
		preferenceKey: preferenceKey,
		defaultValue:  defaultValue,

		btnSave:   l.Theme.Button(values.String(values.StrSave)),
		btnCancel: l.Theme.OutlineButton(values.String(values.StrCancel)),

		preferenceItems:   items,
		optionsRadioGroup: new(widget.Enum),
		Modal:             l.Theme.ModalFloatTitle("list_preference", l.IsMobileView()),
		redirectIcon:      l.Theme.Icons.RedirectIcon,
		viewWarningAction: l.Theme.NewClickable(true),
		copyRedirectURL:   l.Theme.NewClickable(false),
	}

	lp.btnSave.Font.Weight = font.Medium
	lp.btnCancel.Font.Weight = font.Medium

	return &lp
}

func (lp *ListPreferenceModal) ReadPreferenceKeyedValue() string {
	switch lp.preferenceKey {
	case sharedW.CurrencyConversionConfigKey:
		return lp.AssetsManager.GetCurrencyConversionExchange()
	case sharedW.LanguagePreferenceKey:
		return lp.AssetsManager.GetLanguagePreference()
	case sharedW.LogLevelConfigKey:
		return lp.AssetsManager.GetLogLevels()
	default:
		return ""
	}
}

func (lp *ListPreferenceModal) SavePreferenceKeyedValue() {
	val := lp.optionsRadioGroup.Value
	switch lp.preferenceKey {
	case sharedW.CurrencyConversionConfigKey:
		lp.AssetsManager.SetCurrencyConversionExchange(val)
	case sharedW.LanguagePreferenceKey:
		// TODO: We should be able to update dex core's language when the user
		// changes language.
		lp.AssetsManager.SetLanguagePreference(val)
	case sharedW.LogLevelConfigKey:
		lp.AssetsManager.SetLogLevels(val)
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

func (lp *ListPreferenceModal) Handle(gtx C) {
	for lp.btnSave.Button.Clicked(gtx) {
		lp.currentValue = lp.optionsRadioGroup.Value
		lp.SavePreferenceKeyedValue()
		lp.updateButtonClicked(lp.optionsRadioGroup.Value)
		lp.RefreshTheme(lp.ParentWindow())
		lp.Dismiss()
	}

	for lp.btnCancel.Button.Clicked(gtx) {
		lp.Modal.Dismiss()
	}

	if lp.Modal.BackdropClicked(gtx, true) {
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
					layout.Rigid(layout.Spacer{Width: values.MarginPadding5}.Layout),
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

	for lp.viewWarningAction.Clicked(gtx) {
		host := ""
		currentValue := lp.optionsRadioGroup.Value
		for _, v := range lp.preferenceItems {
			if currentValue == v.Key {
				host = v.WarningLink
			}
		}
		info := modal.NewCustomModal(lp.Load).
			Title(values.String(values.StrRestrictedDetail)).
			Body(values.String(values.StrCopyLink)).
			SetCancelable(true).
			UseCustomWidget(func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx C) D {
						border := widget.Border{Color: lp.Theme.Color.Gray4, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
						wrapper := lp.Theme.Card()
						wrapper.Color = lp.Theme.Color.Gray4
						return border.Layout(gtx, func(gtx C) D {
							return wrapper.Layout(gtx, func(gtx C) D {
								return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
									return layout.Flex{}.Layout(gtx,
										layout.Flexed(0.9, lp.Theme.Body1(host).Layout),
										layout.Flexed(0.1, func(gtx C) D {
											return layout.E.Layout(gtx, func(gtx C) D {
												if lp.copyRedirectURL.Clicked(gtx) {
													// clipboard.WriteOp{Text: host}.Add(gtx.Ops)
													gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(host))})
													lp.Toast.Notify(values.String(values.StrCopied))
												}
												return lp.copyRedirectURL.Layout(gtx, lp.Theme.Icons.CopyIcon.Layout24dp)
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
							label := lp.Theme.Body2(values.String(values.StrWebURL))
							label.Color = lp.Theme.Color.GrayText2
							return label.Layout(gtx)
						})
					}),
				)
			}).
			SetPositiveButtonText(values.String(values.StrGotIt))
		lp.ParentWindow().ShowModal(info)
	}

	return lp.Modal.Layout(gtx, w)
}

func (lp *ListPreferenceModal) warningLayout(gtx C, text string) D {
	return lp.viewWarningAction.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = gtx.Constraints.Max.X - 60
				lbl := lp.Theme.Body2(text)
				lbl.Color = lp.Theme.Color.Warning
				lbl.Font.Style = font.Italic
				return lbl.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Right: values.MarginPadding10,
				}.Layout(gtx, lp.redirectIcon.Layout24dp)
			}),
		)
	})
}

func (lp *ListPreferenceModal) layoutItems() []layout.FlexChild {
	items := make([]layout.FlexChild, 0)
	warningText := ""
	currentValue := lp.optionsRadioGroup.Value
	for _, v := range lp.preferenceItems {
		text := values.String(v.Value)
		if lp.isWalletAccount {
			text = v.Value
		}

		if currentValue == v.Key {
			warningText = v.Warning
		}

		radioItem := layout.Rigid(lp.Theme.RadioButton(lp.optionsRadioGroup, v.Key, text, lp.Theme.Color.DeepBlue, lp.Theme.Color.Primary).Layout)
		items = append(items, radioItem)
	}
	if warningText != "" {
		warningChild := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return lp.warningLayout(gtx, warningText)
		})

		items = append(items, warningChild)
	}

	return items
}

// GetKeyValue return the value for a key within a set of prefence options.
// The key is case sensitive, `Key` != `key`.
// Returns the empty string if the key is not found.
func GetKeyValue(key string, options []ItemPreference) string {
	for _, option := range options {
		if option.Key == key {
			return option.Value
		}
	}

	return ""
}
