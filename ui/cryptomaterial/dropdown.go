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
	DropdownBasePos uint = 0
)

var MaxWidth = unit.Dp(800)

type DropDown struct {
	theme          *Theme
	items          []DropDownItem
	isOpen         bool
	backdrop       *widget.Clickable
	Position       uint
	revs           bool
	selectedIndex  int
	Color          color.NRGBA
	background     color.NRGBA
	dropdownIcon   *widget.Icon
	navigationIcon *widget.Icon
	clickable      *Clickable

	group               uint
	closeAllDropdown    func(group uint)
	isOpenDropdownGroup func(group uint) bool
	Width               unit.Dp
	linearLayout        *LinearLayout
	padding             layout.Inset
	shadow              *Shadow

	noSelectedItemText  string
	extraDisplay        func(gtx C) D
	FontWeight          font.Weight
	BorderWidth         unit.Dp
	BorderColor         *color.NRGBA
	NavigationIconColor *color.NRGBA
	Hoverable           bool
}

type DropDownItem struct {
	Text      string
	Icon      *Image
	clickable *Clickable
	// DisplayFn is an alternate display function that conforms to the dropdown
	// item list display.
	DisplayFn func(gtx C) D
}

// DropDown returns a dropdown component. {pos} parameter signifies the position
// of the dropdown in a dropdown group on the UI, the first dropdown should be assigned
// pos 0, next 1..etc. incorrectly assigned Dropdown pos will result in inconsistent
// dropdown backdrop.
func (t *Theme) DropDown(items []DropDownItem, group uint, pos uint) *DropDown {
	d := &DropDown{
		theme:          t,
		isOpen:         false,
		Position:       pos,
		selectedIndex:  0,
		Color:          t.Color.Gray2,
		background:     t.Color.Surface,
		dropdownIcon:   t.dropDownIcon,
		navigationIcon: t.navigationCheckIcon,
		clickable:      t.NewClickable(true),
		backdrop:       new(widget.Clickable),

		group:               group,
		closeAllDropdown:    t.closeAllDropdownMenus,
		isOpenDropdownGroup: t.isOpenDropdownGroup,
		linearLayout: &LinearLayout{
			Width:  WrapContent,
			Height: WrapContent,
			Border: Border{Radius: Radius(8)},
		},
		padding: layout.Inset{Top: values.MarginPadding8, Bottom: values.MarginPadding8},
		shadow:  t.Shadow(),
	}

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

func (d *DropDown) ClearSelection(text string) {
	d.selectedIndex = -1
	d.noSelectedItemText = text
}

func (d *DropDown) SelectedIndex() int {
	return d.selectedIndex
}

func (d *DropDown) Len() int {
	return len(d.items)
}

func (d *DropDown) SetExtraDisplay(extraDisplay func(gtx C) D) {
	d.extraDisplay = extraDisplay
}

func (d *DropDown) handleEvents() {
	if d.isOpen {
		for i := range d.items {
			index := i
			for d.items[index].clickable.Clicked() {
				d.selectedIndex = index
				d.isOpen = false
				break
			}
		}
	} else {
		for d.clickable.Clicked() {
			d.isOpen = true
		}
	}

	for d.backdrop.Clicked() {
		d.closeAllDropdown(d.group)
	}
}

func (d *DropDown) Changed() bool {
	if d.isOpen {
		for i := range d.items {
			index := i
			for d.items[index].clickable.Clicked() {
				d.selectedIndex = index
				d.isOpen = false
				return true
			}
		}
	}

	return false
}

func (d *DropDown) layoutActiveIcon(gtx C, index int) D {
	var icon *Icon
	if !d.isOpen {
		icon = NewIcon(d.dropdownIcon)
		icon.Color = d.theme.Color.Gray1
	} else if index == d.selectedIndex {
		icon = NewIcon(d.navigationIcon)
		if d.NavigationIconColor != nil {
			icon.Color = *d.NavigationIconColor
		} else {
			icon.Color = d.theme.Color.Gray1
		}
	}

	return layout.E.Layout(gtx, func(gtx C) D {
		if icon == nil {
			return D{}
		}
		return icon.Layout(gtx, values.MarginPadding20)
	})
}

func (d *DropDown) layoutOption(gtx C, itemIndex int) D {
	item := d.items[itemIndex]
	radius := Radius(0)
	clickable := item.clickable
	if !d.isOpen {
		radius = Radius(8)
		clickable = d.clickable
		clickable.Hoverable = d.Hoverable
	}

	padding := values.MarginPadding10
	if d.isOpen {
		padding = values.MarginPadding8
	}

	dropdownWidth := gtx.Dp(d.Width)
	dropdownItemWidth := dropdownWidth
	if dropdownWidth <= 0 {
		dropdownWidth = gtx.Dp(defaultDropdownWidth(d.revs))
	}

	return LinearLayout{
		Width:     dropdownWidth,
		Height:    WrapContent,
		Clickable: clickable,
		Padding:   layout.UniformInset(padding),
		Border:    Border{Radius: radius},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if item.Icon == nil {
				return D{}
			}

			dropdownItemWidth -= gtx.Dp(values.MarginPadding24) // account for the dropdown Icon
			return item.Icon.Layout20dp(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = dropdownItemWidth - gtx.Dp(values.MarginPadding50) // give some space for the dropdown Icon
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if item.DisplayFn != nil {
				return item.DisplayFn(gtx)
			}

			return layout.Inset{
				Right: unit.Dp(5),
				Left:  unit.Dp(5),
			}.Layout(gtx, func(gtx C) D {
				lbl := d.theme.Body2(item.Text)
				if !d.isOpen && len(item.Text) > 20 {
					lbl.Text = item.Text[:20-3] + "..."
				}
				lbl.Font.Weight = d.FontWeight
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return d.layoutActiveIcon(gtx, itemIndex)
		}),
	)
}

// defaultDropdownWidth returns the default width for a dropdown depending on
// it's position.
func defaultDropdownWidth(reversePosition bool) unit.Dp {
	if reversePosition {
		return values.MarginPadding140
	}
	return values.MarginPadding180
}

func (d *DropDown) Layout(gtx C, dropPos int, reversePos bool) D {
	d.handleEvents()

	iLeft := dropPos
	iRight := 0
	alig := layout.NW
	d.revs = reversePos
	if reversePos {
		alig = layout.NE
		iLeft = 10
		iRight = dropPos
	}

	if d.Position == DropdownBasePos && d.isOpenDropdownGroup(d.group) {
		maxY := unit.Dp(len(d.items)) * values.MarginPadding50
		if d.extraDisplay != nil {
			maxY += values.MarginPadding50
		}

		gtx.Constraints.Max.Y = gtx.Dp(maxY)
		if d.isOpen {
			return d.backdrop.Layout(gtx, func(gtx C) D {
				return layout.Stack{Alignment: alig}.Layout(gtx,
					layout.Stacked(func(gtx C) D {
						return d.openedLayout(gtx, iLeft, iRight)
					}),
				)
			})
		}

		return d.backdrop.Layout(gtx, func(gtx C) D {
			return layout.Stack{Alignment: alig}.Layout(gtx,
				layout.Stacked(func(gtx C) D {
					return d.closedLayout(gtx, iLeft, iRight)
				}),
			)
		})

	} else if d.isOpen {
		return layout.Stack{Alignment: alig}.Layout(gtx,
			layout.Stacked(func(gtx C) D {
				return d.openedLayout(gtx, iLeft, iRight)
			}),
		)
	}

	return layout.Stack{Alignment: alig}.Layout(gtx,
		layout.Stacked(func(gtx C) D {
			return d.closedLayout(gtx, iLeft, iRight)
		}),
	)
}

// openedLayout computes dropdown layout when dropdown is opened.
func (d *DropDown) openedLayout(gtx C, iLeft int, iRight int) D {
	return layout.Inset{
		Left:  unit.Dp(float32(iLeft)),
		Right: unit.Dp(float32(iRight)),
	}.Layout(gtx, func(gtx C) D {
		return d.dropDownItemMenu(gtx)
	})
}

// closedLayout computes dropdown layout when dropdown is closed.
func (d *DropDown) closedLayout(gtx C, iLeft int, iRight int) D {
	return layout.Inset{
		Left:  unit.Dp(float32(iLeft)),
		Right: unit.Dp(float32(iRight)),
	}.Layout(gtx, func(gtx C) D {
		return d.drawLayout(gtx, func(gtx C) D {
			lay := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if len(d.items) == 0 {
						if d.extraDisplay != nil {
							return d.extraDisplay(gtx)
						} else {
							return d.theme.Body2(d.noSelectedItemText).Layout(gtx)
						}
					}
					return d.layoutOption(gtx, d.selectedIndex)
				}))
			if d.Width <= 0 {
				d.Width = defaultDropdownWidth(d.revs)
			}
			return lay
		})
	})
}

