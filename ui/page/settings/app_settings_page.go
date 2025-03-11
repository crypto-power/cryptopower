package settings

import (
	"image/color"
	"io"
	"regexp"
	"strings"
	"time"

	"decred.org/dcrdex/dex"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/ext"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/logger"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/preference"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const AppSettingsPageID = "Settings"

type (
	C = layout.Context
	D = layout.Dimensions
)

type row struct {
	title     string
	clickable *cryptomaterial.Clickable
	label     cryptomaterial.Label
}

type AppSettingsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	pageContainer *widget.List

	changeStartupPass       *cryptomaterial.Clickable
	network                 *cryptomaterial.Clickable
	language                *cryptomaterial.Clickable
	currency                *cryptomaterial.Clickable
	help                    *cryptomaterial.Clickable
	about                   *cryptomaterial.Clickable
	appearanceMode          *cryptomaterial.Clickable
	startupPassword         *cryptomaterial.Switch
	transactionNotification *cryptomaterial.Switch
	backButton              cryptomaterial.IconButton
	infoButton              cryptomaterial.IconButton
	networkInfoButton       cryptomaterial.IconButton
	logLevel                *cryptomaterial.Clickable
	viewLog                 *cryptomaterial.Clickable
	deleteDEX               *cryptomaterial.Clickable
	backupDEX               *cryptomaterial.Clickable
	copyDEXSeed             cryptomaterial.Button
	dexSeed                 dex.Bytes

	governanceAPI *cryptomaterial.Switch
	exchangeAPI   *cryptomaterial.Switch
	feeRateAPI    *cryptomaterial.Switch
	vspAPI        *cryptomaterial.Switch
	updateAPI     *cryptomaterial.Switch
	privacyActive *cryptomaterial.Switch

	isDarkModeOn      bool
	isStartupPassword bool
}

func NewAppSettingsPage(l *load.Load) *AppSettingsPage {
	pg := &AppSettingsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(AppSettingsPageID),
		pageContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},

		startupPassword:         l.Theme.Switch(),
		transactionNotification: l.Theme.Switch(),
		governanceAPI:           l.Theme.Switch(),
		exchangeAPI:             l.Theme.Switch(),
		feeRateAPI:              l.Theme.Switch(),
		vspAPI:                  l.Theme.Switch(),
		updateAPI:               l.Theme.Switch(),
		privacyActive:           l.Theme.Switch(),

		changeStartupPass: l.Theme.NewClickable(false),
		network:           l.Theme.NewClickable(false),
		language:          l.Theme.NewClickable(false),
		currency:          l.Theme.NewClickable(false),
		help:              l.Theme.NewClickable(false),
		about:             l.Theme.NewClickable(false),
		appearanceMode:    l.Theme.NewClickable(false),
		logLevel:          l.Theme.NewClickable(false),
		viewLog:           l.Theme.NewClickable(false),
		deleteDEX:         l.Theme.NewClickable(false),
		backupDEX:         l.Theme.NewClickable(false),
		copyDEXSeed:       l.Theme.Button(values.String(values.StrCopy)),
	}

	_, pg.networkInfoButton = components.SubpageHeaderButtons(l)
	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.backButton = components.GetBackButton(l)
	pg.isDarkModeOn = pg.AssetsManager.IsDarkModeOn()

	pg.copyDEXSeed.TextSize = values.TextSize14
	pg.copyDEXSeed.Background = color.NRGBA{}
	pg.copyDEXSeed.HighlightColor = pg.Theme.Color.SurfaceHighlight
	pg.copyDEXSeed.Color = pg.Theme.Color.Primary
	pg.copyDEXSeed.Inset = layout.UniformInset(values.MarginPadding16)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *AppSettingsPage) OnNavigatedTo() {
	pg.updateSettingOptions()
	pg.ListenForRateWarningMsgChange()
}

func (pg *AppSettingsPage) ListenForRateWarningMsgChange() {
	// add rate listener
	warningMsgListener := &ext.WarningMsgListener{
		OnWarningMsgUpdated: func(warning string) {
			if warning != "" {
				go pg.showAutoChangeRateSourceNotice(warning)
			}
		},
	}
	if !pg.AssetsManager.RateSource.IsWarningMsgListenerExist(AppSettingsPageID) {
		if err := pg.AssetsManager.RateSource.AddWarningMsgListener(warningMsgListener, AppSettingsPageID); err != nil {
			log.Error("Can't listen warning message.")
		}
	}
}

