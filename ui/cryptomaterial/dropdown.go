package cryptomaterial

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	DropdownBasePos = 0
	// maxDropdownItemTextLen is the maximum len of a dropdown item text.
	// Dropdown item text that exceed maxDropdownItemTextLen will be truncated
	// and an ellipsis will be shown at the end.
	maxDropdownItemTextLen = 20
)

var MaxWidth = unit.Dp(800)

type DropDown struct {
	theme          *Theme
	items          []DropDownItem
	expanded       bool
	GroupPosition  uint
	revs           bool
	selectedIndex  int
	dropdownIcon   *widget.Icon
	navigationIcon *widget.Icon
	clickable      *Clickable
	maxTextLeng    int

	group                    uint
	isDropdownGroupCollapsed func(group uint) bool
	Width                    unit.Dp
	linearLayout             *LinearLayout
	padding                  layout.Inset
	shadow                   *Shadow
	expandedViewAlignment    layout.Direction

	noSelectedItem DropDownItem

	FontWeight  font.Weight
	BorderWidth unit.Dp
	BorderColor *color.NRGBA
	Background  *color.NRGBA
	// SelectedItemIconColor is a custom color that will be applied to the icon
	// use in identifying selected item when this dropdown is expanded.
	SelectedItemIconColor        *color.NRGBA
	CollapsedLayoutTextDirection layout.Direction
	// Set Hoverable to false to make this dropdown's collapsed layout
	// non-hoverable (default: true).
	Hoverable bool
	// Set MakeCollapsedLayoutVisibleWhenExpanded to true to make this
	// dropdown's collapsed layout visible when its dropdown is expanded.
	MakeCollapsedLayoutVisibleWhenExpanded bool
	// ExpandedLayoutInset is information about this dropdown's expanded layout
	// position. It's Top value must be set if
	// MakeCollapsedLayoutVisibleWhenExpanded is true.
	ExpandedLayoutInset  layout.Inset
	collapsedLayoutInset layout.Inset

	convertTextSize func(unit.Sp) unit.Sp
}

type DropDownItem struct {
	Text      string
	Icon      *Image
	clickable *Clickable
	// DisplayFn is an alternate display function that can be used to layout the
	// item instead of using the default item layout.
	DisplayFn func(gtx C) D
	// Set to true for items that cannot be selected.
	PreventSelection bool
}

// DropDown is like DropdownWithCustomPos but uses default values for
// groupPosition, and dropdownInset parameters.
func (t *Theme) DropDown(items []DropDownItem, group uint, reversePos bool) *DropDown {
	return t.DropdownWithCustomPos(items, group, 0, 0, reversePos)
}

// DropdownWithCustomPos returns a dropdown component. {groupPosition} parameter
// signifies the position of the dropdown in a dropdown group on the UI, the
// first dropdown should be assigned pos 0, next 1..etc. incorrectly assigned
// Dropdown pos will result in inconsistent dropdown backdrop. {dropdownInset}
// parameter is the left  inset for the dropdown if {reversePos} is false, else
// it'll become the right inset for the dropdown.
func (t *Theme) DropdownWithCustomPos(items []DropDownItem, group uint, groupPosition uint, dropdownInset int, reversePos bool) *DropDown {
	d := &DropDown{
		theme:          t,
		expanded:       false,
		GroupPosition:  groupPosition,
		selectedIndex:  0,
		dropdownIcon:   t.dropDownIcon,
		navigationIcon: t.navigationCheckIcon,
		Hoverable:      true,
		clickable:      t.NewClickable(true),

		group:                    group,
		isDropdownGroupCollapsed: t.isDropdownGroupCollapsed,
		linearLayout: &LinearLayout{
			Width:  WrapContent,
			Height: WrapContent,
			Border: Border{Radius: Radius(8)},
		},
		padding:                      layout.Inset{Top: values.MarginPadding8, Bottom: values.MarginPadding8},
		shadow:                       t.Shadow(),
		CollapsedLayoutTextDirection: layout.W,
	}

	d.revs = reversePos
	d.expandedViewAlignment = layout.NW
	d.ExpandedLayoutInset = layout.Inset{Left: unit.Dp(dropdownInset)}
	if d.revs {
		d.expandedViewAlignment = layout.NE
		d.ExpandedLayoutInset.Left = values.MarginPadding10
		d.ExpandedLayoutInset.Right = unit.Dp(dropdownInset)
	}
	d.collapsedLayoutInset = d.ExpandedLayoutInset

	d.clickable.ChangeStyle(t.Styles.DropdownClickableStyle)
	d.clickable.Radius = Radius(8)

	d.BorderColor = &d.linearLayout.Border.Color

	for i := range items {
		items[i].clickable = t.NewClickable(true)
		d.items = append(d.items, items[i])
	}

	t.dropDownMenus = append(t.dropDownMenus, d)
	return d
}

func (d *DropDown) Selected() string {
	return d.items[d.SelectedIndex()].Text
}

