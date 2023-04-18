package root

import (
	"errors"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	libutils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/page/info"
	"code.cryptopower.dev/group/cryptopower/ui/utils"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

const (
	CreateWalletID    = "create_wallet"
	defaultWalletName = "myWallet"
)

type walletAction struct {
	title     string
	clickable *cryptomaterial.Clickable
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

	walletActions []*walletAction

	assetTypeSelector     *components.AssetTypeSelector
	assetTypeError        cryptomaterial.Label
	walletName            cryptomaterial.Editor
	watchOnlyWalletHex    cryptomaterial.Editor
	passwordEditor        cryptomaterial.Editor
	confirmPasswordEditor cryptomaterial.Editor
	watchOnlyCheckBox     cryptomaterial.CheckBoxStyle
	materialLoader        material.LoaderStyle

	continueBtn cryptomaterial.Button
	restoreBtn  cryptomaterial.Button
	importBtn   cryptomaterial.Button
	backButton  cryptomaterial.IconButton

	selectedWalletAction int

	showLoader bool
	isLoading  bool
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
		assetTypeSelector: components.NewAssetTypeSelector(l),
		list:              layout.List{Axis: layout.Vertical},

		continueBtn:          l.Theme.Button(values.String(values.StrContinue)),
		restoreBtn:           l.Theme.Button(values.String(values.StrRestore)),
		importBtn:            l.Theme.Button(values.String(values.StrImport)),
		watchOnlyCheckBox:    l.Theme.CheckBox(new(widget.Bool), values.String(values.StrImportWatchingOnlyWallet)),
		selectedWalletAction: -1,
		assetTypeError:       l.Theme.Body1(""),

		Load: l,
	}

	bg := l.Theme.Color.White
	if l.WL.AssetsManager.IsDarkModeOn() {
		bg = l.Theme.Color.Background
	}
	pg.assetTypeSelector.SetBackground(bg)

	pg.walletName = l.Theme.Editor(new(widget.Editor), values.String(values.StrEnterWalletName))
	pg.walletName.Editor.SingleLine, pg.walletName.Editor.Submit = true, true
	pg.confirmPasswordEditor.Hint = values.String(values.StrWalletName)

	pg.watchOnlyWalletHex = l.Theme.Editor(new(widget.Editor), values.String(values.StrExtendedPubKey))
	pg.watchOnlyWalletHex.Editor.SingleLine, pg.watchOnlyWalletHex.Editor.Submit, pg.watchOnlyWalletHex.IsTitleLabel = false, true, false

	pg.passwordEditor = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrSpendingPassword))
	pg.passwordEditor.Editor.SingleLine, pg.passwordEditor.Editor.Submit = true, true
	pg.passwordEditor.Hint = values.String(values.StrSpendingPassword)

	pg.confirmPasswordEditor = l.Theme.EditorPassword(new(widget.Editor), values.String(values.StrSpendingPassword))
	pg.confirmPasswordEditor.Editor.SingleLine, pg.confirmPasswordEditor.Editor.Submit = true, true
	pg.confirmPasswordEditor.Hint = values.String(values.StrConfirmSpendingPassword)

	pg.materialLoader = material.Loader(l.Theme.Base)

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
	radius := cryptomaterial.CornerRadius{
		TopRight:    8,
		BottomRight: 8,
		TopLeft:     8,
		BottomLeft:  8,
	}

	walletActions := []*walletAction{
		{
			title:     values.String(values.StrNewWallet),
			clickable: pg.Theme.NewClickable(true),
			border: cryptomaterial.Border{
				Radius: radius,
				Color:  pg.Theme.Color.DefaultThemeColors().White,
				Width:  values.MarginPadding2,
			},
			width: values.MarginPadding110,
		},
		{
			title:     values.String(values.StrRestoreExistingWallet),
			clickable: pg.Theme.NewClickable(true),
			border: cryptomaterial.Border{
				Radius: radius,
				Color:  pg.Theme.Color.DefaultThemeColors().White,
				Width:  values.MarginPadding2,
			},
			width: values.MarginPadding195,
		},
	}

	pg.walletActions = walletActions
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
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(pg.backButton.Layout),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Left: values.MarginPadding10,
							}.Layout(gtx, pg.Theme.H6(values.String(values.StrCreateWallet)).Layout)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
						return layout.Inset{
							Top:   values.MarginPadding26,
							Right: values.MarginPadding20,
						}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(pg.walletTypeSection),
								layout.Rigid(pg.walletOptions),
							)
						})
					})
				}),
			)
		})
	})
}

// todo bitcoin wallet creation
func (pg *CreateWallet) walletTypeSection(gtx C) D {
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
						return pg.assetTypeSelector.Layout(pg.ParentWindow(), gtx)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding4, Bottom: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
						pg.assetTypeError.Color = pg.Theme.Color.Danger
						pg.assetTypeError.TextSize = values.TextSize14
						return pg.assetTypeError.Layout(gtx)
					})
				}),
			)
		}),
	)
}