// Show warning about fetch exchange rate setting
// when exchange is changed due to unable to fetch rate
func (pg *AppSettingsPage) showAutoChangeRateSourceNotice(warnMsg string) {
	lowStorageModal := modal.NewWarningModal(pg.Load, values.String(values.StrFetchRateWarningTitle),
		func(_ bool, _ *modal.InfoModal) bool {
			return true
		}).
		Body(warnMsg).
		SetPositiveButtonText(values.String(values.StrOK))
	pg.ParentWindow().ShowModal(lowStorageModal)
}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *AppSettingsPage) Layout(gtx C) D {
	body := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.pageHeaderLayout),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Top:    values.MarginPadding20,
					Bottom: values.MarginPadding20,
				}.Layout(gtx, pg.pageContentLayout)
			}),
		)
	}

	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx, body)
	}

	return pg.layoutDesktop(gtx, body)
}

func (pg *AppSettingsPage) layoutDesktop(gtx C, body func(gtx C) D) D {
	return layout.UniformInset(values.MarginPadding20).Layout(gtx, body)
}

func (pg *AppSettingsPage) layoutMobile(gtx C, body func(gtx C) D) D {
	return components.UniformMobile(gtx, false, true, body)
}

func (pg *AppSettingsPage) pageHeaderLayout(gtx C) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding500)
		if pg.Load.IsMobileView() {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
		}
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Top:   values.MarginPadding2,
					Right: values.MarginPadding16,
				}.Layout(gtx, pg.backButton.Layout)
			}),
			layout.Rigid(pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize20), values.String(values.StrSettings)).Layout),
		)
	})
}

func (pg *AppSettingsPage) pageContentLayout(gtx C) D {
	pageContent := []func(gtx C) D{
		pg.general(),
		pg.networkSettings(),
		pg.dexSettings(),
		pg.security(),
		pg.info(),
		pg.debug(),
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Center.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding500)
		if pg.Load.IsMobileView() {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
		}
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		return pg.Theme.List(pg.pageContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
			return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, pageContent[i])
		})
	})
}

func (pg *AppSettingsPage) wrapSection(gtx C, title string, body layout.Widget) D {
	return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Orientation: layout.Vertical,
			Width:       cryptomaterial.WrapContent,
			Height:      cryptomaterial.WrapContent,
			Background:  pg.Theme.Color.Surface,
			Direction:   layout.Center,
			Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
			Padding: layout.Inset{
				Top:   values.MarginPadding24,
				Left:  values.MarginPadding16,
				Right: values.MarginPadding16,
			},
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											txt := pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize20), title)
											txt.Color = pg.Theme.Color.DeepBlue
											txt.Font.Weight = font.SemiBold
											return txt.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											return layout.Center.Layout(gtx, func(gtx C) D {
												if title == values.String(values.StrPrivacySettings) {
													pg.networkInfoButton.Inset = layout.UniformInset(values.MarginPadding0)
													pg.networkInfoButton.Size = values.MarginPaddingTransform(pg.Load.IsMobileView(), values.MarginPadding20)
													return pg.networkInfoButton.Layout(gtx)
												}
												return D{}
											})
										}),
									)
								})
							}),

							layout.Flexed(1, func(gtx C) D {
								switch title {
								case values.String(values.StrSecurity):
									pg.infoButton.Inset = layout.UniformInset(values.MarginPadding0)
									pg.infoButton.Size = values.MarginPaddingTransform(pg.Load.IsMobileView(), values.MarginPadding20)
									return layout.E.Layout(gtx, pg.infoButton.Layout)

								case values.String(values.StrGeneral):
									return layout.E.Layout(gtx, func(gtx C) D {
										appearanceIcon := pg.Theme.Icons.DarkMode
										if pg.isDarkModeOn {
											appearanceIcon = pg.Theme.Icons.LightMode
										}
										return pg.appearanceMode.Layout(gtx, func(gtx C) D {
											return appearanceIcon.LayoutTransform(gtx, pg.Load.IsMobileView(), values.MarginPadding20)
										})
									})
								case values.String(values.StrPrivacySettings):
									return layout.E.Layout(gtx, pg.privacyActive.Layout)
								default:
									return D{}
								}
							}),
						)
					}),
					layout.Rigid(body),
				)
			}),
		)
	})
}

