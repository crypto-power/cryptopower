package send

import (
	"fmt"
	// "image/color"
	// "strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	// sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	// "github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

type recipient struct {
	*load.Load

	deleteBtn   *cryptomaterial.Clickable
	description cryptomaterial.Editor

	sendDestination *destination
	amount          *sendAmount
}

func newRecipient(l *load.Load, assetType libUtil.AssetType) *recipient {
	rp := &recipient{
		Load: l,
	}

	rp.amount = newSendAmount(l.Theme, assetType)
	rp.sendDestination = newSendDestination(l, assetType)

	rp.description = rp.Theme.Editor(new(widget.Editor), values.String(values.StrNote))
	rp.description.Editor.SingleLine = false
	rp.description.Editor.SetText("")
	// Set the maximum characters the editor can accept.
	rp.description.Editor.MaxLen = MaxTxLabelSize

	return rp
}

func (rp *recipient) recipientLayout(gtx C, index int, showIcon bool, window app.WindowNavigator) layout.Widget {
	rp.handle()
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.WrapContent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return rp.topLayout(gtx, index, showIcon)
			}),
			layout.Rigid(func(gtx C) D {
				inset := layout.Inset{
					Top: values.MarginPaddingMinus5,
				}
				layoutBody := rp.sendDestination.destinationAddressEditor.Layout
				if !rp.sendDestination.sendToAddress {
					layoutBody = rp.walletAccountlayout(gtx, window)
				}
				return inset.Layout(gtx, func(gtx C) D {
					return rp.sendDestination.accountSwitch.Layout(gtx, layoutBody)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return rp.txLabelSection(gtx)
			}),
		)
	}
}

func (rp *recipient) topLayout(gtx C, index int, showIcon bool) D {
	titleTxt := rp.Theme.Label(values.TextSize16, fmt.Sprintf("To: Recipient %v", index))
	titleTxt.Color = rp.Theme.Color.GrayText2
	if !showIcon {
		return titleTxt.Layout(gtx)
	}

	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return titleTxt.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, rp.Theme.Icons.DeleteIcon.Layout20dp)
		}),
	)
}

func (rp *recipient) walletAccountlayout(gtx C, window app.WindowNavigator) layout.Widget {
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return rp.contentWrapper(gtx, "Destination Wallet", func(gtx C) D {
					return rp.sendDestination.destinationWalletSelector.Layout(window, gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return rp.contentWrapper(gtx, values.String(values.StrAccount), func(gtx C) D {
					return rp.sendDestination.destinationAccountSelector.Layout(window, gtx)
				})
			}),
		)
	}
}

func (rp *recipient) contentWrapper(gtx C, title string, content layout.Widget) D {
	return layout.Inset{
		Bottom: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				lbl := rp.Theme.Label(values.TextSize16, title)
				lbl.Font.Weight = font.SemiBold
				return layout.Inset{
					Bottom: values.MarginPadding4,
				}.Layout(gtx, lbl.Layout)
			}),
			layout.Rigid(content),
		)
	})
}

func (rp *recipient) addressAndAmountlayout(gtx C, window app.WindowNavigator) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			// if rp.exchangeRate != -1 && rp.usdExchangeSet {
			// 	return layout.Flex{
			// 		Axis:      layout.Horizontal,
			// 		Alignment: layout.Middle,
			// 	}.Layout(gtx,
			// 		layout.Flexed(0.45, func(gtx C) D {
			// 			return rp.amount.amountEditor.Layout(gtx)
			// 		}),
			// 		layout.Flexed(0.1, func(gtx C) D {
			// 			return layout.Center.Layout(gtx, func(gtx C) D {
			// 				icon := rp.Theme.Icons.CurrencySwapIcon
			// 				return icon.Layout12dp(gtx)
			// 			})
			// 		}),
			// 		layout.Flexed(0.45, func(gtx C) D {
			// 			return rp.amount.usdAmountEditor.Layout(gtx)
			// 		}),
			// 	)
			// }
			return rp.amount.amountEditor.Layout(gtx)
		}),
		// layout.Rigid(func(gtx C) D {
		// 	if rp.exchangeRateMessage == "" {
		// 		return layout.Dimensions{}
		// 	}
		// 	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// 		layout.Rigid(func(gtx C) D {
		// 			return layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
		// 				gtx.Constraints.Min.X = gtx.Constraints.Max.X
		// 				gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding1)
		// 				return cryptomaterial.Fill(gtx, rp.Theme.Color.Gray1)
		// 			})
		// 		}),
		// 		layout.Rigid(func(gtx C) D {
		// 			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		// 				layout.Rigid(func(gtx C) D {
		// 					label := rp.Theme.Body2(rp.exchangeRateMessage)
		// 					label.Color = rp.Theme.Color.Danger
		// 					if rp.isFetchingExchangeRate {
		// 						label.Color = rp.Theme.Color.Primary
		// 					}
		// 					return label.Layout(gtx)
		// 				}),
		// 				layout.Rigid(func(gtx C) D {
		// 					if rp.isFetchingExchangeRate {
		// 						return layout.Dimensions{}
		// 					}
		// 					gtx.Constraints.Min.X = gtx.Constraints.Max.X
		// 					return layout.E.Layout(gtx, rp.retryExchange.Layout)
		// 				}),
		// 			)
		// 		}),
		// 	)
		// }),
	)
}

func (rp *recipient) txLabelSection(gtx C) D {
	count := len(rp.description.Editor.Text())
	txt := fmt.Sprintf("%s (%d/%d)", values.String(values.StrDescriptionNote), count, rp.description.Editor.MaxLen)
	return rp.contentWrapper(gtx, txt, rp.description.Layout)
}

func (rp *recipient) handle() {
	rp.sendDestination.handle()
	rp.amount.handle()
}