func (d *DropDown) ClearWithSelectedItem(item DropDownItem) {
	d.selectedIndex = -1
	d.noSelectedItem = item
}

func (d *DropDown) SelectedIndex() int {
	return d.selectedIndex
}

func (d *DropDown) SetConvertTextSize(fun func(unit.Sp) unit.Sp) {
	d.convertTextSize = fun
}

func (d *DropDown) getTextSize(textSize unit.Sp) unit.Sp {
	if d.convertTextSize == nil {
		return textSize
	}
	return d.convertTextSize(textSize)
}

func (d *DropDown) Len() int {
	return len(d.items)
}

func (d *DropDown) handleEvents() {
	if d.expanded {
		for i := range d.items {
			index := i
			item := d.items[index]
			for item.clickable.Clicked() {
				d.expanded = false
				if !item.PreventSelection {
					d.selectedIndex = index
				}
				break
			}
		}
	} else {
		for d.clickable.Clicked() {
			d.expanded = true
		}
	}
}

func (d *DropDown) Changed() bool {
	if d.expanded {
		for i := range d.items {
			index := i
			item := d.items[index]
			for item.clickable.Clicked() {
				d.expanded = false
				if item.PreventSelection {
					return false
				}
				oldSelected := d.selectedIndex
				d.selectedIndex = index
				return oldSelected != index
			}
		}

		// If no dropdown item was clicked, check if there's a click on the
		// backdrop and close all dropdowns.
		if len(d.theme.DropdownBackdrop.Clicks()) > 0 {
			d.theme.closeAllDropdowns()
		}
	}

	return false
}

// defaultDropdownWidth returns the default width for a dropdown depending on
// it's position.
func defaultDropdownWidth(reversePosition bool) unit.Dp {
	if reversePosition {
		return values.MarginPadding140
	}
	return values.MarginPadding180
}

func (d *DropDown) Reversed() bool {
	return d.revs
}

func (d *DropDown) SetMaxTextLeng(leng int) {
	d.maxTextLeng = leng
}

func (d *DropDown) Layout(gtx C) D {
	d.handleEvents()
	if d.maxTextLeng == 0 {
		d.maxTextLeng = maxDropdownItemTextLen
	}

	if d.MakeCollapsedLayoutVisibleWhenExpanded {
		return d.collapsedAndExpandedLayout(gtx)
	}

	if d.GroupPosition == DropdownBasePos && d.isDropdownGroupCollapsed(d.group) {
		maxY := unit.Dp(len(d.items)) * values.MarginPadding50
		gtx.Constraints.Max.Y = gtx.Dp(maxY)
		if d.expanded {
			return layout.Stack{Alignment: d.expandedViewAlignment}.Layout(gtx, layout.Stacked(d.expandedLayout))
		}

		return layout.Stack{Alignment: d.expandedViewAlignment}.Layout(gtx, layout.Stacked(d.collapsedLayout))
	} else if d.expanded {
		return layout.Stack{Alignment: d.expandedViewAlignment}.Layout(gtx, layout.Stacked(d.expandedLayout))
	}

	return layout.Stack{Alignment: d.expandedViewAlignment}.Layout(gtx, layout.Stacked(d.collapsedLayout))
}

// collapsedAndExpandedLayout stacks the expanded view right below the collapsed
// view (only if d.expanded = true) such that both the current selection and the
// list of items are visible.
func (d *DropDown) collapsedAndExpandedLayout(gtx C) D {
	layoutContents := []layout.StackChild{layout.Expanded(func(gtx C) D {
		expanded := d.expanded
		d.expanded = false // enforce a collapsed layout display before creating the layout Dimensions and undo later.
		display := d.collapsedLayout(gtx)
		d.expanded = expanded
		return display
	})}

	if d.expanded {
		layoutContents = append(layoutContents, layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			// Adding d.ExpandedLayoutInset.Top accounts for the the extra
			// shift in vertical space set by d.ExpandedLayoutInset.Top to
			// ensure the expanded view has enough space for its elements.
			maxY := unit.Dp(len(d.items))*values.MarginPadding50 + d.ExpandedLayoutInset.Top
			gtx.Constraints.Max.Y = gtx.Dp(maxY)
			return d.expandedLayout(gtx)
		}))
	}

	return layout.Stack{Alignment: d.expandedViewAlignment}.Layout(gtx, layoutContents...)
}

// expandedLayout computes dropdown layout when dropdown is opened.
func (d *DropDown) expandedLayout(gtx C) D {
	return d.ExpandedLayoutInset.Layout(gtx, func(gtx C) D {
		return d.drawLayout(gtx, func(gtx C) D {
			list := &layout.List{Axis: layout.Vertical}
			return list.Layout(gtx, len(d.items), func(gtx C, index int) D {
				if len(d.items) == 0 {
					return D{}
				}

				item := d.items[index]
				body := LinearLayout{
					Width:     MatchParent,
					Height:    WrapContent,
					Padding:   layout.Inset{Right: values.MarginPadding5},
					Direction: layout.W,
				}

				return d.itemLayout(gtx, index, item.clickable, &item, 8, &body)
			})
		})
	})
}

