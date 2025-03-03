package info

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/privacy"
	"github.com/crypto-power/cryptopower/ui/page/staking"
	"github.com/crypto-power/cryptopower/ui/page/transaction"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/decred/dcrd/dcrutil/v4"
)

const InfoID = "Info"

type (
	C = layout.Context
	D = layout.Dimensions
)

type WalletInfo struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	wallet sharedW.Asset

	container *widget.List

	transactions       []*sharedW.Transaction
	recentTransactions *cryptomaterial.ClickableList

	stakes       []*sharedW.Transaction
	recentStakes *cryptomaterial.ClickableList

	mixerInfoButton,
	mixerRedirectButton cryptomaterial.IconButton
	unmixedBalance sharedW.AssetAmount

	viewAllTxButton,
	viewAllStakeButton cryptomaterial.Button

	walletSyncInfo *components.WalletSyncInfo

	materialLoader     material.LoaderStyle
	showMaterialLoader bool
}

func NewInfoPage(l *load.Load, wallet sharedW.Asset, backup func(sharedW.Asset)) *WalletInfo {
	pg := &WalletInfo{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(InfoID),
		wallet:           wallet,
		container: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		recentTransactions: l.Theme.NewClickableList(layout.Vertical),
		recentStakes:       l.Theme.NewClickableList(layout.Vertical),
		materialLoader:     material.Loader(l.Theme.Base),
	}
	pg.walletSyncInfo = components.NewWalletSyncInfo(l, wallet, pg.reload, backup)
	pg.recentTransactions.Radius = cryptomaterial.Radius(14)
	pg.recentTransactions.IsShadowEnabled = true
	pg.recentStakes.Radius = cryptomaterial.Radius(14)
	pg.recentStakes.IsShadowEnabled = true

	pg.viewAllTxButton = pg.Theme.OutlineButton(values.String(values.StrViewAll))
	pg.viewAllTxButton.Font.Weight = font.Medium
	pg.viewAllTxButton.TextSize = values.TextSize16
	pg.viewAllTxButton.Inset = layout.UniformInset(0)
	pg.viewAllTxButton.HighlightColor = color.NRGBA{}

	pg.viewAllStakeButton = pg.Theme.OutlineButton(values.String(values.StrViewAll))
	pg.viewAllStakeButton.Font.Weight = font.Medium
	pg.viewAllStakeButton.TextSize = values.TextSize16
	pg.viewAllStakeButton.Inset = layout.UniformInset(0)
	pg.viewAllStakeButton.HighlightColor = color.NRGBA{}

	pg.mixerRedirectButton, pg.mixerInfoButton = components.SubpageHeaderButtons(l)
	pg.mixerRedirectButton.Icon = pg.Theme.Icons.NavigationArrowForward
	pg.mixerRedirectButton.Size = values.MarginPadding20

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *WalletInfo) OnNavigatedTo() {
	pg.walletSyncInfo.Init()
	pg.walletSyncInfo.ListenForNotifications() // stopped in OnNavigatedFrom()

	go pg.loadTransactions()

	if pg.wallet.GetAssetType() == libutils.DCRWalletAsset {
		go pg.loadStakes()

		if pg.wallet.(*dcr.Asset).IsAccountMixerActive() {
			pg.listenForMixerNotifications()
			pg.reloadMixerBalances()
		}
	}
}

func (pg *WalletInfo) reload() {
	pg.ParentWindow().Reload()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
// Layout lays out the widgets for the main wallets pg.
func (pg *WalletInfo) Layout(gtx C) D {
	return pg.Theme.List(pg.container).Layout(gtx, 1, func(gtx C, _ int) D {
		items := []layout.FlexChild{layout.Rigid(pg.walletSyncInfo.WalletInfoLayout)}

		items = append(items, layout.Rigid(layout.Spacer{Height: values.MarginPadding16}.Layout))

		if pg.wallet.GetAssetType() == libutils.DCRWalletAsset && pg.wallet.(*dcr.Asset).IsAccountMixerActive() {
			items = append(items, layout.Rigid(pg.mixerLayout))
		}
		if pg.showMaterialLoader {
			items = append(items, layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, pg.materialLoader.Layout)
			}))
		}
		if len(pg.transactions) > 0 {
			items = append(items, layout.Rigid(pg.recentTransactionLayout))
		}

		if len(pg.stakes) > 0 {
			items = append(items, layout.Rigid(pg.recentStakeLayout))
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
	})
}

