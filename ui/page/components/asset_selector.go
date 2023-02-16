package components

import (
	"image/color"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/text"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

const AssetTypeSelectorID = "AssetTypeSelectorID"

// AssetTypeSelector models a wiget for use for selecting asset types.
type AssetTypeSelector struct {
	openSelectorDialog *cryptomaterial.Clickable
	*assetTypeModal
	changed bool

	hint            string
	isDisableBorder bool
	background      color.NRGBA
}

// AssetType models asset types.
type AssetType struct {
	Type utils.AssetType
	Icon *cryptomaterial.Image
}

// assetTypeItem wraps the asset type in a clickable.
type assetTypeItem struct {
	item      *AssetType
	clickable *cryptomaterial.Clickable
}

type assetTypeModal struct {
	*load.Load
	*cryptomaterial.Modal

	selectedAssetType  *AssetType
	assetTypeCallback  func(*AssetType)
	dialogTitle        string
	onAssetTypeClicked func(*AssetType)
	assetTypeList      layout.List
	assetTypeItems     []*assetTypeItem
	eventQueue         event.Queue
	isCancelable       bool
}

// NewAssetTypeSelector creates an assetType selector component.
// It opens a modal to select a desired assetType.
func NewAssetTypeSelector(l *load.Load) *AssetTypeSelector {
	ats := &AssetTypeSelector{
		openSelectorDialog: l.Theme.NewClickable(true),
	}

	ats.assetTypeModal = newAssetTypeModal(l).
		assetTypeClicked(func(assetType *AssetType) {
			if ats.selectedAssetType != nil {
				if ats.selectedAssetType.Type.String() != assetType.Type.String() {
					ats.changed = true
				}
			}
			ats.SetSelectedAssetType(assetType)
			if ats.assetTypeCallback != nil {
				ats.assetTypeCallback(assetType)
			}
		})
	ats.assetTypeItems = ats.buildExchangeItems()
	ats.hint = values.String(values.StrSelectAssetType)
	return ats
}

// SupportedAssetTypes returns a slice containing all the asset types
// Currently supported.
func (ats *AssetTypeSelector) SupportedAssetTypes() []*AssetType {
	assetTypes := ats.WL.AssetsManager.AllAssetTypes()

	var assetType []*AssetType
	for _, at := range assetTypes {
		asset := &AssetType{
			Type: at,
			Icon: ats.setAssetTypeIcon(at.ToStringLower()),
		}

		assetType = append(assetType, asset)
	}

	return assetType
}

func (ats *AssetTypeSelector) setAssetTypeIcon(assetType string) *cryptomaterial.Image {
	switch assetType {
	case utils.DCRWalletAsset.ToStringLower():
		return ats.Theme.Icons.DecredLogo
	case utils.BTCWalletAsset.ToStringLower():
		return ats.Theme.Icons.BTC
	default:
		return ats.Theme.Icons.AddExchange
	}
}

// SetBackground sets backgound
func (ats *AssetTypeSelector) SetBackground(background color.NRGBA) *AssetType {
	ats.background = background
	return ats.selectedAssetType
}

// SetHint sets hint for selector
func (ats *AssetTypeSelector) SetHint(hint string) *AssetType {
	ats.hint = hint
	return ats.selectedAssetType
}

// DisableBorder will disable border on layout selected Asset type.
func (ats *AssetTypeSelector) DisableBorder() *AssetType {
	ats.isDisableBorder = true
	return ats.selectedAssetType
}

// SelectedAssetType returns the currently selected Asset type.
func (ats *AssetTypeSelector) SelectedAssetType() *AssetType {
	return ats.selectedAssetType
}

// SetSelectedAssetType sets assetType as the current selected asset type.
func (ats *AssetTypeSelector) SetSelectedAssetType(assetType *AssetType) {
	ats.selectedAssetType = assetType
}

// Title Sets the title of the asset type list dialog.
func (ats *AssetTypeSelector) Title(title string) *AssetTypeSelector {
	ats.dialogTitle = title
	return ats
}

// AssetTypeSelected sets the callback executed when an asset type is selected.
func (ats *AssetTypeSelector) AssetTypeSelected(callback func(*AssetType)) *AssetTypeSelector {
	ats.assetTypeCallback = callback
	return ats
}

func (ats *AssetTypeSelector) Handle(window app.WindowNavigator) {
	for ats.openSelectorDialog.Clicked() {
		ats.title(ats.dialogTitle)
		window.ShowModal(ats.assetTypeModal)
	}
}

func (ats *AssetTypeSelector) Layout(window app.WindowNavigator, gtx C) D {
	ats.Handle(window)

	linearLayout := cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     cryptomaterial.WrapContent,
		Padding:    layout.UniformInset(values.MarginPadding12),
		Background: ats.background,
		Clickable:  ats.openSelectorDialog,
	}
	if !ats.isDisableBorder {
		linearLayout.Border = cryptomaterial.Border{
			Width:  values.MarginPadding2,
			Color:  ats.Theme.Color.Gray2,
			Radius: cryptomaterial.Radius(8),
		}
	}
	return linearLayout.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if ats.selectedAssetType == nil {
				return D{}
			}
			return layout.Inset{
				Right: values.MarginPadding8,
			}.Layout(gtx, ats.selectedAssetType.Icon.Layout24dp)
		}),
		layout.Rigid(func(gtx C) D {
			txt := ats.Theme.Label(values.TextSize16, ats.hint)
			txt.Color = ats.Theme.Color.Gray7
			if ats.selectedAssetType != nil {
				txt = ats.Theme.Label(values.TextSize16, ats.selectedAssetType.Type.String())
				txt.Color = ats.Theme.Color.Text
			}
			return txt.Layout(gtx)

		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						ic := cryptomaterial.NewIcon(ats.Theme.Icons.DropDownIcon)
						ic.Color = ats.Theme.Color.Gray1
						return ic.Layout(gtx, values.MarginPadding20)
					}),
				)
			})
		}),
	)
}

