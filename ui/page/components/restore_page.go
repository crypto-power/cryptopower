package components

import (
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/values"
)

const CreateRestorePageID = "Restore"

var tabTitles = []string{
	values.String(values.StrSeedWords),
	values.String(values.StrHex),
}

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
	tabs              *cryptomaterial.SegmentedControl
	tabIndex          int
	backButton        cryptomaterial.IconButton
	seedRestorePage   *SeedRestore
	walletName        string
	walletType        libutils.AssetType
	toggleSeedInput   *cryptomaterial.Switch
	seedInputEditor   cryptomaterial.Editor
	confirmSeedButton cryptomaterial.Button
	restoreInProgress bool
}

func NewRestorePage(l *load.Load, walletName string, walletType libutils.AssetType, onRestoreComplete func()) *Restore {
	pg := &Restore{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(CreateRestorePageID),
		seedRestorePage:  NewSeedRestorePage(l, walletName, walletType, onRestoreComplete),
		tabIndex:         0,
		restoreComplete:  onRestoreComplete,
		walletName:       walletName,
		walletType:       walletType,
		toggleSeedInput:  l.Theme.Switch(),
		tabs:             l.Theme.SegmentedControl(tabTitles, cryptomaterial.SegmentTypeGroup),
	}

	pg.backButton = GetBackButtons(l)
	pg.backButton.Icon = pg.Theme.Icons.ContentClear
	textSize16 := values.TextSizeTransform(l.IsMobileView(), values.TextSize16)

	pg.seedInputEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrEnterWalletSeed))
	pg.seedInputEditor.Editor.SingleLine = false
	pg.seedInputEditor.Editor.SetText("")
	pg.seedInputEditor.TextSize = textSize16

	pg.confirmSeedButton = l.Theme.Button("")
	pg.confirmSeedButton.Font.Weight = font.Medium
	pg.confirmSeedButton.SetEnabled(false)
	pg.confirmSeedButton.TextSize = textSize16
	pg.tabs.DisableUniform(true)
	pg.tabs.SetDisableAnimation(true)

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
	body := func(gtx C) D {
		sp := SubPage{
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
	return cryptomaterial.UniformPadding(gtx, body, pg.IsMobileView())
}

func (pg *Restore) restoreLayout(gtx layout.Context) layout.Dimensions {
	return pg.tabs.Layout(gtx, func(gtx C) D {
		if pg.tabs.SelectedIndex() == 0 {
			return pg.seedWordsLayout(gtx)
		}
		return pg.seedInputLayout(gtx)
	}, pg.IsMobileView())
}

func (pg *Restore) seedWordsLayout(gtx C) D {
	textSize16 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize16)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.toggleSeedInput.Layout)
					}),
					layout.Rigid(pg.Theme.Label(textSize16, values.String(values.StrPasteSeedWords)).Layout),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if pg.toggleSeedInput.IsChecked() || pg.tabIndex == 1 {
				return pg.seedInputLayout(gtx)
			}
			return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, pg.indexLayout)
		}),
	)
}

func (pg *Restore) seedInputLayout(gtx C) D {
	if pg.tabIndex == 0 {
		pg.seedInputEditor.Hint = values.String(values.StrEnterWalletSeed)
		pg.confirmSeedButton.Text = values.String(values.StrValidateWalSeed)
	} else {
		pg.seedInputEditor.Hint = values.String(values.StrEnterWalletHex)
		pg.confirmSeedButton.Text = values.String(values.StrValidateWalHex)
	}
	return layout.Inset{
		Top: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			return HorizontalInset(values.MarginPadding16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(layout.Spacer{Height: values.MarginPadding24}.Layout),
					layout.Rigid(pg.seedInputEditor.Layout),
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.E.Layout(gtx, func(gtx C) D {
							return VerticalInset(values.MarginPadding16).Layout(gtx, pg.confirmSeedButton.Layout)
						})
					}),
				)
			})
		})
	})
}

func (pg *Restore) indexLayout(gtx C) D {
	return pg.seedRestorePage.Layout(gtx)
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
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *Restore) HandleUserInteractions() {
	if pg.tabs.Changed() {
		pg.tabIndex = pg.tabs.SelectedIndex()
	}

	if !pg.toggleSeedInput.IsChecked() && pg.toggleSeedInput.Changed() {
		pg.seedRestorePage.setEditorFocus()
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
	clearEditor := func() {
		pg.restoreInProgress = false
		pg.seedInputEditor.Editor.SetText("")
	}

	seedOrHex := strings.TrimSpace(pg.seedInputEditor.Editor.Text())
	// Check if the user did input a hex or seed. If its a hex set the correct tabindex.
	if len(seedOrHex) > MaxSeedBytes {
		pg.tabIndex = 0
	} else {
		pg.tabIndex = 1
	}

	if !sharedW.VerifySeed(seedOrHex, pg.walletType) {
		errMsg := values.String(values.StrInvalidHex)
		if pg.tabIndex == 0 {
			errMsg = values.String(values.StrInvalidSeedPhrase)
		}
		errModal := modal.NewErrorModal(pg.Load, errMsg, modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		clearEditor()
		return
	}

	walletWithSameSeed, err := pg.AssetsManager.WalletWithSeed(pg.walletType, seedOrHex)
	if err != nil {
		log.Error(err)
		errMsg := values.String(values.StrInvalidHex)
		if pg.tabIndex == 0 {
			errMsg = values.String(values.StrSeedValidationFailed)
		}
		errModal := modal.NewErrorModal(pg.Load, errMsg, modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		clearEditor()
		return
	}

	if walletWithSameSeed != -1 {
		errModal := modal.NewErrorModal(pg.Load, values.String(values.StrSeedAlreadyExist), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		clearEditor()
		return
	}

	walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		Title(values.String(values.StrEnterWalDetails)).
		EnableName(false).
		ShowWalletInfoTip(true).
		SetParent(pg).
		SetPositiveButtonCallback(func(walletName, password string, m *modal.CreatePasswordModal) bool {
			_, err := pg.AssetsManager.RestoreWallet(pg.walletType, pg.walletName, seedOrHex, password, sharedW.PassphraseTypePass)
			if err != nil {
				errString := err.Error()
				if err.Error() == libutils.ErrExist {
					errString = values.StringF(values.StrWalletExist, pg.walletName)
				}
				m.SetError(errString)
				m.SetLoading(false)
				clearEditor()
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
			clearEditor()
			return true
		})
	pg.ParentWindow().ShowModal(walletPasswordModal)
}
