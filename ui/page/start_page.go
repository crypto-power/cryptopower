package page

import (
	"context"
	"os"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/crypto-power/cryptopower/app"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/root"
	"github.com/crypto-power/cryptopower/ui/preference"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	StartPageID = "start_page"
	// startupSettingsPageIndex is the index of the settings setup page.
	startupSettingsPageIndex = 3
	// advancedSettingsOptionIndex is the index of the advanced settings option.
	advancedSettingsOptionIndex = 1
)

// settingsOptionPageWidth is an arbitrary width for the settings setup
// page.
var settingsOptionPageWidth = values.MarginPadding570
var titler = cases.Title(language.Und)

type (
	C = layout.Context
	D = layout.Dimensions
)

type settingsOption struct {
	title      string
	message    string
	infoButton cryptomaterial.IconButton
	clickable  *cryptomaterial.Clickable
}

type onBoardingScreen struct {
	title    string
	subTitle string

	image        *cryptomaterial.Image     // optional
	indicatorBtn *cryptomaterial.Clickable // optional
}

type startPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	ctx context.Context

	addWalletButton cryptomaterial.Button
	nextButton      cryptomaterial.Button
	backButton      cryptomaterial.Clickable

	settingsOptions []*settingsOption

	onBoardingScreens []onBoardingScreen
	languageDropdown  *cryptomaterial.DropDown

	loading          bool
	isQuitting       bool
	displayStartPage bool

	currentPageIndex            int
	selectedSettingsOptionIndex int
}

func NewStartPage(ctx context.Context, l *load.Load, isShuttingDown ...bool) app.Page {
	sp := &startPage{
		ctx:              ctx,
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(StartPageID),
		loading:          true,
		displayStartPage: true,

		addWalletButton: l.Theme.Button(values.String(values.StrAddWallet)),
		nextButton:      l.Theme.Button(values.String(values.StrNext)),
		backButton:      *l.Theme.NewClickable(true),
	}

	if len(isShuttingDown) > 0 {
		sp.isQuitting = isShuttingDown[0]
	}

	sp.initPage()

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
		sp.currentPageIndex = -1
		sp.setLanguagePref(true)
		// Set the log levels.
		sp.AssetsManager.GetLogLevels()
		if sp.AssetsManager.IsStartupSecuritySet() {
			sp.unlock()
		} else {
			go sp.openWalletsAndDisplayHomePage("")
		}
	} else {
		sp.loading = false
	}
}

