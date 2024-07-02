package cryptomaterial

import (
	"fmt"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/values"
)

type Modal struct {
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Modal satisfy the app.Modal interface. It also defines
	// helper methods for accessing the WindowNavigator that displayed this
	// modal.
	*app.GenericPageModal

	overlayColor color.NRGBA
	background   color.NRGBA
	list         *widget.List
	button       *widget.Clickable
	// overlayBlinder sits between the overlay and modal widget
	// it acts to intercept and prevent clicks on widget from getting
	// to the overlay.
	overlayBlinder *widget.Clickable
	card           Card
	scroll         ListStyle
	padding        unit.Dp

	isFloatTitle         bool
	isDisabled           bool
	showScrollBar        bool
	isMobileView         bool
	firstLoadWithContext func(gtx C)
}

// The firstLoad() parameter is used to perform actions
// that require Context before Layout() is called.
func (t *Theme) ModalFloatTitle(id string, isMobileView bool, firstLoad func(gtx C)) *Modal {
	mod := t.Modal(id, isMobileView, firstLoad)
	mod.isFloatTitle = true
	return mod
}

func (t *Theme) Modal(id string, isMobileView bool, firstLoad func(gtx C)) *Modal {
	overlayColor := t.Color.Black
	overlayColor.A = 200

	uniqueID := fmt.Sprintf("%s-%d", id, GenerateRandomNumber())
	m := &Modal{
		GenericPageModal: app.NewGenericPageModal(uniqueID),
		overlayColor:     overlayColor,
		background:       t.Color.Surface,
		list: &widget.List{
			List: layout.List{Axis: layout.Vertical, Alignment: layout.Middle},
		},
		button:         new(widget.Clickable),
		overlayBlinder: new(widget.Clickable),
		card:           t.Card(),
		padding:        values.MarginPadding24,
		isMobileView:   isMobileView,
	}
	m.firstLoadWithContext = firstLoad

	m.scroll = t.List(m.list)

	return m
}

// Dismiss removes the modal from the window. Does nothing if the modal was
// not previously pushed into a window.
func (m *Modal) Dismiss() {
	// ParentWindow will only be accessible if this modal has been
	// pushed into display by a WindowNavigator.
	if parentWindow := m.ParentWindow(); parentWindow != nil {
		parentWindow.DismissModal(m.ID())
	} else {
		panic("can't dismiss a modal that has not been displayed")
	}
}

// IsShown is true if this modal has been pushed into a window and is currently
// the top modal in the window.
func (m *Modal) IsShown() bool {
	if parentWindow := m.ParentWindow(); parentWindow != nil {
		topModal := parentWindow.TopModal()
		return topModal != nil && topModal.ID() == m.ID()
	}
	return false
}

// Layout renders the modal widget to screen. The modal assumes the size of
// its content plus padding.
func (m *Modal) Layout(gtx C, widgets []layout.Widget, width ...float32) D {
	if m.firstLoadWithContext != nil {
		m.firstLoadWithContext(gtx)
		m.firstLoadWithContext = nil
	}
	mGtx := gtx
	if m.isDisabled {
		mGtx = gtx.Disabled()
	}
	dims := layout.Stack{Alignment: layout.Center}.Layout(mGtx,
		layout.Expanded(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			fillMax(gtx, m.overlayColor, CornerRadius{})

			return m.button.Layout(gtx, func(gtx C) D {
				semantic.Button.Add(gtx.Ops)
				return D{Size: gtx.Constraints.Min}
			})
		}),
		layout.Stacked(func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			var widgetFuncs []layout.Widget
			var title layout.Widget

			if m.isFloatTitle && len(widgets) > 0 {
				title = widgets[0]
				widgetFuncs = append(widgetFuncs, widgets[1:]...)
			} else {
				widgetFuncs = append(widgetFuncs, widgets...)
			}

			maxWidth := float32(450)
			if len(width) > 0 && width[0] > 0 {
				maxWidth = width[0]
			} else if currentAppWidth := gtx.Metric.PxToDp(gtx.Constraints.Max.X); currentAppWidth <= values.StartMobileView {
				// maxWidth must be less than currentAppWidth on mobile, so the
				// modal does not touch the left and right edges of the screen.
				maxWidth = 0.9 * float32(currentAppWidth)
			}
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(maxWidth))
			inset := layout.Inset{
				Top:    unit.Dp(30),
				Bottom: unit.Dp(30),
			}
			uniformInset := layout.UniformInset(m.padding)
			horizontalMargin := values.MarginPaddingTransform(m.isMobileView, values.MarginPadding24)
			uniformInset.Left = horizontalMargin
			uniformInset.Right = horizontalMargin
			return inset.Layout(gtx, func(gtx C) D {
				return layout.Stack{Alignment: layout.Center}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return m.overlayBlinder.Layout(gtx, func(gtx C) D {
							return D{Size: gtx.Constraints.Min}
						})
					}),
					layout.Stacked(func(gtx C) D {
						return LinearLayout{
							Orientation: layout.Vertical,
							Width:       WrapContent,
							Height:      WrapContent,
							Padding:     uniformInset,
							Alignment:   layout.Middle,
							Border: Border{
								Radius: Radius(14),
							},
							Direction:  layout.Center,
							Background: m.background,
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								if m.isFloatTitle && len(widgets) > 0 {
									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									if m.padding == unit.Dp(0) {
										return uniformInset.Layout(gtx, title)
									}

									inset := layout.Inset{
										Top:    unit.Dp(5),
										Bottom: unit.Dp(5),
									}
									return inset.Layout(gtx, title)
								}
								return D{}
							}),
							layout.Rigid(func(gtx C) D {
								mTB := unit.Dp(5)
								mLR := unit.Dp(0)
								if m.padding == unit.Dp(0) {
									mLR = mTB
								}
								inset := layout.Inset{
									Top:    mTB,
									Bottom: mTB,
									Left:   mLR,
									Right:  mLR,
								}
								if m.showScrollBar {
									return m.scroll.Layout(gtx, len(widgetFuncs), func(gtx C, i int) D {
										gtx.Constraints.Min.X = gtx.Constraints.Max.X
										return inset.Layout(gtx, widgetFuncs[i])
									})
								}
								list := &layout.List{Axis: layout.Vertical}
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return list.Layout(gtx, len(widgetFuncs), func(gtx C, i int) D {
									return inset.Layout(gtx, widgetFuncs[i])
								})
							}),
						)
					}),
				)
			})
		}),
	)

	return dims
}

func (m *Modal) BackdropClicked(gtx C, minimizable bool) bool {
	if minimizable {
		return m.button.Clicked(gtx)
	}

	return false
}

func (m *Modal) SetPadding(padding unit.Dp) {
	m.padding = padding
}

func (m *Modal) ShowScrollbar(showScrollBar bool) {
	m.showScrollBar = showScrollBar
}

func (m *Modal) SetDisabled(disabled bool) {
	m.isDisabled = disabled
}
