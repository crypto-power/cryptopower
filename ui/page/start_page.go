package page

import (
	"os"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/root"
	"github.com/crypto-power/cryptopower/ui/values"
)

const StartPageID = "start_page"

type (
	C = layout.Context
	D = layout.Dimensions
)

type onBoardingScreen struct {
	title        string
	subTitle     string
	image        *cryptomaterial.Image
	indicatorBtn *cryptomaterial.Clickable
}

type startPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	addWalletButton cryptomaterial.Button
	nextButton      cryptomaterial.Button
	skipButton      cryptomaterial.Button

	onBoardingScreens []onBoardingScreen

	loading          bool
	isQuitting       bool
	displayStartPage bool

	currentPage int
}

func NewStartPage(l *load.Load, isShuttingDown ...bool) app.Page {
	sp := &startPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(StartPageID),
		loading:          true,
		displayStartPage: true,

		addWalletButton: l.Theme.Button(values.String(values.StrAddWallet)),
		nextButton:      l.Theme.Button(values.String(values.StrNext)),
		skipButton:      l.Theme.OutlineButton(values.String(values.StrSkip)),
	}

	if len(isShuttingDown) > 0 {
		sp.isQuitting = isShuttingDown[0]
	}

	sp.initPages()

	return sp
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (sp *startPage) OnNavigatedTo() {
	sp.WL.AssetsManager = sp.WL.Wallet.GetAssetsManager()

	if sp.isQuitting {
		log.Info("Displaying the shutdown wallets view page")

		sp.loading = true
		return
	}

	if sp.WL.AssetsManager.LoadedWalletsCount() > 0 {
		sp.currentPage = -1
		sp.setLanguageSetting()
		// Set the log levels.
		sp.WL.AssetsManager.GetLogLevels()
		if sp.WL.AssetsManager.IsStartupSecuritySet() {
			sp.unlock()
		} else {
			go sp.openWallets("")
		}
	} else {
		sp.loading = false
	}
}

func (sp *startPage) initPages() {
	sp.onBoardingScreens = []onBoardingScreen{
		{
			title:        values.String(values.StrMultiWalletSupport),
			subTitle:     values.String(values.StrMultiWalletSupportSubtext),
			image:        sp.Theme.Icons.MultiWalletIcon,
			indicatorBtn: sp.Theme.NewClickable(false),
		},
		{
			title:        values.String(values.StrCrossPlatform),
			subTitle:     values.String(values.StrCrossPlatformSubtext),
			image:        sp.Theme.Icons.CrossPlatformIcon,
			indicatorBtn: sp.Theme.NewClickable(false),
		},
		{
			title:        values.String(values.StrIntegratedExchange),
			subTitle:     values.String(values.StrIntegratedExchangeSubtext),
			image:        sp.Theme.Icons.IntegratedExchangeIcon,
			indicatorBtn: sp.Theme.NewClickable(false),
		},
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
			sp.WL.AssetsManager.Shutdown()
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
	err := sp.WL.AssetsManager.OpenWallets(password)
	if err != nil {
		log.Errorf("Error opening wallet: %v", err)
		// show err dialog
		return err
	}

	sp.ParentNavigator().ClearStackAndDisplay(root.NewHomePage(sp.Load))
	return nil
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (sp *startPage) HandleUserInteractions() {
	if sp.addWalletButton.Clicked() {
		sp.ParentNavigator().Display(root.NewCreateWallet(sp.Load))
	}

	if sp.skipButton.Clicked() {
		sp.currentPage = -1
	}

	for sp.nextButton.Clicked() {
		if sp.currentPage == len(sp.onBoardingScreens)-1 { // index starts at 0
			sp.currentPage = -1 // we have reached the last screen.
		} else {
			sp.currentPage++
		}
	}

	for i, onBoardingScreen := range sp.onBoardingScreens {
		if onBoardingScreen.indicatorBtn.Clicked() {
			sp.currentPage = i
		}
	}

	if sp.displayStartPage {
		time.AfterFunc(time.Second*2, func() {
			sp.displayStartPage = false
			sp.ParentWindow().Reload()
		})
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
	gtx.Constraints.Min = gtx.Constraints.Max // use maximum height & width
	sp.Load.SetCurrentAppWidth(gtx.Constraints.Max.X)
	if sp.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return sp.layoutMobile(gtx)
	}
	return sp.layoutDesktop(gtx)
}

// Desktop layout
func (sp *startPage) layoutDesktop(gtx C) D {
	if sp.currentPage < 0 {
		return sp.loadingSection(gtx)
	}

	if sp.displayStartPage {
		return sp.pageLayout(gtx, func(gtx C) D {
			welcomeText := sp.Theme.Label(values.TextSize60, strings.ToUpper(values.String(values.StrAppName)))
			welcomeText.Alignment = text.Middle
			welcomeText.Font.Weight = font.Bold
			return welcomeText.Layout(gtx)
		})
	}
	return sp.onBoardingScreensLayout(gtx)
}

func (sp *startPage) pageLayout(gtx C, body layout.Widget) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.MatchParent,
		Orientation: layout.Vertical,
		Alignment:   layout.Middle,
		Direction:   layout.Center,
	}.Layout2(gtx, body)
}

