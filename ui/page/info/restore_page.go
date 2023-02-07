package info

import (
	"image"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"

	"code.cryptopower.dev/group/cryptopower/app"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	libUtils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

const CreateRestorePageID = "Restore"

var tabTitles = []string{"Seed Words", "Hex"}

type Restore struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the ParentNavigator that displayed this page
	// and the root WindowNavigator. The ParentNavigator is also the root
	// WindowNavigator if this page is displayed from the StartPage, otherwise
	// the ParentNavigator is the MainPage.
	*app.GenericPageModal
	restoreComplete   func()
	tabList           *cryptomaterial.ClickableList
	tabIndex          int
	backButton        cryptomaterial.IconButton
	seedRestorePage   *SeedRestore
	walletName        string
	walletType        utils.AssetType
	toggleSeedInput   *cryptomaterial.Switch
	seedInputEditor   cryptomaterial.Editor
	confirmSeedButton cryptomaterial.Button
	restoreInProgress bool
}

func NewRestorePage(l *load.Load, walletName string, walletType utils.AssetType, onRestoreComplete func()) *Restore {
	pg := &Restore{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(CreateRestorePageID),
		seedRestorePage:  NewSeedRestorePage(l, walletName, walletType, onRestoreComplete),
		tabIndex:         0,
		tabList:          l.Theme.NewClickableList(layout.Horizontal),
		restoreComplete:  onRestoreComplete,
		walletName:       walletName,
		walletType:       walletType,
		toggleSeedInput:  l.Theme.Switch(),
	}

	pg.backButton, _ = components.SubpageHeaderButtons(l)
	pg.backButton.Icon = pg.Theme.Icons.ContentClear

	pg.seedInputEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrEnterWalletSeed))
	pg.seedInputEditor.Editor.SingleLine = false
	pg.seedInputEditor.Editor.SetText("")

	pg.confirmSeedButton = l.Theme.Button("")
	pg.confirmSeedButton.Font.Weight = text.Medium
	pg.confirmSeedButton.SetEnabled(false)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *Restore) OnNavigatedTo() {
	pg.toggleSeedInput.SetChecked(false)
	pg.seedRestorePage.OnNavigatedTo()
	pg.seedRestorePage.SetParentNav(pg.ParentWindow())
}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *Restore) Layout(gtx C) D {
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}
func (pg *Restore) layoutDesktop(gtx C) D {
	body := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrRestoreWallet),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				return pg.restoreLayout(gtx)
			},
		}
		return sp.Layout(pg.ParentWindow(), gtx)
	}
	return components.UniformPadding(gtx, body)
}

func (pg *Restore) layoutMobile(gtx C) D {
	body := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrRestoreWallet),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				return pg.restoreMobileLayout(gtx)
			},
		}
		return sp.Layout(pg.ParentWindow(), gtx)
	}
	return components.UniformMobile(gtx, false, false, body)
}

func (pg *Restore) restoreLayout(gtx layout.Context) layout.Dimensions {
	return components.UniformPadding(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.tabLayout),
			layout.Rigid(pg.Theme.Separator().Layout),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.toggleSeedInput.Layout)
						}),
						layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrPasteSeedWords)).Layout),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if pg.toggleSeedInput.IsChecked() {
					return layout.Inset{
						Top: values.MarginPadding16,
					}.Layout(gtx, func(gtx C) D {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.MatchParent,
							Orientation: layout.Vertical,
							Margin:      layout.Inset{Bottom: values.MarginPadding16},
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return pg.Theme.Card().Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) D {
											return layout.Inset{
												Left:  values.MarginPadding16,
												Right: values.MarginPadding16,
												Top:   values.MarginPadding30}.Layout(gtx, func(gtx C) D {
												return pg.seedInputEditor.Layout(gtx)
											})
										}),
										layout.Rigid(func(gtx C) D {
											return layout.Flex{}.Layout(gtx,
												layout.Flexed(1, func(gtx C) D {
													return layout.E.Layout(gtx, func(gtx C) D {
														return layout.Inset{
															Left:   values.MarginPadding16,
															Right:  values.MarginPadding16,
															Top:    values.MarginPadding16,
															Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
															pg.confirmSeedButton.Text = values.String(values.StrValidateWalSeed)
															return pg.confirmSeedButton.Layout(gtx)
														})
													})
												}),
											)
										}),
									)

								})
							}),
						)
					})
				}
				return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, pg.indexLayout)
			}),
		)
	})
}

