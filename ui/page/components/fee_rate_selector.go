package components

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/values"
)

type walletTypeCallbackFunc func() libutils.AssetType

// FeeRateSelector represent a tx fee selector UI component.
type FeeRateSelector struct {
	*load.Load

	feeRateText string
	// SaveRate is a material button to trigger save tx fee rate.
	SaveRate cryptomaterial.Button

	fetchedRatesDropDown *cryptomaterial.DropDown

	feeRateSwitch *cryptomaterial.SegmentedControl

	ratesEditor  cryptomaterial.Editor
	priority     string
	rateEditMode bool
	fetchingRate bool
	// EstSignedSize holds the estimated size of signed tx.
	EstSignedSize string
	// TxFee stores the estimated transaction fee for a tx.
	TxFee string
	// TxFeeUSD stores the estimated tx fee in USD.
	TxFeeUSD        string
	showSizeAndCost bool

	// selectedWalletType provides a callback function that can be used
	// independent of the set wallet within the Load input parameter.
	selectedWalletType walletTypeCallbackFunc

	// USDExchangeSet determines if this component will in addition
	// to the TxFee show the USD rate of fee.
	USDExchangeSet bool
}

// NewFeeRateSelector create and return an instance of FeeRateSelector.
// Since the feeRate selector can be used before the selected wallet is set
// a Load independent callback function is provided to help address that case scenario.
func NewFeeRateSelector(l *load.Load, callback walletTypeCallbackFunc) *FeeRateSelector {
	fs := &FeeRateSelector{
		Load:               l,
		selectedWalletType: callback,
	}

	fs.feeRateText = " - "
	fs.EstSignedSize = "-"
	fs.TxFee = " - "
	fs.TxFeeUSD = " - "
	fs.priority = values.String(values.StrUnknown)
	fs.SaveRate = fs.Theme.Button(values.String(values.StrSave))

	fs.feeRateSwitch = fs.Theme.SegmentedControl([]string{
		"Fetched",
		"Manual",
	}, cryptomaterial.SegmentTypeDynamicSplit)
	fs.feeRateSwitch.LayoutPadding = layout.Inset{Top: values.MarginPadding4}

	fs.SaveRate.TextSize = values.TextSize16
	fs.SaveRate.Inset = layout.Inset{
		Top:    values.MarginPadding12,
		Right:  values.MarginPadding16,
		Bottom: values.MarginPadding12,
		Left:   values.MarginPadding16,
	}

	fs.fetchedRatesDropDown = fs.Theme.DropDown([]cryptomaterial.DropDownItem{}, values.WalletsDropdownGroup, false)
	fs.fetchedRatesDropDown.BorderColor = &fs.Theme.Color.Gray2

	fs.ratesEditor = fs.Theme.Editor(new(widget.Editor), "In "+fs.ratesUnit())
	fs.ratesEditor.HasCustomButton = false
	fs.ratesEditor.Bordered = false
	fs.ratesEditor.Editor.SingleLine = true
	fs.ratesEditor.TextSize = values.TextSize14

	return fs
}

// ShowSizeAndCost turns the showSizeAndCost Field to true
// the component will show the estimated size and Fee when
// showSizeAndCost is true.
func (fs *FeeRateSelector) ShowSizeAndCost() *FeeRateSelector {
	fs.showSizeAndCost = true
	return fs
}

func (fs *FeeRateSelector) isFeerateAPIApproved() bool {
	return fs.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.FeeRateHTTPAPI)
}

// Layout draws the UI components.
func (fs *FeeRateSelector) Layout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			title := fs.Theme.Body1("Fee Rate")
			title.Font.Weight = font.SemiBold
			return layout.Inset{Bottom: values.MarginPadding4}.Layout(gtx, title.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					layoutBody := func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(.8, func(gtx C) D {
								border := cryptomaterial.Border{Color: fs.Load.Theme.Color.Gray2, Width: values.MarginPadding1, Radius: cryptomaterial.CornerRadius{TopRight: 0, TopLeft: 8, BottomRight: 0, BottomLeft: 8}}
								return border.Layout(gtx, func(gtx C) D {
									return layout.Inset{
										Top:    values.MarginPadding7,
										Bottom: values.MarginPadding7,
										Right:  values.MarginPadding12,
										Left:   values.MarginPadding12,
									}.Layout(gtx, fs.ratesEditor.Layout)
								})
							}),
							layout.Rigid(func(gtx C) D {
								card := cryptomaterial.Card{Color: fs.SaveRate.Background}
								card.Radius = cryptomaterial.CornerRadius{TopRight: 8, TopLeft: 0, BottomRight: 8, BottomLeft: 0}
								return card.Layout(gtx, fs.SaveRate.Layout)
							}),
						)
					}
					if fs.feeRateSwitch.SelectedSegment() == "Fetched" {
						fs.fetchedRatesDropDown.Width = unit.Dp(gtx.Constraints.Max.X) / 2
						layoutBody = fs.fetchedRatesDropDown.Layout
					}
					if !fs.isFeerateAPIApproved() || fs.selectedWalletType() == libutils.LTCWalletAsset || fs.selectedWalletType() == libutils.DCRWalletAsset {
						gtx = gtx.Disabled()
					}
					return fs.feeRateSwitch.Layout(gtx, layoutBody)
				}),
				layout.Rigid(func(gtx C) D {
					txt := fmt.Sprintf("Priority: %s, Transaction Size: %s", fs.priority, fs.EstSignedSize)
					if fs.showSizeAndCost {
						feeText := fs.TxFee
						if fs.USDExchangeSet {
							feeText = fmt.Sprintf("%s (%s)", fs.TxFee, fs.TxFeeUSD)
						}
						txt = fmt.Sprintf("Priority: %s, Transaction Size: %s, Cost: %s", fs.priority, fs.EstSignedSize, feeText)
					}
					lbl := fs.Theme.Label(values.TextSize14, txt)
					lbl.Color = fs.Theme.Color.GrayText2

					// update label text and color if any of the conditions are met below
					if !fs.isFeerateAPIApproved() {
						lbl.Text = values.StringF(values.StrNotAllowed, values.String(values.StrFeeRates))
						lbl.Color = fs.Theme.Color.Danger
					} else if fs.selectedWalletType() == libutils.LTCWalletAsset || fs.selectedWalletType() == libutils.DCRWalletAsset {
						// TODO: Add fee rate API query for LTC
						lbl.Text = values.StringF(values.StrNotSupported, values.String(values.StrFeeRateAPI))
						lbl.Color = fs.Theme.Color.Danger
					}

					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
								return layout.E.Layout(gtx, lbl.Layout)
							})
						}),
					)
				}),
			)
		}),
	)
}

