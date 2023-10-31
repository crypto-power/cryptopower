package root

import (
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/security"
	s "github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const WalletSettingsPageID = "WalletSettings"

type clickableRowData struct {
	clickable *cryptomaterial.Clickable
	labelText string
	title     string
}

type accountData struct {
	*sharedW.Account
	clickable *cryptomaterial.Clickable
}

type WalletSettingsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	wallet   sharedW.Asset
	accounts []*accountData

	pageContainer *widget.List
	accountsList  *cryptomaterial.ClickableList

	changePass, rescan                         *cryptomaterial.Clickable
	changeAccount, checklog, checkStats        *cryptomaterial.Clickable
	changeWalletName, addAccount, deleteWallet *cryptomaterial.Clickable
	verifyMessage, validateAddr, signMessage   *cryptomaterial.Clickable
	updateConnectToPeer, setGapLimit           *cryptomaterial.Clickable

	backButton cryptomaterial.IconButton
	infoButton cryptomaterial.IconButton

	spendUnconfirmed  *cryptomaterial.Switch
	spendUnmixedFunds *cryptomaterial.Switch
	connectToPeer     *cryptomaterial.Switch

	walletCallbackFunc func()

	peerAddr string
}

func NewWalletSettingsPage(l *load.Load, walletCallbackFunc func()) *WalletSettingsPage {
	pg := &WalletSettingsPage{
		Load:                l,
		GenericPageModal:    app.NewGenericPageModal(WalletSettingsPageID),
		wallet:              l.WL.SelectedWallet.Wallet,
		changePass:          l.Theme.NewClickable(false),
		rescan:              l.Theme.NewClickable(false),
		setGapLimit:         l.Theme.NewClickable(false),
		changeAccount:       l.Theme.NewClickable(false),
		checklog:            l.Theme.NewClickable(false),
		checkStats:          l.Theme.NewClickable(false),
		changeWalletName:    l.Theme.NewClickable(false),
		addAccount:          l.Theme.NewClickable(false),
		deleteWallet:        l.Theme.NewClickable(false),
		verifyMessage:       l.Theme.NewClickable(false),
		validateAddr:        l.Theme.NewClickable(false),
		signMessage:         l.Theme.NewClickable(false),
		updateConnectToPeer: l.Theme.NewClickable(false),

		spendUnconfirmed:  l.Theme.Switch(),
		spendUnmixedFunds: l.Theme.Switch(),
		connectToPeer:     l.Theme.Switch(),

		pageContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		accountsList:       l.Theme.NewClickableList(layout.Vertical),
		walletCallbackFunc: walletCallbackFunc,
	}

	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *WalletSettingsPage) OnNavigatedTo() {
	pg.spendUnconfirmed.SetChecked(pg.readBool(sharedW.SpendUnconfirmedConfigKey))
	pg.spendUnmixedFunds.SetChecked(pg.readBool(sharedW.SpendUnmixedFundsKey))

	pg.loadPeerAddress()

	pg.loadWalletAccount()
}

func (pg *WalletSettingsPage) readBool(key string) bool {
	return pg.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(key, false)
}

func (pg *WalletSettingsPage) isPrivacyModeOn() bool {
	return pg.WL.AssetsManager.IsPrivacyModeOn()
}

func (pg *WalletSettingsPage) loadPeerAddress() {
	if !pg.isPrivacyModeOn() {
		pg.peerAddr = pg.WL.SelectedWallet.Wallet.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")
		pg.connectToPeer.SetChecked(false)
		if pg.peerAddr != "" {
			pg.connectToPeer.SetChecked(true)
		}
	}
}

func (pg *WalletSettingsPage) loadWalletAccount() {
	walletAccounts := make([]*accountData, 0)
	accounts, err := pg.wallet.GetAccountsRaw()
	if err != nil {
		log.Errorf("error retrieving wallet accounts: %v", err)
		return
	}

	for _, acct := range accounts.Accounts {
		if acct.Number == dcr.ImportedAccountNumber {
			continue
		}
		walletAccounts = append(walletAccounts, &accountData{
			Account:   acct,
			clickable: pg.Theme.NewClickable(false),
		})
	}

	pg.accounts = walletAccounts
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *WalletSettingsPage) Layout(gtx C) D {
	body := func(gtx C) D {
		w := []func(gtx C) D{
			func(gtx C) D {
				return layout.Inset{
					Bottom: values.MarginPadding26,
				}.Layout(gtx, pg.Theme.Label(values.TextSize20, values.String(values.StrSettings)).Layout)
			},
			pg.generalSection(),
			pg.account(),
			pg.securityTools(),
			pg.debug(),
			pg.dangerZone(),
		}

		return pg.Theme.List(pg.pageContainer).Layout(gtx, len(w), func(gtx C, i int) D {
			return w[i](gtx)
		})
	}

	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx, body)
	}
	return pg.layoutDesktop(gtx, body)
}