func (sp *startPage) initPage() {
	sp.languageDropdown = sp.Theme.DropDown([]cryptomaterial.DropDownItem{
		{Text: titler.String(values.StrEnglish)},
		{Text: titler.String(values.StrFrench)},
		{Text: titler.String(values.StrSpanish)},
	}, nil, values.StartPageDropdownGroup, true)

	sp.languageDropdown.MakeCollapsedLayoutVisibleWhenExpanded = true
	sp.languageDropdown.Background = &sp.Theme.Color.Surface
	sp.languageDropdown.FontWeight = font.SemiBold
	sp.languageDropdown.SelectedItemIconColor = &sp.Theme.Color.Primary
	sp.languageDropdown.BorderWidth = 2
	sp.languageDropdown.Width = values.MarginPadding120
	sp.languageDropdown.ExpandedLayoutInset = layout.Inset{Top: values.MarginPadding50}
	sp.languageDropdown.MakeCollapsedLayoutVisibleWhenExpanded = true

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
			title:        values.String(values.StrIntegratedExchangeFunctionality),
			subTitle:     values.String(values.StrIntegratedExchangeSubtext),
			image:        sp.Theme.Icons.IntegratedExchangeIcon,
			indicatorBtn: sp.Theme.NewClickable(false),
		},
		{
			title:    values.String(values.StrChooseSetupType),
			subTitle: values.String(values.StrLanguage),
		},
	}

	sp.settingsOptions = []*settingsOption{
		{
			title:      values.String(values.StrRecommended),
			message:    values.String(values.StrRecommendedSettingsMsg),
			infoButton: sp.Theme.IconButton(sp.Theme.Icons.ActionInfo),
			clickable:  sp.Theme.NewClickable(false),
		},
		{
			title:      values.String(values.StrAdvanced),
			message:    values.String(values.StrAdvancedSettingsMsg),
			infoButton: sp.Theme.IconButton(sp.Theme.Icons.ActionInfo),
			clickable:  sp.Theme.NewClickable(false),
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
			err := sp.openWalletsAndDisplayHomePage(password)
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

func (sp *startPage) openWalletsAndDisplayHomePage(password string) error {
	err := sp.AssetsManager.OpenWallets(password)
	if err != nil {
		log.Errorf("Error opening wallet: %v", err)
		// show err dialog
		return err
	}

	sp.ParentNavigator().ClearStackAndDisplay(root.NewHomePage(sp.ctx, sp.Load))
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
			sp.setLanguagePref(false)
			sp.updateSettings()
			sp.ParentNavigator().Display(root.NewHomePage(sp.ctx, sp.Load))
		})
		sp.ParentNavigator().Display(createWalletPage)
	}

	for sp.nextButton.Clicked() {
		// TODO: Handle Selected settings option (language and advanced or
		// recommended settings). Might requires refactor of settings page.
		if sp.currentPageIndex == len(sp.onBoardingScreens)-1 { // index starts at 0
			sp.currentPageIndex = -1 // we have reached the last screen.
		} else {
			sp.currentPageIndex++
		}
	}

	for i, item := range sp.settingsOptions {
		for item.clickable.Clicked() { // TODO: Show settings page and allow user pick settings for advanced setup.
			sp.selectedSettingsOptionIndex = i
		}

		for item.infoButton.Button.Clicked() {
			body := values.String(values.StrRecommendedModalBody)
			if i == advancedSettingsOptionIndex {
				body = values.String(values.StrAdvancedModalBody)
			}
			infoModal := modal.NewCustomModal(sp.Load).
				Title(item.title+" "+values.String(values.StrSettings)).
				Body(body).
				SetCancelable(true).
				SetContentAlignment(layout.Center, layout.Center, layout.Center).
				SetPositiveButtonText(values.String(values.StrGotIt))
			sp.ParentWindow().ShowModal(infoModal)
		}
	}

	if sp.languageDropdown.Changed() {
		// Refresh the user language now.
		values.SetUserLanguage(sp.selectedLanguageKey())
		sp.RefreshTheme(sp.ParentWindow())
	}

	for sp.backButton.Clicked() {
		sp.currentPageIndex--
	}

	for i, onBoardingScreen := range sp.onBoardingScreens {
		if i < startupSettingsPageIndex {
			if onBoardingScreen.indicatorBtn.Clicked() {
				sp.currentPageIndex = i
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
	if sp.currentPageIndex < 0 || sp.isQuitting {
		return sp.loadingSection(gtx)
	}

	if sp.displayStartPage {
		return sp.pageLayout(gtx, func(gtx C) D {
			welcomeText := sp.Theme.Label(sp.ConvertTextSize(values.TextSize60), strings.ToUpper(values.String(values.StrAppName)))
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
		Padding:     layout.UniformInset(values.MarginPadding12),
	}.Layout2(gtx, body)
}

func (sp *startPage) loadingSection(gtx C) D {
	return sp.pageLayout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle, Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					welcomeText := sp.Theme.Label(sp.ConvertTextSize(values.TextSize60), strings.ToUpper(values.String(values.StrAppName)))
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
				if !sp.loading {
					welcomeText := sp.Theme.Label(sp.ConvertTextSize(values.TextSize24), values.String(values.StrWelcomeNote))
					welcomeText.Alignment = text.Middle
					return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, welcomeText.Layout)
				}

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
			}),
			layout.Rigid(func(gtx C) D {
				if sp.loading {
					return D{}
				}
				inset := layout.Inset{Top: values.MarginPadding100}
				if sp.IsMobileView() {
					inset.Top += values.MarginPadding168
				}
				gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding350)
				return inset.Layout(gtx, sp.addWalletButton.Layout)
			}),
		)
	})
}