func (pg *AppSettingsPage) general() layout.Widget {
	return func(gtx C) D {
		return pg.wrapSection(gtx, values.String(values.StrGeneral), func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if !pg.CanChangeNetworkType() {
						return D{}
					}
					networkRow := row{
						title:     values.String(values.StrNetwork),
						clickable: pg.network,
						label:     pg.Theme.Body2(pg.AssetsManager.NetType().Display()),
					}
					return pg.clickableRow(gtx, networkRow)
				}),
				layout.Rigid(func(gtx C) D {
					languageRow := row{
						title:     values.String(values.StrLanguage),
						clickable: pg.language,
						label:     pg.Theme.Body2(pg.AssetsManager.GetLanguagePreference()),
					}
					return pg.clickableRow(gtx, languageRow)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.subSectionSwitch(gtx, values.String(values.StrTxNotification), pg.transactionNotification)
				}),
			)
		})
	}
}

func (pg *AppSettingsPage) networkSettings() layout.Widget {
	return func(gtx C) D {
		return pg.wrapSection(gtx, values.String(values.StrPrivacySettings), func(gtx C) D {
			if pg.AssetsManager.IsPrivacyModeOn() {
				return D{}
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					lKey := pg.AssetsManager.GetCurrencyConversionExchange()
					l := preference.GetKeyValue(lKey, preference.ExchOptions)
					exchangeRate := row{
						title:     values.String(values.StrExchangeRate),
						clickable: pg.currency,
						label:     pg.Theme.Body2(values.String(l)),
					}
					return pg.clickableRow(gtx, exchangeRate)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.subSectionSwitch(gtx, values.String(values.StrGovernanceAPI), pg.governanceAPI)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.subSectionSwitch(gtx, values.String(values.StrExchangeAPI), pg.exchangeAPI)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.subSectionSwitch(gtx, values.String(values.StrFeeRateAPI), pg.feeRateAPI)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.subSectionSwitch(gtx, values.String(values.StrVSPAPI), pg.vspAPI)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.subSectionSwitch(gtx, values.String(values.StrUpdateAPI), pg.updateAPI)
				}),
			)
		})
	}
}

func (pg *AppSettingsPage) dexSettings() layout.Widget {
	return func(gtx C) D {
		if !pg.AssetsManager.DEXCInitialized() || !pg.AssetsManager.DexClient().InitializedWithPassword() {
			return D{}
		}

		return pg.wrapSection(gtx, values.String(values.StrDEX), func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					backupDEX := row{
						title:     values.String(values.StrBackupDEXSeed),
						clickable: pg.backupDEX,
						label:     pg.Theme.Body2(""),
					}
					return pg.clickableRow(gtx, backupDEX)
				}),
				layout.Rigid(func(gtx C) D {
					deleteDEXClientRow := row{
						title:     values.String(values.StrResetDEXData),
						clickable: pg.deleteDEX,
						label:     pg.Theme.Body2(""),
					}
					return pg.clickableRow(gtx, deleteDEXClientRow)
				}),
			)
		})
	}
}

func (pg *AppSettingsPage) security() layout.Widget {
	return func(gtx C) D {
		return pg.wrapSection(gtx, values.String(values.StrSecurity), func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return pg.subSectionSwitch(gtx, values.String(values.StrStartupPassword), pg.startupPassword)
				}),
				layout.Rigid(func(gtx C) D {
					if pg.isStartupPassword {
						changeStartupPassRow := row{
							title:     values.String(values.StrChangeStartupPassword),
							clickable: pg.changeStartupPass,
							label:     pg.Theme.Body1(""),
						}
						return pg.clickableRow(gtx, changeStartupPassRow)
					}
					return D{}
				}),
			)
		})
	}
}

func (pg *AppSettingsPage) info() layout.Widget {
	return func(gtx C) D {
		return pg.wrapSection(gtx, values.String(values.StrInfo), func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					helpRow := row{
						title:     values.String(values.StrHelp),
						clickable: pg.help,
						label:     pg.Theme.Body2(""),
					}
					return pg.clickableRow(gtx, helpRow)
				}),
				layout.Rigid(func(gtx C) D {
					aboutRow := row{
						title:     values.String(values.StrAbout),
						clickable: pg.about,
						label:     pg.Theme.Body2(""),
					}
					return pg.clickableRow(gtx, aboutRow)
				}),
			)
		})
	}
}

