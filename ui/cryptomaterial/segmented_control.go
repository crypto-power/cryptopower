package cryptomaterial

import (
	"sync"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/values"
)

type SegmentType int

const (
	SegmentTypeGroup SegmentType = iota
	SegmentTypeSplit
)

type SegmentedControl struct {
	theme *Theme
	list  *ClickableList

	leftNavBtn,
	rightNavBtn *Clickable

	Padding layout.Inset

	selectedIndex int
	segmentTitles []string

	changed bool
	mu      sync.Mutex

	isSwipeActionEnabled bool
	slideAction          *SlideAction
	slideActionTitle     *SlideAction
	segmentType          SegmentType

	allowCycle   bool
	isMobileView bool
}

// Segmented control is a linear set of two or more segments, each of which functions as a button.
func (t *Theme) SegmentedControl(segmentTitles []string, segmentType SegmentType) *SegmentedControl {
	list := t.NewClickableList(layout.Horizontal)
	list.IsHoverable = false

	sc := &SegmentedControl{
		list:                 list,
		theme:                t,
		segmentTitles:        segmentTitles,
		leftNavBtn:           t.NewClickable(false),
		rightNavBtn:          t.NewClickable(false),
		isSwipeActionEnabled: true,
		segmentType:          segmentType,
		slideAction:          NewSlideAction(),
		slideActionTitle:     NewSlideAction(),
		Padding:              layout.UniformInset(values.MarginPadding8),
	}

	sc.slideAction.Draged(func(dragDirection SwipeDirection) {
		isNext := dragDirection == SwipeLeft
		sc.handleActionEvent(isNext)
	})

	sc.slideActionTitle.SetDragEffect(50)

	sc.slideActionTitle.Draged(func(dragDirection SwipeDirection) {
		isNext := dragDirection == SwipeRight
		sc.handleActionEvent(isNext)
	})

	return sc
}

func (sc *SegmentedControl) SetEnableSwipe(enable bool) {
	sc.isSwipeActionEnabled = enable
}

// Layout handles the segmented control's layout, it receives an optional isMobileView bool
// parameter which is used to determine if the segmented control should be displayed in mobile view
// or not. If the parameter is not provided, isMobileView defaults to false.
func (sc *SegmentedControl) Layout(gtx C, body func(gtx C) D, isMobileView ...bool) D {
	sc.isMobileView = len(isMobileView) > 0 && isMobileView[0]

	return UniformPadding(gtx, func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if sc.segmentType == SegmentTypeGroup {
					return sc.GroupTileLayout(gtx)
				}
				return sc.splitTileLayout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					if !sc.isSwipeActionEnabled {
						return body(gtx)
					}
					return sc.slideAction.DragLayout(gtx, func(gtx C) D {
						return sc.slideAction.TransformLayout(gtx, body)
					})
				})
			}),
		)
	})
}

func (sc *SegmentedControl) GroupTileLayout(gtx C) D {
	sc.handleEvents()

	return LinearLayout{
		Width:      WrapContent,
		Height:     WrapContent,
		Background: sc.theme.Color.Gray2,
		Border:     Border{Radius: Radius(8)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return sc.slideActionTitle.DragLayout(gtx, func(gtx C) D {
				return sc.list.Layout(gtx, len(sc.segmentTitles), func(gtx C, i int) D {
					isSelectedSegment := sc.SelectedIndex() == i
					textSize := values.TextSize16
					if sc.isMobileView {
						textSize = values.TextSize12
					}
					return layout.Center.Layout(gtx, func(gtx C) D {
						bg := sc.theme.Color.SurfaceHighlight
						txt := sc.theme.DecoratedText(textSize, sc.segmentTitles[i], sc.theme.Color.GrayText1, font.SemiBold)
						border := Border{Radius: Radius(0)}
						if isSelectedSegment {
							bg = sc.theme.Color.Surface
							txt.Color = sc.theme.Color.Text
							border = Border{Radius: Radius(8)}
						}
						return LinearLayout{
							Width:      WrapContent,
							Height:     WrapContent,
							Background: bg,
							Margin:     layout.UniformInset(values.MarginPadding5),
							Border:     border,
							Padding:    sc.Padding,
						}.Layout2(gtx, txt.Layout)
					})
				})
			})
		}),
	)
}

