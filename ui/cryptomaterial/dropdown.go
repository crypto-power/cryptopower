package cryptomaterial

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
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

	list   *widget.List
	scroll ListStyle

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

func (t *Theme) NewCommonDropDown(items []DropDownItem, lastSelectedItem *DropDownItem, width unit.Dp, group uint, reversePos bool) *DropDown {
	d := t.DropDown(items, lastSelectedItem, group, reversePos)
	d.FontWeight = font.SemiBold
	d.SelectedItemIconColor = &t.Color.Primary
	d.BorderWidth = 2
	d.Width = width
	d.ExpandedLayoutInset = layout.Inset{Top: values.MarginPadding44}
	d.MakeCollapsedLayoutVisibleWhenExpanded = true
	return d
}

// DropDown is like DropdownWithCustomPos but uses default values for
// groupPosition, and dropdownInset parameters. Provide lastSelectedItem to
// select the item that matches the lastSelectedItem parameter. It's a no-op if
// the item is not found or if the item has PreventSelection set to true.
func (t *Theme) DropDown(items []DropDownItem, lastSelectedItem *DropDownItem, group uint, reversePos bool) *DropDown {
	d := t.DropdownWithCustomPos(items, group, 0, 0, reversePos)
	if lastSelectedItem != nil {
		for index, item := range d.items {
			if item.Text == lastSelectedItem.Text && !item.PreventSelection {
				d.selectedIndex = index
			}
		}
	}
	return d
}

// DropdownWithCustomPos returns a dropdown component. {groupPosition} parameter
// signifies the position of the dropdown in a dropdown group on the UI, the
// first dropdown should be assigned pos 0, next 1..etc. incorrectly assigned
// Dropdown pos will result in inconsistent dropdown backdrop. {dropdownInset}
// parameter is the left  inset for the dropdown if {reversePos} is false, else
// it'll become the right inset for the dropdown.
func (t *Theme) DropdownWithCustomPos(items []DropDownItem, group uint, groupPosition uint, dropdownInset int, reversePos bool) *DropDown {
	d := &DropDown{
		theme:                    t,
		expanded:                 false,
		GroupPosition:            groupPosition,
		selectedIndex:            0,
		dropdownIcon:             t.dropDownIcon,
		navigationIcon:           t.navigationCheckIcon,
		Hoverable:                true,
		clickable:                t.NewClickable(true),
		Width:                    WrapContent,
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
		list: &widget.List{
			List: layout.List{Axis: layout.Vertical, Alignment: layout.Middle},
		},
	}
	d.scroll = t.List(d.list)
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

func (d *DropDown) SetSelectedValue(selectedValue string) {
	for index, item := range d.items {
		if item.Text == selectedValue {
			d.selectedIndex = index
			return
		}
	}
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

func (d *DropDown) handleEvents(gtx C) {
	if d.expanded {
		if d.clickable.Clicked(gtx) {
			d.expanded = false
		}
		for i := range d.items {
			index := i
			item := d.items[index]
			if item.clickable.Clicked(gtx) {
				d.expanded = false
				if !item.PreventSelection {
					d.selectedIndex = index
				}
				break
			}
		}
	} else {
		if d.clickable.Clicked(gtx) {
			d.expanded = true
		}
	}
}

func (d *DropDown) Changed(gtx C) bool {
	if d.expanded {
		if d.clickable.Clicked(gtx) {
			d.expanded = false
		}
		for i := range d.items {
			index := i
			item := d.items[index]
			if item.clickable.Clicked(gtx) {
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
		if d.theme.DropdownBackdrop.Clicked(gtx) {
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
	d.handleEvents(gtx)
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
			if len(d.items) == 0 {
				return D{}
			}
			return d.expandedLayout(gtx)
		}))
	}

	return layout.Stack{Alignment: d.expandedViewAlignment}.Layout(gtx, layoutContents...)
}

// expandedLayout computes dropdown layout when dropdown is opened.
func (d *DropDown) expandedLayout(gtx C) D {
	m := op.Record(gtx.Ops)
	gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	d.updateDropdownWidth(gtx, true)
	d.updateDropdownBackground(true)
	d.ExpandedLayoutInset.Layout(gtx, func(gtx C) D {
		return d.linearLayout.Layout2(gtx, d.listItemLayout)
	})
	op.Defer(gtx.Ops, m.Stop())
	return D{}
}

func (d *DropDown) listItemLayout(gtx C) D {
	return d.scroll.Layout(gtx, len(d.items), func(gtx C, index int) D {
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
}

// collapsedLayout computes dropdown layout when dropdown is closed.
func (d *DropDown) collapsedLayout(gtx C) D {
	d.updateDropdownWidth(gtx, false)
	d.updateDropdownBackground(false)
	return d.collapsedLayoutInset.Layout(gtx, func(gtx C) D {
		return d.linearLayout.Layout2(gtx, func(gtx C) D {
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
			gtx.Constraints.Min.Y = item.getItemSize(gtx, d).Y
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Spacing: layout.SpaceAround}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Right: values.MarginPadding5}.Layout(gtx, item.Icon.Layout24dp)
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			if item.DisplayFn != nil {
				return item.DisplayFn(gtx)
			}

			return bodyLayout.Layout2(gtx, d.renderItemLabel(item.Text))
		}),
		layout.Rigid(func(gtx C) D {
			if !item.PreventSelection {
				return d.layoutActiveIcon(gtx, item, index)
			}
			return D{
				Size: image.Point{
					X: gtx.Dp(values.MarginPadding20),
				},
			}
		}),
	)
}

func (d *DropDown) layoutActiveIcon(gtx C, item *DropDownItem, index int) D {
	var icon *Icon
	if !d.expanded {
		icon = NewIcon(d.dropdownIcon)
	} else if index == d.selectedIndex {
		icon = NewIcon(d.navigationIcon)
	}

	if icon == nil {
		return D{
			Size: image.Point{
				X: gtx.Dp(values.MarginPadding20),
			},
		} // return early
	}

	icon.Color = d.theme.Color.Gray1
	if d.expanded && d.SelectedItemIconColor != nil {
		icon.Color = *d.SelectedItemIconColor
	}
	gtx.Constraints.Min.Y = item.getItemSize(gtx, d).Y
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Spacing: layout.SpaceAround}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return icon.Layout(gtx, values.MarginPadding20)
		}),
	)
}