func (pg *AppSettingsPage) debug() layout.Widget {
	return func(gtx C) D {
		return pg.wrapSection(gtx, values.String(values.StrDebug), func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					logLevel := row{
						title:     values.String(values.StrLogLevel),
						clickable: pg.logLevel,
						label:     pg.Theme.Body2(pg.AssetsManager.GetLogLevels()),
					}
					return pg.clickableRow(gtx, logLevel)
				}),
				layout.Rigid(func(gtx C) D {
					viewLogRow := row{
						title:     values.String(values.StrViewAppLog),
						clickable: pg.viewLog,
						label:     pg.Theme.Body2(""),
					}
					return pg.clickableRow(gtx, viewLogRow)
				}),
			)
		})
	}
}

func (pg *AppSettingsPage) subSection(gtx C, title string, body layout.Widget) D {
	return layout.Inset{Top: values.MarginPadding5, Bottom: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(pg.subSectionLabel(title)),
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, body)
			}),
		)
	})
}

func (pg *AppSettingsPage) subSectionSwitch(gtx C, title string, option *cryptomaterial.Switch) D {
	return pg.subSection(gtx, title, option.Layout)
}

func (pg *AppSettingsPage) clickableRow(gtx C, row row) D {
	return row.clickable.Layout(gtx, func(gtx C) D {
		return layout.Inset{Top: values.MarginPadding5, Bottom: values.MarginPaddingMinus5}.Layout(gtx, func(gtx C) D {
			return pg.subSection(gtx, row.title, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(row.label.Layout),
					layout.Rigid(func(gtx C) D {
						return pg.Theme.NewIcon(pg.Theme.Icons.ChevronRight).LayoutTransform(gtx, pg.Load.IsMobileView(), values.MarginPadding20)
					}),
				)
			})
		})
	})
}

