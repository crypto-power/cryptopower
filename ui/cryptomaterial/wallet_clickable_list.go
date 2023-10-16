package cryptomaterial

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/crypto-power/cryptopower/libwallet/utils"
)

type SelectedWalletItem struct {
	AssetType utils.AssetType
	Index     int
}

type WalletClickableList struct {
	layout.List
	theme           *Theme
	clickables      []*Clickable
	Radius          CornerRadius // this radius is used by the clickable
	selectedItem    int
	DividerHeight   unit.Dp
	IsShadowEnabled bool
	IsHoverable     bool
	ItemIDs         map[*Clickable]SelectedWalletItem
}

func (t *Theme) NewWalletClickableList(axis layout.Axis) *WalletClickableList {
	click := &WalletClickableList{
		theme:        t,
		selectedItem: -1,
		List: layout.List{
			Axis: axis,
		},
		IsHoverable: true,
	}

	return click
}

func (cl *WalletClickableList) ItemClicked() (bool, SelectedWalletItem) {
	for _, clickable := range cl.clickables {
		if clickable.Clicked() {
			if itemID, exists := cl.ItemIDs[clickable]; exists {
				return true, itemID
			}
		}
	}
	return false, SelectedWalletItem{}
}

func (cl *WalletClickableList) handleClickables(itemIDs []SelectedWalletItem) {
	if len(cl.clickables) != len(itemIDs) {

		// Resize the clickables slice to match the size of the provided itemIDs
		cl.clickables = make([]*Clickable, len(itemIDs))

		// Initialize the ItemIDs map
		cl.ItemIDs = make(map[*Clickable]SelectedWalletItem)

		for i, itemID := range itemIDs {
			clickable := cl.theme.NewClickable(cl.IsHoverable)
			cl.clickables[i] = clickable
			cl.ItemIDs[clickable] = itemID
		}
	}

	// Handle clicked items (this was already part of your original handleClickables method)
	for index, clickable := range cl.clickables {
		for clickable.Clicked() {
			cl.selectedItem = index
		}
	}
}

func (cl *WalletClickableList) Layout(gtx layout.Context, itemIDs []SelectedWalletItem, w layout.ListElement) layout.Dimensions {
	cl.handleClickables(itemIDs)
	count := len(itemIDs) // Using len(itemIDs) as the count.
	return cl.List.Layout(gtx, count, func(gtx C, i int) D {
		if cl.IsShadowEnabled && cl.clickables[i].button.Hovered() {
			shadow := cl.theme.Shadow()
			shadow.SetShadowRadius(14)
			shadow.SetShadowElevation(5)
			return shadow.Layout(gtx, func(gtx C) D {
				return cl.row(gtx, count, i, w)
			})
		}
		return cl.row(gtx, count, i, w)
	})
}

func (cl *WalletClickableList) row(gtx layout.Context, count int, i int, w layout.ListElement) layout.Dimensions {
	if i == 0 { // first item
		cl.clickables[i].Radius.TopLeft = cl.Radius.TopLeft
		cl.clickables[i].Radius.TopRight = cl.Radius.TopRight
	}
	if i == count-1 { // last item
		cl.clickables[i].Radius.BottomLeft = cl.Radius.BottomLeft
		cl.clickables[i].Radius.BottomRight = cl.Radius.BottomRight
	}
	row := cl.clickables[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return w(gtx, i)
	})

	// add divider to all rows except last
	if i < (count-1) && cl.DividerHeight > 0 {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y += gtx.Dp(cl.DividerHeight)
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
		)
	}
	return row
}
