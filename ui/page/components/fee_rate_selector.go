package components

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/crypto-power/cryptopower/app"
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
	// EditRates is a material button to trigger edit/save tx fee rate.
	EditRates cryptomaterial.Button
	// FetchRates button initiates call to rates HTTP API.
	FetchRates   cryptomaterial.Button
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

	// ContainerInset should be used to set the inset for FeeRateSelector
	// component container.
	ContainerInset layout.Inset
	// WrapperInset should be used to set the inset for for the wrapper
	// container.
	WrapperInset layout.Inset
	// TitleInset sets the inset for the title label.
	TitleInset layout.Inset
	// TitleFontWeight sets the font weight for the title label.
	TitleFontWeight font.Weight
	// USDExchangeSet determines if this component will in addition to the TxFee
	// show the USD rate of fee.
	USDExchangeSet bool
}

// NewFeeRateSelector create and return an instance of FeeRateSelector. Since
// the feeRate selector can be used before the selected wallet is set a Load
// independent callback function is provided to help address that case scenario.
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
	fs.EditRates = fs.Theme.Button(values.String(values.StrEdit))
	fs.FetchRates = fs.Theme.Button(values.String(values.StrFetchRates))

	buttonInset := layout.Inset{
		Top:    values.MarginPadding4,
		Right:  values.MarginPadding8,
		Bottom: values.MarginPadding4,
		Left:   values.MarginPadding8,
	}
	fs.FetchRates.TextSize, fs.EditRates.TextSize = values.TextSize12, values.TextSize12
	fs.FetchRates.Inset, fs.EditRates.Inset = buttonInset, buttonInset

	fs.ratesEditor = fs.Theme.Editor(new(widget.Editor), "In "+fs.ratesUnit())
	fs.ratesEditor.HasCustomButton = false
	fs.ratesEditor.Editor.SingleLine = true
	fs.ratesEditor.TextSize = values.TextSize14
	fs.ContainerInset = layout.Inset{Bottom: values.MarginPadding15}
	fs.WrapperInset = layout.Inset{Bottom: values.MarginPadding15}
	fs.TitleInset = layout.Inset{Bottom: values.MarginPadding0}

	return fs
}

// ShowSizeAndCost turns the showSizeAndCost Field to true the component will
// show the estimated size and Fee when showSizeAndCost is true.
func (fs *FeeRateSelector) ShowSizeAndCost() *FeeRateSelector {
	fs.showSizeAndCost = true
	return fs
}

func (fs *FeeRateSelector) isFeerateAPIApproved() bool {
	return fs.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.FeeRateHTTPAPI)
}