func (pg *AppSettingsPage) subSectionLabel(title string) layout.Widget {
	return func(gtx C) D {
		return pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize16), title).Layout(gtx)
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *AppSettingsPage) HandleUserInteractions(gtx C) {
	if pg.network.Clicked(gtx) {
		currentNetType := string(pg.AssetsManager.NetType())
		networkSelectorModal := preference.NewListPreference(pg.Load, "", currentNetType, preference.NetworkTypes).
			Title(values.StrNetwork).
			UpdateValues(func(selectedNetType string) {
				if selectedNetType != currentNetType {
					ChangeNetworkType(pg.Load, pg.ParentWindow(), selectedNetType)
				}
			})
		pg.ParentWindow().ShowModal(networkSelectorModal)
	}

	if pg.language.Clicked(gtx) {
		langSelectorModal := preference.NewListPreference(pg.Load,
			sharedW.LanguagePreferenceKey, values.DefaultLanguage, preference.LangOptions).
			Title(values.StrLanguage).
			UpdateValues(func(_ string) {
				values.SetUserLanguage(pg.AssetsManager.GetLanguagePreference())
			})
		pg.ParentWindow().ShowModal(langSelectorModal)
	}

	if pg.backButton.Button.Clicked(gtx) {
		pg.ParentNavigator().CloseCurrentPage()
	}

	if pg.currency.Clicked(gtx) {
		currencySelectorModal := preference.NewListPreference(pg.Load,
			sharedW.CurrencyConversionConfigKey, values.DefaultExchangeValue,
			preference.ExchOptions).
			Title(values.StrExchangeRate).
			UpdateValues(func(_ string) {})
		pg.ParentWindow().ShowModal(currencySelectorModal)
	}

	if pg.appearanceMode.Clicked(gtx) {
		pg.isDarkModeOn = !pg.isDarkModeOn
		pg.AssetsManager.SetDarkMode(pg.isDarkModeOn)
		pg.RefreshTheme(pg.ParentWindow())
	}

	if pg.transactionNotification.Changed(gtx) {
		pg.AssetsManager.SetTransactionsNotifications(pg.transactionNotification.IsChecked())
	}
	if pg.governanceAPI.Changed(gtx) {
		pg.AssetsManager.SetHTTPAPIPrivacyMode(libutils.GovernanceHTTPAPI, pg.governanceAPI.IsChecked())
	}
	if pg.exchangeAPI.Changed(gtx) {
		pg.AssetsManager.SetHTTPAPIPrivacyMode(libutils.ExchangeHTTPAPI, pg.exchangeAPI.IsChecked())
	}
	if pg.feeRateAPI.Changed(gtx) {
		pg.AssetsManager.SetHTTPAPIPrivacyMode(libutils.FeeRateHTTPAPI, pg.feeRateAPI.IsChecked())
	}
	if pg.vspAPI.Changed(gtx) {
		pg.AssetsManager.SetHTTPAPIPrivacyMode(libutils.VspAPI, pg.vspAPI.IsChecked())
	}
	if pg.updateAPI.Changed(gtx) {
		pg.AssetsManager.SetHTTPAPIPrivacyMode(libutils.UpdateAPI, pg.updateAPI.IsChecked())
	}

	if pg.privacyActive.Changed(gtx) {
		pg.AssetsManager.SetPrivacyMode(pg.privacyActive.IsChecked())
		pg.updatePrivacySettings()
	}

	if pg.infoButton.Button.Clicked(gtx) {
		info := modal.NewCustomModal(pg.Load).
			SetContentAlignment(layout.Center, layout.Center, layout.Center).
			Body(values.String(values.StrStartupPasswordInfo)).
			PositiveButtonWidth(values.MarginPadding100)
		pg.ParentWindow().ShowModal(info)
	}

	if pg.networkInfoButton.Button.Clicked(gtx) {
		info := modal.NewCustomModal(pg.Load).
			SetContentAlignment(layout.Center, layout.Center, layout.Center).
			Title(values.String(values.StrPrivacyModeInfo)).
			Body(values.String(values.StrPrivacyModeInfoDesc)).
			PositiveButtonWidth(values.MarginPadding100)
		pg.ParentWindow().ShowModal(info)
	}

	if pg.help.Clicked(gtx) {
		pg.ParentNavigator().Display(NewHelpPage(pg.Load))
	}

	if pg.about.Clicked(gtx) {
		pg.ParentNavigator().Display(NewAboutPage(pg.Load))
	}

	if pg.logLevel.Clicked(gtx) {
		logLevelSelector := preference.NewListPreference(pg.Load,
			sharedW.LogLevelConfigKey, libutils.DefaultLogLevel, preference.LogOptions).
			Title(values.StrLogLevel).
			UpdateValues(func(val string) {
				_ = logger.SetLogLevels(val)
			})
		pg.ParentWindow().ShowModal(logLevelSelector)
	}

	if pg.viewLog.Clicked(gtx) {
		pg.ParentNavigator().Display(NewLogPage(pg.Load, pg.AssetsManager.LogFile(), values.String(values.StrAppLog)))
	}

	if pg.copyDEXSeed.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(pg.dexSeed.String()))})
		pg.copyDEXSeed.Text = values.String(values.StrCopied)
		pg.copyDEXSeed.Color = pg.Theme.Color.Success
		time.AfterFunc(time.Second*3, func() {
			pg.copyDEXSeed.Text = values.String(values.StrCopy)
			pg.copyDEXSeed.Color = pg.Theme.Color.Primary
			pg.ParentWindow().Reload()
		})
	}

	if pg.deleteDEX.Clicked(gtx) {
		// Show warning modal.
		deleteDEXModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrResetDEXData)).
			Body(values.String(values.StrResetDEXDataWarning)).
			SetNegativeButtonText(values.String(values.StrCancel)).
			SetPositiveButtonText(values.String(values.StrReset)).
			SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
				if pg.AssetsManager.DEXCInitialized() {
					if err := pg.AssetsManager.DeleteDEXData(); err != nil {
						return false
					}
					pg.showNoticeSuccess(values.String(values.StrDEXResetSuccessful))
				}
				return true
			}).
			PositiveButtonStyle(pg.Theme.Color.Surface, pg.Theme.Color.Danger)
		pg.ParentWindow().ShowModal(deleteDEXModal)
	}

	if pg.changeStartupPass.Clicked(gtx) {
		currentPasswordModal := modal.NewCreatePasswordModal(pg.Load).
			EnableName(false).
			EnableConfirmPassword(false).
			Title(values.String(values.StrConfirmStartupPass)).
			PasswordHint(values.String(values.StrCurrentStartupPass)).
			SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
				if !utils.StringNotEmpty(password) {
					pm.SetError(values.String(values.StrErrPassEmpty))
					return false
				}
				err := pg.AssetsManager.VerifyStartupPassphrase(password)
				if err != nil {
					pm.SetError(err.Error())
					return false
				}
				pm.Dismiss()

				// change password
				newPasswordModal := modal.NewCreatePasswordModal(pg.Load).
					Title(values.String(values.StrCreateStartupPassword)).
					EnableName(false).
					PasswordHint(values.String(values.StrNewStartupPass)).
					ConfirmPasswordHint(values.String(values.StrConfirmNewStartupPass)).
					SetPositiveButtonCallback(func(_, newPassword string, m *modal.CreatePasswordModal) bool {
						if !utils.StringNotEmpty(newPassword) {
							m.SetError(values.String(values.StrErrPassEmpty))
							return false
						}
						err := pg.AssetsManager.ChangeStartupPassphrase(password, newPassword, sharedW.PassphraseTypePass)
						if err != nil {
							m.SetError(err.Error())
							return false
						}
						pg.showNoticeSuccess(values.String(values.StrStartupPassConfirm))
						m.Dismiss()
						return true
					})
				pg.ParentWindow().ShowModal(newPasswordModal)
				return true
			})
		pg.ParentWindow().ShowModal(currentPasswordModal)
	}

	if pg.startupPassword.Changed(gtx) {
		if pg.startupPassword.IsChecked() {
			createPasswordModal := modal.NewCreatePasswordModal(pg.Load).
				Title(values.String(values.StrCreateStartupPassword)).
				EnableName(false).
				SetCancelable(false).
				PasswordHint(values.String(values.StrStartupPassword)).
				ConfirmPasswordHint(values.String(values.StrConfirmStartupPass)).
				SetPositiveButtonCallback(func(_, password string, m *modal.CreatePasswordModal) bool {
					if !utils.StringNotEmpty(password) {
						m.SetError(values.String(values.StrErrPassEmpty))
						return false
					}
					err := pg.AssetsManager.SetStartupPassphrase(password, sharedW.PassphraseTypePass)
					if err != nil {
						m.SetError(err.Error())
						return false
					}
					pg.showNoticeSuccess(values.StringF(values.StrStartupPasswordEnabled, values.String(values.StrEnabled)))
					m.Dismiss()
					pg.isStartupPassword = true
					return true
				}).
				SetNegativeButtonCallback(func() {
					pg.startupPassword.SetChecked(false)
				})
			pg.ParentWindow().ShowModal(createPasswordModal)
		} else {
			currentPasswordModal := modal.NewCreatePasswordModal(pg.Load).
				EnableName(false).
				SetCancelable(false).
				EnableConfirmPassword(false).
				Title(values.String(values.StrConfirmRemoveStartupPass)).
				PasswordHint(values.String(values.StrStartupPassword)).
				SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
					err := pg.AssetsManager.RemoveStartupPassphrase(password)
					if err != nil {
						pm.SetError(err.Error())
						return false
					}
					pg.showNoticeSuccess(values.StringF(values.StrStartupPasswordEnabled, values.String(values.StrDisabled)))
					pm.Dismiss()
					pg.isStartupPassword = false
					return true
				}).
				SetNegativeButtonCallback(func() {
					pg.startupPassword.SetChecked(true)
				})
			pg.ParentWindow().ShowModal(currentPasswordModal)
		}
	}

	if pg.backupDEX.Clicked(gtx) {
		// Show modal asking for dex password and then reveal the seed.
		dexPasswordModal := modal.NewCreatePasswordModal(pg.Load).
			EnableName(false).
			EnableConfirmPassword(false).
			Title(values.String(values.StrDexPassword)).
			SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
				dexSeed, err := pg.AssetsManager.DexClient().ExportSeed([]byte(password))
				if err != nil {
					pm.SetError(err.Error())
					return false
				}

				pg.dexSeed = dex.Bytes(dexSeed)
				pg.showDEXSeedModal()
				return true
			})

		dexPasswordModal.SetPasswordTitleVisibility(false)
		pg.ParentWindow().ShowModal(dexPasswordModal)
	}
}

