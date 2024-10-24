package page

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	gioApp "gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/appos"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/root"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/preference"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/shirou/gopsutil/mem"
)

const (
	StartPageID = "start_page"
	// startupSettingsPageIndex is the index of the settings setup page.
	startupSettingsPageIndex = 1
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

	image *cryptomaterial.Image
}

type startPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	ctx context.Context

	addWalletButton     cryptomaterial.Button
	skipButton          cryptomaterial.Button
	nextButton          cryptomaterial.Button
	backButton          cryptomaterial.IconButton
	networkSwitchButton *cryptomaterial.Clickable

	settingsOptions []*settingsOption

	onBoardingScreens []onBoardingScreen
	languageDropdown  *cryptomaterial.DropDown

	loading          bool
	isQuitting       bool
	displayStartPage bool

	currentPageIndex            int
	selectedSettingsOptionIndex int

	introductionSlider *cryptomaterial.Slider
	logo               *cryptomaterial.Image
}

func NewStartPage(ctx context.Context, l *load.Load, isShuttingDown ...bool) app.Page {
	sp := &startPage{
		ctx:              ctx,
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(StartPageID),
		loading:          true,
		displayStartPage: true,

		addWalletButton:     l.Theme.Button(values.String(values.StrAddWallet)),
		nextButton:          l.Theme.Button(values.String(values.StrNext)),
		skipButton:          l.Theme.OutlineButton(values.String(values.StrSkip)),
		backButton:          components.GetBackButton(l),
		networkSwitchButton: l.Theme.NewClickable(true),
		introductionSlider:  l.Theme.Slider(),
		logo:                l.Theme.Icons.AppIcon,
	}

	sp.introductionSlider.IndicatorBackgroundColor = values.TransparentColor(values.TransparentWhite, 1)
	sp.introductionSlider.SelectedIndicatorColor = sp.Theme.Color.Primary
	sp.introductionSlider.SetDisableDirectionBtn(true)
	sp.introductionSlider.ControlInset = layout.Inset{
		Right: values.MarginPadding16,
		Left:  values.MarginPadding16,
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
		// Mobile devices usually have very limited amount of ram available to an application
		// Ensure mobile users are using badgedb
		if appos.Current().IsMobile() {
			sp.checkDBFile()
			return
		}
		sp.checkStartupSecurityAndStartApp()
	} else {
		sp.loading = false
	}
}