func (pg *WalletInfo) mixerLayout(gtx C) D {
	return layout.Inset{
		Bottom: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return components.MixerComponent{
			Load:           pg.Load,
			WalletName:     pg.wallet.GetWalletName(),
			UnmixedBalance: pg.unmixedBalance.String(),
			ForwardButton:  pg.mixerRedirectButton,
			InfoButton:     pg.mixerInfoButton,
			Width:          cryptomaterial.MatchParent,
			Height:         cryptomaterial.WrapContent,
		}.MixerLayout(gtx)
	})
}

func (pg *WalletInfo) recentTransactionLayout(gtx C) D {
	return pg.pageContentWrapper(gtx, values.String(values.StrRecentTransactions), pg.viewAllTxButton.Layout, func(gtx C) D {
		return pg.recentTransactions.Layout(gtx, len(pg.transactions), func(gtx C, index int) D {
			tx := pg.transactions[index]
			isHiddenSeparator := index == len(pg.transactions)-1
			return pg.walletTxWrapper(gtx, tx, isHiddenSeparator)
		})
	})
}

func (pg *WalletInfo) recentStakeLayout(gtx C) D {
	return pg.pageContentWrapper(gtx, values.String(values.StrStakingActivity), pg.viewAllStakeButton.Layout, func(gtx C) D {
		return pg.recentStakes.Layout(gtx, len(pg.stakes), func(gtx C, index int) D {
			tx := pg.stakes[index]
			isHiddenSeparator := index == len(pg.stakes)-1
			return pg.walletTxWrapper(gtx, tx, isHiddenSeparator)
		})
	})
}

func (pg *WalletInfo) pageContentWrapper(gtx C, sectionTitle string, redirectBtn, body layout.Widget) D {
	return layout.Inset{
		Bottom: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Bottom: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									if sectionTitle == "" {
										return D{}
									}

									txt := pg.Theme.Body1(sectionTitle)
									txt.Font.Weight = font.SemiBold
									return txt.Layout(gtx)
								}),
								layout.Flexed(1, func(gtx C) D {
									if redirectBtn != nil {
										return layout.E.Layout(gtx, redirectBtn)
									}
									return D{}
								}),
							)
						})
					}),
					layout.Rigid(body),
				)
			})
		})
	})
}

func (pg *WalletInfo) walletTxWrapper(gtx C, tx *sharedW.Transaction, isHiddenSeparator bool) D {
	if !isHiddenSeparator {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		separator := pg.Theme.Separator()
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return components.LayoutTransactionRow(gtx, pg.Load, pg.wallet, tx, true)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					// Show bottom divider for all rows except last
					return layout.Inset{Left: values.MarginPadding32}.Layout(gtx, separator.Layout)
				})
			}),
		)
	}

	return components.LayoutTransactionRow(gtx, pg.Load, pg.wallet, tx, true)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *WalletInfo) HandleUserInteractions(gtx C) {
	// Process subpage events too.
	pg.walletSyncInfo.HandleUserInteractions(gtx)

	if clicked, selectedItem := pg.recentTransactions.ItemClicked(); clicked {
		pg.ParentNavigator().Display(transaction.NewTransactionDetailsPage(pg.Load, pg.wallet, pg.transactions[selectedItem]))
	}

	if clicked, selectedItem := pg.recentStakes.ItemClicked(); clicked {
		pg.ParentNavigator().Display(transaction.NewTransactionDetailsPage(pg.Load, pg.wallet, pg.stakes[selectedItem]))
	}

	// Navigate to mixer page when wallet mixer slider forward button is clicked.
	if pg.mixerRedirectButton.Button.Clicked(gtx) {
		pg.ParentNavigator().Display(privacy.NewAccountMixerPage(pg.Load, pg.wallet.(*dcr.Asset)))
	}

	if pg.viewAllTxButton.Button.Clicked(gtx) {
		pg.ParentNavigator().Display(transaction.NewTransactionsPage(pg.Load, pg.wallet))
	}

	if pg.viewAllStakeButton.Button.Clicked(gtx) {
		pg.ParentNavigator().Display(staking.NewStakingPage(pg.Load, pg.wallet.(*dcr.Asset)))
	}
}