func (pg *AppSettingsPage) showDEXSeedModal() {
	seedModal := modal.NewSuccessModal(pg.Load, values.String(values.StrDEXSeed), modal.DefaultClickFunc()).
		UseCustomWidget(func(gtx C) D {
			seedText := pg.Theme.Body1(formatDEXSeedAsString(pg.dexSeed))
			seedText.Alignment = text.Middle
			seedText.Color = pg.Theme.Color.GrayText2
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(seedText.Layout),
				layout.Rigid(pg.copyDEXSeed.Layout),
			)
		}).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			utils.ZeroBytes(pg.dexSeed)
			return true
		})
	pg.ParentWindow().ShowModal(seedModal)
}

func formatDEXSeedAsString(seed dex.Bytes) string {
	if len(seed) == 128 { // 64 bytes, 128 hex character legacy seed
		chunkRegex := regexp.MustCompile(`.{1,32}`)
		chunks := chunkRegex.FindAllString(seed.String(), -1)

		var seedChunks []string
		subChunkRegex := regexp.MustCompile(`.{1,8}`)
		for _, chunk := range chunks {
			subChunks := subChunkRegex.FindAllString(chunk, -1)
			seedChunks = append(seedChunks, strings.Join(subChunks, "  "))
		}

		return strings.Join(seedChunks, "\n")
	}
	return seed.String()
}

