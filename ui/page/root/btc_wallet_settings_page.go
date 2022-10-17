package root

import (
	"strings"

	"gioui.org/layout"

	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet/assets/btc"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	s "gitlab.com/raedah/cryptopower/ui/page/settings"
	"gitlab.com/raedah/cryptopower/ui/utils"
	"gitlab.com/raedah/cryptopower/ui/values"
)

const BTCWalletSettingsPageID = "BTCWalletSettings"

type btcAccountData struct {
	*btc.AccountResult
	clickable *cryptomaterial.Clickable
}

type BTCWalletSettingsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	wallet   *btc.BTCAsset
	accounts []*btcAccountData

	pageContainer layout.List
	accountsList  *cryptomaterial.ClickableList

	changePass, rescan, resetDexData           *cryptomaterial.Clickable
	changeAccount, checklog, checkStats        *cryptomaterial.Clickable
	changeWalletName, addAccount, deleteWallet *cryptomaterial.Clickable
	verifyMessage, validateAddr, signMessage   *cryptomaterial.Clickable
	updateConnectToPeer                        *cryptomaterial.Clickable

	backButton cryptomaterial.IconButton
	infoButton cryptomaterial.IconButton

	fetchProposal     *cryptomaterial.Switch
	proposalNotif     *cryptomaterial.Switch
	spendUnconfirmed  *cryptomaterial.Switch
	spendUnmixedFunds *cryptomaterial.Switch
	connectToPeer     *cryptomaterial.Switch

	peerAddr string
}

func NewBTCWalletSettingsPage(l *load.Load) *BTCWalletSettingsPage {
	pg := &BTCWalletSettingsPage{
		Load:                l,
		GenericPageModal:    app.NewGenericPageModal(BTCWalletSettingsPageID),
		wallet:              l.WL.SelectedBTCWallet.Wallet,
		changePass:          l.Theme.NewClickable(false),
		rescan:              l.Theme.NewClickable(false),
		resetDexData:        l.Theme.NewClickable(false),
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

		fetchProposal:     l.Theme.Switch(),
		proposalNotif:     l.Theme.Switch(),
		spendUnconfirmed:  l.Theme.Switch(),
		spendUnmixedFunds: l.Theme.Switch(),
		connectToPeer:     l.Theme.Switch(),

		pageContainer: layout.List{Axis: layout.Vertical},
		accountsList:  l.Theme.NewClickableList(layout.Vertical),
	}

	pg.backButton, pg.infoButton = components.SubpageHeaderButtons(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *BTCWalletSettingsPage) OnNavigatedTo() {
	// set switch button state on page load

	pg.loadWalletAccount()
}

func (pg *BTCWalletSettingsPage) loadWalletAccount() {
	walletAccounts := make([]*btcAccountData, 0)
	accounts, err := pg.wallet.GetAccountsRaw()
	if err != nil {
		log.Errorf("error retrieving wallet accounts: %v", err)
		return
	}

	for _, acct := range accounts.Accounts {
		if acct.AccountNumber == btc.ImportedAccountNumber {
			continue
		}

		walletAccounts = append(walletAccounts, &btcAccountData{
			AccountResult: acct,
			clickable:     pg.Theme.NewClickable(false),
		})
	}

	pg.accounts = walletAccounts
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *BTCWalletSettingsPage) Layout(gtx C) D {
	body := func(gtx C) D {
		w := []func(gtx C) D{
			func(gtx C) D {
				return layout.Inset{
					Bottom: values.MarginPadding26,
				}.Layout(gtx, pg.Theme.Label(values.TextSize20, values.String(values.StrSettings)).Layout)
			},
			pg.generalSection(),
			pg.account(),
			pg.dangerZone(),
		}

		return pg.pageContainer.Layout(gtx, len(w), func(gtx C, i int) D {
			return layout.Inset{Left: values.MarginPadding50}.Layout(gtx, w[i])
		})
	}

	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx, body)
	}
	return pg.layoutDesktop(gtx, body)
}

func (pg *BTCWalletSettingsPage) layoutDesktop(gtx C, body layout.Widget) D {
	return components.UniformPadding(gtx, body)
}

func (pg *BTCWalletSettingsPage) layoutMobile(gtx C, body layout.Widget) D {
	return components.UniformMobile(gtx, false, false, body)
}

func (pg *BTCWalletSettingsPage) generalSection() layout.Widget {
	dim := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.sectionContent(pg.changePass, values.String(values.StrSpendingPassword))),
			layout.Rigid(pg.sectionContent(pg.changeWalletName, values.String(values.StrRenameWalletSheetTitle))),
		)
	}

	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrGeneral), dim)
	}
}

