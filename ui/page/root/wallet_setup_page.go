package root

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	libutils "gitlab.com/raedah/cryptopower/libwallet/utils"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/page/info"
	"gitlab.com/raedah/cryptopower/ui/utils"
	"gitlab.com/raedah/cryptopower/ui/values"
)

const (
	CreateWalletID    = "create_wallet"
	defaultWalletName = "myWallet"
)

type walletType struct {
	clickable *cryptomaterial.Clickable
	logo      *cryptomaterial.Image
	name      string
}

type decredAction struct {
	title     string
	clickable *cryptomaterial.Clickable
	border    cryptomaterial.Border
	width     unit.Dp
}

type bitcoinAction struct {
	title     string
	clickable *cryptomaterial.Clickable
	action    func()
	border    cryptomaterial.Border
	width     unit.Dp
}

type CreateWallet struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scrollContainer *widget.List
	list            layout.List

	walletTypes    []*walletType
	decredActions  []*decredAction
	bitcoinActions []*bitcoinAction

	walletName         cryptomaterial.Editor
	watchOnlyWalletHex cryptomaterial.Editor
	watchOnlyCheckBox  cryptomaterial.CheckBoxStyle

	continueBtn    cryptomaterial.Button
	BTCcontinueBtn cryptomaterial.Button
	restoreBtn     cryptomaterial.Button
	importBtn      cryptomaterial.Button
	backButton     cryptomaterial.IconButton

	selectedWalletType          int
	selectedDecredWalletAction  int
	selectedBitcoinWalletAction int

	showLoader bool
}

func NewCreateWallet(l *load.Load) *CreateWallet {
	pg := &CreateWallet{
		GenericPageModal: app.NewGenericPageModal(CreateWalletID),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},
		list: layout.List{Axis: layout.Vertical},

		continueBtn:                 l.Theme.Button(values.String(values.StrContinue)),
		BTCcontinueBtn:              l.Theme.Button(values.String(values.StrContinue)),
		restoreBtn:                  l.Theme.Button(values.String(values.StrRestore)),
		importBtn:                   l.Theme.Button(values.String(values.StrImport)),
		watchOnlyCheckBox:           l.Theme.CheckBox(new(widget.Bool), values.String(values.StrImportWatchingOnlyWallet)),
		selectedWalletType:          -1,
		selectedDecredWalletAction:  -1,
		selectedBitcoinWalletAction: -1,

		Load: l,
	}

	pg.walletName = l.Theme.Editor(new(widget.Editor), values.String(values.StrEnterWalletName))
	pg.walletName.Editor.SingleLine, pg.walletName.Editor.Submit, pg.walletName.IsTitleLabel = true, true, false

	pg.watchOnlyWalletHex = l.Theme.Editor(new(widget.Editor), values.String(values.StrExtendedPubKey))
	pg.watchOnlyWalletHex.Editor.SingleLine, pg.watchOnlyWalletHex.Editor.Submit, pg.watchOnlyWalletHex.IsTitleLabel = false, true, false

	pg.backButton, _ = components.SubpageHeaderButtons(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *CreateWallet) OnNavigatedTo() {
	pg.showLoader = false
	pg.initPageItems()
}

