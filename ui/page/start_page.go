package page

import (
	"os"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/root"
	"github.com/crypto-power/cryptopower/ui/values"
)

const StartPageID = "start_page"

type (
	C = layout.Context
	D = layout.Dimensions
)

type setupAction struct {
	title     string
	subTitle  string
	clickable *cryptomaterial.Clickable
	border    cryptomaterial.Border
	width     unit.Dp
}

type onBoardingScreen struct {
	title    string
	subTitle string

	image            *cryptomaterial.Image
	indicatorBtn     *cryptomaterial.Clickable
	languageDropdown *cryptomaterial.DropDown
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
	backButton      cryptomaterial.Clickable

	setupActions []*setupAction

	onBoardingScreens []onBoardingScreen

	loading          bool
	isQuitting       bool
	displayStartPage bool

	currentPage         int
	selectedSetupAction int
}

func NewStartPage(l *load.Load, isShuttingDown ...bool) app.Page {
	sp := &startPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(StartPageID),
		loading:          true,
		displayStartPage: true,

		addWalletButton:     l.Theme.Button(values.String(values.StrAddWallet)),
		nextButton:          l.Theme.Button(values.String(values.StrNext)),
		skipButton:          l.Theme.OutlineButton(values.String(values.StrSkip)),
		backButton:          *l.Theme.NewClickable(true),
		selectedSetupAction: -1,
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
	if sp.isQuitting {
		log.Info("Displaying the shutdown wallets view page")

		sp.loading = true
		return
	}

	if sp.AssetsManager.LoadedWalletsCount() > 0 {
		sp.currentPage = -1
		sp.setLanguageSetting()
		// Set the log levels.
		sp.AssetsManager.GetLogLevels()
		if sp.AssetsManager.IsStartupSecuritySet() {
			sp.unlock()
		} else {
			go sp.openWallets("")
		}
	} else {
		sp.loading = false
	}
}

func (sp *startPage) initSetupItems() {
	radius := cryptomaterial.CornerRadius{
		TopRight:    8,
		TopLeft:     8,
		BottomRight: 8,
		BottomLeft:  8,
	}

	setupActions := []*setupAction{
		{
			title:     values.String(values.StrRecommended),
			subTitle:  values.String(values.StrRecommendedContent),
			clickable: sp.Theme.NewClickable(false),
			border: cryptomaterial.Border{
				Radius: radius,
				Color:  sp.Theme.Color.DefaultThemeColors().White,
				Width:  values.MarginPadding2,
			},
			width: values.MarginPadding110,
		},
		{
			title:     values.String(values.StrAdvanced),
			subTitle:  values.String(values.StrAdvancedContent),
			clickable: sp.Theme.NewClickable(false),
			border: cryptomaterial.Border{
				Radius: radius,
				Color:  sp.Theme.Color.DefaultThemeColors().White,
				Width:  values.MarginPadding2,
			},
			width: values.MarginPadding110,
		},
	}
	sp.setupActions = setupActions
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
		{
			title:    values.String(values.StrChooseSetupType),
			subTitle: values.String(values.StrLanguage),
			languageDropdown: sp.Theme.DropDown([]cryptomaterial.DropDownItem{
				{Text: values.String(values.StrEnglish)},
				{Text: values.String(values.StrSpanish)},
				{Text: values.String(values.StrFrench)},
			}, values.StartPageDropdownGroup, false),
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
			sp.AssetsManager.Shutdown()
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
	err := sp.AssetsManager.OpenWallets(password)
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
		createWalletPage := components.NewCreateWallet(sp.Load, func() {
			sp.ParentNavigator().Display(root.NewHomePage(sp.Load))
		})
		sp.ParentNavigator().Display(createWalletPage)
	}

	if sp.skipButton.Clicked() {
		sp.currentPage = -1
	}

	for sp.nextButton.Clicked() {
		// if sp.selectedSetupAction == 0 {
		// 	sp.recommendedSettings()
		// } else if sp.selectedSetupAction == 1 {
		// 	sp.ParentNavigator().Display(settings.NewSettingsPage(sp.Load))
		// }
		//Requires refactor of settings page
		if sp.currentPage == len(sp.onBoardingScreens)-1 { // index starts at 0
			sp.currentPage = -1 // we have reached the last screen.
		} else {
			sp.currentPage++
		}
	}

	for i, item := range sp.setupActions {
		for item.clickable.Clicked() {
			sp.selectedSetupAction = i
		}
	}

	for sp.backButton.Clicked() {
		sp.currentPage--
	}

	for i, onBoardingScreen := range sp.onBoardingScreens {
		if i < 3 {
			if onBoardingScreen.indicatorBtn.Clicked() {
				sp.currentPage = i
			}
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
	if sp.Load.IsMobileView() {
		return sp.layoutMobile(gtx)
	}
	return sp.layoutDesktop(gtx)
}

// Desktop layout
func (sp *startPage) layoutDesktop(gtx C) D {
	if sp.currentPage < 0 || sp.isQuitting {
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
				netType := sp.AssetsManager.NetType().Display()
				nType := sp.Theme.Label(values.TextSize20, netType)
				nType.Font.Weight = font.Medium
				return layout.Inset{Top: values.MarginPadding14}.Layout(gtx, nType.Layout)
			}),
			layout.Rigid(func(gtx C) D {
				if sp.loading {
					loadStatus := sp.Theme.Label(values.TextSize20, values.String(values.StrLoading))
					if sp.AssetsManager.LoadedWalletsCount() > 0 {
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
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if sp.currentPage == 3 {
				return layout.Inset{Bottom: values.MarginPaddingMinus145, Left: values.MarginPadding20, Top: values.MarginPadding20}.Layout(gtx, sp.pageHeaderLayout)
			}
			return D{}
		}),
		layout.Rigid(func(gtx C) D {
			return sp.pageLayout(gtx, func(gtx C) D {
				if sp.currentPage > 2 {
					return layout.Flex{
						Alignment: layout.Middle,
						Axis:      layout.Vertical,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							lblTitle := sp.Theme.Label(values.TextSize16, sp.onBoardingScreens[sp.currentPage].title)
							lblTitle.Font.Weight = font.Bold
							return layout.Inset{Bottom: values.MarginPadding80}.Layout(gtx, lblTitle.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx, sp.languageLayout)
						}),
						layout.Rigid(sp.setupButton),
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding570)
							return layout.Inset{Top: values.MarginPadding20}.Layout(gtx, sp.nextButton.Layout)
						}),
					)
				}
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
		}),
	)

}

