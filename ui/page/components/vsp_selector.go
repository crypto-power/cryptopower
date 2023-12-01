package components

import (
	"context"
	"fmt"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

type VSPSelector struct {
	*load.Load
	dcrWallet *dcr.Asset

	dialogTitle string

	changed      bool
	showVSPModal *cryptomaterial.Clickable
	selectedVSP  *dcr.VSP
}

func NewVSPSelector(l *load.Load, dcrWallet *dcr.Asset) *VSPSelector {
	v := &VSPSelector{
		Load:         l,
		dcrWallet:    dcrWallet,
		showVSPModal: l.Theme.NewClickable(true),
	}
	return v
}

func (v *VSPSelector) Title(title string) *VSPSelector {
	v.dialogTitle = title
	return v
}

func (v *VSPSelector) Changed() bool {
	changed := v.changed
	v.changed = false
	return changed
}

func (v *VSPSelector) SelectVSP(vspHost string) {
	for _, vsp := range v.dcrWallet.KnownVSPs() {
		if vsp.Host == vspHost {
			v.changed = true
			v.selectedVSP = vsp
			break
		}
	}
}

func (v *VSPSelector) SelectedVSP() *dcr.VSP {
	return v.selectedVSP
}

func (v *VSPSelector) handle(window app.WindowNavigator) {
	if v.showVSPModal.Clicked() {
		modal := newVSPSelectorModal(v.Load, v.dcrWallet).
			title(values.String(values.StrVotingServiceProvider)).
			vspSelected(func(info *dcr.VSP) {
				v.SelectVSP(info.Host)
			})
		window.ShowModal(modal)
	}
}

func (v *VSPSelector) Layout(window app.WindowNavigator, gtx layout.Context) layout.Dimensions {
	v.handle(window)

	border := widget.Border{
		Color:        v.Theme.Color.Gray2,
		CornerRadius: values.MarginPadding8,
		Width:        values.MarginPadding2,
	}

	return border.Layout(gtx, func(gtx C) D {
		return layout.UniformInset(values.MarginPadding12).Layout(gtx, func(gtx C) D {
			return v.showVSPModal.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if v.selectedVSP == nil {
							txt := v.Theme.Label(values.TextSize16, values.String(values.StrSelectVSP))
							txt.Color = v.Theme.Color.GrayText3
							return txt.Layout(gtx)
						}
						return v.Theme.Label(values.TextSize16, v.selectedVSP.Host).Layout(gtx)
					}),
					layout.Flexed(1, func(gtx C) D {
						return layout.E.Layout(gtx, func(gtx C) D {
							return layout.Flex{}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									if v.selectedVSP == nil {
										return layout.Dimensions{}
									}
									txt := v.Theme.Label(values.TextSize16, fmt.Sprintf("%v%%", v.selectedVSP.FeePercentage))
									return txt.Layout(gtx)
								}),
								layout.Rigid(func(gtx C) D {
									inset := layout.Inset{
										Left: values.MarginPadding15,
									}
									return inset.Layout(gtx, func(gtx C) D {
										ic := cryptomaterial.NewIcon(v.Theme.Icons.DropDownIcon)
										ic.Color = v.Theme.Color.Gray1
										return ic.Layout(gtx, values.MarginPadding20)
									})
								}),
							)
						})
					}),
				)
			})
		})
	})
}

type vspSelectorModal struct {
	*load.Load
	*cryptomaterial.Modal

	dialogTitle string

	inputVSP cryptomaterial.Editor
	addVSP   cryptomaterial.Button

	selectedVSP *dcr.VSP
	vspList     *cryptomaterial.ClickableList

	vspSelectedCallback func(*dcr.VSP)

	dcrImpl *dcr.Asset

	materialLoader material.LoaderStyle
	isLoadingVSP   bool
}

func newVSPSelectorModal(l *load.Load, dcrWallet *dcr.Asset) *vspSelectorModal {
	v := &vspSelectorModal{
		Load:  l,
		Modal: l.Theme.ModalFloatTitle("VSPSelectorModal"),

		inputVSP:       l.Theme.Editor(new(widget.Editor), values.String(values.StrAddVSP)),
		addVSP:         l.Theme.Button(values.String(values.StrSave)),
		vspList:        l.Theme.NewClickableList(layout.Vertical),
		dcrImpl:        dcrWallet,
		materialLoader: material.Loader(l.Theme.Base),
	}
	v.inputVSP.Editor.SingleLine = true

	v.addVSP.SetEnabled(false)

	return v
}

func (v *vspSelectorModal) OnResume() {
	if len(v.dcrImpl.KnownVSPs()) == 0 {
		go func() {
			v.isLoadingVSP = true // This is used to set the UI to loading VSP state.
			v.dcrImpl.ReloadVSPList(context.TODO())
			// set isLoadingVSP to false, this indicates to the UI that we are done
			// loading vsp(s)
			v.isLoadingVSP = false
			v.ParentWindow().Reload()
		}()
	}
}