// Layout draws the UI components.
func (fs *FeeRateSelector) Layout(gtx C) D {
	return fs.ContainerInset.Layout(gtx, func(gtx C) D {
		border := widget.Border{CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
		wrapper := fs.Load.Theme.Card()
		return border.Layout(gtx, func(gtx C) D {
			return wrapper.Layout(gtx, func(gtx C) D {
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.WrapContent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Vertical,
					Margin:      fs.WrapperInset,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						title := fs.Theme.Body1(values.String(values.StrTxFee))
						title.Font.Weight = fs.TitleFontWeight
						return fs.TitleInset.Layout(gtx, func(gtx C) D {
							return title.Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						border := widget.Border{Color: fs.Load.Theme.Color.Gray2, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
						wrapper := fs.Load.Theme.Card()
						wrapper.Color = fs.Load.Theme.Color.Background
						return border.Layout(gtx, func(gtx C) D {
							return wrapper.Layout(gtx, func(gtx C) D {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X // Wrapper should fill available width
								return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											return layout.Inset{Bottom: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
												return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
													layout.Rigid(func(gtx C) D {
														if fs.rateEditMode {
															gtx.Constraints.Max.X = gtx.Constraints.Max.X / 3
															return fs.ratesEditor.Layout(gtx)
														}
														feerateLabel := fs.Theme.Label(values.TextSize14, fs.feeRateText)
														feerateLabel.Font.Weight = font.SemiBold
														return feerateLabel.Layout(gtx)
													}),
													layout.Rigid(func(gtx C) D {
														return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, fs.EditRates.Layout)
													}),
													layout.Rigid(func(gtx C) D {
														if fs.fetchingRate {
															return layout.Inset{
																Left:   values.MarginPadding18,
																Right:  values.MarginPadding8,
																Bottom: values.MarginPadding4,
															}.Layout(gtx, func(gtx C) D {
																return material.Loader(fs.Theme.Base).Layout(gtx)
															})
														}

														str := fs.FetchRates.Text
														if !fs.isFeerateAPIApproved() {
															str = values.StringF(values.StrNotAllowed, values.String(values.StrFeeRates))
															fs.FetchRates.SetEnabled(false)
														} else if fs.selectedWalletType() == libutils.LTCWalletAsset {
															// TODO: Add fee rate API query for LTC
															str = values.StringF(values.StrNotSupported, values.String(values.StrFeeRateAPI))
															fs.FetchRates.SetEnabled(false)
														}
														fs.FetchRates.Text = str
														return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, fs.FetchRates.Layout)
													}),
												)
											})
										}),
										layout.Rigid(func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													priorityLabel := fs.Theme.Label(values.TextSize14, values.StringF(values.StrPriority, " : "))
													priorityLabel.Font.Weight = font.SemiBold
													return priorityLabel.Layout(gtx)
												}),
												layout.Rigid(func(gtx C) D {
													priorityVal := fs.Theme.Label(values.TextSize14, fs.priority)
													priorityVal.Font.Style = font.Italic
													return priorityVal.Layout(gtx)
												}),
											)
										}),
										layout.Rigid(func(gtx C) D {
											if fs.showSizeAndCost {
												return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
													layout.Rigid(func(gtx C) D {
														sizeLabel := fs.Theme.Label(values.TextSize14, values.StringF(values.StrTxSize, " : "))
														sizeLabel.Font.Weight = font.SemiBold
														return sizeLabel.Layout(gtx)
													}),
													layout.Rigid(func(gtx C) D {
														txSize := fs.Theme.Label(values.TextSize14, fs.EstSignedSize)
														txSize.Font.Style = font.Italic
														return txSize.Layout(gtx)
													}),
												)
											}
											return D{}
										}),
										layout.Rigid(func(gtx C) D {
											if fs.showSizeAndCost {
												return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
													layout.Rigid(func(gtx C) D {
														sizeLabel := fs.Theme.Label(values.TextSize14, values.StringF(values.StrCost, " : "))
														sizeLabel.Font.Weight = font.SemiBold
														return sizeLabel.Layout(gtx)
													}),
													layout.Rigid(func(gtx C) D {
														feeText := fs.TxFee
														if fs.USDExchangeSet {
															feeText = fmt.Sprintf("%s (%s)", fs.TxFee, fs.TxFeeUSD)
														}

														txSize := fs.Theme.Label(values.TextSize14, feeText)
														txSize.Font.Style = font.Italic
														return txSize.Layout(gtx)
													}),
												)
											}

											return D{}
										}),
									)
								})
							})
						})
					}),
				)
			})
		})
	})
}

// FetchFeeRate will fetch the fee rate from the HTTP API.
func (fs *FeeRateSelector) FetchFeeRate(window app.WindowNavigator, selectedWallet *load.WalletMapping) {
	if fs.fetchingRate {
		return
	}
	fs.fetchingRate = true
	defer func() {
		fs.fetchingRate = false
	}()

	feeRates, err := selectedWallet.GetAPIFeeRate()
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
			rateInt, err := selectedWallet.SetAPIFeeRate(rate)
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
	fs.EditRates.SetEnabled(true)
}

// OnEditRateClicked is called when the edit feerate button is clicked.
func (fs *FeeRateSelector) OnEditRateClicked(selectedWallet *load.WalletMapping) {
	fs.rateEditMode = !fs.rateEditMode
	if fs.rateEditMode {
		fs.EditRates.Text = values.String(values.StrSave)
	} else {
		rateStr := fs.ratesEditor.Editor.Text()
		rateInt, err := selectedWallet.SetAPIFeeRate(rateStr)
		if err != nil {
			fs.feeRateText = " - "
		} else {
			fs.feeRateText = fs.addRatesUnits(rateInt)
		}
		fs.EditRates.Text = values.String(values.StrEdit)
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
