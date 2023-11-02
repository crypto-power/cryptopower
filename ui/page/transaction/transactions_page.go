package transaction

import (
	"fmt"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	TransactionsPageID = "Transactions"

	// pageSize defines the number of transactions that can be fetched at ago.
	pageSize = int32(20)
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var txTabs = []string{
	values.String(values.StrTxOverview),
	values.String(values.StrStakingActivity),
}

type TransactionsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	separator cryptomaterial.Line

	selectedTabIndex int

	txTypeDropDown *cryptomaterial.DropDown
	walletDropDown *cryptomaterial.DropDown

	transactionList  *cryptomaterial.ClickableList
	previousTxFilter int32
	scroll           *components.Scroll[*sharedW.Transaction]

	tab *cryptomaterial.SegmentedControl

	materialLoader material.LoaderStyle

	sourceWalletSelector *components.WalletAndAccountSelector
	assetWallets         []sharedW.Asset
	selectedWallet       sharedW.Asset

	isHomepageLayout,
	showLoader,
	hasDCRwallet,
	showDisabledLayout bool
}

func NewTransactionsPage(l *load.Load, isHomepageLayout bool) *TransactionsPage {
	pg := &TransactionsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TransactionsPageID),
		separator:        l.Theme.Separator(),
		transactionList:  l.Theme.NewClickableList(layout.Vertical),
		isHomepageLayout: isHomepageLayout,
		tab:              l.Theme.SegmentedControl(txTabs),
	}

	// init the asset selector
	if isHomepageLayout {
		pg.initWalletDropdown()
	} else {
		pg.selectedWallet = l.WL.SelectedWallet.Wallet
	}

	pg.scroll = components.NewScroll(l, pageSize, pg.loadTransactions)

	pg.transactionList.Radius = cryptomaterial.Radius(14)
	pg.transactionList.IsShadowEnabled = true

	pg.materialLoader = material.Loader(l.Theme.Base)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *TransactionsPage) OnNavigatedTo() {
	pg.refreshAvailableTxType()

	//TODO: Add wallet sync listener
	if !pg.selectedWallet.IsSynced() {
		// Events are disabled until the wallet is fully synced.
		return
	}

	pg.listenForTxNotifications() // tx ntfn listener is stopped in OnNavigatedFrom().
	go pg.scroll.FetchScrollData(false, pg.ParentWindow())
}

// initWalletSelector initializes the wallet selector dropdown to enable
// filtering transactions for a specific wallet when this page is used to
// display transactions for multiple wallets.
func (pg *TransactionsPage) initWalletDropdown() {
	pg.assetWallets = pg.WL.AllSortedWalletList()
	if pg.tab.SelectedSegment() != values.String(values.StrTxOverview) {
		pg.assetWallets = pg.WL.SortedWalletList(utils.DCRWalletAsset)
	}

	items := []cryptomaterial.DropDownItem{}
	for _, wal := range pg.assetWallets {
		assetType := wal.GetAssetType()
		if assetType == utils.DCRWalletAsset && !pg.hasDCRwallet {
			pg.hasDCRwallet = true
		}

		item := cryptomaterial.DropDownItem{
			Text: wal.GetWalletName(),
			Icon: pg.Theme.AssetIcon(assetType),
		}
		items = append(items, item)
	}

	pg.walletDropDown = pg.Theme.DropDown(items, values.WalletsDropdownGroup, 0)
	pg.selectedWallet = pg.assetWallets[pg.walletDropDown.SelectedIndex()]
}

func (pg *TransactionsPage) pageTitle(gtx C) D {
	txt := pg.Theme.Label(values.TextSize20, values.String(values.StrTransactions))
	txt.Font.Weight = font.SemiBold
	return txt.Layout(gtx)
}

func (pg *TransactionsPage) refreshAvailableTxType() {
	pg.showLoader = true
	wal := pg.selectedWallet

	items := []cryptomaterial.DropDownItem{}
	_, keysinfo := components.TxPageDropDownFields(wal.GetAssetType(), pg.selectedTabIndex)
	for _, name := range keysinfo {
		item := cryptomaterial.DropDownItem{}
		item.Text = name
		items = append(items, item)
	}
	pg.txTypeDropDown = pg.Theme.DropDown(items, values.TxDropdownGroup, 2)
	// Do this in background to prevent the app from freezing when counting
	// wallet txs. This is needed in situatuin where the wallet has lots of
	// txs to be counted.
	go func() {
		countfn := func(fType int32) int {
			count, _ := wal.CountTransactions(fType)
			return count
		}

		items := []cryptomaterial.DropDownItem{}
		mapinfo, keysinfo := components.TxPageDropDownFields(wal.GetAssetType(), pg.selectedTabIndex)
		for _, name := range keysinfo {
			fieldtype := mapinfo[name]
			item := cryptomaterial.DropDownItem{}
			if pg.selectedTabIndex == 0 {
				item.Text = fmt.Sprintf("%s (%d)", name, countfn(fieldtype))
			} else {
				item.Text = name
			}
			items = append(items, item)
		}
		pg.txTypeDropDown = pg.Theme.DropDown(items, values.TxDropdownGroup, 2)
		pg.ParentWindow().Reload()
		pg.showLoader = false
	}()
}