func (pg *WalletSettingsPage) layoutDesktop(gtx C, body layout.Widget) D {
	return body(gtx)
}

func (pg *WalletSettingsPage) layoutMobile(gtx C, body layout.Widget) D {
	return components.UniformMobile(gtx, false, false, body)
}

func (pg *WalletSettingsPage) generalSection() layout.Widget {
	dim := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if pg.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
					return D{}
				}
				return layout.Inset{}.Layout(gtx, pg.sectionContent(pg.changePass, values.String(values.StrSpendingPassword)))
			}),
			layout.Rigid(pg.sectionContent(pg.changeWalletName, values.String(values.StrRenameWalletSheetTitle))),
			layout.Rigid(func(gtx C) D {
				if pg.wallet.GetAssetType() == libutils.DCRWalletAsset {
					return pg.subSection(gtx, values.String(values.StrUnconfirmedFunds), pg.spendUnconfirmed.Layout)
				}
				return D{}
			}),
			layout.Rigid(func(gtx C) D {
				if pg.wallet.GetAssetType() == libutils.DCRWalletAsset {
					return pg.subSection(gtx, values.String(values.StrAllowSpendingFromUnmixedAccount), pg.spendUnmixedFunds.Layout)
				}
				return D{}
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(pg.subSectionSwitch(values.String(values.StrConnectToSpecificPeer), pg.connectToPeer)),
					layout.Rigid(func(gtx C) D {
						if pg.WL.SelectedWallet.Wallet.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "") == "" && pg.isPrivacyModeOn() {
							return D{}
						}

						peerAddrRow := clickableRowData{
							title:     values.String(values.StrPeer),
							clickable: pg.updateConnectToPeer,
							labelText: pg.peerAddr,
						}
						return pg.clickableRow(gtx, peerAddrRow)
					}),
				)
			}),
		)
	}

	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrGeneral), dim)
	}
}

func (pg *WalletSettingsPage) account() layout.Widget {
	dim := func(gtx C) D {
		return pg.accountsList.Layout(gtx, len(pg.accounts), func(gtx C, a int) D {
			return pg.subSection(gtx, pg.accounts[a].Name, pg.Theme.Icons.ChevronRight.Layout24dp)
		})
	}
	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrAccount), dim)
	}
}

func (pg *WalletSettingsPage) debug() layout.Widget {
	dims := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.sectionContent(pg.rescan, values.String(values.StrRescanBlockchain))),
			layout.Rigid(func(gtx C) D {
				if pg.wallet.GetAssetType() == libutils.DCRWalletAsset {
					return pg.sectionDimension(gtx, pg.setGapLimit, values.String(values.StrSetGapLimit))
				}
				return D{}
			}),
			layout.Rigid(pg.sectionContent(pg.checklog, values.String(values.StrCheckWalletLog))),
			layout.Rigid(pg.sectionContent(pg.checkStats, values.String(values.StrCheckStatistics))),
		)
	}

	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrDebug), dims)
	}
}

func (pg *WalletSettingsPage) securityTools() layout.Widget {
	dims := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.sectionContent(pg.verifyMessage, values.String(values.StrVerifyMessage))),
			layout.Rigid(pg.sectionContent(pg.validateAddr, values.String(values.StrValidateMsg))),
			layout.Rigid(pg.sectionContent(pg.signMessage, values.String(values.StrSignMessage))),
		)
	}

	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrSecurityTools), dims)
	}
}

func (pg *WalletSettingsPage) dangerZone() layout.Widget {
	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrDangerZone),
			pg.sectionContent(pg.deleteWallet, values.String(values.StrRemoveWallet)),
		)
	}
}