func (sp *startPage) loadingSection(gtx C) D {
	return sp.pageLayout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle, Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					welcomeText := sp.Theme.Label(values.TextSize60, strings.ToUpper(values.String(values.StrAppName)))
					welcomeText.Alignment = text.Middle
					welcomeText.Font.Weight = font.Bold
					return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, welcomeText.Layout)
				})
			}),
			layout.Rigid(func(gtx C) D {
				netType := sp.WL.Wallet.Net.Display()
				nType := sp.Theme.Label(values.TextSize20, netType)
				nType.Font.Weight = font.Medium
				return layout.Inset{Top: values.MarginPadding14}.Layout(gtx, nType.Layout)
			}),
			layout.Rigid(func(gtx C) D {
				if sp.loading {
					loadStatus := sp.Theme.Label(values.TextSize20, values.String(values.StrLoading))
					if sp.WL.AssetsManager.LoadedWalletsCount() > 0 {
						switch {
						case sp.isQuitting:
							loadStatus.Text = values.String(values.StrClosingWallet)

							for {
								// Closes all pending modals still open.
								modal := sp.ParentWindow().TopModal()
								if modal == nil {
									// No modal that exists.
									break
								}
								sp.ParentWindow().DismissModal(modal.ID())
							}

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
			layout.Rigid(func(gtx C) D {
				if sp.loading {
					return D{}
				}
				gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding350)
				return layout.Inset{
					Top:   values.MarginPadding100,
					Left:  values.MarginPadding24,
					Right: values.MarginPadding24,
				}.Layout(gtx, sp.addWalletButton.Layout)

			}),
		)
	})
}

// Mobile layout
func (sp *startPage) layoutMobile(gtx C) D {
	if sp.currentPage < 0 {
		return sp.loadingSection(gtx)
	}

	if sp.displayStartPage {
		return sp.pageLayout(gtx, func(gtx C) D {
			welcomeText := sp.Theme.Label(values.TextSize60, strings.ToUpper(values.String(values.StrAppName)))
			welcomeText.Alignment = text.Middle
			welcomeText.Font.Weight = font.Bold
			return welcomeText.Layout(gtx)
		})
	}
	return sp.onBoardingScreensLayout(gtx)
}

func (sp *startPage) onBoardingScreensLayout(gtx C) D {
	return sp.pageLayout(gtx, func(gtx C) D {
		return layout.Flex{
			Alignment: layout.Middle,
			Axis:      layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				list := &layout.List{Axis: layout.Horizontal}
				return list.Layout(gtx, len(sp.onBoardingScreens), func(gtx C, i int) D {
					if i == sp.currentPage {
						return sp.pageSections(gtx, sp.onBoardingScreens[i])
					}
					return D{}
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Top:    values.MarginPadding35,
					Bottom: values.MarginPadding35,
				}.Layout(gtx, sp.currentPageIndicatorLayout)
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding350)
				return sp.nextButton.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding350)
					return sp.skipButton.Layout(gtx)
				})
			}),
		)
	})
}

func (sp *startPage) pageSections(gtx C, onBoardingScreen onBoardingScreen) D {
	return layout.Flex{Alignment: layout.Middle, Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return onBoardingScreen.image.LayoutSize2(gtx, values.MarginPadding280, values.MarginPadding172)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Center.Layout(gtx, func(gtx C) D {
				lblPageTitle := sp.Theme.Label(values.TextSize32, onBoardingScreen.title)
				lblPageTitle.Alignment = text.Middle
				lblPageTitle.Font.Weight = font.Bold
				return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, lblPageTitle.Layout)
			})
		}),
		layout.Rigid(func(gtx C) D {
			lblSubTitle := sp.Theme.Label(values.TextSize16, onBoardingScreen.subTitle)
			return layout.Inset{Top: values.MarginPadding14}.Layout(gtx, lblSubTitle.Layout)
		}),
	)
}

func (sp *startPage) currentPageIndicatorLayout(gtx C) D {
	if sp.currentPage < 0 {
		return D{}
	}

	list := &layout.List{Axis: layout.Horizontal}
	return list.Layout(gtx, len(sp.onBoardingScreens), func(gtx C, i int) D {
		ic := cryptomaterial.NewIcon(sp.Theme.Icons.ImageBrightness1)
		ic.Color = values.TransparentColor(values.TransparentBlack, 0.2)
		if i == sp.currentPage {
			ic.Color = sp.Theme.Color.Primary
		}
		return layout.Inset{
			Right: values.MarginPadding4,
			Left:  values.MarginPadding4,
		}.Layout(gtx, func(gtx C) D {
			return sp.onBoardingScreens[i].indicatorBtn.Layout(gtx, func(gtx C) D {
				return ic.Layout(gtx, values.MarginPadding12)
			})
		})
	})
}

func (sp *startPage) setLanguageSetting() {
	langPre := sp.WL.AssetsManager.GetLanguagePreference()
	if langPre == "" {
		sp.WL.AssetsManager.SetLanguagePreference(values.DefaultLangauge)
	}
	values.SetUserLanguage(langPre)
}
