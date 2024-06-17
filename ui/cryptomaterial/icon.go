// SPDX-License-Identifier: Unlicense OR MIT

package cryptomaterial

import (
	"image/color"

	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/ui/values"
)

type Icon struct {
	*widget.Icon
	Color color.NRGBA
}

// NewIcon returns a new Icon from IconVG data.
func NewIcon(icon *widget.Icon) *Icon {
	return &Icon{
		Icon: icon,
	}
}

// NewIcon from theme a new Icon from IconVG data with style color.
func (t *Theme) NewIcon(icon *widget.Icon) *Icon {
	return &Icon{
		Icon:  icon,
		Color: t.Styles.IconButtonColorStyle.Foreground,
	}
}

func (icon *Icon) Layout24dp(gtx C) D {
	return icon.Layout(gtx, values.MarginPadding24)
}
func (icon *Icon) Layout20dp(gtx C) D {
	return icon.Layout(gtx, values.MarginPadding20)
}

func (icon *Icon) Layout18dp(gtx C) D {
	return icon.Layout(gtx, values.MarginPadding18)
}

func (icon *Icon) Layout16dp(gtx C) D {
	return icon.Layout(gtx, values.MarginPadding16)
}

func (icon *Icon) Layout12dp(gtx C) D {
	return icon.Layout(gtx, values.MarginPadding12)
}

// LayoutTransform is used to scale images for mobile view.
func (icon *Icon) LayoutTransform(gtx C, isMobileView bool, size unit.Dp) D {
	if isMobileView {
		size = values.MarginPaddingTransform(isMobileView, size)
	}
	return icon.Layout(gtx, size)
}

func (icon *Icon) Layout(gtx C, iconSize unit.Dp) D {
	cl := color.NRGBA{A: 0xff}
	if icon.Color != (color.NRGBA{}) {
		cl = icon.Color
	}
	gtx.Constraints.Min.X = gtx.Dp(iconSize)
	return icon.Icon.Layout(gtx, cl)
}