func (pg *WalletSettingsPage) pageSections(gtx C, title string, body layout.Widget) D {
	dims := func(gtx C, title string, body layout.Widget) D {
		return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							txt := pg.Theme.Label(values.TextSize14, title)
							txt.Color = pg.Theme.Color.GrayText2
							return txt.Layout(gtx)
						}),
						layout.Flexed(1, func(gtx C) D {
							if title == values.String(values.StrSecurityTools) {
								pg.infoButton.Inset = layout.UniformInset(values.MarginPadding0)
								pg.infoButton.Size = values.MarginPadding16
								return layout.E.Layout(gtx, pg.infoButton.Layout)
							}
							if title == values.String(values.StrAccount) {
								return layout.E.Layout(gtx, func(gtx C) D {
									if pg.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
										return D{}
									}
									return pg.addAccount.Layout(gtx, pg.Theme.Icons.AddIcon.Layout24dp)
								})
							}

							return D{}
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Bottom: values.MarginPadding10,
						Top:    values.MarginPadding7,
					}.Layout(gtx, pg.Theme.Separator().Layout)
				}),
				layout.Rigid(body),
			)
		})
	}

	return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		return dims(gtx, title, body)
	})
}

func (pg *WalletSettingsPage) sectionContent(clickable *cryptomaterial.Clickable, title string) layout.Widget {
	return func(gtx C) D {
		return pg.sectionDimension(gtx, clickable, title)
	}
}

func (pg *WalletSettingsPage) sectionDimension(gtx C, clickable *cryptomaterial.Clickable, title string) D {
	return clickable.Layout(gtx, func(gtx C) D {
		textLabel := pg.Theme.Label(values.TextSize16, title)
		if title == values.String(values.StrRemoveWallet) {
			textLabel.Color = pg.Theme.Color.Danger
		}
		return layout.Inset{
			Bottom: values.MarginPadding20,
		}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(textLabel.Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, pg.Theme.Icons.ChevronRight.Layout24dp)
				}),
			)
		})
	})
}

func (pg *WalletSettingsPage) subSection(gtx C, title string, body layout.Widget) D {
	return layout.Inset{Top: values.MarginPadding5, Bottom: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(pg.Theme.Label(values.TextSize16, title).Layout),
			layout.Flexed(1, func(gtx C) D {
				switch title {
				case values.String(values.StrConnectToSpecificPeer):
					if pg.isPrivacyModeOn() {
						textlabel := pg.Theme.Label(values.TextSize12, values.String(values.StrPrivacyModeActive))
						textlabel.Color = pg.Theme.Color.GrayText2
						body = textlabel.Layout
					}
				}
				return layout.E.Layout(gtx, body)
			}),
		)
	})
}

func (pg *WalletSettingsPage) subSectionSwitch(title string, option *cryptomaterial.Switch) layout.Widget {
	return func(gtx C) D {
		return pg.subSection(gtx, title, option.Layout)
	}
}

func (pg *WalletSettingsPage) changeSpendingPasswordModal() {
	currentSpendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		Title(values.String(values.StrConfirmSpendingPassword)).
		PasswordHint(values.String(values.StrCurrentSpendingPassword)).
		EnableName(false).
		EnableConfirmPassword(false).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := pg.wallet.UnlockWallet(password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			pg.wallet.LockWallet()

			// change password
			newSpendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
				Title(values.String(values.StrChangeSpendingPass)).
				EnableName(false).
				PasswordHint(values.String(values.StrNewSpendingPassword)).
				ConfirmPasswordHint(values.String(values.StrConfirmNewSpendingPassword)).
				SetPositiveButtonCallback(func(walletName, newPassword string, m *modal.CreatePasswordModal) bool {
					err := pg.wallet.ChangePrivatePassphraseForWallet(password,
						newPassword, sharedW.PassphraseTypePass)
					if err != nil {
						m.SetError(err.Error())
						m.SetLoading(false)
						return false
					}

					info := modal.NewSuccessModal(pg.Load, values.StringF(values.StrSpendingPasswordUpdated),
						modal.DefaultClickFunc())
					pg.ParentWindow().ShowModal(info)
					return true
				})
			pg.ParentWindow().ShowModal(newSpendingPasswordModal)
			return true
		})
	pg.ParentWindow().ShowModal(currentSpendingPasswordModal)
}

