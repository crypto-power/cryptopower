package security

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const ValidateAddressPageID = "ValidateAddress"

const (
	none = iota
	valid
	invalid
	notOwned
)

type ValidateAddressPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	wallet sharedW.Asset

	addressEditor         *cryptomaterial.Editor
	clearBtn, validateBtn cryptomaterial.Button
	stateValidate         int
	backButton            cryptomaterial.IconButton
	pageContainer         *widget.List
}

func NewValidateAddressPage(l *load.Load, wallet sharedW.Asset) *ValidateAddressPage {
	pg := &ValidateAddressPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ValidateAddressPageID),
		wallet:           wallet,
		pageContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}

	pg.backButton = components.GetBackButton(l)
	addressEditor := l.Theme.Editor(new(widget.Editor), values.String(values.StrAddress))
	pg.addressEditor = &addressEditor
	pg.addressEditor.Editor.SingleLine = true
	pg.addressEditor.Editor.Submit = true

	pg.validateBtn = l.Theme.Button(values.String(values.StrValidate))
	pg.validateBtn.Font.Weight = font.Medium

	pg.clearBtn = l.Theme.OutlineButton(values.String(values.StrClear))
	pg.clearBtn.Font.Weight = font.Medium

	pg.stateValidate = none

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ValidateAddressPage) OnNavigatedTo() {
	pg.addressEditor.SetFocus()
	pg.validateBtn.SetEnabled(utils.StringNotEmpty(pg.addressEditor.Editor.Text()))
}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ValidateAddressPage) Layout(gtx C) D {
	pg.handleEditorEvents(gtx)
	body := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrValidateAddr),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				return pg.Theme.List(pg.pageContainer).Layout(gtx, 1, func(gtx C, i int) D {
					return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
							layout.Rigid(pg.addressSection()),
						)
					})
				})
			},
		}
		return sp.Layout(pg.ParentWindow(), gtx)
	}
	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx, body)
	}
	return pg.layoutDesktop(gtx, body)
}

func (pg *ValidateAddressPage) layoutDesktop(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return body(gtx)
}

func (pg *ValidateAddressPage) layoutMobile(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return components.UniformMobile(gtx, false, false, body)
}

func (pg *ValidateAddressPage) addressSection() layout.Widget {
	return func(gtx C) D {
		return pg.pageSections(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pg.description()),
				layout.Rigid(pg.addressEditor.Layout),
				layout.Rigid(pg.actionButtons()),
			)
		})
	}
}

func (pg *ValidateAddressPage) description() layout.Widget {
	return func(gtx C) D {
		desc := pg.Theme.Caption(values.String(values.StrValidateNote))
		desc.Color = pg.Theme.Color.GrayText2
		return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, desc.Layout)
	}
}

func (pg *ValidateAddressPage) actionButtons() layout.Widget {
	return func(gtx C) D {
		dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.clearBtn.Layout)
							}),
							layout.Rigid(pg.validateBtn.Layout),
						)
					})
				})
			}),
		)
		return dims
	}
}

func (pg *ValidateAddressPage) pageSections(gtx C, body layout.Widget) D {
	return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Spacing: layout.SpaceAround}.Layout(gtx,
					layout.Rigid(body),
				)
			})
		})
	})
}

func (pg *ValidateAddressPage) handleEditorEvents(gtx C) {
	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(gtx, pg.addressEditor)
	if isChanged {
		pg.stateValidate = none
	}

	if pg.validateBtn.Clicked(gtx) || isSubmit {
		pg.validateAddress()
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ValidateAddressPage) HandleUserInteractions(gtx C) {
	pg.validateBtn.SetEnabled(utils.StringNotEmpty(pg.addressEditor.Editor.Text()))

	if pg.clearBtn.Clicked(gtx) {
		pg.clearPage()
	}
}

func (pg *ValidateAddressPage) clearPage() {
	pg.stateValidate = none
	pg.addressEditor.Editor.SetText("")
}

func (pg *ValidateAddressPage) validateAddress() {
	address := pg.addressEditor.Editor.Text()
	pg.addressEditor.SetError("")

	if !utils.StringNotEmpty(address) {
		pg.addressEditor.SetError(values.String(values.StrEnterValidAddress))
		return
	}

	var verifyMsgAddr string
	var info *modal.InfoModal

	if !pg.wallet.IsAddressValid(address) {
		verifyMsgAddr = values.String(values.StrInvalidAddress)
		info = modal.NewErrorModal(pg.Load, verifyMsgAddr, modal.DefaultClickFunc())
	} else {
		if !pg.wallet.HaveAddress(address) {
			verifyMsgAddr = values.String(values.StrNotOwned)
		} else {
			verifyMsgAddr = values.String(values.StrOwned)
		}
		info = modal.NewSuccessModal(pg.Load, verifyMsgAddr, modal.DefaultClickFunc())
	}

	pg.ParentWindow().ShowModal(info)
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *ValidateAddressPage) OnNavigatedFrom() {}
