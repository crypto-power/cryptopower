package settings

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
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

	governanceAPI *cryptomaterial.Switch
	exchangeAPI   *cryptomaterial.Switch
	feeRateAPI    *cryptomaterial.Switch
	vspAPI        *cryptomaterial.Switch
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
		privacyActive:           l.Theme.Switch(),

		changeStartupPass: l.Theme.NewClickable(false),
		language:          l.Theme.NewClickable(false),
		currency:          l.Theme.NewClickable(false),
		help:              l.Theme.NewClickable(false),
		about:             l.Theme.NewClickable(false),
		appearanceMode:    l.Theme.NewClickable(false),
		logLevel:          l.Theme.NewClickable(false),
		viewLog:           l.Theme.NewClickable(false),
		deleteDEX:         l.Theme.NewClickable(false),
	}

	_, pg.networkInfoButton = components.SubpageHeaderButtons(l)
	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.isDarkModeOn = pg.AssetsManager.IsDarkModeOn()

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *AppSettingsPage) OnNavigatedTo() {
	pg.updateSettingOptions()
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
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.W.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding16,
						}.Layout(gtx, pg.backButton.Layout)
					}),
					layout.Rigid(pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize20), values.String(values.StrSettings)).Layout),
				)
			})
		}),
	)
}