func (v *vspSelectorModal) Handle() {
	v.addVSP.SetEnabled(v.editorsNotEmpty(v.inputVSP.Editor))
	if v.addVSP.Clicked() {
		if !utils.ValidateHost(v.inputVSP.Editor.Text()) {
			v.inputVSP.SetError(values.StringF(values.StrValidateHostErr, v.inputVSP.Editor.Text()))
			return
		}
		go func() {
			err := v.dcrImpl.SaveVSP(v.inputVSP.Editor.Text())
			if err != nil {
				errModal := modal.NewErrorModal(v.Load, err.Error(), modal.DefaultClickFunc())
				v.ParentWindow().ShowModal(errModal)
			} else {
				v.inputVSP.Editor.SetText("")
			}
		}()
	}

	if v.Modal.BackdropClicked(true) {
		v.Dismiss()
	}

	if clicked, selectedItem := v.vspList.ItemClicked(); clicked {
		v.selectedVSP = v.dcrImpl.KnownVSPs()[selectedItem]
		v.vspSelectedCallback(v.selectedVSP)
		v.Dismiss()
	}
}

func (v *vspSelectorModal) title(title string) *vspSelectorModal {
	v.dialogTitle = title
	return v
}

func (v *vspSelectorModal) vspSelected(callback func(*dcr.VSP)) *vspSelectorModal {
	v.vspSelectedCallback = callback
	return v
}

func (v *vspSelectorModal) Layout(gtx layout.Context) layout.Dimensions {
	return v.Modal.Layout(gtx, []layout.Widget{
		func(gtx C) D {
			title := v.Theme.Label(values.TextSize20, v.dialogTitle)
			// Override title when VSP is loading.
			if v.isLoadingVSP {
				title = v.Theme.Label(values.TextSize20, values.String(values.StrLoadingVSP))
			}
			title.Font.Weight = font.SemiBold
			return title.Layout(gtx)
		},
		func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					// Return 0 dimension if VSP is loading.
					if v.isLoadingVSP {
						return D{}
					}

					txt := v.Theme.Label(values.TextSize14, values.String(values.StrAddress))
					txt.Color = v.Theme.Color.GrayText2
					txtFee := v.Theme.Label(values.TextSize14, values.String(values.StrFee))
					txtFee.Color = v.Theme.Color.GrayText2
					return EndToEndRow(gtx, txt.Layout, txtFee.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					// if VSP(s) are being loaded, show loading UI.
					if v.isLoadingVSP {
						return layout.Inset{Top: values.MarginPadding140,
							Right:  values.MarginPadding140,
							Bottom: values.MarginPadding140,
							Left:   values.MarginPadding140}.Layout(gtx, v.materialLoader.Layout)
					}

					// if no vsp loaded, display a no vsp text
					vsps := v.dcrImpl.KnownVSPs()
					if len(vsps) == 0 && !v.isLoadingVSP {
						noVsp := v.Theme.Label(values.TextSize14, values.String(values.StrNoVSPLoaded))
						noVsp.Color = v.Theme.Color.GrayText2
						return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, noVsp.Layout)
					}

					return v.vspList.Layout(gtx, len(vsps), func(gtx C, i int) D {
						// Show scrollbar on VSP selector modal
						v.Modal.ShowScrollbar(true)
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(0.8, func(gtx C) D {
								return layout.Inset{Top: values.MarginPadding12, Bottom: values.MarginPadding12}.Layout(gtx, func(gtx C) D {
									txt := v.Theme.Label(values.TextSize14, fmt.Sprintf("%v%%", vsps[i].FeePercentage))
									txt.Color = v.Theme.Color.GrayText1
									return EndToEndRow(gtx, v.Theme.Label(values.TextSize16, vsps[i].Host).Layout, txt.Layout)
								})
							}),
							layout.Rigid(func(gtx C) D {
								if v.selectedVSP == nil || v.selectedVSP.Host != vsps[i].Host {
									return layout.Dimensions{}
								}
								ic := cryptomaterial.NewIcon(v.Theme.Icons.NavigationCheck)
								return ic.Layout(gtx, values.MarginPadding20)
							}),
						)
					})
				}),
			)
		},
		func(gtx C) D {
			// Return 0 dimension if VSP is loading.
			if v.isLoadingVSP {
				return D{}
			}

			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, v.inputVSP.Layout),
				layout.Rigid(v.addVSP.Layout),
			)
		},
	})
}

func (v *vspSelectorModal) editorsNotEmpty(editors ...*widget.Editor) bool {
	for _, e := range editors {
		if strings.TrimSpace(e.Text()) == "" {
			return false
		}
	}

	return true
}

func (v *vspSelectorModal) OnDismiss() {}