// Reslice the dropdowns
func ResliceDropdown(dropdowns []*DropDown, indexToRemove int) []*DropDown {
	return append(dropdowns[:indexToRemove], dropdowns[indexToRemove+1:]...)
}

// Display one dropdown at a time
func DisplayOneDropdown(gtx C, dropdowns ...*DropDown) {
	var menus []*DropDown
	for i, menu := range dropdowns {
		if menu.clickable.Clicked(gtx) {
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

func (d *DropDown) SetItems(items []DropDownItem) {
	d.items = make([]DropDownItem, 0)
	d.selectedIndex = 0
	for _, i := range items {
		i.clickable = d.theme.NewClickable(true)
		d.items = append(d.items, i)
	}
}

func (d *DropDown) ItemsLen() int {
	return len(d.items)
}

func (i *DropDownItem) getItemSize(gtx C, d *DropDown) image.Point {
	tmpGtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: gtx.Constraints,
		Metric:      gtx.Metric,
	}
	if i.DisplayFn != nil {
		return i.DisplayFn(tmpGtx).Size
	}
	return LinearLayout{
		Width:     MatchParent,
		Height:    WrapContent,
		Padding:   layout.Inset{Right: values.MarginPadding5},
		Direction: layout.W,
	}.Layout2(tmpGtx, d.renderItemLabel(i.Text)).Size
}

func (d *DropDown) renderItemLabel(text string) layout.Widget {
	return func(gtx C) D {
		lbl := d.theme.Body2(text)
		lbl.MaxLines = 1
		if !d.expanded && len(text) > d.maxTextLeng {
			lbl.Text = text[:d.maxTextLeng-3 /* subtract space for the ellipsis */] + "..."
		}
		lbl.Font.Weight = d.FontWeight
		lbl.TextSize = d.getTextSize(values.TextSize14)
		return lbl.Layout(gtx)
	}
}

func (d *DropDown) getCurrentSize(gtx C) image.Point {
	var selectedItem DropDownItem
	if len(d.items) > 0 && d.selectedIndex > -1 {
		selectedItem = d.items[d.selectedIndex]
	} else {
		selectedItem = d.noSelectedItem
	}
	return selectedItem.getItemSize(gtx, d)
}

func (d *DropDown) getMaxWidth(gtx C) int {
	maxWidth := 0
	if len(d.items) > 0 {
		for _, item := range d.items {
			itemWidth := item.getItemSize(gtx, d).X
			if itemWidth > maxWidth {
				maxWidth = itemWidth
			}
		}
	}
	return maxWidth
}

func (d *DropDown) updateDropdownBackground(expanded bool) {
	if expanded {
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
}

func (d *DropDown) updateDropdownWidth(gtx C, expanded bool) {
	switch d.Width {
	case WrapContent:
		width := 0
		if expanded {
			width = d.getMaxWidth(gtx)
		} else {
			width = d.getCurrentSize(gtx).X
		}
		if width > 0 {
			d.linearLayout.Width = width + gtx.Dp(values.MarginPadding70)
		} else {
			d.linearLayout.Width = gtx.Dp(defaultDropdownWidth(d.revs))
		}
	case MatchParent:
		d.linearLayout.Width = MatchParent
	case 0:
		d.linearLayout.Width = gtx.Dp(defaultDropdownWidth(d.revs))
	default:
		d.linearLayout.Width = gtx.Dp(d.Width)

	}
}
