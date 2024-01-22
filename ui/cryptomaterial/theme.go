// SPDX-License-Identifier: Unlicense OR MIT

package cryptomaterial

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/values"

	"golang.org/x/exp/shiny/materialdesign/icons"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Theme struct {
	Shaper text.Shaper
	Base   *material.Theme
	Color  *values.Color
	Styles *values.WidgetStyles

	Icons                 *Icons
	TextSize              unit.Sp
	checkBoxCheckedIcon   *widget.Icon
	checkBoxUncheckedIcon *widget.Icon
	radioCheckedIcon      *widget.Icon
	radioUncheckedIcon    *widget.Icon
	chevronUpIcon         *widget.Icon
	dropDownIcon          *widget.Icon
	chevronDownIcon       *widget.Icon
	navigationCheckIcon   *widget.Icon
	NavMoreIcon           *widget.Icon
	expandIcon            *Image
	collapseIcon          *Image

	dropDownMenus    []*DropDown
	DropdownBackdrop *widget.Clickable

	allEditors  []*Editor
	backButtons []*widget.Clickable
}

func NewTheme(fontCollection []text.FontFace, decredIcons map[string]image.Image, isDarkModeOn bool) *Theme {
	t := &Theme{
		Shaper:           *text.NewShaper(fontCollection),
		Base:             material.NewTheme(fontCollection),
		Color:            &values.Color{},
		Icons:            &Icons{},
		Styles:           values.DefaultWidgetStyles(),
		TextSize:         values.TextSize16,
		DropdownBackdrop: new(widget.Clickable),
	}
	t.SwitchDarkMode(isDarkModeOn, decredIcons)
	t.checkBoxCheckedIcon = MustIcon(widget.NewIcon(icons.ToggleCheckBox))
	t.checkBoxUncheckedIcon = MustIcon(widget.NewIcon(icons.ToggleCheckBoxOutlineBlank))
	t.radioCheckedIcon = MustIcon(widget.NewIcon(icons.ToggleRadioButtonChecked))
	t.radioUncheckedIcon = MustIcon(widget.NewIcon(icons.ToggleRadioButtonUnchecked))
	t.chevronUpIcon = MustIcon(widget.NewIcon(icons.NavigationExpandLess))
	t.chevronDownIcon = MustIcon(widget.NewIcon(icons.NavigationExpandMore))
	t.NavMoreIcon = MustIcon(widget.NewIcon(icons.NavigationMoreHoriz))
	t.navigationCheckIcon = MustIcon(widget.NewIcon(icons.NavigationCheck))
	t.dropDownIcon = MustIcon(widget.NewIcon(icons.NavigationArrowDropDown))

	return t
}

func (t *Theme) SwitchDarkMode(isDarkModeOn bool, decredIcons map[string]image.Image) {
	t.Color = t.Color.DefaultThemeColors()
	t.Icons.DefaultIcons()
	expandIcon := "expand_icon"
	collapseIcon := "collapse_icon"
	if isDarkModeOn {
		t.Icons.DarkModeIcons()
		t.Color.DarkThemeColors() // override defaults with dark themed colors
		expandIcon = "expand_dm"
		collapseIcon = "collapse_dm"
	}

	t.expandIcon = NewImage(decredIcons[expandIcon])
	t.collapseIcon = NewImage(decredIcons[collapseIcon])

	t.updateStyles(isDarkModeOn)
}

// UpdateStyles update the style definition for different widgets. This should
// be done whenever the base theme changes to ensure that the style definitions
// use the values for the latest theme.
func (t *Theme) updateStyles(isDarkModeOn bool) {
	// update switch style colors
	t.Styles.SwitchStyle.ActiveColor = t.Color.Primary
	t.Styles.SwitchStyle.InactiveColor = t.Color.Gray3
	t.Styles.SwitchStyle.ThumbColor = t.Color.White

	// update icon button style colors
	t.Styles.IconButtonColorStyle.Background = color.NRGBA{}
	t.Styles.IconButtonColorStyle.Foreground = t.Color.Gray1

	// update Collapsible widget style colors
	t.Styles.CollapsibleStyle.Background = t.Color.Surface
	t.Styles.CollapsibleStyle.Foreground = color.NRGBA{}

	// update clickable colors
	t.Styles.ClickableStyle.Color = t.Color.SurfaceHighlight
	t.Styles.ClickableStyle.HoverColor = t.Color.Gray5

	// dropdown clickable colors
	t.Styles.DropdownClickableStyle.Color = t.Color.SurfaceHighlight
	col := t.Color.Gray3
	if isDarkModeOn {
		col = t.Color.Gray5
	}
	t.Styles.DropdownClickableStyle.HoverColor = Hovered(col)
}

func (t *Theme) Background(gtx layout.Context, w layout.Widget) {
	layout.Stack{
		Alignment: layout.N,
	}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return fill(gtx, t.Color.Gray4)
		}),
		layout.Stacked(w),
	)
}

func (t *Theme) Surface(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return layout.Stack{
		Alignment: layout.Center,
	}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return fill(gtx, t.Color.Surface)
		}),
		layout.Stacked(w),
	)
}

func (t *Theme) ImageIcon(gtx layout.Context, icon image.Image, size int) layout.Dimensions {
	return NewImage(icon).LayoutSize(gtx, unit.Dp(float32(size)))
}

func MustIcon(ic *widget.Icon, err error) *widget.Icon {
	if err != nil {
		panic(err)
	}
	return ic
}

func rgb(c uint32) color.NRGBA {
	return argb(0xff000000 | c)
}

func argb(c uint32) color.NRGBA {
	return color.NRGBA{A: uint8(c >> 24), R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c)}
}

