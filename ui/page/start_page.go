package page

import (
	"os"
	"strings"

	"gioui.org/layout"
	"gioui.org/text"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/root"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

const StartPageID = "start_page"

type (
	C = layout.Context
	D = layout.Dimensions
)

type startPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	addWalletButton cryptomaterial.Button

	loading    bool
	isQuitting bool
}

func NewStartPage(l *load.Load, isShuttingDown ...bool) app.Page {
	sp := &startPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(StartPageID),
		loading:          true,

		addWalletButton: l.Theme.Button(values.String(values.StrAddWallet)),
	}

	if len(isShuttingDown) > 0 {
		sp.isQuitting = isShuttingDown[0]
	}

	return sp
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (sp *startPage) OnNavigatedTo() {
	sp.WL.MultiWallet = sp.WL.Wallet.GetMultiWallet()

	if sp.isQuitting {
		log.Info("Displaying the shutdown wallets view page")

		sp.loading = true
		return
	}

	if sp.WL.MultiWallet.LoadedWalletsCount() > 0 {
		sp.setLanguageSetting()
		// Set the log levels.
		sp.WL.MultiWallet.GetLogLevels()
		if sp.WL.MultiWallet.IsStartupSecuritySet() {
			sp.unlock()
		} else {
			go sp.openWallets("")
		}
	} else {
		sp.loading = false
	}
}

func (sp *startPage) unlock() {
	startupPasswordModal := modal.NewCreatePasswordModal(sp.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrUnlockWithPassword)).
		PasswordHint(values.String(values.StrStartupPassword)).
		SetNegativeButtonText(values.String(values.StrExit)).
		SetNegativeButtonCallback(func() {
			sp.WL.MultiWallet.Shutdown()
			os.Exit(0)
		}).
		SetCancelable(false).
		SetPositiveButtonText(values.String(values.StrUnlock)).
		SetPositiveButtonCallback(func(_, password string, m *modal.CreatePasswordModal) bool {
			err := sp.openWallets(password)
			if err != nil {
				m.SetError(err.Error())
				m.SetLoading(false)
				return false
			}

			m.Dismiss()
			return true
		})
	sp.ParentWindow().ShowModal(startupPasswordModal)
}

func (sp *startPage) openWallets(password string) error {
	err := sp.WL.MultiWallet.OpenWallets(password)
	if err != nil {
		log.Errorf("Error opening wallet: %v", err)
		// show err dialog
		return err
	}

	onWalSelected := func() {
		sp.ParentNavigator().ClearStackAndDisplay(root.NewMainPage(sp.Load))
	}
	onDexServerSelected := func(server string) {
		log.Info("Not implemented yet...", server)
	}
	sp.ParentNavigator().ClearStackAndDisplay(root.NewWalletDexServerSelector(sp.Load, onWalSelected, onDexServerSelected))
	return nil
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (sp *startPage) HandleUserInteractions() {
	for sp.addWalletButton.Clicked() {
		sp.ParentNavigator().Display(root.NewCreateWallet(sp.Load))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (sp *startPage) OnNavigatedFrom() {}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (sp *startPage) Layout(gtx C) D {
	if sp.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return sp.layoutMobile(gtx)
	}
	return sp.layoutDesktop(gtx)
}

// Desktop layout
func (sp *startPage) layoutDesktop(gtx C) D {
	gtx.Constraints.Min = gtx.Constraints.Max // use maximum height & width
	return layout.Flex{
		Alignment: layout.Middle,
		Axis:      layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return sp.loadingSection(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if sp.loading {
				return D{}
			}

			gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding350)
			return layout.Inset{
				Left:  values.MarginPadding24,
				Right: values.MarginPadding24,
			}.Layout(gtx, sp.addWalletButton.Layout)
		}),
	)
}

func (sp *startPage) loadingSection(gtx C) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X // use maximum width
	if sp.loading {
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	} else {
		gtx.Constraints.Min.Y = (gtx.Constraints.Max.Y * 65) / 100 // use 65% of view height
	}

	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Stacked(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle, Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Center.Layout(gtx, func(gtx C) D {
						welcomeText := sp.Theme.Label(values.TextSize60, strings.ToUpper(values.String(values.StrAppName)))
						welcomeText.Alignment = text.Middle
						welcomeText.Font.Weight = text.Bold
						return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, welcomeText.Layout)
					})
				}),
				layout.Rigid(func(gtx C) D {
					netType := sp.WL.Wallet.Net.Display()
					nType := sp.Theme.Label(values.TextSize20, netType)
					nType.Font.Weight = text.Medium
					return layout.Inset{Top: values.MarginPadding14}.Layout(gtx, nType.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					if sp.loading {
						loadStatus := sp.Theme.Label(values.TextSize20, values.String(values.StrLoading))
						if sp.WL.MultiWallet.LoadedWalletsCount() > 0 {
							switch {
							case sp.isQuitting:
								loadStatus.Text = values.String(values.StrClosingWallet)
							default:
								loadStatus.Text = values.String(values.StrOpeningWallet)
							}
						}

						return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, loadStatus.Layout)
					}

					welcomeText := sp.Theme.Label(values.TextSize24, values.String(values.StrWelcomeNote))
					welcomeText.Alignment = text.Middle
					return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, welcomeText.Layout)
				}),
			)
		}),
	)
}

// Mobile layout
func (sp *startPage) layoutMobile(gtx C) D {
	gtx.Constraints.Min = gtx.Constraints.Max // use maximum height & width
	return layout.Flex{
		Alignment: layout.Middle,
		Axis:      layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return sp.loadingSection(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if sp.loading {
				return D{}
			}

			gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding350)
			return layout.Inset{
				Left:  values.MarginPadding24,
				Right: values.MarginPadding24,
			}.Layout(gtx, sp.addWalletButton.Layout)
		}),
	)
}

func (sp *startPage) setLanguageSetting() {
	langPre := sp.WL.MultiWallet.GetLanguagePreference()
	if langPre == "" {
		sp.WL.MultiWallet.SetLanguagePreference(values.DefaultLangauge)
	}
	values.SetUserLanguage(langPre)
}