func (sp *startPage) onBoardingScreensLayout(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if sp.currentPageIndex != startupSettingsPageIndex {
				return D{}
			}
			return layout.Inset{Bottom: values.MarginPaddingMinus145}.Layout(gtx, sp.pageHeaderLayout)
		}),
		layout.Rigid(func(gtx C) D {
			return sp.pageLayout(gtx, func(gtx C) D {
				sp.nextButton.Inset = layout.UniformInset(values.MarginPadding15)
				if sp.IsMobileView() {
					sp.nextButton.Inset = layout.UniformInset(values.MarginPadding12)
				}
				if sp.currentPageIndex < startupSettingsPageIndex {
					return layout.Flex{
						Alignment: layout.Middle,
						Axis:      layout.Vertical,
					}.Layout(gtx,
						layout.Rigid(sp.onBoardingScreenLayout),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Top:    values.MarginPadding30,
								Bottom: values.MarginPadding30,
							}.Layout(gtx, sp.currentPageIndicatorLayout)
						}),
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Dp(values.MarginPaddingTransform(sp.IsMobileView(), values.MarginPadding420))
							if !sp.IsMobileView() {
								return sp.nextButton.Layout(gtx)
							}
							return layout.Inset{Top: values.MarginPadding64}.Layout(gtx, sp.nextButton.Layout)
						}),
					)
				}

				return layout.Flex{
					Alignment: layout.Middle,
					Axis:      layout.Vertical,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Bottom: values.MarginPaddingMinus195}.Layout(gtx, sp.pageHeaderLayout)
					}),
					layout.Rigid(func(gtx C) D {
						return sp.pageLayout(gtx, func(gtx C) D {
							return layout.Stack{Alignment: layout.Center}.Layout(gtx,
								layout.Expanded(func(gtx C) D {
									return layout.Inset{Top: values.MarginPadding250}.Layout(gtx, func(gtx C) D {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(sp.settingsOptionsLayout),
											layout.Rigid(func(gtx C) D {
												gtx.Constraints.Min.X = gtx.Dp(settingsOptionPageWidth)
												inset := layout.Inset{Top: values.MarginPadding90}
												if !sp.IsMobileView() {
													inset.Top = values.MarginPadding20
												}
												return inset.Layout(gtx, sp.nextButton.Layout)
											}),
										)
									})
								}),
								layout.Stacked(func(gtx C) D {
									return layout.Inset{Top: values.MarginPaddingMinus200}.Layout(gtx, func(gtx C) D {
										return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												titleLabel := sp.Theme.Label(values.TextSize16, sp.onBoardingScreens[sp.currentPageIndex].title)
												titleLabel.Font.Weight = font.Bold
												return layout.Inset{Bottom: values.MarginPadding40}.Layout(gtx, titleLabel.Layout)
											}),
											layout.Rigid(func(gtc C) D {
												gtx.Constraints.Max.Y = gtx.Dp(values.MarginPadding48)
												return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
													layout.Rigid(func(gtx C) D {
														langTitle := sp.Theme.Label(values.TextSize16, values.String(values.StrLanguage))
														langTitle.Font.Weight = font.Bold
														return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, langTitle.Layout)
													}),
													layout.Rigid(func(gtx C) D {
														return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, sp.languageDropdown.Layout)
													}),
												)
											}),
										)
									})
								}),
							)
						})
					}),
				)
			})
		}),
	)
}

func (sp *startPage) onBoardingScreenLayout(gtx C) D {
	list := layout.List{Axis: layout.Horizontal}
	return list.Layout(gtx, len(sp.onBoardingScreens), func(gtx C, i int) D {
		if i != sp.currentPageIndex {
			return D{}
		}
		return sp.pageSections(gtx, sp.onBoardingScreens[sp.currentPageIndex])
	})
}