func ChangeNetworkType(load *load.Load, windowNav app.WindowNavigator, newNetType string) {
	modalTitle := values.String(values.StrSwitchToTestnet)
	if newNetType == string(libutils.Mainnet) {
		modalTitle = values.String(values.StrSwitchToMainnet)
	}

	confirmNetworkSwitchModal := modal.NewCustomModal(load).
		Title(modalTitle).
		Body("Your app will restart to apply this change. Continue?"). // TODO: localize
		SetCancelable(true).
		SetPositiveButtonText(values.String(values.StrYes)).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			newAssetsManager, err := load.ChangeAssetsManagerNetwork(libutils.ToNetworkType(newNetType))
			if err != nil {
				errorModal := modal.NewErrorModal(load, err.Error(), modal.DefaultClickFunc())
				windowNav.ShowModal(errorModal)
			} else {
				// Complete the network switch in the background.
				go load.ChangeAssetsManager(newAssetsManager, windowNav)
			}
			return true
		})
	windowNav.ShowModal(confirmNetworkSwitchModal)
}

func (pg *AppSettingsPage) showNoticeSuccess(title string) {
	info := modal.NewSuccessModal(pg.Load, title, modal.DefaultClickFunc())
	pg.ParentWindow().ShowModal(info)
}

func (pg *AppSettingsPage) updateSettingOptions() {
	isPassword := pg.AssetsManager.IsStartupSecuritySet()
	pg.startupPassword.SetChecked(false)
	pg.isStartupPassword = false
	if isPassword {
		pg.startupPassword.SetChecked(isPassword)
		pg.isStartupPassword = true
	}

	pg.updatePrivacySettings()
}

func (pg *AppSettingsPage) updatePrivacySettings() {
	privacyOn := pg.AssetsManager.IsPrivacyModeOn()
	pg.setInitialSwitchStatus(pg.privacyActive, privacyOn)
	if !privacyOn {
		pg.setInitialSwitchStatus(pg.transactionNotification, pg.AssetsManager.IsTransactionNotificationsOn())
		pg.setInitialSwitchStatus(pg.governanceAPI, pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI))
		pg.setInitialSwitchStatus(pg.exchangeAPI, pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.ExchangeHTTPAPI))
		pg.setInitialSwitchStatus(pg.feeRateAPI, pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.FeeRateHTTPAPI))
		pg.setInitialSwitchStatus(pg.vspAPI, pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.VspAPI))
		pg.setInitialSwitchStatus(pg.updateAPI, pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.UpdateAPI))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *AppSettingsPage) OnNavigatedFrom() {
	utils.ZeroBytes(pg.dexSeed)
	// remove fetch exchange rate warning msg listener
	pg.AssetsManager.RateSource.RemoveWarningMsgListener(AppSettingsPageID)
}

func (pg *AppSettingsPage) setInitialSwitchStatus(switchComponent *cryptomaterial.Switch, isChecked bool) {
	switchComponent.SetChecked(false)
	if isChecked {
		switchComponent.SetChecked(isChecked)
	}
}