func (pg *WalletSettingsPage) deleteWalletModal() {
	textModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrWalletName)).
		SetTextWithTemplate(modal.RemoveWalletInfoTemplate, pg.WL.SelectedWallet.Wallet.GetWalletName()).
		PositiveButtonStyle(pg.Load.Theme.Color.Surface, pg.Load.Theme.Color.Danger).
		SetPositiveButtonCallback(func(walletName string, m *modal.TextInputModal) bool {
			if walletName != pg.WL.SelectedWallet.Wallet.GetWalletName() {
				m.SetError(values.String(values.StrWalletNameMismatch))
				m.SetLoading(false)
				return false
			}

			walletDeleted := func() {
				m.Dismiss()
				if pg.WL.AssetsManager.LoadedWalletsCount() > 0 {
					pg.walletCallbackFunc()
				} else {
					pg.ParentWindow().CloseAllPages()
				}
			}

			if pg.wallet.IsWatchingOnlyWallet() {
				// no password is required for watching only wallets.
				err := pg.WL.AssetsManager.DeleteWallet(pg.WL.SelectedWallet.Wallet.GetWalletID(), "")
				if err != nil {
					m.SetError(err.Error())
					m.SetLoading(false)
				} else {
					walletDeleted()
				}
				return false
			}

			walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
				EnableName(false).
				EnableConfirmPassword(false).
				Title(values.String(values.StrConfirmToRemove)).
				SetNegativeButtonCallback(func() {
					m.SetLoading(false)
				}).
				SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
					err := pg.WL.AssetsManager.DeleteWallet(pg.WL.SelectedWallet.Wallet.GetWalletID(), password)
					if err != nil {
						pm.SetError(err.Error())
						pm.SetLoading(false)
						return false
					}

					walletDeleted()
					pm.Dismiss() // calls RefreshWindow.
					return true
				})
			pg.ParentWindow().ShowModal(walletPasswordModal)
			return true
		})
	textModal.Title(values.String(values.StrRemoveWallet)).
		SetPositiveButtonText(values.String(values.StrRemove))
	pg.ParentWindow().ShowModal(textModal)
}

func (pg *WalletSettingsPage) renameWalletModal() {
	textModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrWalletName)).
		PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
		SetPositiveButtonCallback(func(newName string, tm *modal.TextInputModal) bool {
			name := strings.TrimSpace(newName)
			if !utils.ValidateLengthName(name) {
				tm.SetError(values.String(values.StrWalletNameLengthError))
				tm.SetLoading(false)
				return false
			}

			err := pg.WL.SelectedWallet.Wallet.RenameWallet(name)
			if err != nil {
				tm.SetError(err.Error())
				tm.SetLoading(false)
				return false
			}
			info := modal.NewSuccessModal(pg.Load, values.StringF(values.StrWalletRenamed), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(info)
			return true
		})
	textModal.Title(values.String(values.StrRenameWalletSheetTitle)).
		SetPositiveButtonText(values.String(values.StrRename))
	pg.ParentWindow().ShowModal(textModal)
}

func (pg *WalletSettingsPage) showSPVPeerDialog() {
	textModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrIPAddress)).
		PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
		SetPositiveButtonCallback(func(ipAddress string, tim *modal.TextInputModal) bool {
			if !utils.ValidateHost(ipAddress) {
				tim.SetError(values.StringF(values.StrValidateHostErr, ipAddress))
				tim.SetLoading(false)
				return false
			}
			if ipAddress != "" {
				pg.WL.SelectedWallet.Wallet.SetSpecificPeer(ipAddress)
				pg.loadPeerAddress()
			}
			return true
		})
	textModal.Title(values.String(values.StrConnectToSpecificPeer)).
		SetPositiveButtonText(values.String(values.StrConfirm)).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetNegativeButtonCallback(func() {
			pg.peerAddr = pg.WL.SelectedWallet.Wallet.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")
			pg.connectToPeer.SetChecked(pg.peerAddr != "")
		})
	pg.ParentWindow().ShowModal(textModal)
}

func (pg *WalletSettingsPage) clickableRow(gtx C, row clickableRowData) D {
	return row.clickable.Layout(gtx, func(gtx C) D {
		return pg.subSection(gtx, row.title, func(gtx C) D {
			lbl := pg.Theme.Label(values.TextSize16, row.labelText)
			lbl.Color = pg.Theme.Color.GrayText2
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(lbl.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, pg.Theme.Icons.ChevronRight.Layout24dp)
				}),
			)
		})
	})
}