func (pg *TransactionsPage) loadTransactions(offset, pageSize int32) ([]*sharedW.Transaction, int, bool, error) {
	wal := pg.selectedWallet
	mapinfo, _ := components.TxPageDropDownFields(wal.GetAssetType(), pg.selectedTabIndex)
	if len(mapinfo) < 1 {
		err := fmt.Errorf("asset type(%v) and tab index(%d) found", wal.GetAssetType(), pg.selectedTabIndex)
		return nil, -1, false, err
	}

	selectedVal, _, _ := strings.Cut(pg.txTypeDropDown.Selected(), " ")
	txFilter, ok := mapinfo[selectedVal]
	if !ok {
		err := fmt.Errorf("unsupported field(%v) for asset type(%v) and tab index(%d) found",
			selectedVal, wal.GetAssetType(), pg.selectedTabIndex)
		return nil, -1, false, err
	}

	isReset := pg.previousTxFilter != txFilter
	if isReset {
		// reset the offset to zero
		offset = 0
		pg.previousTxFilter = txFilter
	}

	tempTxs, err := wal.GetTransactionsRaw(offset, pageSize, txFilter, true)
	if err != nil {
		err = fmt.Errorf("Error loading transactions: %v", err)
	}
	return tempTxs, len(tempTxs), isReset, err
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *TransactionsPage) Layout(gtx C) D {
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *TransactionsPage) syncOverlay(gtx C, body layout.Widget) D {
	// If wallet is not synced, display the overlay with the message.
	pg.showDisabledLayout = pg.isHomepageLayout && (!pg.selectedWallet.IsSynced() || pg.selectedWallet.IsRescanning())

	overlay := layout.Stacked(func(gtx C) D { return D{} })
	if pg.showDisabledLayout {
		gtxCopy := gtx
		overlay = layout.Stacked(func(gtx C) D {
			return components.DisablePageWithOverlay(pg.Load, nil, gtxCopy, values.String(values.StrFunctionUnavailable), nil)
		})
		// Disable main page from recieving events
		gtx = gtx.Disabled()
	}

	return layout.Stack{}.Layout(gtx, layout.Expanded(body), overlay)
}

func (pg *TransactionsPage) layoutDesktop(gtx C) D {
	pg.scroll.OnScrollChangeListener(pg.ParentWindow())
	wal := pg.WL.SelectedWallet.Wallet

	txlisingView := layout.Flexed(1, func(gtx C) D {
		return layout.Inset{Top: values.MarginPadding0}.Layout(gtx, func(gtx C) D {
			return layout.Stack{Alignment: layout.NW}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return layout.Inset{
						Top: values.MarginPadding60,
					}.Layout(gtx, func(gtx C) D {
						return pg.syncOverlay(gtx, func(gtx C) D {
							itemCount := pg.scroll.ItemsCount()
							card := pg.Theme.Card()
							// return "No transactions yet" text if there are no transactions
							if itemCount == 0 {
								padding := values.MarginPadding16
								txt := pg.Theme.Body1(values.String(values.StrNoTransactions))
								txt.Color = pg.Theme.Color.GrayText3
								return card.Layout(gtx, func(gtx C) D {
									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									return layout.Center.Layout(gtx, func(gtx C) D {
										return layout.Inset{Top: padding, Bottom: padding}.Layout(gtx, txt.Layout)
									})
								})
							}

							return pg.scroll.List().Layout(gtx, 1, func(gtx C, i int) D {
								return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
									return card.Layout(gtx, func(gtx C) D {
										if itemCount == -1 || pg.showLoader {
											gtx.Constraints.Min.X = gtx.Constraints.Max.X
											return layout.Center.Layout(gtx, pg.materialLoader.Layout)
										}

										wallTxs := pg.scroll.FetchedData()
										return pg.transactionList.Layout(gtx, len(wallTxs), func(gtx C, index int) D {
											tx := wallTxs[index]

											return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													return components.LayoutTransactionRow(gtx, pg.Load, pg.selectedWallet, row, true)
												}),
												layout.Rigid(func(gtx C) D {
													// No divider for last row
													if row.Index == len(wallTxs)-1 {
														return layout.Dimensions{}
													}

													return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
														layout.Rigid(func(gtx C) D {
															return components.LayoutTransactionRow(gtx, pg.Load, wal, tx, true)
														}),
														layout.Rigid(func(gtx C) D {
															// No divider for last row
															if index == len(wallTxs)-1 {
																return layout.Dimensions{}
															}

															gtx.Constraints.Min.X = gtx.Constraints.Max.X
															separator := pg.Theme.Separator()
															return layout.E.Layout(gtx, func(gtx C) D {
																// Show bottom divider for all rows except last
																return layout.Inset{Left: values.MarginPadding56}.Layout(gtx, separator.Layout)
															})
														}),
													)
												}),
											)
										})
									})
								})
							})
						})
					})
				}),
				layout.Expanded(func(gtx C) D {
					if pg.isHomepageLayout && len(pg.assetWallets) > 1 {
						return pg.walletDropDown.Layout(gtx, 0, false)
					}
					return D{}
				}),
				layout.Expanded(func(gtx C) D {
					if pg.showDisabledLayout {
						return D{}
					}
					return pg.txTypeDropDown.Layout(gtx, 0, true)
				}),
			)
		})
	})

	items := []layout.FlexChild{}
	if pg.hasDCRwallet && pg.selectedWallet.GetAssetType() == utils.DCRWalletAsset {
		// Layouts only supported by DCR
		items = append(items, layout.Rigid(pg.sectionNavTab))
	}

	items = append(items, txlisingView)
	if pg.isHomepageLayout {
		return components.UniformPadding(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
}

func (pg *TransactionsPage) sectionNavTab(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Direction:   layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, pg.tab.Layout)
	})
}