func (pg *CreateWallet) initPageItems() {
	walletTypes := []*walletType{
		{
			logo:      pg.Theme.Icons.DecredLogo,
			name:      "Decred",
			clickable: pg.Theme.NewClickable(true),
		},
		{
			logo:      pg.Theme.Icons.BTC,
			name:      "Bitcoin",
			clickable: pg.Theme.NewClickable(true),
		},
	}

	leftRadius := cryptomaterial.CornerRadius{
		TopLeft:    8,
		BottomLeft: 8,
	}

	rightRadius := cryptomaterial.CornerRadius{
		TopRight:    8,
		BottomRight: 8,
	}

	decredActions := []*decredAction{
		{
			title:     values.String(values.StrNewWallet),
			clickable: pg.Theme.NewClickable(true),
			border: cryptomaterial.Border{
				Radius: leftRadius,
				Color:  pg.Theme.Color.Gray1,
				Width:  values.MarginPadding2,
			},
			width: values.MarginPadding110,
		},
		{
			title:     values.String(values.StrRestoreExistingWallet),
			clickable: pg.Theme.NewClickable(true),
			border: cryptomaterial.Border{
				Radius: rightRadius,
				Color:  pg.Theme.Color.Gray1,
				Width:  values.MarginPadding2,
			},
			width: values.MarginPadding195,
		},
	}

	bitcoinActions := []*bitcoinAction{
		{
			title:     values.String(values.StrNewWallet),
			clickable: pg.Theme.NewClickable(true),
			border: cryptomaterial.Border{
				Radius: leftRadius,
				Color:  pg.Theme.Color.Gray1,
				Width:  values.MarginPadding2,
			},
			width: values.MarginPadding110,
		},
		{
			title:     values.String(values.StrRestoreExistingWallet),
			clickable: pg.Theme.NewClickable(true),
			border: cryptomaterial.Border{
				Radius: rightRadius,
				Color:  pg.Theme.Color.Gray1,
				Width:  values.MarginPadding2,
			},
			width: values.MarginPadding195,
		},
	}

	pg.walletTypes = walletTypes
	pg.decredActions = decredActions
	pg.bitcoinActions = bitcoinActions
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *CreateWallet) OnNavigatedFrom() {}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *CreateWallet) Layout(gtx C) D {
	pageContent := []func(gtx C) D{
		pg.Theme.H6(values.String(values.StrSelectWalletType)).Layout,
		pg.walletTypeSection,
		func(gtx C) D {
			switch pg.selectedWalletType {
			case 0:
				return pg.decredWalletOptions(gtx)
			case 1:
				return pg.bitcoinWalletOptions(gtx) // todo btc functionality
			default:
				return D{}
			}
		},
	}

	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.MatchParent,
		Direction: layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:     gtx.Dp(values.MarginPadding377),
			Height:    cryptomaterial.MatchParent,
			Alignment: layout.Middle,
			Margin: layout.Inset{
				Top:    values.MarginPadding44,
				Bottom: values.MarginPadding30,
			},
		}.Layout2(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pg.backButton.Layout),
				layout.Rigid(func(gtx C) D {
					return pg.Theme.List(pg.scrollContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
						return layout.Inset{
							Top:    values.MarginPadding26,
							Bottom: values.MarginPadding10,
							Right:  values.MarginPadding10,
						}.Layout(gtx, pageContent[i])
					})
				}),
			)
		})
	})
}

// todo bitcoin wallet creation
func (pg *CreateWallet) walletTypeSection(gtx C) D {
	list := layout.List{}
	return list.Layout(gtx, len(pg.walletTypes), func(gtx C, i int) D {
		item := pg.walletTypes[i]

		// set selected item background color
		backgroundColor := pg.Theme.Color.Surface
		if pg.selectedWalletType == i {
			backgroundColor = pg.Theme.Color.Gray2
		}

		return cryptomaterial.LinearLayout{
			Width:       gtx.Dp(values.MarginPadding172),
			Height:      gtx.Dp(values.MarginPadding174),
			Orientation: layout.Vertical,
			Alignment:   layout.Middle,
			Direction:   layout.Center,
			Border: cryptomaterial.Border{
				Color: pg.Theme.Color.Gray2,
				Width: values.MarginPadding2,
			},
			Background: backgroundColor,
			Clickable:  item.clickable,
			Margin: layout.Inset{
				Top:   values.MarginPadding10,
				Right: values.MarginPadding6,
			},
			Padding: layout.UniformInset(values.MarginPadding24),
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Bottom: values.MarginPadding14,
				}.Layout(gtx, func(gtx C) D {
					return item.logo.LayoutSize(gtx, values.MarginPadding70)
				})
			}),
			layout.Rigid(pg.Theme.Label(values.TextSize16, item.name).Layout),
		)
	})
}

func (pg *CreateWallet) decredWalletOptions(gtx C) D {
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {

			list := layout.List{}
			return list.Layout(gtx, len(pg.decredActions), func(gtx C, i int) D {
				item := pg.decredActions[i]

				// set selected item background color
				col := pg.Theme.Color.Surface
				if pg.selectedDecredWalletAction == i {
					col = pg.Theme.Color.Gray2
				}

				return cryptomaterial.LinearLayout{
					Width:       gtx.Dp(item.width),
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Vertical,
					Alignment:   layout.Middle,
					Direction:   layout.Center,
					Background:  col,
					Clickable:   item.clickable,
					Border:      item.border,
					Padding:     layout.UniformInset(values.MarginPadding12),
					Margin:      layout.Inset{Bottom: values.MarginPadding15},
				}.Layout2(gtx, pg.Theme.Label(values.TextSize16, item.title).Layout)
			})
		}),
		layout.Rigid(func(gtx C) D {
			switch pg.selectedDecredWalletAction {
			case 0:
				return pg.createNewWallet(gtx)
			case 1:
				return pg.restoreWallet(gtx)
			default:
				return D{}
			}
		}),
	)
}