// collapsedLayout computes dropdown layout when dropdown is closed.
func (d *DropDown) collapsedLayout(gtx C) D {
	return d.collapsedLayoutInset.Layout(gtx, func(gtx C) D {
		return d.drawLayout(gtx, func(gtx C) D {
			var selectedItem DropDownItem
			if len(d.items) > 0 && d.selectedIndex > -1 {
				selectedItem = d.items[d.selectedIndex]
			} else {
				selectedItem = d.noSelectedItem
			}

			bodyLayout := LinearLayout{
				Width:     MatchParent,
				Height:    WrapContent,
				Padding:   layout.Inset{Right: values.MarginPadding5},
				Direction: d.CollapsedLayoutTextDirection,
			}

			// d.Hoverable is set after creating the dropdown but before drawing
			// the layout.
			d.clickable.Hoverable = d.Hoverable
			return d.itemLayout(gtx, d.selectedIndex, d.clickable, &selectedItem, 8, &bodyLayout)
		})
	})
}

func (d *DropDown) itemLayout(gtx C, index int, clickable *Clickable, item *DropDownItem, radius int, bodyLayout *LinearLayout) D {
	padding := values.MarginPadding10
	if item.Icon != nil {
		padding = values.MarginPadding8
	}

	return LinearLayout{
		Width:     MatchParent,
		Height:    WrapContent,
		Clickable: clickable,
		Padding:   layout.UniformInset(padding),
		Border:    Border{Radius: Radius(radius)},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if item.Icon == nil {
				return D{}
			}
			return layout.Inset{Right: values.MarginPadding5}.Layout(gtx, item.Icon.Layout24dp)
		}),
		layout.Flexed(1, func(gtx C) D {
			if item.DisplayFn != nil {
				return item.DisplayFn(gtx)
			}

			return bodyLayout.Layout2(gtx, func(gtx C) D {
				lbl := d.theme.Body2(item.Text)
				if !d.expanded && len(item.Text) > d.maxTextLeng {
					lbl.Text = item.Text[:d.maxTextLeng-3 /* subtract space for the ellipsis */] + "..."
				}
				lbl.Font.Weight = d.FontWeight
				lbl.TextSize = d.getTextSize(values.TextSize14)
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			if !item.PreventSelection {
				return d.layoutActiveIcon(gtx, index)
			}
			return D{}
		}),
	)
}

func (d *DropDown) layoutActiveIcon(gtx C, index int) D {
	var icon *Icon
	if !d.expanded {
		icon = NewIcon(d.dropdownIcon)
	} else if index == d.selectedIndex {
		icon = NewIcon(d.navigationIcon)
	}

	if icon == nil {
		return D{} // return early
	}

	icon.Color = d.theme.Color.Gray1
	if d.expanded && d.SelectedItemIconColor != nil {
		icon.Color = *d.SelectedItemIconColor
	}

	return icon.Layout(gtx, values.MarginPadding20)
}

// drawLayout wraps the page tx and sync section in a card layout
func (d *DropDown) drawLayout(gtx C, body layout.Widget) D {
	if d.Width <= 0 {
		d.Width = defaultDropdownWidth(d.revs)
	}
	d.linearLayout.Width = gtx.Dp(d.Width)

	if d.expanded {
		d.linearLayout.Background = d.theme.Color.Surface
		d.linearLayout.Padding = d.padding
		d.linearLayout.Shadow = d.shadow
	} else {
		if d.Background != nil {
			d.linearLayout.Background = *d.Background
		} else {
			d.linearLayout.Background = d.theme.Color.Gray2
		}
		d.linearLayout.Padding = layout.Inset{}
		d.linearLayout.Shadow = nil
	}

	if d.BorderWidth > 0 {
		d.linearLayout.Border.Width = d.BorderWidth
	}

	if d.BorderColor != nil {
		d.linearLayout.Border.Color = *d.BorderColor
	}

	return d.linearLayout.Layout2(gtx, body)
}

// Reslice the dropdowns
func ResliceDropdown(dropdowns []*DropDown, indexToRemove int) []*DropDown {
	return append(dropdowns[:indexToRemove], dropdowns[indexToRemove+1:]...)
}

// Display one dropdown at a time
func DisplayOneDropdown(dropdowns ...*DropDown) {
	var menus []*DropDown
	for i, menu := range dropdowns {
		if menu.clickable.Clicked() {
			menu.expanded = true
			menus = ResliceDropdown(dropdowns, i)
			for _, menusToClose := range menus {
				menusToClose.expanded = false
			}
		}
	}
}

// Items returns the items of the dropdown.
func (d *DropDown) Items() []DropDownItem {
	return d.items
}

func (d *DropDown) ItemsLen() int {
	return len(d.items)
}