func (pg *CreateWallet) walletOptions(gtx C) D {
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceEnd}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			list := layout.List{}
			return list.Layout(gtx, len(pg.walletActions), func(gtx C, i int) D {
				item := pg.walletActions[i]

				// set selected item background color
				col := pg.Theme.Color.Surface
				title := pg.Theme.Label(values.TextSize16, item.title)
				title.Color = pg.Theme.Color.Gray1

				radius := cryptomaterial.CornerRadius{
					TopRight:    8,
					BottomRight: 8,
					TopLeft:     8,
					BottomLeft:  8,
				}
				border := pg.Theme.Color.White
				if pg.WL.AssetsManager.IsDarkModeOn() {
					border = pg.Theme.Color.Background
				}
				item.border = cryptomaterial.Border{
					Radius: radius,
					Color:  border,
					Width:  values.MarginPadding2,
				}

				if pg.selectedWalletAction == i {

					col = pg.Theme.Color.White
					title.Color = pg.Theme.Color.Primary

					if pg.WL.AssetsManager.IsDarkModeOn() {
						col = pg.Theme.Color.Gray2
						title.Color = pg.Theme.Color.White
					}

					item.border = cryptomaterial.Border{
						Radius: radius,
						Color:  pg.Theme.Color.Primary,
						Width:  values.MarginPadding2,
					}
				}

				if item.clickable.IsHovered() {
					item.border = cryptomaterial.Border{
						Radius: radius,
						Color:  pg.Theme.Color.Gray1,
						Width:  values.MarginPadding2,
					}
					title.Color = pg.Theme.Color.Gray1
				}

				return layout.Inset{
					Right: values.MarginPadding8,
				}.Layout(gtx, func(gtx C) D {
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
					}.Layout2(gtx, title.Layout)
				})
			})
		}),
		layout.Rigid(func(gtx C) D {
			switch pg.selectedWalletAction {
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

func (pg *CreateWallet) createNewWallet(gtx C) D {
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top:    values.MarginPadding14,
				Bottom: values.MarginPadding20,
			}.Layout(gtx, pg.walletName.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top:    values.MarginPadding8,
				Bottom: values.MarginPadding20,
			}.Layout(gtx, pg.passwordEditor.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Top:    values.MarginPadding8,
				Bottom: values.MarginPadding20,
			}.Layout(gtx, pg.confirmPasswordEditor.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						if pg.isLoading {
							gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding20)
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return pg.materialLoader.Layout(gtx)
						}
						return pg.continueBtn.Layout(gtx)
					})
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

	// decred wallet type sub action
	for i, item := range pg.walletActions {
		for item.clickable.Clicked() {
			pg.selectedWalletAction = i
		}
	}

	// editor event listener
	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(pg.walletName.Editor, pg.watchOnlyWalletHex.Editor, pg.passwordEditor.Editor, pg.confirmPasswordEditor.Editor)
	if isChanged {
		// reset error when any editor is modified
		pg.walletName.SetError("")
		pg.passwordEditor.SetError("")
		pg.confirmPasswordEditor.SetError("")
		pg.watchOnlyWalletHex.SetError("")
	}

	// create wallet action
	if (pg.continueBtn.Clicked() || isSubmit) && pg.validCreateWalletInputs() {
		go func() {
			defer func() {
				pg.isLoading = false
			}()
			pg.isLoading = true

			switch *pg.assetTypeSelector.SelectedAssetType() {
			case libutils.DCRWalletAsset:
				wal, err := pg.WL.AssetsManager.CreateNewDCRWallet(pg.walletName.Editor.Text(), pg.passwordEditor.Editor.Text(), sharedW.PassphraseTypePass)
				if err != nil {
					if err.Error() == libutils.ErrExist {
						pg.walletName.SetError(values.StringF(values.StrWalletExist, pg.walletName.Editor.Text()))
						return
					}

					errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
					pg.ParentWindow().ShowModal(errModal)
					return
				}

				dcrUniqueImpl := wal.(*dcr.DCRAsset)
				err = dcrUniqueImpl.CreateMixerAccounts(values.String(values.StrMixed), values.String(values.StrUnmixed), pg.passwordEditor.Editor.Text())
				if err != nil {
					errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
					pg.ParentWindow().ShowModal(errModal)
					return
				}
				wal.SetBoolConfigValueForKey(sharedW.AccountMixerConfigSet, true)

			case libutils.BTCWalletAsset:
				_, err := pg.WL.AssetsManager.CreateNewBTCWallet(pg.walletName.Editor.Text(), pg.passwordEditor.Editor.Text(), sharedW.PassphraseTypePass)
				if err != nil {
					if err.Error() == libutils.ErrExist {
						pg.walletName.SetError(values.StringF(values.StrWalletExist, pg.walletName.Editor.Text()))
						return
					}

					errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
					pg.ParentWindow().ShowModal(errModal)
					return
				}

			case libutils.LTCWalletAsset:
				_, err := pg.WL.AssetsManager.CreateNewLTCWallet(pg.walletName.Editor.Text(), pg.passwordEditor.Editor.Text(), sharedW.PassphraseTypePass)
				if err != nil {
					if err.Error() == libutils.ErrExist {
						pg.walletName.SetError(values.StringF(values.StrWalletExist, pg.walletName.Editor.Text()))
						return
					}

					errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
					pg.ParentWindow().ShowModal(errModal)
					return
				}

			case libutils.ETHWalletAsset:
				_, err := pg.WL.AssetsManager.CreateNewETHWallet(pg.walletName.Editor.Text(), pg.passwordEditor.Editor.Text(), sharedW.PassphraseTypePass)
				if err != nil {
					if err.Error() == libutils.ErrExist {
						pg.walletName.SetError(values.StringF(values.StrWalletExist, pg.walletName.Editor.Text()))
						return
					}

					errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
					pg.ParentWindow().ShowModal(errModal)
					return
				}

			}
			pg.handlerWalletDexServerSelectorCallBacks()
		}()
	}

	// restore wallet actions
	if pg.restoreBtn.Clicked() && pg.validRestoreWalletInputs() {
		afterRestore := func() {
			// todo setup mixer for restored accounts automatically
			pg.handlerWalletDexServerSelectorCallBacks()
		}
		ast := pg.assetTypeSelector.SelectedAssetType()
		pg.ParentNavigator().Display(info.NewRestorePage(pg.Load, pg.walletName.Editor.Text(), *ast, afterRestore))
	}

	// imported wallet click action control
	if (pg.importBtn.Clicked() || isSubmit) && pg.validRestoreWalletInputs() {
		pg.showLoader = true
		var err error
		go func() {
			switch *pg.assetTypeSelector.SelectedAssetType() {
			case libutils.DCRWalletAsset:
				var walletWithXPub int
				walletWithXPub, err = pg.WL.AssetsManager.DCRWalletWithXPub(pg.watchOnlyWalletHex.Editor.Text())
				if walletWithXPub == -1 {
					_, err = pg.WL.AssetsManager.CreateNewDCRWatchOnlyWallet(pg.walletName.Editor.Text(), pg.watchOnlyWalletHex.Editor.Text())
				} else {
					err = errors.New(values.String(values.StrXpubWalletExist))
				}
			case libutils.BTCWalletAsset:
				var walletWithXPub int
				walletWithXPub, err = pg.WL.AssetsManager.BTCWalletWithXPub(pg.watchOnlyWalletHex.Editor.Text())
				if walletWithXPub == -1 {
					_, err = pg.WL.AssetsManager.CreateNewBTCWatchOnlyWallet(pg.walletName.Editor.Text(), pg.watchOnlyWalletHex.Editor.Text())
				} else {
					err = errors.New(values.String(values.StrXpubWalletExist))
				}
			case libutils.LTCWalletAsset:
				var walletWithXPub int
				walletWithXPub, err = pg.WL.AssetsManager.LTCWalletWithXPub(pg.watchOnlyWalletHex.Editor.Text())
				if walletWithXPub == -1 {
					_, err = pg.WL.AssetsManager.CreateNewLTCWatchOnlyWallet(pg.walletName.Editor.Text(), pg.watchOnlyWalletHex.Editor.Text())
				} else {
					err = errors.New(values.String(values.StrXpubWalletExist))
				}
			}

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

func (pg *CreateWallet) passwordsMatch(editors ...*widget.Editor) bool {
	if len(editors) < 2 {
		return false
	}

	password := editors[0]
	matching := editors[1]

	if password.Text() != matching.Text() {
		pg.confirmPasswordEditor.SetError(values.String(values.StrPasswordNotMatch))
		return false
	}

	pg.confirmPasswordEditor.SetError("")
	return true
}

func (pg *CreateWallet) validCreateWalletInputs() bool {
	pg.walletName.SetError("")
	pg.assetTypeError = pg.Theme.Body1("")

	if pg.assetTypeSelector.SelectedAssetType() == nil {
		pg.assetTypeError = pg.Theme.Body1(values.String(values.StrSelectAssetType))
		return false
	}

	if !utils.StringNotEmpty(pg.walletName.Editor.Text()) {
		pg.walletName.SetError(values.String(values.StrEnterWalletName))
		return false
	}

	if !utils.ValidateLengthName(pg.walletName.Editor.Text()) {
		pg.walletName.SetError(values.String(values.StrWalletNameLengthError))
		return false
	}

	validPassword := utils.EditorsNotEmpty(pg.confirmPasswordEditor.Editor)
	if len(pg.passwordEditor.Editor.Text()) > 0 {
		passwordsMatch := pg.passwordsMatch(pg.passwordEditor.Editor, pg.confirmPasswordEditor.Editor)
		return validPassword && passwordsMatch
	}

	return true
}

func (pg *CreateWallet) validRestoreWalletInputs() bool {
	pg.walletName.SetError("")
	pg.watchOnlyWalletHex.SetError("")
	pg.assetTypeError = pg.Theme.Body1("")

	if pg.assetTypeSelector.SelectedAssetType() == nil {
		pg.assetTypeError = pg.Theme.Body1(values.String(values.StrSelectAssetType))
		return false
	}

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