func (sp *startPage) settingsOptionsLayout(gtx C) D {
	padding := values.MarginPadding16
	optionWidth := (settingsOptionPageWidth - padding) / unit.Dp(len(sp.settingsOptions))
	return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			list := layout.List{Alignment: layout.Middle}
			height := gtx.Dp(180)
			width := gtx.Dp(optionWidth)
			if sp.IsMobileView() {
				width = cryptomaterial.MatchParent
				list.Axis = layout.Vertical
				height = gtx.Dp(116)
			}
			return list.Layout(gtx, len(sp.settingsOptions), func(gtx C, i int) D {
				item := sp.settingsOptions[i]
				btnTitle := sp.Theme.Label(values.TextSize20, item.title)
				btnTitle.Font.Weight = font.Bold
				content := sp.Theme.Label(values.TextSize16, item.message)
				content.Alignment = text.Alignment(layout.Middle)
				item.infoButton.Size = values.MarginPaddingTransform(sp.IsMobileView(), values.MarginPadding20)

				borderWidth := values.MarginPadding2
				if sp.selectedSettingsOptionIndex != i && !item.clickable.IsHovered() {
					borderWidth = 0
				}

				inset := layout.Inset{}
				if i == 0 && sp.IsMobileView() {
					inset.Top = values.MarginPadding150
				}
				if i == 0 && !sp.IsMobileView() {
					inset.Right = padding
				}
				return inset.Layout(gtx, func(gtx C) D {
					return cryptomaterial.LinearLayout{
						Width:       width,
						Height:      height,
						Orientation: layout.Vertical,
						Direction:   layout.Center,
						Alignment:   layout.Middle,
						Clickable:   item.clickable,
						Background:  sp.Theme.Color.DefaultThemeColors().White,
						Border: cryptomaterial.Border{
							Radius: cryptomaterial.Radius(8),
							Color:  sp.Theme.Color.Primary,
							Width:  borderWidth,
						},
						Margin: layout.Inset{Bottom: values.MarginPadding15},
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							widgets := []func(gtx C) D{
								btnTitle.Layout,
								item.infoButton.Layout,
							}
							options := components.FlexOptions{
								Axis:      layout.Horizontal,
								Alignment: layout.Middle,
							}
							return components.FlexLayout(gtx, options, widgets)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, content.Layout)
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
				inset := layout.Inset{Top: values.MarginPadding24}
				if sp.IsMobileView() {
					inset.Top = values.MarginPadding64
					if onBoardingScreen.title == values.String(values.StrIntegratedExchangeFunctionality) {
						onBoardingScreen.title = values.String(values.StrIntegratedExchange)
					}
				}
				lblPageTitle := sp.Theme.Label(values.TextSize32, onBoardingScreen.title)
				lblPageTitle.Alignment = text.Middle
				lblPageTitle.Font.Weight = font.Bold
				return inset.Layout(gtx, lblPageTitle.Layout)
			})
		}),
		layout.Rigid(func(gtx C) D {
			lblSubTitle := sp.Theme.Label(sp.ConvertTextSize(values.TextSize16), onBoardingScreen.subTitle)
			return layout.Inset{Top: values.MarginPadding14}.Layout(gtx, lblSubTitle.Layout)
		}),
	)
}

func (sp *startPage) currentPageIndicatorLayout(gtx C) D {
	if sp.currentPageIndex < 0 {
		return D{}
	}

	list := &layout.List{Axis: layout.Horizontal}
	return list.Layout(gtx, len(sp.onBoardingScreens), func(gtx C, i int) D {
		return layout.Inset{Top: values.MarginPadding35, Bottom: values.MarginPadding35}.Layout(gtx, func(gtx C) D {
			if i > startupSettingsPageIndex-1 {
				return D{}
			}

			ic := cryptomaterial.NewIcon(sp.Theme.Icons.DotIcon)
			ic.Color = values.TransparentColor(values.TransparentBlack, 0.2)
			if i == sp.currentPageIndex {
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
	})
}

func (sp *startPage) setLanguagePref(useExistingUserPreference bool) {
	var lang string
	if useExistingUserPreference {
		lang = sp.AssetsManager.GetLanguagePreference()
	} else {
		lang = sp.selectedLanguageKey()
	}
	if lang == "" {
		lang = values.DefaultLanguage
	}
	sp.AssetsManager.SetLanguagePreference(lang)
	values.SetUserLanguage(lang)
}

func (sp *startPage) selectedLanguageKey() string {
	selectedLang := sp.languageDropdown.Selected()
	for _, opt := range preference.LangOptions {
		if strings.ToLower(selectedLang) == opt.Value {
			return opt.Key
		}
	}
	return values.DefaultLanguage
}

func (sp *startPage) updateSettings() {
	wantAdvanced := sp.selectedSettingsOptionIndex == advancedSettingsOptionIndex
	if wantAdvanced {
		return // nothing to do?
	}

	sp.AssetsManager.SetTransactionsNotifications(true)
	sp.AssetsManager.SetCurrencyConversionExchange(values.BinanceExchange)
	sp.AssetsManager.SetHTTPAPIPrivacyMode(libutils.GovernanceHTTPAPI, true)
	sp.AssetsManager.SetHTTPAPIPrivacyMode(libutils.ExchangeHTTPAPI, true)
	sp.AssetsManager.SetHTTPAPIPrivacyMode(libutils.FeeRateHTTPAPI, true)
	sp.AssetsManager.SetHTTPAPIPrivacyMode(libutils.VspAPI, true)
}