func (pg *WalletInfo) listenForMixerNotifications() {
	accountMixerNotificationListener := &dcr.AccountMixerNotificationListener{
		OnAccountMixerStarted: func(_ int) {
			pg.reloadMixerBalances()
			pg.ParentWindow().Reload()
		},
		OnAccountMixerEnded: func(_ int) {
			pg.reloadMixerBalances()
			pg.ParentWindow().Reload()
		},
	}
	err := pg.wallet.(*dcr.Asset).AddAccountMixerNotificationListener(accountMixerNotificationListener, InfoID)
	if err != nil {
		log.Errorf("Error adding account mixer notification listener: %+v", err)
		return
	}

	// this is needed to refresh the UI on every block
	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnBlockAttached: func(_ int, _ int32) {
			pg.reloadMixerBalances()
			pg.ParentWindow().Reload()
		},
	}
	err = pg.wallet.(*dcr.Asset).AddTxAndBlockNotificationListener(txAndBlockNotificationListener, InfoID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}
}

func (pg *WalletInfo) reloadMixerBalances() {
	accounts, _ := pg.wallet.GetAccountsRaw()
	for _, acct := range accounts.Accounts {
		if acct.Number == pg.wallet.(*dcr.Asset).UnmixedAccountNumber() {
			bal := acct.Balance.Total
			// to prevent NPE set default amount 0 if asset amount is nil
			if bal == nil {
				bal = dcr.Amount(dcrutil.Amount(0))
			}
			pg.unmixedBalance = bal
		}
	}
}

// Reload tx list when there is new tx. Called from parent page
func (pg *WalletInfo) ListenForNewTx(walletID int) {
	if walletID != pg.wallet.GetWalletID() {
		return
	}
	pg.loadTransactions()
}

func (pg *WalletInfo) loadTransactions() {
	pg.showMaterialLoader = true
	mapInfo, _ := components.TxPageDropDownFields(pg.wallet.GetAssetType(), 0)
	if len(mapInfo) == 0 {
		log.Errorf("no tx filters for asset type (%v)", pg.wallet.GetAssetType())
		return
	}

	txs, err := pg.wallet.GetTransactionsRaw(0, 3, mapInfo[values.String(values.StrAll)], true, "")
	if err != nil {
		log.Errorf("error loading transactions: %v", err)
		return
	}
	pg.transactions = txs
	pg.showMaterialLoader = false
	pg.ParentWindow().Reload()
}

func (pg *WalletInfo) loadStakes() {
	pg.stakes = make([]*sharedW.Transaction, 0)

	txs, err := pg.wallet.GetTransactionsRaw(0, 10, libutils.TxFilterStaking, true, "")
	if err != nil {
		log.Errorf("error loading staking activities: %v", err)
		return
	}
	for _, stakeTx := range txs {
		if (stakeTx.Type == dcr.TxTypeTicketPurchase) || (stakeTx.Type == dcr.TxTypeRevocation) {
			pg.stakes = append(pg.stakes, stakeTx)
		}
	}
	if len(pg.stakes) > 3 {
		pg.stakes = pg.stakes[:3]
	}
	pg.ParentWindow().Reload()
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *WalletInfo) OnNavigatedFrom() {
	pg.walletSyncInfo.StopListeningForNotifications()
	if pg.wallet.GetAssetType() == libutils.DCRWalletAsset {
		pg.wallet.(*dcr.Asset).RemoveAccountMixerNotificationListener(InfoID)
	}
}