func (pg *AppSettingsPage) pageContentLayout(gtx C) D {
	pageContent := []func(gtx C) D{
		pg.general(),
		pg.networkSettings(),
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
				layout.Rigid(func(gtx C) D {
					if pg.AssetsManager.NetType() != libutils.Testnet || !pg.AssetsManager.DexcReady() {
						return D{}
					}

					deleteDEXClientRow := row{
						title:     values.String(values.StrDeleteDEXData),
						clickable: pg.deleteDEX,
						label:     pg.Theme.Body2(""),
					}
					return pg.clickableRow(gtx, deleteDEXClientRow)
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
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(row.label.Layout),
					layout.Rigid(func(gtx C) D {
						return pg.Theme.Icons.ChevronRight.LayoutTransform(gtx, pg.Load.IsMobileView(), values.MarginPadding20)
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
func (pg *AppSettingsPage) HandleUserInteractions() {
	for pg.language.Clicked() {
		langSelectorModal := preference.NewListPreference(pg.Load,
			sharedW.LanguagePreferenceKey, values.DefaultLanguage, preference.LangOptions).
			Title(values.StrLanguage).
			UpdateValues(func(_ string) {
				values.SetUserLanguage(pg.AssetsManager.GetLanguagePreference())
			})
		pg.ParentWindow().ShowModal(langSelectorModal)
		break
	}

	for pg.backButton.Button.Clicked() {
		pg.ParentNavigator().CloseCurrentPage()
	}

	for pg.currency.Clicked() {
		currencySelectorModal := preference.NewListPreference(pg.Load,
			sharedW.CurrencyConversionConfigKey, values.DefaultExchangeValue,
			preference.ExchOptions).
			Title(values.StrExchangeRate).
			UpdateValues(func(_ string) {})
		pg.ParentWindow().ShowModal(currencySelectorModal)
		break
	}

	for pg.appearanceMode.Clicked() {
		pg.isDarkModeOn = !pg.isDarkModeOn
		pg.AssetsManager.SetDarkMode(pg.isDarkModeOn)
		pg.RefreshTheme(pg.ParentWindow())
	}

	if pg.transactionNotification.Changed() {
		pg.AssetsManager.SetTransactionsNotifications(pg.transactionNotification.IsChecked())
	}
	if pg.governanceAPI.Changed() {
		pg.AssetsManager.SetHTTPAPIPrivacyMode(libutils.GovernanceHTTPAPI, pg.governanceAPI.IsChecked())
	}
	if pg.exchangeAPI.Changed() {
		pg.AssetsManager.SetHTTPAPIPrivacyMode(libutils.ExchangeHTTPAPI, pg.exchangeAPI.IsChecked())
	}
	if pg.feeRateAPI.Changed() {
		pg.AssetsManager.SetHTTPAPIPrivacyMode(libutils.FeeRateHTTPAPI, pg.feeRateAPI.IsChecked())
	}
	if pg.vspAPI.Changed() {
		pg.AssetsManager.SetHTTPAPIPrivacyMode(libutils.VspAPI, pg.vspAPI.IsChecked())
	}

	if pg.privacyActive.Changed() {
		pg.AssetsManager.SetPrivacyMode(pg.privacyActive.IsChecked())
		pg.updatePrivacySettings()
	}

	if pg.infoButton.Button.Clicked() {
		info := modal.NewCustomModal(pg.Load).
			SetContentAlignment(layout.Center, layout.Center, layout.Center).
			Body(values.String(values.StrStartupPasswordInfo)).
			PositiveButtonWidth(values.MarginPadding100)
		pg.ParentWindow().ShowModal(info)
	}

	if pg.networkInfoButton.Button.Clicked() {
		info := modal.NewCustomModal(pg.Load).
			SetContentAlignment(layout.Center, layout.Center, layout.Center).
			Title(values.String(values.StrPrivacyModeInfo)).
			Body(values.String(values.StrPrivacyModeInfoDesc)).
			PositiveButtonWidth(values.MarginPadding100)
		pg.ParentWindow().ShowModal(info)
	}

	if pg.help.Clicked() {
		pg.ParentNavigator().Display(NewHelpPage(pg.Load))
	}

	if pg.about.Clicked() {
		pg.ParentNavigator().Display(NewAboutPage(pg.Load))
	}

	for pg.logLevel.Clicked() {
		logLevelSelector := preference.NewListPreference(pg.Load,
			sharedW.LogLevelConfigKey, libutils.DefaultLogLevel, preference.LogOptions).
			Title(values.StrLogLevel).
			UpdateValues(func(val string) {
				logger.SetLogLevels(val)
			})
		pg.ParentWindow().ShowModal(logLevelSelector)
		break
	}

	if pg.viewLog.Clicked() {
		pg.ParentNavigator().Display(NewLogPage(pg.Load, pg.AssetsManager.LogFile(), values.String(values.StrAppLog)))
	}

	if pg.deleteDEX.Clicked() {
		// Show warning modal.
		deleteDEXModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrDeleteDEXData)).
			Body(values.String(values.StrDeleteDEXDataWarning)).
			SetNegativeButtonText(values.String(values.StrCancel)).
			SetPositiveButtonText(values.String(values.StrDelete)).
			SetPositiveButtonCallback(func(_ bool, in *modal.InfoModal) bool {
				if pg.AssetsManager.DexcReady() {
					if err := pg.AssetsManager.DeleteDEXData(); err != nil {
						return false
					}
				}
				return true
			}).
			PositiveButtonStyle(pg.Theme.Color.Surface, pg.Theme.Color.Danger)
		pg.ParentWindow().ShowModal(deleteDEXModal)
	}

	for pg.changeStartupPass.Clicked() {
		currentPasswordModal := modal.NewCreatePasswordModal(pg.Load).
			EnableName(false).
			EnableConfirmPassword(false).
			Title(values.String(values.StrConfirmStartupPass)).
			PasswordHint(values.String(values.StrCurrentStartupPass)).
			SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
				if !utils.StringNotEmpty(password) {
					pm.SetError(values.String(values.StrErrPassEmpty))
					pm.SetLoading(false)
					return false
				}
				err := pg.AssetsManager.VerifyStartupPassphrase(password)
				if err != nil {
					pm.SetError(err.Error())
					pm.SetLoading(false)
					return false
				}
				pm.Dismiss()

				// change password
				newPasswordModal := modal.NewCreatePasswordModal(pg.Load).
					Title(values.String(values.StrCreateStartupPassword)).
					EnableName(false).
					PasswordHint(values.String(values.StrNewStartupPass)).
					ConfirmPasswordHint(values.String(values.StrConfirmNewStartupPass)).
					SetPositiveButtonCallback(func(walletName, newPassword string, m *modal.CreatePasswordModal) bool {
						if !utils.StringNotEmpty(newPassword) {
							m.SetError(values.String(values.StrErrPassEmpty))
							m.SetLoading(false)
							return false
						}
						err := pg.AssetsManager.ChangeStartupPassphrase(password, newPassword, sharedW.PassphraseTypePass)
						if err != nil {
							m.SetError(err.Error())
							m.SetLoading(false)
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
		break
	}

	if pg.startupPassword.Changed() {
		if pg.startupPassword.IsChecked() {
			createPasswordModal := modal.NewCreatePasswordModal(pg.Load).
				Title(values.String(values.StrCreateStartupPassword)).
				EnableName(false).
				SetCancelable(false).
				PasswordHint(values.String(values.StrStartupPassword)).
				ConfirmPasswordHint(values.String(values.StrConfirmStartupPass)).
				SetPositiveButtonCallback(func(walletName, password string, m *modal.CreatePasswordModal) bool {
					if !utils.StringNotEmpty(password) {
						m.SetError(values.String(values.StrErrPassEmpty))
						m.SetLoading(false)
						return false
					}
					err := pg.AssetsManager.SetStartupPassphrase(password, sharedW.PassphraseTypePass)
					if err != nil {
						m.SetError(err.Error())
						m.SetLoading(false)
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
						pm.SetLoading(false)
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
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *AppSettingsPage) OnNavigatedFrom() {}

func (pg *AppSettingsPage) setInitialSwitchStatus(switchComponent *cryptomaterial.Switch, isChecked bool) {
	switchComponent.SetChecked(false)
	if isChecked {
		switchComponent.SetChecked(isChecked)
	}
}