func (sp *startPage) initPage() {
	sp.languageDropdown = sp.Theme.NewCommonDropDown([]cryptomaterial.DropDownItem{
		{Text: titler.String(values.StrEnglish)},
		{Text: titler.String(values.StrFrench)},
		{Text: titler.String(values.StrSpanish)},
	}, nil, values.MarginPadding120, values.StartPageDropdownGroup, false)

	sp.onBoardingScreens = []onBoardingScreen{
		{
			title:    values.String(values.StrMultiWalletSupport),
			subTitle: values.String(values.StrMultiWalletSupportSubtext),
			image:    sp.Theme.Icons.MultiWalletIcon,
		},
		{
			title:    values.String(values.StrCrossPlatform),
			subTitle: values.String(values.StrCrossPlatformSubtext),
			image:    sp.Theme.Icons.CrossPlatformIcon,
		},
		{
			title:    values.String(values.StrIntegratedExchangeFunctionality),
			subTitle: values.String(values.StrIntegratedExchangeSubtext),
			image:    sp.Theme.Icons.IntegratedExchangeIcon,
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
func (sp *startPage) HandleUserInteractions(gtx C) {
	if sp.networkSwitchButton.Clicked(gtx) {
		newNetType := libutils.Testnet
		if sp.AssetsManager.NetType() == libutils.Testnet {
			newNetType = libutils.Mainnet
		}
		settings.ChangeNetworkType(sp.Load, sp.ParentWindow(), string(newNetType))
	}

	if sp.addWalletButton.Clicked(gtx) {
		createWalletPage := components.NewCreateWallet(sp.Load, func(newWallet sharedW.Asset) {
			if newWallet != nil {
				newWallet.SaveUserConfigValue(sharedW.AutoSyncConfigKey, true)
			}
			sp.setLanguagePref(false)
			sp.ParentNavigator().Display(root.NewHomePage(sp.ctx, sp.Load))
		})
		sp.ParentNavigator().Display(createWalletPage)
	}

	if sp.nextButton.Clicked(gtx) {
		if sp.currentPageIndex < startupSettingsPageIndex {
			if sp.introductionSlider.IsLastSlide() {
				sp.currentPageIndex++
			} else {
				sp.introductionSlider.NextSlide()
			}
		} else {
			sp.updateSettings()
			sp.currentPageIndex = -1
		}
	}

	if sp.skipButton.Clicked(gtx) {
		sp.currentPageIndex = 1
	}

	for i, item := range sp.settingsOptions {
		if item.clickable.Clicked(gtx) {
			sp.selectedSettingsOptionIndex = i
			if item.title == values.String(values.StrAdvanced) {
				sp.ParentWindow().Display(settings.NewAppSettingsPage(sp.Load))
			}
		}

		if item.infoButton.Button.Clicked(gtx) {
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

	if sp.languageDropdown.Changed(gtx) {
		// Refresh the user language now.
		values.SetUserLanguage(sp.selectedLanguageKey())
		sp.RefreshTheme(sp.ParentWindow())
	}

	for sp.backButton.Button.Clicked(gtx) {
		sp.introductionSlider.ResetSlide()
		if sp.currentPageIndex < 0 {
			sp.currentPageIndex = 1
		} else {
			sp.currentPageIndex--
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
	sp.updateNextButtonText()
	if sp.currentPageIndex < 0 || sp.isQuitting {
		return sp.welcomePage(gtx)
	}

	if sp.displayStartPage {
		return sp.pageLayout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle, Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return sp.logo.LayoutSize(gtx, values.MarginPadding150)
				}),
				layout.Rigid(func(gtx C) D {
					welcomeText := sp.Theme.Label(sp.ConvertTextSize(values.TextSize60), strings.ToUpper(values.String(values.StrAppName)))
					welcomeText.Alignment = text.Middle
					welcomeText.Font.Weight = font.Bold
					return welcomeText.Layout(gtx)
				}),
			)
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

func (sp *startPage) welcomePage(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return sp.loadingSection(gtx)
		}),
		layout.Expanded(func(gtx C) D {
			if sp.loading {
				return D{}
			}
			return sp.pageHeaderLayout(gtx, "", true)
		}),
	)
}

func (sp *startPage) loadingSection(gtx C) D {
	return sp.pageLayout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle, Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return sp.logo.LayoutSize(gtx, values.MarginPadding150)
			}),
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

func (sp *startPage) introScreenLayout(gtx C) D {
	sliderWidget := make([]layout.Widget, 0)
	for i := range sp.onBoardingScreens {
		onBoardingScreen := sp.onBoardingScreens[i]
		dims := func(gtx C) D {
			return layout.Inset{
				Bottom: values.MarginPadding40,
			}.Layout(gtx, func(gtx C) D {
				return sp.pageSections(gtx, onBoardingScreen)
			})
		}
		sliderWidget = append(sliderWidget, dims)
	}
	return sp.introductionSlider.Layout(gtx, sliderWidget)
}

func (sp *startPage) updateNextButtonText() {
	if sp.currentPageIndex < startupSettingsPageIndex && sp.introductionSlider.IsLastSlide() {
		sp.nextButton.Text = values.String(values.StrGetStarted)
		return
	}
	sp.nextButton.Text = values.String(values.StrNext)
}

func (sp *startPage) introScreenButtons(gtx C) D {
	if sp.introductionSlider.IsLastSlide() {
		return sp.nextButton.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {

			return sp.skipButton.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return sp.nextButton.Layout(gtx)
		}),
	)
}

func (sp *startPage) onBoardingScreensLayout(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if sp.currentPageIndex < startupSettingsPageIndex {
				return sp.pageLayout(gtx, func(gtx C) D {
					sp.nextButton.Inset = layout.UniformInset(values.MarginPadding15)
					if sp.IsMobileView() {
						sp.nextButton.Inset = layout.UniformInset(values.MarginPadding12)
					}
					return layout.Flex{
						Alignment: layout.Middle,
						Axis:      layout.Vertical,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{
								Bottom: values.MarginPadding30,
							}.Layout(gtx, sp.introScreenLayout)
						}),
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Dp(values.MarginPaddingTransform(sp.IsMobileView(), values.MarginPadding420))
							if !sp.IsMobileView() {
								return sp.introScreenButtons(gtx)
							}
							return layout.Inset{Top: values.MarginPadding64}.Layout(gtx, sp.introScreenButtons)
						}),
					)
				})
			}
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return sp.pageHeaderLayout(gtx, values.String(values.StrChooseSetupType), false)
				}),
				layout.Expanded(func(gtx C) D {
					return sp.pageLayout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								if !sp.IsMobileView() {
									return D{}
								}
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceAround}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										titleLabel := sp.Theme.Label(values.TextSize16, values.String(values.StrChooseSetupType))
										titleLabel.Font.Weight = font.Bold
										return titleLabel.Layout(gtx)
									}),
								)
							}),
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
				layout.Expanded(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							langTitle := sp.Theme.Label(values.TextSize16, values.String(values.StrLanguage))
							langTitle.Font.Weight = font.Bold
							return layout.Inset{Top: values.MarginPadding20}.Layout(gtx, langTitle.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.UniformInset(values.MarginPadding10).Layout(gtx, sp.languageDropdown.Layout)
						}),
					)
				}),
			)
		}),
	)
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
				borderColor := sp.Theme.Color.Primary
				if sp.selectedSettingsOptionIndex != i && !item.clickable.IsHovered() {
					borderWidth = 0
					borderColor = sp.Theme.Color.Gray1
				}

				if item.clickable.IsHovered() {
					borderColor = sp.Theme.Color.Gray1
				}

				inset := layout.Inset{}
				if i == 0 && sp.IsMobileView() {
					inset.Top = values.MarginPadding40
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
						Border: cryptomaterial.Border{
							Radius: cryptomaterial.Radius(8),
							Color:  borderColor,
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

func (sp *startPage) pageHeaderLayout(gtx C, headerText string, hideHeaderText bool) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
		Padding:     layout.UniformInset(values.MarginPadding12),
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: values.MarginPadding10, Right: values.MarginPadding10}.Layout(gtx, sp.backButton.Layout)
				}),

				layout.Rigid(func(gtx C) D {
					lbl := sp.Theme.H6(headerText)
					lbl.TextSize = values.TextSizeTransform(sp.IsMobileView(), values.TextSize20)
					if hideHeaderText || sp.IsMobileView() { // hide title when size is not fit
						return D{}
					}

					return lbl.Layout(gtx)
				}),
			)
		}),
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
		return
	}

	sp.AssetsManager.SetTransactionsNotifications(true)
	sp.AssetsManager.SetCurrencyConversionExchange(values.BinanceExchange)
	sp.AssetsManager.SetHTTPAPIPrivacyMode(libutils.GovernanceHTTPAPI, true)
	sp.AssetsManager.SetHTTPAPIPrivacyMode(libutils.ExchangeHTTPAPI, true)
	sp.AssetsManager.SetHTTPAPIPrivacyMode(libutils.FeeRateHTTPAPI, true)
	sp.AssetsManager.SetHTTPAPIPrivacyMode(libutils.VspAPI, true)
	sp.AssetsManager.SetHTTPAPIPrivacyMode(libutils.UpdateAPI, true)
}