func (pg *Restore) restoreMobileLayout(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.tabLayout),
			layout.Rigid(pg.Theme.Separator().Layout),
			layout.Flexed(1, func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, pg.indexLayout)
			}),
		)
	})
}

func (pg *Restore) indexLayout(gtx C) D {
	return pg.seedRestorePage.Layout(gtx)
}

func (pg *Restore) switchTab(tabIndex int) {
	if tabIndex == 0 {
		pg.seedRestorePage.OnNavigatedTo()
	} else {
		pg.showHexRestoreModal()
	}
}

func (pg *Restore) tabLayout(gtx C) D {
	var dims layout.Dimensions
	return layout.Inset{
		Top: values.MarginPaddingMinus30,
	}.Layout(gtx, func(gtx C) D {
		return pg.tabList.Layout(gtx, len(tabTitles), func(gtx C, i int) D {
			return layout.Stack{Alignment: layout.S}.Layout(gtx,
				layout.Stacked(func(gtx C) D {
					return layout.Inset{
						Right:  values.MarginPadding24,
						Bottom: values.MarginPadding8,
					}.Layout(gtx, func(gtx C) D {
						return layout.Center.Layout(gtx, func(gtx C) D {
							lbl := pg.Theme.Label(values.TextSize16, tabTitles[i])
							lbl.Color = pg.Theme.Color.GrayText1
							if pg.tabIndex == i {
								lbl.Color = pg.Theme.Color.Primary
								dims = lbl.Layout(gtx)
							}

							return lbl.Layout(gtx)
						})
					})
				}),
				layout.Stacked(func(gtx C) D {
					if pg.tabIndex != i {
						return D{}
					}

					tabHeight := gtx.Dp(values.MarginPadding2)
					tabRect := image.Rect(0, 0, dims.Size.X, tabHeight)

					return layout.Inset{
						Left: values.MarginPaddingMinus22,
					}.Layout(gtx, func(gtx C) D {
						paint.FillShape(gtx.Ops, pg.Theme.Color.Primary, clip.Rect(tabRect).Op())
						return layout.Dimensions{
							Size: image.Point{X: dims.Size.X, Y: tabHeight},
						}
					})
				}),
			)
		})
	})
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *Restore) OnNavigatedFrom() {
	pg.seedRestorePage.OnNavigatedFrom()

	//pg.PopWindowPage()
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *Restore) HandleUserInteractions() {
	if clicked, selectedItem := pg.tabList.ItemClicked(); clicked {
		if pg.tabIndex != selectedItem {
			pg.tabIndex = selectedItem
			pg.switchTab(pg.tabIndex)
		}
	}

	if pg.tabIndex == 0 {
		pg.seedRestorePage.HandleUserInteractions()
	}

	if len(strings.TrimSpace(pg.seedInputEditor.Editor.Text())) != 0 {
		pg.confirmSeedButton.SetEnabled(true)
	}

	if pg.confirmSeedButton.Clicked() {
		if !pg.restoreInProgress {
			go pg.restoreFromSeedEditor()
		}
	}
}