func (pg *WalletSettingsPage) showWarningModalDialog(title, msg string) {
	warningModal := modal.NewCustomModal(pg.Load).
		Title(title).
		Body(msg).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetNegativeButtonCallback(func() {
			pg.connectToPeer.SetChecked(true)
		}).
		SetNegativeButtonText(values.String(values.StrCancel)).
		PositiveButtonStyle(pg.Theme.Color.Surface, pg.Theme.Color.Danger).
		SetPositiveButtonText(values.String(values.StrRemove)).
		SetPositiveButtonCallback(func(isChecked bool, im *modal.InfoModal) bool {
			// TODO: Check if deletion happened successfully
			// Since only one peer is available at time, the single peer key can
			// be set to empty string to delete its entry..
			pg.WL.SelectedWallet.Wallet.RemovePeers()
			pg.peerAddr = ""
			return true
		})
	pg.ParentWindow().ShowModal(warningModal)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *WalletSettingsPage) HandleUserInteractions() {
	for pg.changePass.Clicked() {
		pg.changeSpendingPasswordModal()
		break
	}

	if pg.rescan.Clicked() {
		go func() {
			info := modal.NewCustomModal(pg.Load).
				Title(values.String(values.StrRescanBlockchain)).
				Body(values.String(values.StrRescanInfo)).
				SetNegativeButtonText(values.String(values.StrCancel)).
				PositiveButtonStyle(pg.Theme.Color.Primary, pg.Theme.Color.Surface).
				SetPositiveButtonText(values.String(values.StrRescan)).
				SetPositiveButtonCallback(func(_ bool, im *modal.InfoModal) bool {
					err := pg.WL.SelectedWallet.Wallet.RescanBlocks()
					if err != nil {
						errorModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
						pg.ParentWindow().ShowModal(errorModal)
						im.Dismiss()

						return false
					}

					im.Dismiss()
					return true
				})

			pg.ParentWindow().ShowModal(info)
		}()
	}

	for pg.setGapLimit.Clicked() {
		pg.gapLimitModal()
	}

	for pg.deleteWallet.Clicked() {
		pg.deleteWalletModal()
		break
	}

	for pg.changeWalletName.Clicked() {
		pg.renameWalletModal()
		break
	}

	if pg.infoButton.Button.Clicked() {
		info := modal.NewCustomModal(pg.Load).
			PositiveButtonStyle(pg.Theme.Color.Primary, pg.Theme.Color.Surface).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetupWithTemplate(modal.SecurityToolsInfoTemplate).
			Title(values.String(values.StrSecurityTools))
		pg.ParentWindow().ShowModal(info)
	}

	if pg.spendUnconfirmed.Changed() {
		pg.WL.SelectedWallet.Wallet.SaveUserConfigValue(sharedW.SpendUnconfirmedConfigKey, pg.spendUnconfirmed.IsChecked())
	}

	if pg.spendUnmixedFunds.Changed() {
		if pg.spendUnmixedFunds.IsChecked() {
			textModal := modal.NewTextInputModal(pg.Load).
				SetTextWithTemplate(modal.AllowUnmixedSpendingTemplate).
				// Hint(""). Code not deleted because a proper hint is required.
				PositiveButtonStyle(pg.Load.Theme.Color.Danger, pg.Load.Theme.Color.InvText).
				SetPositiveButtonCallback(func(textInput string, tim *modal.TextInputModal) bool {
					if textInput != values.String(values.StrAwareOfRisk) {
						tim.SetError(values.String(values.StrConfirmPending))
						tim.SetLoading(false)
					} else {
						pg.WL.SelectedWallet.Wallet.SetBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, true)
						tim.Dismiss()
					}
					return false
				})
			textModal.Title(values.String(values.StrConfirmUmixedSpending)).
				SetPositiveButtonText(values.String(values.StrConfirm)).
				SetNegativeButtonCallback(func() {
					pg.spendUnmixedFunds.SetChecked(false)
				})
			pg.ParentWindow().ShowModal(textModal)

		} else {
			pg.WL.SelectedWallet.Wallet.SetBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false)
		}
	}

	if pg.connectToPeer.Changed() && !pg.isPrivacyModeOn() {
		if pg.connectToPeer.IsChecked() {
			pg.showSPVPeerDialog()
			return
		}

		title := values.String(values.StrRemovePeer)
		msg := values.String(values.StrRemovePeerWarn)
		pg.showWarningModalDialog(title, msg)
	}

	if pg.updateConnectToPeer.Clicked() && !pg.isPrivacyModeOn() {
		pg.showSPVPeerDialog()
	}

	if pg.verifyMessage.Clicked() {
		pg.ParentNavigator().Display(security.NewVerifyMessagePage(pg.Load))
	}

	if pg.validateAddr.Clicked() {
		pg.ParentNavigator().Display(security.NewValidateAddressPage(pg.Load))
	}

	if pg.signMessage.Clicked() {
		pg.ParentNavigator().Display(security.NewSignMessagePage(pg.Load))
	}

	if pg.checklog.Clicked() {
		pg.ParentNavigator().Display(s.NewLogPage(pg.Load, pg.wallet.LogFile(), values.String(values.StrWalletLog)))
	}

	if pg.checkStats.Clicked() {
		pg.ParentNavigator().Display(s.NewStatPage(pg.Load))
	}

	for pg.addAccount.Clicked() {
		newPasswordModal := modal.NewCreatePasswordModal(pg.Load).
			Title(values.String(values.StrCreateNewAccount)).
			EnableName(true).
			NameHint(values.String(values.StrAcctName)).
			EnableConfirmPassword(false).
			PasswordHint(values.String(values.StrSpendingPassword)).
			SetPositiveButtonCallback(func(accountName, password string, m *modal.CreatePasswordModal) bool {
				_, err := pg.wallet.CreateNewAccount(accountName, password)
				if err != nil {
					m.SetError(err.Error())
					m.SetLoading(false)
					return false
				}
				pg.loadWalletAccount()
				m.Dismiss()

				info := modal.NewSuccessModal(pg.Load, values.StringF(values.StrAcctCreated),
					modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(info)
				return true
			})
		pg.ParentWindow().ShowModal(newPasswordModal)
		break
	}

	if clicked, selectedItem := pg.accountsList.ItemClicked(); clicked {
		switch pg.wallet.GetAssetType() {
		case libutils.BTCWalletAsset:
			pg.ParentNavigator().Display(s.NewAcctBTCDetailsPage(pg.Load, pg.accounts[selectedItem].Account))
		case libutils.DCRWalletAsset:
			pg.ParentNavigator().Display(s.NewAcctDetailsPage(pg.Load, pg.accounts[selectedItem].Account))
		case libutils.LTCWalletAsset:
			pg.ParentNavigator().Display(s.NewAcctLTCDetailsPage(pg.Load, pg.accounts[selectedItem].Account))
		}
	}
}