func (pg *CreateWallet) bitcoinWalletOptions(gtx C) D {
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			list := layout.List{}
			return list.Layout(gtx, len(pg.bitcoinActions), func(gtx C, i int) D {
				item := pg.bitcoinActions[i]

				// set selected item background color
				col := pg.Theme.Color.Surface
				if pg.selectedBitcoinWalletAction == i {
					col = pg.Theme.Color.Gray2
				}

				return cryptomaterial.LinearLayout{
					Width:       gtx.Dp(item.width),
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Vertical,
					Alignment:   layout.Middle,
					Direction:   layout.Center,
					Background:  col,
					Clickable:   item.clickable,
					Border:      item.border,
					Padding:     layout.UniformInset(values.MarginPadding12),
					Margin:      layout.Inset{Bottom: values.MarginPadding15},
				}.Layout2(gtx, pg.Theme.Label(values.TextSize16, item.title).Layout)
			})
		}),
		layout.Rigid(func(gtx C) D {
			switch pg.selectedBitcoinWalletAction {
			case 0:
				return pg.createNewBTCWallet(gtx)
			case 1:
				return pg.restoreWallet(gtx)
			default:
				return D{}
			}
		}),
	)
}

func (pg *CreateWallet) createNewWallet(gtx C) D {
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrWhatToCallWallet)).Layout),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top:    values.MarginPadding14,
				Bottom: values.MarginPadding20,
			}.Layout(gtx, pg.walletName.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, pg.continueBtn.Layout)
				}),
			)
		}),
	)
}

func (pg *CreateWallet) createNewBTCWallet(gtx C) D {
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrWhatToCallWallet)).Layout),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top:    values.MarginPadding14,
				Bottom: values.MarginPadding20,
			}.Layout(gtx, pg.walletName.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, pg.BTCcontinueBtn.Layout)
				}),
			)
		}),
	)
}