func (pg *Restore) showHexRestoreModal() {
	hexModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrEnterHex)).
		PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
		SetPositiveButtonCallback(func(hex string, hm *modal.TextInputModal) bool {
			if !pg.verifyHex(hex) {
				hm.SetError(values.String(values.StrInvalidHex))
				hm.SetLoading(false)
				return false
			}

			passwordModal := modal.NewCreatePasswordModal(pg.Load).
				Title(values.String(values.StrEnterWalDetails)).
				EnableName(false).
				ShowWalletInfoTip(true).
				SetParent(pg).
				SetNegativeButtonCallback(func() {
					pg.tabIndex = 0
					pg.switchTab(pg.tabIndex)
				}).
				SetPositiveButtonCallback(func(walletName, password string, m *modal.CreatePasswordModal) bool {
					_, err := pg.WL.AssetsManager.RestoreWallet(pg.walletType, pg.walletName, hex, password, sharedW.PassphraseTypePass)
					if err != nil {
						m.SetError(err.Error())
						m.SetLoading(false)
						return false
					}

					successModal := modal.NewSuccessModal(pg.Load, values.String(values.StrWalletRestored), modal.DefaultClickFunc())
					pg.ParentWindow().ShowModal(successModal)
					m.Dismiss()
					if pg.restoreComplete == nil {
						pg.ParentNavigator().CloseCurrentPage()
					} else {
						pg.restoreComplete()
					}
					return true
				})
			pg.ParentWindow().ShowModal(passwordModal)

			hm.Dismiss()
			return true
		})
	hexModal.Title(values.String(values.StrRestoreWithHex)).
		SetPositiveButtonText(values.String(values.StrSubmit)).
		SetNegativeButtonCallback(func() {
			pg.tabIndex = 0
			pg.switchTab(pg.tabIndex)
		})
	pg.ParentWindow().ShowModal(hexModal)
}

func (pg *Restore) verifyHex(hex string) bool {
	if !sharedW.VerifySeed(hex) {
		return false
	}

	// Compare with existing wallets seed. On positive match abort import
	// to prevent duplicate wallet. walletWithSameSeed >= 0 if there is a match.
	walletWithSameSeed, err := pg.WL.AssetsManager.WalletWithSeed(pg.walletType, hex)
	if err != nil {
		log.Error(err)
		return false
	}

	if walletWithSameSeed != -1 {
		errModal := modal.NewErrorModal(pg.Load, values.String(values.StrSeedAlreadyExist), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		return false
	}

	return true
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this page wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *Restore) KeysToHandle() key.Set {
	if pg.tabIndex == 0 {
		return pg.seedRestorePage.KeysToHandle()
	}
	return ""
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *Restore) HandleKeyPress(evt *key.Event) {
	if pg.tabIndex == 0 {
		pg.seedRestorePage.HandleKeyPress(evt)
	}
}

func (pg *Restore) restoreFromSeedEditor() {
	pg.restoreInProgress = true
	defer func() {
		pg.restoreInProgress = false
		pg.seedInputEditor.Editor.SetText("")
	}()

	seed := strings.TrimSpace(pg.seedInputEditor.Editor.Text())
	if !sharedW.VerifySeed(seed) {
		errModal := modal.NewErrorModal(pg.Load, values.String(values.StrInvalidSeedPhrase), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		return
	}

	walletWithSameSeed, err := pg.WL.AssetsManager.WalletWithSeed(pg.walletType, seed)
	if err != nil {
		log.Error(err)
		errModal := modal.NewErrorModal(pg.Load, values.String(values.StrSeedValidationFailed), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		return
	}

	if walletWithSameSeed != -1 {
		errModal := modal.NewErrorModal(pg.Load, values.String(values.StrSeedAlreadyExist), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		return
	}

	walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		Title(values.String(values.StrEnterWalDetails)).
		EnableName(false).
		ShowWalletInfoTip(true).
		SetParent(pg).
		SetPositiveButtonCallback(func(walletName, password string, m *modal.CreatePasswordModal) bool {
			_, err := pg.WL.AssetsManager.RestoreWallet(pg.walletType, pg.walletName, seed, password, sharedW.PassphraseTypePass)
			if err != nil {
				errString := err.Error()
				if err.Error() == libUtils.ErrExist {
					errString = values.StringF(values.StrWalletExist, pg.walletName)
				}
				m.SetError(errString)
				m.SetLoading(false)
				return false
			}

			infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrWalletRestored), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(infoModal)
			m.Dismiss()
			if pg.restoreComplete == nil {
				pg.ParentNavigator().CloseCurrentPage()
			} else {
				pg.restoreComplete()
			}
			return true
		})
	pg.ParentWindow().ShowModal(walletPasswordModal)

}
