package components

import (
	"image/color"
	"regexp"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const defaultScale = .7

var (
	doubleOrMoreDecimalPlaces = regexp.MustCompile(`(([0-9]{1,3},*)*\.)\d{2,}`)
	oneDecimalPlace           = regexp.MustCompile(`(([0-9]{1,3},*)*\.)\d`)
	noDecimal                 = regexp.MustCompile(`([0-9]{1,3},*)+`)
)

func formatBalance(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp, scale float32, col color.NRGBA, displayUnitText bool, fontweight font.Weight) D {

	startIndex := 0
	stopIndex := 0

	if doubleOrMoreDecimalPlaces.MatchString(amount) {
		decimalIndex := strings.Index(amount, ".")
		startIndex = decimalIndex + 3
	} else if oneDecimalPlace.MatchString(amount) {
		decimalIndex := strings.Index(amount, ".")
		startIndex = decimalIndex + 2
	} else if noDecimal.MatchString(amount) {
		loc := noDecimal.FindStringIndex(amount)
		startIndex = loc[1] // start scaling from the end
	}

	stopIndex = getIndexUnit(amount)
	isUnitExist := stopIndex != -1
	if isUnitExist {
		stopIndex = len(amount)
	}

	mainText, subText, unitText := amount[:startIndex], amount[startIndex:stopIndex], amount[stopIndex:]

	subTextSize := unit.Sp(float32(mainTextSize) * scale)

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := l.Theme.Label(mainTextSize, mainText)
			txt.Color = col
			txt.Font.Weight = fontweight
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			txt := l.Theme.Label(subTextSize, subText)
			txt.Color = col
			txt.Font.Weight = fontweight
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if displayUnitText {
				txt := l.Theme.Label(mainTextSize, unitText)
				txt.Font.Weight = fontweight
				return txt.Layout(gtx)
			}

			txt := l.Theme.Label(subTextSize, unitText)
			txt.Font.Weight = fontweight
			return txt.Layout(gtx)
		}),
	)
}

// getIndexUnit returns index of unit currency in amount and
// helps to break out the unit part from the amount string.
func getIndexUnit(amount string) int {
	if strings.Contains(amount, string(utils.BTCWalletAsset)) {
		return strings.Index(amount, " "+string(utils.BTCWalletAsset))
	} else if strings.Contains(amount, string(utils.DCRWalletAsset)) {
		return strings.Index(amount, " "+string(utils.DCRWalletAsset))
	} else if strings.Contains(amount, string(utils.LTCWalletAsset)) {
		return strings.Index(amount, " "+string(utils.LTCWalletAsset))
	} else {
		return -1
	}
}

// LayoutBalance aligns the main and sub DCR balances horizontally, putting the sub
// balance at the baseline of the row.
func LayoutBalance(gtx layout.Context, l *load.Load, amount string) layout.Dimensions {
	return formatBalance(gtx, l, amount, values.TextSize20, defaultScale, l.Theme.Color.Text, false, font.Normal)
}

func LayoutBalanceWithUnit(gtx layout.Context, l *load.Load, amount string) layout.Dimensions {
	return formatBalance(gtx, l, amount, values.TextSize20, defaultScale, l.Theme.Color.PageNavText, true, font.Normal)
}

func LayoutBalanceWithUnitSize(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, defaultScale, l.Theme.Color.PageNavText, true, font.Normal)
}

func LayoutBalanceSize(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, defaultScale, l.Theme.Color.Text, false, font.Normal)
}

func LayoutBalanceSizeWeight(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp, weight font.Weight) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, defaultScale, l.Theme.Color.Text, false, weight)
}

func LayoutBalanceSizeScale(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp, scale float32) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, scale, l.Theme.Color.Text, false, font.Normal)
}

func LayoutBalanceColor(gtx layout.Context, l *load.Load, amount string, color color.NRGBA) layout.Dimensions {
	return formatBalance(gtx, l, amount, values.TextSize20, defaultScale, color, false, font.Normal)
}