func (d *DropDown) dropDownItemMenu(gtx C) D {
	listItems := len(d.items)
	return d.drawLayout(gtx, func(gtx C) D {
		list := &layout.List{Axis: layout.Vertical}
		return list.Layout(gtx, listItems, func(gtx C, index int) D {
			if (index == listItems-1 || listItems == 0) && d.extraDisplay != nil {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return d.layoutOption(gtx, index)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dropdownWidth := gtx.Dp(d.Width)
						dropdownItemWidth := dropdownWidth
						if dropdownWidth <= 0 {
							dropdownWidth = gtx.Dp(defaultDropdownWidth(d.revs))
						}

						gtx.Constraints.Max.X = dropdownItemWidth
						return d.extraDisplay(gtx)
					}),
				)
			} else if listItems != 0 {
				return d.layoutOption(gtx, index)
			}

			return D{}
		})
	})
}

// drawLayout wraps the page tx and sync section in a card layout
func (d *DropDown) drawLayout(gtx C, body layout.Widget) D {
	if d.isOpen {
		d.linearLayout.Background = d.background
		d.linearLayout.Padding = d.padding
		d.linearLayout.Shadow = d.shadow
	} else {
		d.linearLayout.Background = d.Color
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
			menu.isOpen = true
			menus = ResliceDropdown(dropdowns, i)
			for _, menusToClose := range menus {
				menusToClose.isOpen = false
			}
		}
	}
}
