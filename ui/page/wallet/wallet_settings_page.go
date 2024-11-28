package wallet

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"decred.org/dcrdex/dex"
	"gioui.org/font"
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
	"github.com/crypto-power/cryptopower/ui/page/info"
	"github.com/crypto-power/cryptopower/ui/page/security"
	"github.com/crypto-power/cryptopower/ui/page/seedbackup"
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

type SettingsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	wallet   sharedW.Asset
	accounts []*accountData

	pageContainer *widget.List

	changePass, viewSeed, rescan               *cryptomaterial.Clickable
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
	changeTab          func(string)

	peerAddr string
}

func NewSettingsPage(l *load.Load, wallet sharedW.Asset, walletCallbackFunc func(), changeTab func(string)) *SettingsPage {
	pg := &SettingsPage{
		Load:                l,
		GenericPageModal:    app.NewGenericPageModal(WalletSettingsPageID),
		wallet:              wallet,
		changePass:          l.Theme.NewClickable(false),
		viewSeed:            l.Theme.NewClickable(false),
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
		walletCallbackFunc: walletCallbackFunc,
		changeTab:          changeTab,
	}

	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.backButton = components.GetBackButton(l)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *SettingsPage) OnNavigatedTo() {
	pg.spendUnconfirmed.SetChecked(pg.readBool(sharedW.SpendUnconfirmedConfigKey))
	pg.spendUnmixedFunds.SetChecked(pg.readBool(sharedW.SpendUnmixedFundsKey))

	pg.loadPeerAddress()

	pg.loadWalletAccount()
}

func (pg *SettingsPage) readBool(key string) bool {
	return pg.wallet.ReadBoolConfigValueForKey(key, false)
}

func (pg *SettingsPage) isPrivacyModeOn() bool {
	return pg.AssetsManager.IsPrivacyModeOn()
}

func (pg *SettingsPage) loadPeerAddress() {
	if !pg.isPrivacyModeOn() {
		pg.peerAddr = pg.wallet.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")
		pg.connectToPeer.SetChecked(pg.peerAddr != "")
	}
}

func (pg *SettingsPage) loadWalletAccount() {
	walletAccounts := make([]*accountData, 0)
	accounts, err := pg.wallet.GetAccountsRaw()
	if err != nil {
		log.Errorf("error retrieving wallet accounts: %v", err)
		return
	}

	for _, acct := range accounts.Accounts {
		if !utils.IsImportedAccount(pg.wallet.GetAssetType(), acct) {
			walletAccounts = append(walletAccounts, &accountData{
				Account:   acct,
				clickable: pg.Theme.NewClickable(false),
			})
		}
	}

	pg.accounts = walletAccounts
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *SettingsPage) Layout(gtx C) D {
	w := []func(gtx C) D{
		pg.generalSection(),
		pg.securityTools(),
		pg.debug(),
		pg.dangerZone(),
	}

	return pg.Theme.List(pg.pageContainer).Layout(gtx, len(w), func(gtx C, i int) D {
		return w[i](gtx)
	})
}