func (sp *startPage) checkDBFile() {
	isNewDB := sp.AssetsManager.DBDriver() == "badgerdb"
	numberOfRam, err := getNumberOfRam()
	if err != nil {
		log.Errorf("Error getting number of ram: %v", err)
		return
	}
	fmt.Println("number of ram", numberOfRam)

	if numberOfRam < 4 && !isNewDB {
		sp.showRemoveWalletWarning()
		return
	}

	sp.checkStartupSecurityAndStartApp()
}

func (sp *startPage) checkStartupSecurityAndStartApp() {
	if sp.AssetsManager.IsStartupSecuritySet() {
		sp.unlock()
	} else {
		sp.loading = true
		go func() { _ = sp.openWalletsAndDisplayHomePage("") }()
	}
}

func (sp *startPage) clearAppDir() {
	homeDir, err := gioApp.DataDir()
	if err != nil {
		log.Error("unable to get home dir: %v", err)
		// return nil, fmt.Errorf("unable to get android home dir: %v", err)
	}

	err = os.RemoveAll(homeDir)
	if err != nil {
		// If an error occurs, handle it (e.g., log it or show a message)
		// showErrorMessage(err)
		log.Error("unable to remove home dir: %v", err)
		return
	}
}

func (sp *startPage) showRemoveWalletWarning() {
	warningModal := modal.NewCustomModal(sp.Load).
		Title(values.String(values.StrDataFileErrorTitle)).
		Body(values.String(values.StrDataFileErrorBody)).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetNegativeButtonCallback(func() {
			sp.checkStartupSecurityAndStartApp()
		}).
		SetNegativeButtonText(values.String(values.StrNo)).
		PositiveButtonStyle(sp.Theme.Color.Surface, sp.Theme.Color.Danger).
		SetPositiveButtonText(values.String(values.StrYes)).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			sp.clearAppDir()
			return true
		})
	sp.ParentWindow().ShowModal(warningModal)
}

// Function to get the number of RAM in GB
func getNumberOfRam() (int, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	// Convert bytes to gigabytes
	return int(vmStat.Total / (1024 * 1024 * 1024)), nil
}
