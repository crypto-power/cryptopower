package components

import (
	"fmt"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
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
		values.String(values.StrFetched),
		values.String(values.StrManual),
	}, cryptomaterial.SegmentTypeDynamicSplit)
	fs.feeRateSwitch.SetEnableSwipe(false)

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
	fs.ratesEditor.IsTitleLabel = false

	return fs
}

// ShowSizeAndCost turns the showSizeAndCost Field to true
// the component will show the estimated size and Fee when
// showSizeAndCost is true.backupLaterbackupLater
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
			title := fs.Theme.Body1(values.String(values.StrFeeRates))
			title.Font.Weight = font.SemiBold
			return layout.Inset{Bottom: values.MarginPadding4}.Layout(gtx, title.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					layoutBody := func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(.8, func(gtx C) D {
								border := cryptomaterial.Border{
									Color: fs.Load.Theme.Color.Gray2,
									Width: values.MarginPadding1,
									Radius: cryptomaterial.CornerRadius{
										TopRight:    0,
										TopLeft:     8,
										BottomRight: 0,
										BottomLeft:  8,
									},
								}
								return border.Layout(gtx, func(gtx C) D {
									return layout.Inset{
										Top:    values.MarginPadding4,
										Bottom: values.MarginPadding4,
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
					// Fee rate fatching not currently supported by LTC
					// TODO: Add fee rate API query for LTC
					if fs.selectedWalletType() == libutils.LTCWalletAsset {
						return layoutBody(gtx)
					}

					if fs.feeRateSwitch.SelectedSegment() == values.String(values.StrFetched) {
						fs.fetchedRatesDropDown.Width = values.MarginPadding510
						layoutBody = fs.fetchedRatesDropDown.Layout
					}

					return fs.feeRateSwitch.Layout(gtx, layoutBody)
				}),
				layout.Rigid(func(gtx C) D {
					col := fs.Theme.Color.GrayText2
					txSize := values.StringF(values.StrTxSize, fmt.Sprintf(": %s", fs.EstSignedSize))
					priority := values.StringF(values.StrPriority, fmt.Sprintf(": %s", fs.priority))
					txt := fmt.Sprintf("%s, %s", priority, txSize)
					if fs.showSizeAndCost {
						feeText := fs.TxFee
						if fs.USDExchangeSet {
							feeText = values.StringF(values.StrCost, fmt.Sprintf("%s (%s)", fs.TxFee, fs.TxFeeUSD))
						}
						txt = fmt.Sprintf("%s, %s, %s", priority, txSize, feeText)
					}

					// update label text and color if any of the conditions are met below
					if !fs.isFeerateAPIApproved() {
						txt = values.StringF(values.StrNotAllowed, values.String(values.StrFeeRates))
						col = fs.Theme.Color.Danger
					}

					lbl := fs.Theme.Label(values.TextSize14, txt)
					lbl.Color = col
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
