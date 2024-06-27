package components

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const AssetTypeSelectorID = "AssetTypeSelectorID"

// AssetTypeSelector models a wiget for use for selecting asset types.
type AssetTypeSelector struct {
	openSelectorDialog *cryptomaterial.Clickable
	*assetTypeModal

	hint            string
	isDisableBorder bool
	background      color.NRGBA
}

// assetTypeItem wraps the asset type in a clickable.
type AssetTypeItem struct {
	Type      libutils.AssetType
	Icon      *cryptomaterial.Image
	clickable *cryptomaterial.Clickable
}

type assetTypeModal struct {
	*load.Load
	*cryptomaterial.Modal

	selectedAssetType  *AssetTypeItem
	assetTypeCallback  func(*AssetTypeItem) bool
	dialogTitle        string
	onAssetTypeClicked func(*AssetTypeItem)
	assetTypeList      layout.List
	assetTypeItems     []*AssetTypeItem
	eventSoruce        input.Source
	isCancelable       bool
}

// NewAssetTypeSelector creates an assetType selector component.
// It opens a modal to select a desired assetType.
func NewAssetTypeSelector(l *load.Load) *AssetTypeSelector {
	ats := &AssetTypeSelector{
		openSelectorDialog: l.Theme.NewClickable(true),
	}

	ats.assetTypeModal = newAssetTypeModal(l).
		assetTypeClicked(func(assetType *AssetTypeItem) {
			ok := true
			if ats.assetTypeCallback != nil {
				ok = ats.assetTypeCallback(assetType)
			}

			if ok && (ats.selectedAssetType == nil || ats.selectedAssetType.Type.String() != assetType.Type.String()) {
				ats.selectedAssetType = assetType
			}
		})
	ats.assetTypeItems = ats.buildExchangeItems()
	ats.hint = values.String(values.StrSelectAssetType)
	return ats
}

// SupportedAssetTypes returns a slice containing all the asset types
// Currently supported.
func (ats *AssetTypeSelector) SupportedAssetTypes() []*AssetTypeItem {
	assetTypes := ats.AssetsManager.AllAssetTypes()

	var assetType []*AssetTypeItem
	for _, at := range assetTypes {
		asset := &AssetTypeItem{
			Type: at,
			Icon: ats.setAssetTypeIcon(at),
		}

		assetType = append(assetType, asset)
	}

	return assetType
}

func (ats *AssetTypeSelector) setAssetTypeIcon(assetType libutils.AssetType) *cryptomaterial.Image {
	image := CoinImageBySymbol(ats.Load, assetType, false)
	if image != nil {
		return image
	}
	return ats.Theme.Icons.AddExchange
}

// SetBackground sets the asset background colour
func (ats *AssetTypeSelector) SetBackground(background color.NRGBA) *AssetTypeSelector {
	ats.background = background
	return ats
}

// SetHint sets hint for selector
func (ats *AssetTypeSelector) SetHint(hint string) *AssetTypeSelector {
	ats.hint = hint
	return ats
}

// DisableBorder will disable border on layout selected Asset type.
func (ats *AssetTypeSelector) DisableBorder() *AssetTypeSelector {
	ats.isDisableBorder = true
	return ats
}

// SelectedAssetType returns the currently selected Asset type.
func (ats *AssetTypeSelector) SelectedAssetType() *libutils.AssetType {
	if ats.selectedAssetType == nil {
		return nil
	}
	return &ats.selectedAssetType.Type
}

// SetSelectedAssetType sets assetType as the current selected asset type.
func (ats *AssetTypeSelector) SetSelectedAssetType(assetType libutils.AssetType) {
	asset := &AssetTypeItem{
		Type:      assetType,
		Icon:      ats.setAssetTypeIcon(assetType),
		clickable: ats.Theme.NewClickable(true),
	}
	ats.selectedAssetType = asset
}

// Title Sets the title of the asset type list dialog.
func (ats *AssetTypeSelector) Title(title string) *AssetTypeSelector {
	ats.dialogTitle = title
	return ats
}

// AssetTypeSelected sets the callback executed when an asset type is selected.
func (ats *AssetTypeSelector) AssetTypeSelected(callback func(*AssetTypeItem) bool) *AssetTypeSelector {
	ats.assetTypeCallback = callback
	return ats
}

func (ats *AssetTypeSelector) Handle(gtx C, window app.WindowNavigator) {
	for ats.openSelectorDialog.Clicked(gtx) {
		ats.title(ats.dialogTitle)
		window.ShowModal(ats.assetTypeModal)
	}
}

func (ats *AssetTypeSelector) Layout(window app.WindowNavigator, gtx C) D {
	ats.Handle(gtx, window)

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
			txt := ats.Theme.Label(values.TextSize14, ats.hint)
			txt.Color = ats.Theme.Color.Gray7
			if ats.selectedAssetType != nil {
				txt = ats.Theme.Label(values.TextSizeTransform(ats.IsMobileView(), values.TextSize16), ats.selectedAssetType.Type.String())
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
		Modal:         l.Theme.ModalFloatTitle(values.String(values.StrSelectAServer), l.IsMobileView()),
		assetTypeList: layout.List{Axis: layout.Vertical},
		isCancelable:  true,
		dialogTitle:   values.String(values.StrSelectAssetType),
	}

	atm.Modal.ShowScrollbar(true)
	return atm
}

func (ats *AssetTypeSelector) buildExchangeItems() []*AssetTypeItem {
	exList := ats.SupportedAssetTypes()
	for _, v := range exList {
		v.clickable = ats.Theme.NewClickable(true)
	}
	return exList
}

func (atm *assetTypeModal) OnResume() {}

func (atm *assetTypeModal) Handle(gtx C) {
	for _, assetTypeItem := range atm.assetTypeItems {
		for assetTypeItem.clickable.Clicked(gtx) {
			atm.onAssetTypeClicked(assetTypeItem)
			atm.Dismiss()
		}
	}

	if atm.Modal.BackdropClicked(gtx, atm.isCancelable) {
		atm.Dismiss()
	}
}

func (atm *assetTypeModal) title(title string) *assetTypeModal {
	atm.dialogTitle = title
	return atm
}

func (atm *assetTypeModal) assetTypeClicked(callback func(*AssetTypeItem)) *assetTypeModal {
	atm.onAssetTypeClicked = callback
	return atm
}

func (atm *assetTypeModal) Layout(gtx C) D {
	atm.eventSoruce = gtx.Source
	w := []layout.Widget{
		func(gtx C) D {
			titleTxt := atm.Theme.Label(values.TextSize20, atm.dialogTitle)
			titleTxt.Color = atm.Theme.Color.Text
			titleTxt.Font.Weight = font.SemiBold
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

func (atm *assetTypeModal) modalListItemLayout(gtx C, assetTypeItem *AssetTypeItem) D {
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
			}.Layout(gtx, assetTypeItem.Icon.Layout24dp)
		}),
		layout.Rigid(func(gtx C) D {
			assetTypeName := atm.Theme.Label(values.TextSize18, assetTypeItem.Type.String())
			assetTypeName.Color = atm.Theme.Color.Text
			assetTypeName.Font.Weight = font.Normal
			return assetTypeName.Layout(gtx)
		}),
	)
}

func (atm *assetTypeModal) OnDismiss() {}
