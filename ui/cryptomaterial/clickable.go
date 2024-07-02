package cryptomaterial

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/ui/values"
)

type Clickable struct {
	button    *widget.Clickable
	style     *values.ClickableStyle
	Hoverable bool
	Radius    CornerRadius
	isEnabled bool
}

func (t *Theme) NewClickable(hoverable bool) *Clickable {
	return &Clickable{
		button:    &widget.Clickable{},
		style:     t.Styles.ClickableStyle,
		Hoverable: hoverable,
		isEnabled: true,
	}
}

func (cl *Clickable) Style() values.ClickableStyle {
	return *cl.style
}

func (cl *Clickable) ChangeStyle(style *values.ClickableStyle) {
	cl.style = style
}

func (cl *Clickable) Clicked(gtx C) bool {
	return cl.button.Clicked(gtx)
}

func (cl *Clickable) IsHovered() bool {
	return cl.button.Hovered()
}

// SetEnabled enables/disables the clickable.
func (cl *Clickable) SetEnabled(enable bool, gtx *layout.Context) layout.Context {
	var mGtx layout.Context
	if gtx != nil && !enable {
		mGtx = gtx.Disabled()
	}

	cl.isEnabled = enable
	return mGtx
}

// Return clickable enabled/disabled state.
func (cl *Clickable) Enabled() bool {
	return cl.isEnabled
}

// LayoutWithInset draws a layout of a clickable and applies an hover effect if
// an hover action is detected. rightInset and bottomInset are used to restrict
// hover layout and should be supplied ONLY if a right or bottom inset/margin
// was applied to w.
func (cl *Clickable) LayoutWithInset(gtx C, w layout.Widget, rightInset, bottomInset unit.Dp) D {
	return cl.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				// Only hover on a widget, ignore inset or margin applied.
				gtx.Constraints.Min.Y = gtx.Constraints.Min.Y - gtx.Dp(bottomInset)
				gtx.Constraints.Min.X = gtx.Constraints.Min.X - gtx.Dp(rightInset)
				tr := gtx.Dp(unit.Dp(cl.Radius.TopRight))
				tl := gtx.Dp(unit.Dp(cl.Radius.TopLeft))
				br := gtx.Dp(unit.Dp(cl.Radius.BottomRight))
				bl := gtx.Dp(unit.Dp(cl.Radius.BottomLeft))
				defer clip.RRect{
					Rect: image.Rectangle{
						Max: image.Point{
							X: gtx.Constraints.Min.X,
							Y: gtx.Constraints.Min.Y,
						},
					},
					NW: tl, NE: tr, SE: br, SW: bl,
				}.Push(gtx.Ops).Pop()
				clip.Rect{Max: gtx.Constraints.Min}.Push(gtx.Ops).Pop()

				if cl.Hoverable && cl.button.Hovered() {
					paint.Fill(gtx.Ops, cl.style.HoverColor)
				}

				for _, c := range cl.button.History() {
					drawInk(gtx, c, cl.style.Color)
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(w),
		)
	})
}

func (cl *Clickable) Layout(gtx C, w layout.Widget) D {
	return cl.LayoutWithInset(gtx, w, 0, 0)
}