// FetchFeeRate will fetch the fee rate from the HTTP API.
func (fs *FeeRateSelector) UpdatedFeeRate(selectedWallet sharedW.Asset) {
	if fs.fetchingRate {
		return
	}
	fs.fetchingRate = true
	defer func() {
		fs.fetchingRate = false
	}()

	feeRates, err := load.GetAPIFeeRate(selectedWallet)
	if err != nil {
		return
	}

	blocksStr := func(b int32) string {
		val := strconv.Itoa(int(b)) + " block"
		if b == 1 {
			return val
		}
		return val + "s"
	}

	items := []cryptomaterial.DropDownItem{}
	for index := range feeRates {
		items = append(items, cryptomaterial.DropDownItem{
			Text: fs.addRatesUnits(feeRates[index].Feerate.ToInt()) + " - " + blocksStr(feeRates[index].ConfirmedBlocks),
		})
	}

	fs.fetchedRatesDropDown = fs.Theme.DropDown(items, values.WalletsDropdownGroup, false)
}

// FetchFeeRate will fetch the fee rate from the HTTP API.
func (fs *FeeRateSelector) FetchFeeRate(window app.WindowNavigator, selectedWallet sharedW.Asset) {
	if fs.fetchingRate {
		return
	}
	fs.fetchingRate = true
	defer func() {
		fs.fetchingRate = false
	}()

	feeRates, err := load.GetAPIFeeRate(selectedWallet)
	if err != nil {
		return
	}

	blocksStr := func(b int32) string {
		val := strconv.Itoa(int(b)) + " block"
		if b == 1 {
			return val
		}
		return val + "s"
	}

	radiogroupbtns := new(widget.Enum)
	items := make([]layout.FlexChild, 0)
	for index, feerate := range feeRates {
		key := strconv.Itoa(index)
		value := fs.addRatesUnits(feerate.Feerate.ToInt()) + " - " + blocksStr(feerate.ConfirmedBlocks)
		radioBtn := fs.Load.Theme.RadioButton(radiogroupbtns, key, value,
			fs.Load.Theme.Color.DeepBlue, fs.Load.Theme.Color.Primary)
		items = append(items, layout.Rigid(radioBtn.Layout))
	}

	info := modal.NewCustomModal(fs.Load).
		Title(values.String(values.StrFeeRates)).
		UseCustomWidget(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		}).
		SetCancelable(true).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetPositiveButtonText(values.String(values.StrSave)).
		SetPositiveButtonCallback(func(isChecked bool, im *modal.InfoModal) bool {
			fields := strings.Fields(radiogroupbtns.Value)
			index, _ := strconv.Atoi(fields[0])
			rate := strconv.Itoa(int(feeRates[index].Feerate.ToInt()))
			rateInt, err := load.SetAPIFeeRate(selectedWallet, rate)
			if err != nil {
				log.Error(err)
				return false
			}

			fs.feeRateText = fs.addRatesUnits(rateInt)
			fs.rateEditMode = false
			blocks := feeRates[index].ConfirmedBlocks
			timeBefore := time.Now().Add(time.Duration(-10*blocks) * time.Minute)
			fs.priority = fmt.Sprintf("%v (~%v)", blocksStr(blocks), TimeAgo(timeBefore.Unix()))
			im.Dismiss()
			return true
		})

	window.ShowModal((info))
	// fs.EditRates.SetEnabled(true)
}

// OnEditRateCliked is called when the edit feerate button is clicked.
func (fs *FeeRateSelector) OnEditRateClicked(selectedWallet sharedW.Asset) {
	rateStr := fs.ratesEditor.Editor.Text()
	rateInt, err := load.SetAPIFeeRate(selectedWallet, rateStr)
	if err != nil {
		fs.feeRateText = " - "
	} else {
		fs.feeRateText = fs.addRatesUnits(rateInt)
	}
}

func (fs *FeeRateSelector) addRatesUnits(rates int64) string {
	return fs.Load.Printer.Sprintf("%d %s", rates, fs.ratesUnit())
}

func (fs *FeeRateSelector) ratesUnit() string {
	switch fs.selectedWalletType() {
	case libutils.LTCWalletAsset:
		return "Lit/kvB"
	default:
		return "Sat/kvB"
	}
}

// SetFeerate updates the fee rate in use upstream.
func (fs *FeeRateSelector) SetFeerate(rateInt int64) {
	if rateInt == 0 {
		fs.feeRateText = " - "
		return
	}
	fs.feeRateText = fs.addRatesUnits(rateInt)
}