func (pg *TransactionsPage) layoutMobile(gtx C) D {
	container := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Stack{Alignment: layout.N}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding60,
						}.Layout(gtx, func(gtx C) D {
							return pg.scroll.List().Layout(gtx, 1, func(gtx C, i int) D {
								return pg.Theme.Card().Layout(gtx, func(gtx C) D {
									// return "No transactions yet" text if there are no transactions
									if pg.scroll.ItemsCount() <= 0 {
										padding := values.MarginPadding16
										txt := pg.Theme.Body1(values.String(values.StrNoTransactions))
										txt.Color = pg.Theme.Color.GrayText3
										gtx.Constraints.Min.X = gtx.Constraints.Max.X
										return layout.Center.Layout(gtx, func(gtx C) D {
											return layout.Inset{Top: padding, Bottom: padding}.Layout(gtx, txt.Layout)
										})
									}
									wallTxs := pg.scroll.FetchedData()
									return pg.transactionList.Layout(gtx, len(wallTxs), func(gtx C, index int) D {
										tx := wallTxs[index]
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												return components.LayoutTransactionRow(gtx, pg.Load, wal, tx, true)
											}),
											layout.Rigid(func(gtx C) D {
												// No divider for last row
												if index == len(wallTxs)-1 {
													return D{}
												}

												gtx.Constraints.Min.X = gtx.Constraints.Max.X
												separator := pg.Theme.Separator()
												return layout.E.Layout(gtx, func(gtx C) D {
													// Show bottom divider for all rows except last
													return layout.Inset{Left: values.MarginPadding56}.Layout(gtx, separator.Layout)
												})
											}),
										)
									})
								})
							})
						})
					}),
					layout.Expanded(func(gtx C) D {
						return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
							return pg.txTypeDropDown.Layout(gtx, 0, true)
						})
					}),
				)
			}),
		)
	}
	return components.UniformMobile(gtx, false, true, container)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *TransactionsPage) HandleUserInteractions() {
	for pg.txTypeDropDown.Changed() {
		go pg.scroll.FetchScrollData(false, pg.ParentWindow())
		break
	}

	if pg.isHomepageLayout && pg.walletDropDown.Changed() {
		pg.selectedWallet = pg.assetWallets[pg.walletDropDown.SelectedIndex()]
		pg.refreshAvailableTxType()
		go pg.scroll.FetchScrollData(false, pg.ParentWindow())
	}

	if clicked, selectedItem := pg.transactionList.ItemClicked(); clicked {
		transactions := pg.scroll.FetchedData()
		pg.ParentNavigator().Display(NewTransactionDetailsPage(pg.Load, pg.WL.SelectedWallet.Wallet, transactions[selectedItem], false))
	}

	dropDownList := []*cryptomaterial.DropDown{pg.txTypeDropDown}
	if pg.isHomepageLayout {
		dropDownList = append(dropDownList, pg.walletDropDown)
	}
	cryptomaterial.DisplayOneDropdown(dropDownList...)

	if pg.tab.Changed() {
		if pg.isHomepageLayout {
			pg.initWalletDropdown()
		}
		pg.refreshAvailableTxType()
		pg.selectedTabIndex = pg.tab.SelectedIndex()
		go pg.scroll.FetchScrollData(false, pg.ParentWindow())
	}
}

func (pg *TransactionsPage) listenForTxNotifications() {
	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(transaction *sharedW.Transaction) {
			pg.scroll.FetchScrollData(false, pg.ParentWindow())
			pg.ParentWindow().Reload()
		},
	}

	err := pg.selectedWallet.Wallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, TransactionsPageID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}
}

func (pg *TransactionsPage) stopTxNotificationsListener() {
	pg.WL.SelectedWallet.Wallet.RemoveTxAndBlockNotificationListener(TransactionsPageID)
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *TransactionsPage) OnNavigatedFrom() {
	pg.stopTxNotificationsListener()
}
