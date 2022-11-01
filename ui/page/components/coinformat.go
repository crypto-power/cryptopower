package components

import (
	"image/color"
	"regexp"
	"strings"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"gioui.org/layout"
	"gioui.org/unit"
)

const defaultScale = .7

var (
	doubleOrMoreDecimalPlaces = regexp.MustCompile(`(([0-9]{1,3},*)*\.)\d{2,}`)
	oneDecimalPlace           = regexp.MustCompile(`(([0-9]{1,3},*)*\.)\d`)
	noDecimal                 = regexp.MustCompile(`([0-9]{1,3},*)+`)
)

func formatBalance(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp, scale float32, col color.NRGBA, displayUnitText bool) D {

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
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			txt := l.Theme.Label(subTextSize, subText)
			txt.Color = col
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if displayUnitText {
				return l.Theme.Label(mainTextSize, unitText).Layout(gtx)
			}

			return l.Theme.Label(subTextSize, unitText).Layout(gtx)
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
	} else {
		return -1
	}
}

// LayoutBalance aligns the main and sub DCR balances horizontally, putting the sub
// balance at the baseline of the row.
func LayoutBalance(gtx layout.Context, l *load.Load, amount string) layout.Dimensions {
	return formatBalance(gtx, l, amount, values.TextSize20, defaultScale, l.Theme.Color.Text, false)
}

func LayoutBalanceWithUnit(gtx layout.Context, l *load.Load, amount string) layout.Dimensions {
	return formatBalance(gtx, l, amount, values.TextSize20, defaultScale, l.Theme.Color.PageNavText, true)
}

func LayoutBalanceWithUnitSize(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, defaultScale, l.Theme.Color.PageNavText, true)
}

func LayoutBalanceSize(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, defaultScale, l.Theme.Color.Text, false)
}

func LayoutBalanceSizeScale(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp, scale float32) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, scale, l.Theme.Color.Text, false)
}

func LayoutBalanceColor(gtx layout.Context, l *load.Load, amount string, color color.NRGBA) layout.Dimensions {
	return formatBalance(gtx, l, amount, values.TextSize20, defaultScale, color, false)
}