func (pg *WalletSettingsPage) gapLimitModal() {
	walGapLim := pg.WL.SelectedWallet.Wallet.ReadStringConfigValueForKey(load.GapLimitConfigKey, "20")
	textModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrGapLimit)).
		SetTextWithTemplate(modal.SetGapLimitTemplate).
		SetText(walGapLim).
		PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
		SetPositiveButtonCallback(func(gapLimit string, tm *modal.TextInputModal) bool {
			val, err := strconv.ParseUint(gapLimit, 10, 32)
			if err != nil {
				tm.SetError(values.String(values.StrGapLimitInputErr))
				tm.SetLoading(false)
				return false
			}

			if val < 1 || val > 1000 {
				tm.SetError(values.String(values.StrGapLimitInputErr))
				tm.SetLoading(false)
				return false
			}
			gLimit := uint32(val)
			tm.SetLoading(true)

			err = pg.WL.SelectedWallet.Wallet.(*dcr.Asset).DiscoverUsage(gLimit)
			if err != nil {
				tm.SetError(err.Error())
				tm.SetLoading(false)
				return false
			}
			tm.SetLoading(false)
			info := modal.NewSuccessModal(pg.Load, values.String(values.StrAddressDiscoveryStarted), modal.DefaultClickFunc()).
				Body(values.String(values.StrAddressDiscoveryStartedBody))
			pg.ParentWindow().ShowModal(info)
			pg.WL.SelectedWallet.Wallet.SetStringConfigValueForKey(load.GapLimitConfigKey, gapLimit)
			return true
		})
	textModal.Title(values.String(values.StrDiscoverAddressUsage)).
		SetPositiveButtonText(values.String(values.StrDiscoverAddressUsage))
	pg.ParentWindow().ShowModal(textModal)
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *WalletSettingsPage) OnNavigatedFrom() {}