func (pg *SettingsPage) generalSection() layout.Widget {
	dim := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if pg.wallet.IsWatchingOnlyWallet() {
					return D{}
				}
				return layout.Inset{}.Layout(gtx, pg.sectionContent(pg.changePass, values.String(values.StrSpendingPassword)))
			}),
			layout.Rigid(pg.sectionContent(pg.changeWalletName, values.String(values.StrRenameWalletSheetTitle))),
			layout.Rigid(func(gtx C) D {
				if !pg.wallet.IsWalletBackedUp() || !pg.wallet.HasWalletSeed() {
					return D{}
				}
				return layout.Inset{}.Layout(gtx, pg.sectionContent(pg.viewSeed, values.String(values.StrExportWalletSeed)))
			}),
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
						if !pg.connectToPeer.IsChecked() {
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

func (pg *SettingsPage) debug() layout.Widget {
	dim := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Bottom: values.MarginPadding24,
				}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							databaseTypeLabel := pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize16), values.String(values.StrDatabaseType))
							return databaseTypeLabel.Layout(gtx)
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, func(gtx C) D {
								dbDriverLabel := pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize16), pg.AssetsManager.DBDriver())
								dbDriverLabel.Color = pg.Theme.Color.GrayText2
								return dbDriverLabel.Layout(gtx)
							})
						}),
					)
				})
			}),
			layout.Rigid(pg.sectionContent(pg.rescan, values.String(values.StrRescanBlockchain))),
			layout.Rigid(func(gtx C) D {
				if pg.wallet.GetAssetType() == libutils.DCRWalletAsset {
					return pg.sectionDimension(gtx, pg.setGapLimit, values.String(values.StrSetGapLimit))
				}
				return D{}
			}),
			layout.Rigid(pg.sectionContent(pg.checklog, values.String(values.StrViewLog))),
			layout.Rigid(pg.sectionContent(pg.checkStats, values.String(values.StrViewStats))),
		)
	}
	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrDebug), dim)
	}
}

func (pg *SettingsPage) securityTools() layout.Widget {
	dim := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.sectionContent(pg.verifyMessage, values.String(values.StrVerifyMessage))),
			layout.Rigid(pg.sectionContent(pg.validateAddr, values.String(values.StrValidateMsg))),
			layout.Rigid(pg.sectionContent(pg.signMessage, values.String(values.StrSignMessage))),
		)
	}
	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrSecurityTools), dim)
	}
}

func (pg *SettingsPage) dangerZone() layout.Widget {
	return func(gtx C) D {
		return pg.pageSections(gtx, values.String(values.StrDangerZone), pg.sectionContent(pg.deleteWallet, values.String(values.StrRemoveWallet)))
	}
}

func (pg *SettingsPage) pageSections(gtx C, title string, body layout.Widget) D {
	dims := func(gtx C, title string, body layout.Widget) D {
		return cryptomaterial.LinearLayout{
			Orientation: layout.Vertical,
			Width:       cryptomaterial.MatchParent,
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
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						txt := pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize20), title)
						txt.Color = pg.Theme.Color.DeepBlue
						txt.Font.Weight = font.SemiBold
						return layout.Inset{Bottom: values.MarginPadding24}.Layout(gtx, txt.Layout)
					}),
					layout.Flexed(1, func(gtx C) D {
						if title == values.String(values.StrSecurityTools) {
							return layout.E.Layout(gtx, func(gtx C) D {
								pg.infoButton.Size = values.MarginPaddingTransform(pg.Load.IsMobileView(), values.MarginPadding20)
								return pg.infoButton.Layout(gtx)
							})
						}
						if title == values.String(values.StrAccount) {
							return layout.E.Layout(gtx, func(gtx C) D {
								if pg.wallet.IsWatchingOnlyWallet() {
									return D{}
								}
								return pg.addAccount.Layout(gtx, func(gtx C) D {
									return pg.Theme.AddIcon().LayoutTransform(gtx, pg.Load.IsMobileView(), values.MarginPadding24)
								})
							})
						}

						return D{}
					}),
				)
			}),
			layout.Rigid(body),
		)
	}

	return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		return dims(gtx, title, body)
	})
}

func (pg *SettingsPage) sectionContent(clickable *cryptomaterial.Clickable, title string) layout.Widget {
	return func(gtx C) D {
		return pg.sectionDimension(gtx, clickable, title)
	}
}

func (pg *SettingsPage) sectionDimension(gtx C, clickable *cryptomaterial.Clickable, title string) D {
	return clickable.Layout(gtx, func(gtx C) D {
		textLabel := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize16), title)
		if title == values.String(values.StrRemoveWallet) {
			textLabel.Color = pg.Theme.Color.Danger
		}
		return layout.Inset{
			Bottom: values.MarginPadding24,
		}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(textLabel.Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return pg.Theme.NewIcon(pg.Theme.Icons.ChevronRight).LayoutTransform(gtx, pg.Load.IsMobileView(), values.MarginPadding24)
					})
				}),
			)
		})
	})
}

