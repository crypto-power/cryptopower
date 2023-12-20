package cryptomaterial

import (
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/crypto-power/cryptopower/ui/values"
)

type SegmentType int

const (
	SegmentTypeGroup SegmentType = iota
	SegmentTypeSplit
	SegmentTypeGroupMax
	SegmentTypeDynamicSplit
)

type SegmentedControl struct {
	theme *Theme
	list  *ClickableList

	leftNavBtn,
	rightNavBtn *Clickable

	Padding        layout.Inset
	ContentPadding layout.Inset
	Alignment      layout.Alignment

	selectedIndex int
	segmentTitles []string

	changed bool
	mu      sync.Mutex

	isSwipeActionEnabled bool
	slideAction          *SlideAction
	slideActionTitle     *SlideAction
	segmentType          SegmentType

	allowCycle            bool
	isMobileView          bool
	disableUniformPadding bool
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
		ContentPadding: layout.Inset{
			Top: values.MarginPadding16,
		},
		Alignment: layout.Middle,
	}

	sc.slideAction.Draged(func(dragDirection SwipeDirection) {
		isNext := dragDirection == SwipeLeft
		sc.handleActionEvent(isNext)
		sc.list.ScrollTo(sc.selectedIndex)
	})

	sc.slideActionTitle.SetDragEffect(50)

	sc.slideActionTitle.Draged(func(dragDirection SwipeDirection) {
		isNext := dragDirection == SwipeRight
		sc.handleActionEvent(isNext)
	})

	return sc
}

func (sc *SegmentedControl) DisableUniform(disable bool) {
	sc.disableUniformPadding = disable
}

func (sc *SegmentedControl) SetEnableSwipe(enable bool) {
	sc.isSwipeActionEnabled = enable
}

// Layout handles the segmented control's layout, it receives an optional isMobileView bool
// parameter which is used to determine if the segmented control should be displayed in mobile view
// or not. If the parameter is not provided, isMobileView defaults to false.
func (sc *SegmentedControl) Layout(gtx C, body func(gtx C) D, isMobileView ...bool) D {
	sc.isMobileView = len(isMobileView) > 0 && isMobileView[0]

	widget := func(gtx C) D {
		return layout.Flex{
			Axis:      layout.Vertical,
			Alignment: sc.Alignment,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if sc.segmentType == SegmentTypeGroup {
					return sc.GroupTileLayout(gtx)
				} else if sc.segmentType == SegmentTypeGroupMax {
					return sc.groupTileMaxLayout(gtx)
				} else if sc.segmentType == SegmentTypeDynamicSplit {
					return sc.dynamicSplitTileLayout(gtx)
				}
				return sc.splitTileLayout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				// design margin is 32px
				return sc.ContentPadding.Layout(gtx, func(gtx C) D {
					if !sc.isSwipeActionEnabled {
						return body(gtx)
					}
					return sc.slideAction.DragLayout(gtx, func(gtx C) D {
						return sc.slideAction.TransformLayout(gtx, body)
					})
				})
			}),
		)
	}

	if sc.disableUniformPadding {
		return widget(gtx)
	}
	return UniformPadding(gtx, widget, sc.isMobileView)
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
					textSize16 := values.TextSizeTransform(sc.isMobileView, values.TextSize16)
					return layout.Center.Layout(gtx, func(gtx C) D {
						bg := sc.theme.Color.SurfaceHighlight
						txt := sc.theme.DecoratedText(textSize16, sc.segmentTitles[i], sc.theme.Color.GrayText1, font.SemiBold)
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
	if sc.isMobileView {
		flexWidthLeft = 0   // hide the left nav button in mobile view
		flexWidthCenter = 1 // occupy the whole width in mobile view
		flexWidthRight = 0  // hide the right nav button in mobile view
	}
	return LinearLayout{
		Width:       MatchParent,
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
					txt.Alignment = text.Middle
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

func (sc *SegmentedControl) groupTileMaxLayout(gtx C) D {
	sc.handleEvents()
	layoutSize := gtx.Constraints.Max.X
	return LinearLayout{
		Width:      layoutSize,
		Height:     WrapContent,
		Background: sc.theme.Color.Gray2,
		Padding:    layout.UniformInset(values.MarginPadding4),
		Border:     Border{Radius: Radius(8)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			textSize16 := values.TextSizeTransform(sc.isMobileView, values.TextSize16)
			return sc.slideActionTitle.DragLayout(gtx, func(gtx C) D {
				return sc.list.Layout(gtx, len(sc.segmentTitles), func(gtx C, i int) D {
					isSelectedSegment := sc.SelectedIndex() == i
					return layout.Center.Layout(gtx, func(gtx C) D {
						bg := sc.theme.Color.SurfaceHighlight
						txt := sc.theme.DecoratedText(textSize16, sc.segmentTitles[i], sc.theme.Color.GrayText1, font.SemiBold)
						border := Border{Radius: Radius(0)}
						if isSelectedSegment {
							bg = sc.theme.Color.Surface
							txt.Color = sc.theme.Color.Text
							border = Border{Radius: Radius(8)}
						}
						return LinearLayout{
							// subtract padding on the x-axis values.MarginPadding4 x2
							Width:      (layoutSize - gtx.Dp(values.MarginPadding8)) / len(sc.segmentTitles),
							Height:     WrapContent,
							Background: bg,
							Border:     border,
							Padding:    sc.Padding,
							Direction:  layout.Center,
						}.Layout2(gtx, txt.Layout)
					})
				})
			})
		}),
	)
}

func (sc *SegmentedControl) dynamicSplitTileLayout(gtx C) D {
	sc.handleEvents()
	return LinearLayout{
		Width:       MatchParent,
		Height:      WrapContent,
		Orientation: layout.Horizontal,
	}.Layout2(gtx, func(gtx C) D {
		return sc.list.Layout(gtx, len(sc.segmentTitles), func(gtx C, i int) D {
			isSelectedSegment := sc.SelectedIndex() == i
			return layout.Center.Layout(gtx, func(gtx C) D {
				bg := sc.theme.Color.Surface
				txt := sc.theme.DecoratedText(values.TextSizeTransform(sc.isMobileView, values.TextSize14), sc.segmentTitles[i], sc.theme.Color.GrayText2, font.SemiBold)
				border := Border{Radius: Radius(12), Color: sc.theme.Color.Gray10}
				if isSelectedSegment {
					bg = sc.theme.Color.Gray2
					txt.Color = sc.theme.Color.Text
				}
				txt.Alignment = text.Middle
				paddingTB := values.MarginPadding4
				paddingLR := values.MarginPadding12
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
	})
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