func (pg *BTCWalletSettingsPage) account() layout.Widget {
	dim := func(gtx C) D {
		return pg.accountsList.Layout(gtx, len(pg.accounts), func(gtx C, a int) D {
			return pg.subSection(gtx, pg.accounts[a].AccountName, pg.Theme.Icons.ChevronRight.Layout24dp)
		})
	}
	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrAccount), dim)
	}
}

func (pg *BTCWalletSettingsPage) dangerZone() layout.Widget {
	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrDangerZone),
			pg.sectionContent(pg.deleteWallet, values.String(values.StrRemoveWallet)),
		)
	}
}

func (pg *BTCWalletSettingsPage) pageSections(gtx C, title string, body layout.Widget) D {
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
									if pg.WL.SelectedBTCWallet.Wallet.IsWatchingOnlyWallet() {
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

func (pg *BTCWalletSettingsPage) sectionContent(clickable *cryptomaterial.Clickable, title string) layout.Widget {
	return func(gtx C) D {
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
}

func (pg *BTCWalletSettingsPage) subSection(gtx C, title string, body layout.Widget) D {
	return layout.Inset{Top: values.MarginPadding5, Bottom: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(pg.Theme.Label(values.TextSize16, title).Layout),
			layout.Flexed(1, func(gtx C) D {
				return layout.E.Layout(gtx, body)
			}),
		)
	})
}

func (pg *BTCWalletSettingsPage) subSectionSwitch(title string, option *cryptomaterial.Switch) layout.Widget {
	return func(gtx C) D {
		return pg.subSection(gtx, title, option.Layout)
	}
}

func (pg *BTCWalletSettingsPage) changeSpendingPasswordModal() {
	currentSpendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		Title(values.String(values.StrChangeSpendingPass)).
		PasswordHint(values.String(values.StrCurrentSpendingPassword)).
		EnableName(false).
		EnableConfirmPassword(false).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := pg.wallet.UnlockWallet([]byte(password))
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
					err := pg.wallet.ChangePrivatePassphraseForWallet([]byte(password),
						[]byte(newPassword), sharedW.PassphraseTypePass)
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

func (pg *BTCWalletSettingsPage) deleteWalletModal() {
	textModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrWalletName)).
		SetTextWithTemplateAndWalletName(modal.RemoveWalletInfoTemplate, pg.WL.SelectedBTCWallet.Wallet.Name). // TODO: crashes when uncommented, shoud be investigated.
		PositiveButtonStyle(pg.Load.Theme.Color.Surface, pg.Load.Theme.Color.Danger).
		SetPositiveButtonCallback(func(walletName string, m *modal.TextInputModal) bool {
			if walletName != pg.WL.SelectedBTCWallet.Wallet.Name {
				m.SetError(values.String(values.StrWalletNameMismatch))
				m.SetLoading(false)
				return false
			}

			walletDeleted := func() {
				if pg.WL.MultiWallet.LoadedWalletsCount() > 0 {
					m.Dismiss()
					pg.ParentNavigator().CloseCurrentPage()
					onWalSelected := func() {
						pg.ParentWindow().CloseCurrentPage()
					}
					onDexServerSelected := func(server string) {
						log.Info("Not implemented yet...", server)
					}
					pg.ParentWindow().Display(NewWalletDexServerSelector(pg.Load, onWalSelected, onDexServerSelected))
				} else {
					m.Dismiss()
					pg.ParentWindow().CloseAllPages()
				}
			}

			if pg.wallet.IsWatchingOnlyWallet() {
				// no password is required for watching only wallets.
				err := pg.WL.MultiWallet.DeleteBTCWallet(pg.WL.SelectedBTCWallet.Wallet.ID, nil)
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
					err := pg.WL.MultiWallet.DeleteBTCWallet(pg.WL.SelectedBTCWallet.Wallet.ID, []byte(password))
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

func (pg *BTCWalletSettingsPage) renameWalletModal() {
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

			err := pg.WL.SelectedBTCWallet.Wallet.RenameWallet(name)
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

func (pg *BTCWalletSettingsPage) clickableRow(gtx C, row clickableRowData) D {
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

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *BTCWalletSettingsPage) HandleUserInteractions() {
	for pg.changePass.Clicked() {
		pg.changeSpendingPasswordModal()
		break
	}

	for pg.deleteWallet.Clicked() {
		pg.deleteWalletModal()
		break
	}

	for pg.changeWalletName.Clicked() {
		pg.renameWalletModal()
		break
	}

	if pg.checklog.Clicked() {
		pg.ParentNavigator().Display(s.NewLogPage(pg.Load))
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
				_, err := pg.wallet.CreateNewAccount(accountName, []byte(password))
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
		pg.ParentNavigator().Display(s.NewAcctBTCDetailsPage(pg.Load, pg.accounts[selectedItem].AccountResult))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *BTCWalletSettingsPage) OnNavigatedFrom() {}