func (pg *SettingsPage) subSection(gtx C, title string, body layout.Widget) D {
	return layout.Inset{Bottom: values.MarginPadding30}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize16), title).Layout),
			layout.Flexed(1, func(gtx C) D {
				switch title {
				case values.String(values.StrConnectToSpecificPeer):
					if pg.isPrivacyModeOn() {
						textlabel := pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize12), values.String(values.StrPrivacyModeActive))
						textlabel.Color = pg.Theme.Color.GrayText2
						body = textlabel.Layout
					}
				}
				return layout.E.Layout(gtx, body)
			}),
		)
	})
}

func (pg *SettingsPage) subSectionSwitch(title string, option *cryptomaterial.Switch) layout.Widget {
	return func(gtx C) D {
		return pg.subSection(gtx, title, option.Layout)
	}
}

func (pg *SettingsPage) changeSpendingPasswordModal() {
	var currentPassword, dexPass string
	// New wallet password modal.
	newSpendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		Title(values.String(values.StrChangeSpendingPass)).
		EnableName(false).
		PasswordHint(values.String(values.StrNewSpendingPassword)).
		ConfirmPasswordHint(values.String(values.StrConfirmNewSpendingPassword)).
		SetPositiveButtonCallback(func(_, newPassword string, m *modal.CreatePasswordModal) bool {
			err := pg.wallet.ChangePrivatePassphraseForWallet(currentPassword,
				newPassword, sharedW.PassphraseTypePass)
			if err != nil {
				m.SetError(err.Error())
				return false
			}

			if dexPass != "" { // update wallet password in dex
				assetID, _ := dex.BipSymbolID(pg.wallet.GetAssetType().ToStringLower())
				err := pg.AssetsManager.DexClient().SetWalletPassword([]byte(dexPass), assetID, []byte(newPassword))
				if err != nil {
					m.SetError(fmt.Errorf("failed to update your dex wallet password, try again: %v", err).Error())

					// Undo password change.
					if err = pg.wallet.ChangePrivatePassphraseForWallet(newPassword, currentPassword, sharedW.PassphraseTypePass); err != nil {
						log.Errorf("Failed to undo wallet passphrase change: %v", err)
					}

					return false
				}
			}

			info := modal.NewSuccessModal(pg.Load, values.StringF(values.StrSpendingPasswordUpdated),
				modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(info)
			return true
		})

	// DEX password modal.
	dexPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrDexPassword)).
		PasswordHint(values.String(values.StrDexPassword)).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := pg.AssetsManager.DexClient().Login([]byte(password))
			if err != nil {
				pm.SetError(err.Error())
				return false
			}

			dexPass = password
			pg.ParentWindow().ShowModal(newSpendingPasswordModal)
			return true
		}).SetCancelable(false)
	dexPasswordModal.SetPasswordTitleVisibility(false)

	// Current wallet password modal.
	currentSpendingPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		Title(values.String(values.StrConfirmSpendingPassword)).
		PasswordHint(values.String(values.StrCurrentSpendingPassword)).
		EnableName(false).
		EnableConfirmPassword(false).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			err := pg.wallet.UnlockWallet(password)
			if err != nil {
				pm.SetError(err.Error())
				return false
			}
			pg.wallet.LockWallet()

			currentPassword = password
			if pg.AssetsManager.DEXCInitialized() {
				// Check if this wallet is used by the dex client.
				assetType := pg.wallet.GetAssetType()
				assetID, ok := dex.BipSymbolID(assetType.ToStringLower())
				if ok {
					walletID, err := pg.AssetsManager.DexClient().WalletIDForAsset(assetID)
					if err != nil {
						log.Errorf("AssetsManager.DexClient.WalletIDForAsset error: %w", err)
					}
					if walletID != nil && pg.wallet.GetWalletID() == *walletID {
						// We need to update the password in dex, and we need
						// the dex password to do so.
						dexPasswordModal = dexPasswordModal.SetDescription(values.StringF(values.StrUpdateDEXWalletPasswordReason, assetType.ToFull(), pg.wallet.GetWalletName()))
						pg.ParentWindow().ShowModal(dexPasswordModal)
						return true
					}
				}
			}

			pg.ParentWindow().ShowModal(newSpendingPasswordModal)
			return true
		})
	pg.ParentWindow().ShowModal(currentSpendingPasswordModal)
}