func (sp *startPage) languageLayout(gtx C) D {
	// return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Direction:   layout.Center,
		Alignment:   layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			langTitle := sp.Theme.Label(values.TextSize16, values.String(values.StrLanguage))
			langTitle.Font.Weight = font.Bold
			return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, langTitle.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return sp.onBoardingScreens[sp.currentPage].languageDropdown.Layout(gtx)
					}),
				)
			})
		}),
	)
}

func (sp *startPage) setupButton(gtx C) D {
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceEnd}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			list := layout.List{}
			return list.Layout(gtx, len(sp.setupActions), func(gtx C, i int) D {
				item := sp.setupActions[i]

				col := sp.Theme.Color.White
				btnTitle := sp.Theme.Label(values.TextSize20, item.title)
				btnTitle.Font.Weight = font.Bold
				content := sp.Theme.Label(values.TextSize16, item.subTitle)
				content.Alignment = text.Alignment(layout.Middle)

				radius := cryptomaterial.CornerRadius{
					TopLeft:     8,
					TopRight:    8,
					BottomRight: 8,
					BottomLeft:  8,
				}
				border := sp.Theme.Color.White

				item.border = cryptomaterial.Border{
					Radius: radius,
					Color:  border,
					Width:  values.MarginPadding2,
				}

				if sp.selectedSetupAction == i {

					col = sp.Theme.Color.White

					item.border = cryptomaterial.Border{
						Radius: radius,
						Color:  sp.Theme.Color.Primary,
						Width:  values.MarginPadding2,
					}
				}

				if item.clickable.IsHovered() {
					item.border = cryptomaterial.Border{
						Radius: radius,
						Color:  sp.Theme.Color.Primary,
						Width:  values.MarginPadding2,
					}
				}

				return layout.Inset{
					Right: values.MarginPadding8,
				}.Layout(gtx, func(gtx C) D {
					return cryptomaterial.LinearLayout{
						Width:       gtx.Dp(270),
						Height:      gtx.Dp(180),
						Orientation: layout.Vertical,
						Direction:   layout.Center,
						Alignment:   layout.Middle,
						Clickable:   item.clickable,
						Border:      item.border,
						Background:  col,
						Padding: layout.Inset{
							Top:   values.MarginPadding20,
							Left:  values.MarginPadding20,
							Right: values.MarginPadding20,
						},
						Margin: layout.Inset{Bottom: values.MarginPadding15},
					}.Layout(gtx,
						layout.Rigid(btnTitle.Layout),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding5, Bottom: values.MarginPadding10}.Layout(gtx, content.Layout)
						}),
					)
				})
			})
		}),
	)
}

func (sp *startPage) pageHeaderLayout(gtx C) layout.Dimensions {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
		Clickable:   &sp.backButton,
		Padding:     layout.UniformInset(values.MarginPadding12),
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return sp.Theme.Icons.ChevronLeft.LayoutSize(gtx, values.MarginPadding24)
		}),
		layout.Rigid(sp.Theme.Label(values.TextSize20, values.String(values.StrBack)).Layout),
	)
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
		if i < 3 {
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
		}
		return D{}
	})
}

func (sp *startPage) setLanguageSetting() {
	langPre := sp.AssetsManager.GetLanguagePreference()
	if langPre == "" {
		sp.AssetsManager.SetLanguagePreference(values.DefaultLangauge)
	}
	values.SetUserLanguage(langPre)
}

//func (sp *startPage) recommendedSettings() {
// 	To be implemented after settings page refactor
// Should set settings for USD exchange, Fee rate api,
// exchange api, and transaction notifications to enabled.
//}