func (pg *CreateWallet) restoreWallet(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrExistingWalletName)).Layout),
		layout.Rigid(pg.watchOnlyCheckBox.Layout),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top:    values.MarginPadding14,
				Bottom: values.MarginPadding20,
			}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(pg.walletName.Layout),
					layout.Rigid(func(gtx C) D {
						if !pg.watchOnlyCheckBox.CheckBox.Value {
							return D{}
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Top:    values.MarginPadding10,
									Bottom: values.MarginPadding8,
								}.Layout(gtx, pg.Theme.Label(values.TextSize16, values.String(values.StrExtendedPubKey)).Layout)
							}),
							layout.Rigid(pg.watchOnlyWalletHex.Layout),
						)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						if pg.showLoader {
							loader := material.Loader(pg.Theme.Base)
							loader.Color = pg.Theme.Color.Gray1
							return loader.Layout(gtx)
						}
						if pg.watchOnlyCheckBox.CheckBox.Value {
							return pg.importBtn.Layout(gtx)
						}
						return pg.restoreBtn.Layout(gtx)
					})
				}),
			)
		}),
	)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *CreateWallet) HandleUserInteractions() {
	// back button action
	if pg.backButton.Button.Clicked() {
		pg.ParentNavigator().CloseCurrentPage()
	}

	// wallet type selection action
	for i, item := range pg.walletTypes {
		for item.clickable.Clicked() {
			pg.selectedWalletType = i
		}
	}

	// decred wallet type sub action
	for i, item := range pg.decredActions {
		for item.clickable.Clicked() {
			pg.selectedDecredWalletAction = i
		}
	}

	for i, item := range pg.bitcoinActions {
		for item.clickable.Clicked() {
			pg.selectedBitcoinWalletAction = i
		}
	}

	// editor event listener
	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(pg.walletName.Editor, pg.watchOnlyWalletHex.Editor)
	if isChanged {
		// reset error when any editor is modified
		pg.walletName.SetError("")
	}

	// create wallet action
	if (pg.continueBtn.Clicked() || isSubmit) && pg.validInputs() {
		spendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
			Title(values.String(values.StrSpendingPassword)).
			SetPositiveButtonCallback(func(_, password string, m *modal.CreatePasswordModal) bool {
				errFunc := func(err string) bool {
					m.SetError(err)
					m.SetLoading(false)
					return false
				}
				wal, err := pg.WL.MultiWallet.CreateNewDCRWallet(pg.walletName.Editor.Text(), password, sharedW.PassphraseTypePass)
				if err != nil {
					if err.Error() == libutils.ErrExist {
						return errFunc(values.StringF(values.StrWalletExist, pg.walletName.Editor.Text()))
					}
					return errFunc(err.Error())
				}
				dcrUniqueImpl := wal.(dcr.DCRUniqueAsset)
				err = dcrUniqueImpl.CreateMixerAccounts(values.String(values.StrMixed), values.String(values.StrUnmixed), password)
				if err != nil {
					return errFunc(err.Error())
				}
				wal.SetBoolConfigValueForKey(sharedW.AccountMixerConfigSet, true)
				m.Dismiss()

				pg.handlerWalletDexServerSelectorCallBacks()
				return true
			})
		pg.ParentWindow().ShowModal(spendingPasswordModal)
	}

	if (pg.BTCcontinueBtn.Clicked() || isSubmit) && pg.validInputs() {
		spendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
			Title(values.String(values.StrSpendingPassword)).
			SetPositiveButtonCallback(func(_, password string, m *modal.CreatePasswordModal) bool {
				errFunc := func(err error) bool {
					m.SetError(err.Error())
					m.SetLoading(false)
					return false
				}
				_, err := pg.WL.MultiWallet.CreateNewBTCWallet(pg.walletName.Editor.Text(), password, sharedW.PassphraseTypePass)
				if err != nil {
					return errFunc(err)
				}
				m.Dismiss()

				pg.handlerWalletDexServerSelectorCallBacks()
				return true
			})
		pg.ParentWindow().ShowModal(spendingPasswordModal)
	}

	// restore wallet actions
	if pg.restoreBtn.Clicked() && pg.validInputs() {
		afterRestore := func() {
			// todo setup mixer for restored accounts automatically
			pg.handlerWalletDexServerSelectorCallBacks()
		}
		pg.ParentNavigator().Display(info.NewRestorePage(pg.Load, pg.walletName.Editor.Text(), afterRestore))
	}

	// imported wallet click action control
	if (pg.importBtn.Clicked() || isSubmit) && pg.validInputs() {
		pg.showLoader = true
		go func() {
			_, err := pg.WL.MultiWallet.CreateNewDCRWatchOnlyWallet(pg.walletName.Editor.Text(), pg.watchOnlyWalletHex.Editor.Text())
			if err != nil {
				if err.Error() == libutils.ErrExist {
					pg.watchOnlyWalletHex.SetError(values.StringF(values.StrWalletExist, pg.walletName.Editor.Text()))
				} else {
					pg.watchOnlyWalletHex.SetError(err.Error())
				}
				pg.showLoader = false
				return
			}
			pg.handlerWalletDexServerSelectorCallBacks()
		}()
	}
}

func (pg *CreateWallet) validInputs() bool {
	pg.walletName.SetError("")
	pg.watchOnlyWalletHex.SetError("")
	if !utils.StringNotEmpty(pg.walletName.Editor.Text()) {
		pg.walletName.SetError(values.String(values.StrEnterWalletName))
		return false
	}

	if !utils.ValidateLengthName(pg.walletName.Editor.Text()) {
		pg.walletName.SetError(values.String(values.StrWalletNameLengthError))
		return false
	}

	if pg.watchOnlyCheckBox.CheckBox.Value && !utils.StringNotEmpty(pg.watchOnlyWalletHex.Editor.Text()) {
		pg.watchOnlyWalletHex.SetError(values.String(values.StrEnterExtendedPubKey))
		return false
	}

	return true
}

func (pg *CreateWallet) handlerWalletDexServerSelectorCallBacks() {
	onWalSelected := func() {
		pg.ParentNavigator().ClearStackAndDisplay(NewMainPage(pg.Load))
	}
	onDexServerSelected := func(server string) {
		log.Info("Not implemented yet...", server)
	}
	pg.ParentNavigator().ClearStackAndDisplay(NewWalletDexServerSelector(pg.Load, onWalSelected, onDexServerSelected))
}