func (pg *SettingsPage) deleteWalletModal() {
	textModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrWalletName)).
		SetTextWithTemplate(modal.RemoveWalletInfoTemplate, pg.wallet.GetWalletName()).
		PositiveButtonStyle(pg.Load.Theme.Color.Surface, pg.Load.Theme.Color.Danger).
		SetPositiveButtonCallback(func(walletName string, m *modal.TextInputModal) bool {
			if walletName != pg.wallet.GetWalletName() {
				m.SetError(values.String(values.StrWalletNameMismatch))
				return false
			}

			walletDeleted := func() {
				m.Dismiss()
				if pg.AssetsManager.LoadedWalletsCount() > 0 {
					pg.walletCallbackFunc()
				} else {
					pg.ParentWindow().CloseAllPages()
				}
			}

			if pg.wallet.IsWatchingOnlyWallet() {
				// no password is required for watching only wallets.
				err := pg.AssetsManager.DeleteWallet(pg.wallet.GetWalletID(), "")
				if err != nil {
					m.SetError(err.Error())
				} else {
					walletDeleted()
				}
				return false
			}

			walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
				EnableName(false).
				EnableConfirmPassword(false).
				Title(values.String(values.StrConfirmToRemove)).
				SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
					err := pg.AssetsManager.DeleteWallet(pg.wallet.GetWalletID(), password)
					if err != nil {
						pm.SetError(err.Error())
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

func (pg *SettingsPage) renameWalletModal() {
	textModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrWalletName)).
		SetText(pg.wallet.GetWalletName()).
		PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
		SetPositiveButtonCallback(func(newName string, tm *modal.TextInputModal) bool {
			name := strings.TrimSpace(newName)
			if !utils.ValidateLengthName(name) {
				tm.SetError(values.String(values.StrWalletNameLengthError))
				return false
			}

			err := pg.wallet.RenameWallet(name)
			if err != nil {
				tm.SetError(err.Error())
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

func (pg *SettingsPage) showSPVPeerDialog() {
	textModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrIPAddress)).
		PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
		SetPositiveButtonCallback(func(ipAddress string, tim *modal.TextInputModal) bool {
			addrs, ok := validatePeerAddressStr(ipAddress)
			if !ok {
				tim.SetError(values.StringF(values.StrValidateHostErr, addrs))
				return false
			}
			pg.wallet.SetSpecificPeer(addrs)
			pg.loadPeerAddress()
			return true
		}).
		SetText(pg.peerAddr)

	textModal.Title(values.String(values.StrConnectToSpecificPeer)).
		SetPositiveButtonText(values.String(values.StrConfirm)).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetNegativeButtonCallback(func() {
			pg.peerAddr = pg.wallet.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")
			pg.connectToPeer.SetChecked(pg.peerAddr != "")
		})
	pg.ParentWindow().ShowModal(textModal)
}

// validatePeerAddressStr validates the provided addrs string to ensure it's a
// valid peer address or a valid list of peer addresses. Returns the validated
// addrs string and true if there are no issues.
func validatePeerAddressStr(addrs string) (string, bool) {
	addresses := strings.Split(strings.Trim(addrs, " "), ";")
	// Prevent duplicate addresses.
	addrMap := make(map[string]*struct{})
	for _, addr := range addresses {
		addr = strings.Trim(addr, " ")
		if addr == "" {
			continue
		}

		host, _, err := net.SplitHostPort(addr)
		if err != nil { // If error, assume it's because no port was supplied, so use the whole address
			host = addr
		}

		if net.ParseIP(host) != nil {
			addrMap[addr] = &struct{}{}
			continue // ok
		}

		if _, err := url.ParseRequestURI(host); err != nil {
			return addr, false
		}

		addrMap[addr] = &struct{}{}
	}

	var addrStr string
	for addr := range addrMap {
		addrStr += ";" + addr
	}
	return strings.Trim(addrStr, ";"), true
}

func (pg *SettingsPage) clickableRow(gtx C, row clickableRowData) D {
	return row.clickable.Layout(gtx, func(gtx C) D {
		return pg.subSection(gtx, row.title, func(gtx C) D {
			lbl := pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize16), row.labelText)
			lbl.Color = pg.Theme.Color.GrayText2
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(lbl.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
						return pg.Theme.NewIcon(pg.Theme.Icons.ChevronRight).LayoutTransform(gtx, pg.Load.IsMobileView(), values.MarginPadding24)
					})
				}),
			)
		})
	})
}