func toPointF(p image.Point) f32.Point {
	return f32.Point{X: float32(p.X), Y: float32(p.Y)}
}

func fillMax(gtx layout.Context, col color.NRGBA, radius CornerRadius) D {
	cs := gtx.Constraints
	d := image.Point{X: cs.Max.X, Y: cs.Max.Y}
	track := image.Rectangle{
		Max: image.Point{X: d.X, Y: d.Y},
	}

	defer clip.RRect{
		Rect: track,
		NE:   radius.TopRight, NW: radius.TopLeft, SE: radius.BottomRight, SW: radius.BottomLeft,
	}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, col)

	return layout.Dimensions{Size: d}
}

func fill(gtx layout.Context, col color.NRGBA) layout.Dimensions {
	cs := gtx.Constraints
	d := image.Point{X: cs.Min.X, Y: cs.Min.Y}
	track := image.Rectangle{
		Max: d,
	}
	defer clip.Rect(track).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, col)

	return layout.Dimensions{Size: d}
}

func Fill(gtx layout.Context, col color.NRGBA) layout.Dimensions {
	return fill(gtx, col)
}

func FillMax(gtx layout.Context, col color.NRGBA, radius int) layout.Dimensions {
	return fillMax(gtx, col, Radius(radius))
}

// mulAlpha scales all color components by alpha/255.
func mulAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	a := uint16(alpha)
	return color.NRGBA{
		A: uint8(uint16(c.A) * a / 255),
		R: uint8(uint16(c.R) * a / 255),
		G: uint8(uint16(c.G) * a / 255),
		B: uint8(uint16(c.B) * a / 255),
	}
}

func (t *Theme) closeAllDropdowns() {
	for _, dropDown := range t.dropDownMenus {
		dropDown.expanded = false
	}
}

// isDropdownGroupCollapsed iterate over Dropdowns registered as a member
// of {group}, returns true if any of the drop down state is open.
func (t *Theme) isDropdownGroupCollapsed(group uint) bool {
	for _, dropDown := range t.dropDownMenus {
		if dropDown.group == group {
			if dropDown.expanded {
				return true
			}
		}
	}
	return false
}

// Disabled blends color towards the luminance and multiplies alpha.
// Blending towards luminance will desaturate the color.
// Multiplying alpha blends the color together more with the background.
func Disabled(c color.NRGBA) (d color.NRGBA) {
	const r = 80 // blend ratio
	lum := approxLuminance(c)
	return color.NRGBA{
		R: byte((int(c.R)*r + int(lum)*(256-r)) / 256),
		G: byte((int(c.G)*r + int(lum)*(256-r)) / 256),
		B: byte((int(c.B)*r + int(lum)*(256-r)) / 256),
		A: byte(int(c.A) * (128 + 32) / 256),
	}
}

// Hovered blends color towards a brighter color.
func Hovered(c color.NRGBA) (d color.NRGBA) {
	const r = 0x20 // lighten ratio
	return color.NRGBA{
		R: byte(255 - int(255-c.R)*(255-r)/256),
		G: byte(255 - int(255-c.G)*(255-r)/256),
		B: byte(255 - int(255-c.B)*(255-r)/256),
		A: c.A,
	}
}

// approxLuminance is a fast approximate version of RGBA.Luminance.
func approxLuminance(c color.NRGBA) byte {
	const (
		r = 13933 // 0.2126 * 256 * 256
		g = 46871 // 0.7152 * 256 * 256
		b = 4732  // 0.0722 * 256 * 256
		t = r + g + b
	)
	return byte((r*int(c.R) + g*int(c.G) + b*int(c.B)) / t)
}

func HandleEditorEvents(editors ...*widget.Editor) (bool, bool) {
	var submit, changed bool
	for _, editor := range editors {
		for _, evt := range editor.Events() {
			switch evt.(type) {
			case widget.ChangeEvent:
				changed = true
			case widget.SubmitEvent:
				submit = true
			}
		}
	}
	return submit, changed
}

func SwitchEditors(event *key.Event, editors ...*widget.Editor) {
	if event.Modifiers != key.ModShift {
		for i := 0; i < len(editors); i++ {
			if editors[i].Focused() {
				if i == len(editors)-1 {
					editors[0].Focus()
				} else {
					editors[i+1].Focus()
				}
			}
		}
	} else {
		for i := 0; i < len(editors); i++ {
			if editors[i].Focused() {
				if i == 0 {
					editors[len(editors)-1].Focus()
				} else {
					editors[i-1].Focus()
				}
			}
		}
	}
}

func (t *Theme) AssetIcon(asset utils.AssetType) *Image {
	var icon *Image
	switch asset {
	case utils.DCRWalletAsset:
		icon = t.Icons.DCR
	case utils.LTCWalletAsset:
		icon = t.Icons.LTC
	case utils.BTCWalletAsset:
		icon = t.Icons.BTC
	default:
		icon = nil
	}
	return icon
}

func (t *Theme) AutoHideSoftKeyBoard(gtx C) {
	isHide := true
	for _, e := range t.allEditors {
		isHide = isHide && !e.Pressed()
	}
	if isHide {
		key.SoftKeyboardOp{Show: false}.Add(gtx.Ops)
	}
}

func (t *Theme) AddBackClick(clickable *widget.Clickable) {
	t.backButtons = append(t.backButtons, clickable)
}

func (t *Theme) OnTapBack() {
	for _, clickable := range t.backButtons {
		clickable.Click()
	}
}

func CentralizeWidget(gtx C, widget layout.Widget) D {
	return LinearLayout{
		Width:       MatchParent,
		Height:      WrapContent,
		Orientation: layout.Horizontal,
		Direction:   layout.Center,
	}.Layout2(gtx, widget)
}