// newAssetTypeModal return a modal used for drawing the asset type list.
func newAssetTypeModal(l *load.Load) *assetTypeModal {
	atm := &assetTypeModal{
		Load:          l,
		Modal:         l.Theme.ModalFloatTitle(values.String(values.StrSelectAServer)),
		assetTypeList: layout.List{Axis: layout.Vertical},
		isCancelable:  true,
		dialogTitle:   values.String(values.StrSelectAssetType),
	}

	atm.Modal.ShowScrollbar(true)
	return atm
}

func (ats *AssetTypeSelector) buildExchangeItems() []*assetTypeItem {
	exList := ats.SupportedAssetTypes()
	exItems := make([]*assetTypeItem, 0)
	for _, v := range exList {
		exItems = append(exItems, &assetTypeItem{
			item:      v,
			clickable: ats.Theme.NewClickable(true),
		})
	}
	return exItems
}

func (atm *assetTypeModal) OnResume() {}

func (atm *assetTypeModal) Handle() {
	if atm.eventQueue != nil {
		for _, assetTypeItem := range atm.assetTypeItems {
			for assetTypeItem.clickable.Clicked() {
				atm.onAssetTypeClicked(assetTypeItem.item)
				atm.Dismiss()
			}
		}
	}

	if atm.Modal.BackdropClicked(atm.isCancelable) {
		atm.Dismiss()
	}
}

func (atm *assetTypeModal) title(title string) *assetTypeModal {
	atm.dialogTitle = title
	return atm
}

func (atm *assetTypeModal) assetTypeClicked(callback func(*AssetType)) *assetTypeModal {
	atm.onAssetTypeClicked = callback
	return atm
}

func (atm *assetTypeModal) Layout(gtx C) D {
	atm.eventQueue = gtx
	w := []layout.Widget{
		func(gtx C) D {
			titleTxt := atm.Theme.Label(values.TextSize20, atm.dialogTitle)
			titleTxt.Color = atm.Theme.Color.Text
			titleTxt.Font.Weight = text.SemiBold
			return layout.Inset{
				Top: values.MarginPaddingMinus15,
			}.Layout(gtx, titleTxt.Layout)
		},
		func(gtx C) D {
			return layout.Stack{Alignment: layout.NW}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return atm.assetTypeList.Layout(gtx, len(atm.assetTypeItems), func(gtx C, index int) D {
						return atm.modalListItemLayout(gtx, atm.assetTypeItems[index])
					})
				}),
			)
		},
	}

	return atm.Modal.Layout(gtx, w)
}

func (atm *assetTypeModal) modalListItemLayout(gtx C, assetTypeItem *assetTypeItem) D {
	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.WrapContent,
		Margin:    layout.Inset{Bottom: values.MarginPadding4},
		Padding:   layout.Inset{Top: values.MarginPadding8, Bottom: values.MarginPadding8},
		Clickable: assetTypeItem.clickable,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding18,
			}.Layout(gtx, assetTypeItem.item.Icon.Layout24dp)
		}),
		layout.Rigid(func(gtx C) D {
			assetTypeName := atm.Theme.Label(values.TextSize18, assetTypeItem.item.Type.String())
			assetTypeName.Color = atm.Theme.Color.Text
			assetTypeName.Font.Weight = text.Normal
			return assetTypeName.Layout(gtx)
		}),
	)
}

func (atm *assetTypeModal) OnDismiss() {}