func (pg *SettingsPage) showWarningModalDialog(title, msg string) {
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
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			// TODO: Check if deletion happened successfully
			// Since only one peer is available at time, the single peer key can
			// be set to empty string to delete its entry..
			pg.wallet.RemovePeers()
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
func (pg *SettingsPage) HandleUserInteractions(gtx C) {
	if pg.changePass.Clicked(gtx) {
		pg.changeSpendingPasswordModal()
	}

	if pg.viewSeed.Clicked(gtx) {
		currentPage := pg.ParentWindow().CurrentPageID()
		pg.ParentWindow().Display(seedbackup.NewBackupInstructionsPage(pg.Load, pg.wallet, func(_ *load.Load, navigator app.WindowNavigator) {
			navigator.ClosePagesAfter(currentPage)
		}))
	}

	if pg.rescan.Clicked(gtx) {
		go func() {
			info := modal.NewCustomModal(pg.Load).
				Title(values.String(values.StrRescanBlockchain)).
				Body(values.String(values.StrRescanInfo)).
				SetNegativeButtonText(values.String(values.StrCancel)).
				PositiveButtonStyle(pg.Theme.Color.Primary, pg.Theme.Color.Surface).
				SetPositiveButtonText(values.String(values.StrRescan)).
				SetPositiveButtonCallback(func(_ bool, im *modal.InfoModal) bool {
					err := pg.wallet.RescanBlocks()
					if err != nil {
						errorModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
						pg.ParentWindow().ShowModal(errorModal)
						im.Dismiss()
						return false
					}

					im.Dismiss()
					pg.changeTab(info.InfoID)
					return true
				})

			pg.ParentWindow().ShowModal(info)
		}()
	}

	if pg.setGapLimit.Clicked(gtx) {
		pg.gapLimitModal()
	}

	if pg.deleteWallet.Clicked(gtx) {
		pg.deleteWalletModal()
	}

	if pg.changeWalletName.Clicked(gtx) {
		pg.renameWalletModal()
	}

	if pg.infoButton.Button.Clicked(gtx) {
		info := modal.NewCustomModal(pg.Load).
			PositiveButtonStyle(pg.Theme.Color.Primary, pg.Theme.Color.Surface).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetupWithTemplate(modal.SecurityToolsInfoTemplate).
			Title(values.String(values.StrSecurityTools))
		pg.ParentWindow().ShowModal(info)
	}

	if pg.spendUnconfirmed.Changed(gtx) {
		pg.wallet.SaveUserConfigValue(sharedW.SpendUnconfirmedConfigKey, pg.spendUnconfirmed.IsChecked())
	}

	if pg.spendUnmixedFunds.Changed(gtx) {
		if pg.spendUnmixedFunds.IsChecked() {
			textModal := modal.NewTextInputModal(pg.Load).
				SetTextWithTemplate(modal.AllowUnmixedSpendingTemplate).
				// Hint(""). Code not deleted because a proper hint is required.
				PositiveButtonStyle(pg.Load.Theme.Color.Danger, pg.Load.Theme.Color.InvText).
				SetPositiveButtonCallback(func(textInput string, tim *modal.TextInputModal) bool {
					if textInput != values.String(values.StrAwareOfRisk) {
						tim.SetError(values.String(values.StrConfirmPending))
					} else {
						pg.wallet.SetBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, true)
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
			mixingEnabled := pg.wallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false)
			if !mixingEnabled {
				pg.spendUnmixedFunds.SetChecked(true)
				mixingNotEnabled := values.String(values.StrMixingNotSetUp)
				info := modal.NewErrorModal(pg.Load, mixingNotEnabled, modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(info)
			} else {
				pg.wallet.SetBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false)
			}
		}
	}

	if pg.connectToPeer.Changed(gtx) && !pg.isPrivacyModeOn() {
		if pg.connectToPeer.IsChecked() {
			pg.showSPVPeerDialog()
			return
		}

		title := values.String(values.StrRemovePeer)
		msg := values.String(values.StrRemovePeerWarn)
		pg.showWarningModalDialog(title, msg)
	}

	if pg.updateConnectToPeer.Clicked(gtx) && !pg.isPrivacyModeOn() {
		pg.showSPVPeerDialog()
	}

	if pg.verifyMessage.Clicked(gtx) {
		pg.ParentNavigator().Display(security.NewVerifyMessagePage(pg.Load, pg.wallet))
	}

	if pg.validateAddr.Clicked(gtx) {
		pg.ParentNavigator().Display(security.NewValidateAddressPage(pg.Load, pg.wallet))
	}

	if pg.signMessage.Clicked(gtx) {
		pg.ParentNavigator().Display(security.NewSignMessagePage(pg.Load, pg.wallet))
	}

	if pg.checklog.Clicked(gtx) {
		pg.ParentNavigator().Display(s.NewLogPage(pg.Load, pg.wallet.LogFile(), values.String(values.StrWalletLog)))
	}

	if pg.checkStats.Clicked(gtx) {
		pg.ParentNavigator().Display(s.NewStatPage(pg.Load, pg.wallet))
	}

	for pg.addAccount.Clicked(gtx) {
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
}

func (pg *SettingsPage) gapLimitModal() {
	walGapLim := pg.wallet.ReadStringConfigValueForKey(load.GapLimitConfigKey, "20")
	textModal := modal.NewTextInputModal(pg.Load).
		Hint(values.String(values.StrGapLimit)).
		SetTextWithTemplate(modal.SetGapLimitTemplate).
		SetText(walGapLim).
		PositiveButtonStyle(pg.Load.Theme.Color.Primary, pg.Load.Theme.Color.InvText).
		SetPositiveButtonCallback(func(gapLimit string, tm *modal.TextInputModal) bool {
			val, err := strconv.ParseUint(gapLimit, 10, 32)
			if err != nil {
				tm.SetError(values.String(values.StrGapLimitInputErr))
				return false
			}

			if val < 1 || val > 1000 {
				tm.SetError(values.String(values.StrGapLimitInputErr))
				return false
			}
			gLimit := uint32(val)

			err = pg.wallet.(*dcr.Asset).DiscoverUsage(gLimit)
			if err != nil {
				tm.SetError(err.Error())
				return false
			}

			info := modal.NewSuccessModal(pg.Load, values.String(values.StrAddressDiscoveryStarted), modal.DefaultClickFunc()).
				Body(values.String(values.StrAddressDiscoveryStartedBody))
			pg.ParentWindow().ShowModal(info)
			pg.wallet.SetStringConfigValueForKey(load.GapLimitConfigKey, gapLimit)
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
func (pg *SettingsPage) OnNavigatedFrom() {}