func (sc *SegmentedControl) splitTileLayout(gtx C) D {
	sc.handleEvents()
	flexWidthLeft := float32(.035)
	flexWidthCenter := float32(.8)
	flexWidthRight := float32(.035)
	linearLayoutWidth := gtx.Dp(values.MarginPadding700)
	if sc.isMobileView {
		flexWidthLeft = 0   // hide the left nav button in mobile view
		flexWidthCenter = 1 // occupy the whole width in mobile view
		flexWidthRight = 0  // hide the right nav button in mobile view
		linearLayoutWidth = MatchParent
	}
	return LinearLayout{
		Width:       linearLayoutWidth,
		Height:      WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
	}.Layout(gtx,
		layout.Flexed(flexWidthLeft, func(gtx C) D {
			return sc.leftNavBtn.Layout(gtx, sc.theme.Icons.ChevronLeft.Layout24dp)
		}),
		layout.Flexed(flexWidthCenter, func(gtx C) D {
			return sc.list.Layout(gtx, len(sc.segmentTitles), func(gtx C, i int) D {
				isSelectedSegment := sc.SelectedIndex() == i
				return layout.Center.Layout(gtx, func(gtx C) D {
					bg := sc.theme.Color.Gray2
					txt := sc.theme.DecoratedText(values.TextSize14, sc.segmentTitles[i], sc.theme.Color.GrayText2, font.SemiBold)
					border := Border{Radius: Radius(8)}
					if isSelectedSegment {
						bg = sc.theme.Color.LightBlue8
						txt.Color = sc.theme.Color.Primary
					}
					paddingTB := values.MarginPadding8
					paddingLR := values.MarginPadding32
					pr := values.MarginPadding6
					if i == len(sc.segmentTitles) { // no need to add padding to the last item
						pr = values.MarginPadding0
					}

					return layout.Inset{Right: pr}.Layout(gtx, func(gtx C) D {
						return LinearLayout{
							Width:  WrapContent,
							Height: WrapContent,
							Padding: layout.Inset{
								Top:    paddingTB,
								Bottom: paddingTB,
								Left:   paddingLR,
								Right:  paddingLR,
							},
							Background: bg,
							Border:     border,
						}.Layout2(gtx, txt.Layout)
					})
				})
			})
		}),
		layout.Flexed(flexWidthRight, func(gtx C) D {
			return sc.rightNavBtn.Layout(gtx, sc.theme.Icons.ChevronRight.Layout24dp)
		}),
	)
}

func (sc *SegmentedControl) handleEvents() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if segmentItemClicked, clickedSegmentIndex := sc.list.ItemClicked(); segmentItemClicked {
		if sc.selectedIndex != clickedSegmentIndex {
			sc.changed = true
		}
		sc.selectedIndex = clickedSegmentIndex
	}

	if sc.leftNavBtn.Clicked() {
		sc.list.Position.First = 0
		sc.list.Position.Offset = 0
		sc.list.Position.BeforeEnd = true
		sc.list.ScrollToEnd = false
	}
	if sc.rightNavBtn.Clicked() {
		sc.list.Position.OffsetLast = 0
		sc.list.Position.BeforeEnd = false
		sc.list.ScrollToEnd = true
	}
}

func (sc *SegmentedControl) SelectedIndex() int {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.selectedIndex
}

func (sc *SegmentedControl) SelectedSegment() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.segmentTitles[sc.selectedIndex]
}

func (sc *SegmentedControl) Changed() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	changed := sc.changed
	sc.changed = false
	return changed
}

func (sc *SegmentedControl) SetSelectedSegment(segment string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	for i, item := range sc.segmentTitles {
		if item == segment {
			sc.selectedIndex = i
			break
		}
	}
}

func (sc *SegmentedControl) handleActionEvent(isNext bool) {
	l := len(sc.segmentTitles) - 1 // index starts at 0
	if isNext {
		if sc.selectedIndex == l {
			if !sc.allowCycle {
				return
			}
			sc.selectedIndex = 0
		} else {
			sc.selectedIndex++
		}
		sc.slideAction.PushLeft()
		sc.slideActionTitle.PushLeft()
	} else {
		if sc.selectedIndex == 0 {
			if !sc.allowCycle {
				return
			}
			sc.selectedIndex = l
		} else {
			sc.selectedIndex--
		}
		sc.slideAction.PushRight()
		sc.slideActionTitle.PushRight()
	}
	sc.changed = true
}
