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

func formatBalance(gtx C, l *load.Load, amount string, mainTextSize unit.Sp, col color.NRGBA, isBoldText, displayUnitText bool) D {

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

	if startIndex > stopIndex || stopIndex <= 0 || stopIndex > len(amount) {
		return D{}
	}

	mainText, subText, unitText := amount[:startIndex], amount[startIndex:stopIndex], amount[stopIndex:]

	subTextSize := unit.Sp(float32(mainTextSize) * defaultScale)

	lblWidget := func(size unit.Sp, text string) D {
		lbl := l.Theme.Label(size, text)
		lbl.Color = col

		if isBoldText {
			lbl.Font.Weight = font.SemiBold
		}

		return lbl.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return lblWidget(mainTextSize, mainText)
		}),
		layout.Rigid(func(gtx C) D {
			return lblWidget(subTextSize, subText)
		}),
		layout.Rigid(func(gtx C) D {
			if displayUnitText {
				return lblWidget(mainTextSize, unitText)
			}
			return lblWidget(subTextSize, unitText)
		}),
	)
}

func formatBalanceWithHidden(gtx C, l *load.Load, amount string, mainTextSize unit.Sp, textFont font.Weight, col color.NRGBA, isUSD bool) D {
	isBalanceHidden := l.AssetsManager.IsTotalBalanceVisible()
	txt := l.Theme.Label(mainTextSize, amount)
	if isUSD {
		if !l.AssetsManager.ExchangeRateFetchingEnabled() {
			txt.Text = "$ --"
		}
	}
	if isBalanceHidden {
		unit := ""
		if !isUSD {
			stopIndex := getIndexUnit(amount)
			isUnitExist := stopIndex == -1
			if isUnitExist {
				stopIndex = len(amount)
			}
			unit = amount[stopIndex:]
		}
		txt.Text = "****** " + unit
	}
	txt.Color = col
	txt.Font.Weight = textFont
	return txt.Layout(gtx)
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
	} else if strings.Contains(amount, string(utils.BCHWalletAsset)) {
		return strings.Index(amount, " "+string(utils.BCHWalletAsset))
	}
	return -1
}

// LayoutBalance aligns the main and sub DCR balances horizontally, putting the sub
// balance at the baseline of the row.
func LayoutBalance(gtx layout.Context, l *load.Load, amount string) layout.Dimensions {
	return formatBalance(gtx, l, amount, values.TextSize20, l.Theme.Color.Text, false, false)
}

func LayoutBalanceWithUnit(gtx layout.Context, l *load.Load, amount string) layout.Dimensions {
	return formatBalance(gtx, l, amount, values.TextSize20, l.Theme.Color.PageNavText, false, true)
}

func LayoutBalanceWithUnitSize(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, l.Theme.Color.PageNavText, false, true)
}

func LayoutBalanceWithUnitSizeBoldText(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, l.Theme.Color.PageNavText, true, true)
}

func LayoutBalanceSize(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, l.Theme.Color.Text, false, false)
}

func LayoutBalanceCustom(gtx layout.Context, l *load.Load, amount string, mainTextSize unit.Sp, isBold bool) layout.Dimensions {
	return formatBalance(gtx, l, amount, mainTextSize, l.Theme.Color.Text, isBold, false)
}

func LayoutBalanceColor(gtx layout.Context, l *load.Load, amount string, color color.NRGBA) layout.Dimensions {
	return formatBalance(gtx, l, amount, values.TextSize20, color, false, false)
}

func LayoutBalanceWithState(gtx layout.Context, l *load.Load, amount string) layout.Dimensions {
	return formatBalanceWithHidden(gtx, l, amount, values.TextSize16, font.Normal, l.Theme.Color.Text, false)
}

func LayoutBalanceColorWithState(gtx layout.Context, l *load.Load, amount string, color color.NRGBA) layout.Dimensions {
	return formatBalanceWithHidden(gtx, l, amount, values.TextSize20, font.Normal, color, false)
}

func LayoutBalanceWithStateSemiBold(gtx layout.Context, l *load.Load, amount string) layout.Dimensions {
	return formatBalanceWithHidden(gtx, l, amount, values.TextSize16, font.SemiBold, l.Theme.Color.Text, false)
}

func LayoutBalanceWithStateUSD(gtx layout.Context, l *load.Load, amount string) layout.Dimensions {
	return formatBalanceWithHidden(gtx, l, amount, values.TextSize16, font.Normal, l.Theme.Color.Text, true)
}

func LayoutBalanceColorWithStateUSD(gtx layout.Context, l *load.Load, amount string, color color.NRGBA) layout.Dimensions {
	return formatBalanceWithHidden(gtx, l, amount, values.TextSize16, font.Normal, color, true)
}
