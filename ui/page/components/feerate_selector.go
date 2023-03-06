package components

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type FeerateSelector struct {
	*load.Load

	feeRateText     string
	EditRates       cryptomaterial.Button
	FetchRates      cryptomaterial.Button
	ratesEditor     cryptomaterial.Editor
	priority        string
	rateEditMode    bool
	fetchingRate    bool
	EstSignedSize   string
	TxCost          string
	TxFee           string
	TxFeeUSD        string
	showSizeAndCost bool

	ContainerInset    layout.Inset
	WrapperInset      layout.Inset
	TitleInset        layout.Inset
	TitleFontWeight   text.Weight
	OuterWrapperColor color.NRGBA
}

func NewFeerateSelector(l *load.Load) *FeerateSelector {
	fs := &FeerateSelector{
		Load: l,
	}

	fs.feeRateText = " - "
	fs.EstSignedSize = "-"
	fs.TxCost = "-"
	fs.TxFee = " - "
	fs.TxFeeUSD = " - "
	fs.priority = values.String(values.StrUnknown)
	fs.EditRates = fs.Theme.Button(values.String(values.StrEdit))
	fs.FetchRates = fs.Theme.Button(values.String(values.StrFetchRates))

	bInset := layout.Inset{
		Top:    values.MarginPadding4,
		Right:  values.MarginPadding8,
		Bottom: values.MarginPadding4,
		Left:   values.MarginPadding8,
	}
	fs.FetchRates.TextSize, fs.EditRates.TextSize = values.TextSize12, values.TextSize12
	fs.FetchRates.Inset, fs.EditRates.Inset = bInset, bInset

	fs.ratesEditor = fs.Theme.Editor(new(widget.Editor), "in Sat/kvB")
	fs.ratesEditor.HasCustomButton = false
	fs.ratesEditor.Editor.SingleLine = true
	fs.ratesEditor.TextSize = values.TextSize14
	fs.ContainerInset = layout.Inset{Bottom: values.MarginPadding15}
	fs.WrapperInset = layout.Inset{Bottom: values.MarginPadding15}
	fs.TitleInset = layout.Inset{Bottom: values.MarginPadding0}
	fs.OuterWrapperColor = l.Theme.Color.Primary

	return fs
}

func (fs *FeerateSelector) ShowSizeAndCost() {
	fs.showSizeAndCost = true
}

func (fs *FeerateSelector) Layout(gtx C) D {
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
						border := widget.Border{Color: fs.Load.Theme.Color.Gray4, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
						wrapper := fs.Load.Theme.Card()
						wrapper.Color = fs.Load.Theme.Color.Gray4
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
														feerateLabel.Font.Weight = text.SemiBold
														return feerateLabel.Layout(gtx)
													}),
													layout.Rigid(func(gtx C) D {
														return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, fs.EditRates.Layout)
													}),
													layout.Rigid(func(gtx C) D {
														if fs.fetchingRate {
															return layout.Inset{Left: values.MarginPadding18,
																Right:  values.MarginPadding8,
																Bottom: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
																return material.Loader(fs.Theme.Base).Layout(gtx)
															})
														}
														return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, fs.FetchRates.Layout)
													}),
												)
											})
										}),
										layout.Rigid(func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													priorityLabel := fs.Theme.Label(values.TextSize14, values.StringF(values.StrPriority, " : "))
													priorityLabel.Font.Weight = text.SemiBold
													return priorityLabel.Layout(gtx)
												}),
												layout.Rigid(func(gtx C) D {
													priorityVal := fs.Theme.Label(values.TextSize14, fs.priority)
													priorityVal.Font.Style = text.Italic
													return priorityVal.Layout(gtx)
												}),
											)
										}),
										layout.Rigid(func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													sizeLabel := fs.Theme.Label(values.TextSize14, values.StringF(values.StrTxSize, " : "))
													sizeLabel.Font.Weight = text.SemiBold
													return sizeLabel.Layout(gtx)
												}),
												layout.Rigid(func(gtx C) D {
													txSize := fs.Theme.Label(values.TextSize14, fs.EstSignedSize)
													txSize.Font.Style = text.Italic
													return txSize.Layout(gtx)
												}),
											)
										}),
										layout.Rigid(func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													sizeLabel := fs.Theme.Label(values.TextSize14, values.StringF(values.StrCost, " : "))
													sizeLabel.Font.Weight = text.SemiBold
													return sizeLabel.Layout(gtx)
												}),
												layout.Rigid(func(gtx C) D {
													txSize := fs.Theme.Label(values.TextSize14, fs.TxFee)
													txSize.Font.Style = text.Italic
													return txSize.Layout(gtx)
												}),
											)
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

func (fs *FeerateSelector) FetchFeeRate(window app.WindowNavigator, selectedWallet *load.WalletMapping) {
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

func (fs *FeerateSelector) OnEditRateCliked(selectedWallet *load.WalletMapping) {
	fs.rateEditMode = !fs.rateEditMode
	if fs.rateEditMode {
		fs.EditRates.Text = values.String(values.StrSave)
	} else {
		rateStr := fs.ratesEditor.Editor.Text()
		rateInt, err := selectedWallet.SetAPIFeeRate(rateStr)
		if err != nil {
			fs.feeRateText = " - "
		}
		fs.feeRateText = fs.addRatesUnits(rateInt)
	}
}

func (fs *FeerateSelector) addRatesUnits(rates int64) string {
	return fs.Load.Printer.Sprintf("%d Sat/kvB", rates)
}
