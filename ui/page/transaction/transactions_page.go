package transaction

import (
	"fmt"
	"sort"
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

	selectedTxCategoryTab int

	txTypeDropDown *cryptomaterial.DropDown
	walletDropDown *cryptomaterial.DropDown

	transactionList  *cryptomaterial.ClickableList
	previousTxFilter int32
	scroll           *components.Scroll[*components.MultiWalletTx]

	txCategoryTab *cryptomaterial.SegmentedControl

	materialLoader material.LoaderStyle

	walletSelector *components.WalletAndAccountSelector
	assetWallets   []sharedW.Asset
	selectedWallet sharedW.Asset

	isHomepageLayout,
	showLoader bool
}

func NewTransactionsPage(l *load.Load, isHomepageLayout bool) *TransactionsPage {
	pg := &TransactionsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TransactionsPageID),
		separator:        l.Theme.Separator(),
		transactionList:  l.Theme.NewClickableList(layout.Vertical),
		isHomepageLayout: isHomepageLayout,
		txCategoryTab:    l.Theme.SegmentedControl(txTabs),
	}

	// init the asset selector
	if isHomepageLayout {
		pg.initWalletDropdown()
	} else {
		pg.selectedWallet = pg.WL.SelectedWallet.Wallet
	}

	pg.scroll = components.NewScroll(l, pageSize, pg.fetchTransactions)

	pg.transactionList.Radius = cryptomaterial.Radius(14)
	pg.transactionList.IsShadowEnabled = true

	pg.materialLoader = material.Loader(pg.Theme.Base)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *TransactionsPage) OnNavigatedTo() {
	pg.refreshAvailableTxType()

	//TODO: Add wallet sync listener
	if pg.selectedWallet != nil && !pg.selectedWallet.IsSynced() {
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
	if pg.txCategoryTab.SelectedSegment() != values.String(values.StrTxOverview) {
		pg.assetWallets = pg.WL.SortedWalletList(utils.DCRWalletAsset)
	}

	if len(pg.assetWallets) > 1 {
		items := []cryptomaterial.DropDownItem{}
		for _, wal := range pg.assetWallets {
			item := cryptomaterial.DropDownItem{
				Text: wal.GetWalletName(),
				Icon: pg.Theme.AssetIcon(wal.GetAssetType()),
			}
			items = append(items, item)
		}

		pg.walletDropDown = pg.Theme.DropDown(items, values.WalletsDropdownGroup, 0)
		pg.walletDropDown.SetDefaultToNone()
	} else {
		pg.selectedWallet = pg.assetWallets[0]
	}
}

func (pg *TransactionsPage) pageTitle(gtx C) D {
	txt := pg.Theme.Label(values.TextSize20, values.String(values.StrTransactions))
	txt.Font.Weight = font.SemiBold
	return txt.Layout(gtx)
}

func (pg *TransactionsPage) getAssetType() utils.AssetType {
	wal := pg.selectedWallet
	var assetType utils.AssetType

	if wal == nil {
		assetType = utils.NilAsset
		if pg.txCategoryTab.SelectedSegment() != values.String(values.StrTxOverview) {
			assetType = utils.DCRWalletAsset
		}
	} else {
		assetType = wal.GetAssetType()
	}
	return assetType
}

func (pg *TransactionsPage) refreshAvailableTxType() {
	items := []cryptomaterial.DropDownItem{}
	_, keysinfo := components.TxPageDropDownFields(pg.getAssetType(), pg.selectedTxCategoryTab)
	for _, name := range keysinfo {
		items = append(items, cryptomaterial.DropDownItem{Text: name})
	}
	pg.txTypeDropDown = pg.Theme.DropDown(items, values.TxDropdownGroup, 2)

	if pg.selectedTxCategoryTab == 0 { // this is only needed for tx and not staking
		pg.showLoader = true
		wal := pg.selectedWallet

		// Do this in background to prevent the app from freezing when counting
		// wallet txs. This is needed in situations where the wallet has lots of
		// txs to be counted.
		go func() {
			countfn := func(fType int32) int {
				var totalCount int
				if wal == nil {
					for _, wal := range pg.WL.AllSortedWalletList() {
						count, _ := wal.CountTransactions(fType)
						totalCount += count
					}
				} else {
					totalCount, _ = wal.CountTransactions(fType)
				}
				return totalCount
			}

			items := []cryptomaterial.DropDownItem{}
			mapinfo, keysinfo := components.TxPageDropDownFields(pg.getAssetType(), pg.selectedTxCategoryTab)
			for _, name := range keysinfo {
				fieldtype := mapinfo[name]
				items = append(items, cryptomaterial.DropDownItem{
					Text: fmt.Sprintf("%s (%d)", name, countfn(fieldtype)),
				})
			}

			pg.txTypeDropDown = pg.Theme.DropDown(items, values.TxDropdownGroup, 2)
			pg.showLoader = false
			pg.ParentWindow().Reload()
		}()
	}
	pg.ParentWindow().Reload()
}

func (pg *TransactionsPage) fetchTransactions(offset, pageSize int32) (txs []*components.MultiWalletTx, totalTxs int, isReset bool, err error) {
	wal := pg.selectedWallet
	if wal == nil {
		txs, totalTxs, isReset, err = pg.multiWalletTxns(offset, pageSize)
	} else {
		txs, totalTxs, isReset, err = pg.loadTransactions(wal, offset, pageSize)
	}

	return txs, totalTxs, isReset, err
}

func (pg *TransactionsPage) multiWalletTxns(offset, pageSize int32) ([]*components.MultiWalletTx, int, bool, error) {
	allTxs := make([]*components.MultiWalletTx, 0)
	var isReset bool

	for _, wal := range pg.assetWallets {
		if !wal.IsSynced() {
			continue // skip wallets that are not synced
		}

		txs, _, reset, err := pg.loadTransactions(wal, offset, pageSize)
		if err != nil {
			return nil, 0, isReset, err
		}
		allTxs = append(allTxs, txs...)

		if reset && !isReset {
			isReset = reset
		}
	}

	sort.Slice(allTxs, func(i, j int) bool {
		return allTxs[i].Timestamp > allTxs[j].Timestamp
	})

	if len(allTxs) > int(pageSize) {
		allTxs = allTxs[:int(pageSize)]
	}

	return allTxs, len(allTxs), isReset, nil
}

func (pg *TransactionsPage) loadTransactions(wal sharedW.Asset, offset, pageSize int32) ([]*components.MultiWalletTx, int, bool, error) {
	mapinfo, _ := components.TxPageDropDownFields(wal.GetAssetType(), pg.selectedTxCategoryTab)
	if len(mapinfo) < 1 {
		err := fmt.Errorf("asset type(%v) and txCategoryTab index(%d) found", wal.GetAssetType(), pg.selectedTxCategoryTab)
		return nil, -1, false, err
	}

	selectedVal, _, _ := strings.Cut(pg.txTypeDropDown.Selected(), " ")
	txFilter, ok := mapinfo[selectedVal]
	if !ok {
		err := fmt.Errorf("unsupported field(%v) for asset type(%v) and txCategoryTab index(%d) found",
			selectedVal, wal.GetAssetType(), pg.selectedTxCategoryTab)
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

	txs := make([]*components.MultiWalletTx, 0)
	for _, tx := range tempTxs {
		txs = append(txs, &components.MultiWalletTx{tx, wal.GetWalletID()})
	}

	return txs, len(txs), isReset, err
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

func (pg *TransactionsPage) walletNotReady() bool {
	return pg.selectedWallet != nil && (!pg.selectedWallet.IsSynced() || pg.selectedWallet.IsRescanning())
}

func (pg *TransactionsPage) txListLayout(gtx C) D {
	pg.scroll.OnScrollChangeListener(pg.ParentWindow())

	layoutContents := []layout.StackChild{layout.Expanded(func(gtx C) D {
		return layout.Inset{Top: values.MarginPadding60}.Layout(gtx, func(gtx C) D {
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
						return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
						if itemCount == -1 || pg.showLoader {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.Center.Layout(gtx, pg.materialLoader.Layout)
						}

						wallTxs := pg.scroll.FetchedData()
						return pg.transactionList.Layout(gtx, len(wallTxs), func(gtx C, index int) D {
							tx, wal := components.TxAndWallet(pg.WL.AssetsManager, wallTxs[index])
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									isHiddenAssetsInfo := true
									if pg.selectedWallet == nil {
										isHiddenAssetsInfo = !isHiddenAssetsInfo
									}
									return components.LayoutTransactionRow(gtx, pg.Load, wal, tx, isHiddenAssetsInfo)
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
						})
					})
				})
				})
			})
		})
	})}

	// disabled overlay
	if pg.walletNotReady() && pg.isHomepageLayout {
		layoutContents = append(layoutContents, layout.Stacked(func(gtx C) D {
			gtx = gtx.Disabled()
			overlayColor := pg.Theme.Color.Gray3
			overlayColor.A = 220
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y - gtx.Dp(values.MarginPadding60)
			cryptomaterial.FillMax(gtx, overlayColor, 10)

			lbl := pg.Theme.Label(values.TextSize20, values.String(values.StrFunctionUnavailable))
			lbl.Font.Weight = font.SemiBold
			lbl.Color = pg.Theme.Color.PageNavText
			return cryptomaterial.CentralizeWidget(gtx, lbl.Layout)
		}))
	}

	return layout.Stack{Alignment: layout.NW}.Layout(gtx, layoutContents...)
}

func (pg *TransactionsPage) layoutDesktop(gtx C) D {
	items := []layout.FlexChild{}
	if pg.selectedWallet == nil || pg.selectedWallet.GetAssetType() == utils.DCRWalletAsset {
		// Only show tx category navigation txCategoryTab for DCR wallets.
		items = append(items, layout.Rigid(pg.txCategoriesNav))
	}

	items = append(items, layout.Rigid(pg.desktopLayoutContent))
	if pg.isHomepageLayout {
		return components.UniformPadding(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
}

func (pg *TransactionsPage) txCategoriesNav(gtx C) D {
	return cryptomaterial.CentralizeWidget(gtx, func(gtx C) D {
		return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, pg.txCategoryTab.Layout)
	})
}

func (pg *TransactionsPage) desktopLayoutContent(gtx C) D {
	if pg.walletNotReady() && pg.walletDropDown == nil {
		return pg.txListLayout(gtx) // nothing else to display on this page at this time
	}

	pageElements := []layout.StackChild{layout.Expanded(pg.txListLayout)}

	if pg.walletDropDown != nil { // display wallet dropdown if available
		walletDropdownWidget := layout.Expanded(func(gtx C) D {
			return pg.walletDropDown.Layout(gtx, 0, false)
		})
		pageElements = append(pageElements, walletDropdownWidget)
	}

	// display tx dropdown if selected wallet is ready and showLoader is false
	if !pg.walletNotReady() && !pg.showLoader {
		pg.ParentWindow().Reload() //refresh UI to display updated txType dropdown

		txDropdownWidget := layout.Expanded(func(gtx C) D {
			return pg.txTypeDropDown.Layout(gtx, 0, true)
		})
		pageElements = append(pageElements, txDropdownWidget)
	}

	return layout.Stack{Alignment: layout.NW}.Layout(gtx, pageElements...)
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
										tx, wal := components.TxAndWallet(pg.WL.AssetsManager, wallTxs[index])
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												showAssets := true
												if pg.selectedWallet == nil {
													showAssets = !showAssets
												}
												return components.LayoutTransactionRow(gtx, pg.Load, wal, tx, showAssets)
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

	if pg.isHomepageLayout && pg.walletDropDown != nil && pg.walletDropDown.Changed() {
		pg.selectedWallet = pg.assetWallets[pg.walletDropDown.SelectedIndex()]
		pg.refreshAvailableTxType()
		go pg.scroll.FetchScrollData(false, pg.ParentWindow())
	}

	if clicked, selectedItem := pg.transactionList.ItemClicked(); clicked {
		transactions := pg.scroll.FetchedData()
		tx, wal := components.TxAndWallet(pg.WL.AssetsManager, transactions[selectedItem])
		pg.ParentNavigator().Display(NewTransactionDetailsPage(pg.Load, wal, tx, false))
	}

	dropDownList := []*cryptomaterial.DropDown{pg.txTypeDropDown}
	if pg.isHomepageLayout && pg.walletDropDown != nil {
		dropDownList = append(dropDownList, pg.walletDropDown)
	}
	cryptomaterial.DisplayOneDropdown(dropDownList...)

	if pg.txCategoryTab.Changed() {
		pg.selectedTxCategoryTab = pg.txCategoryTab.SelectedIndex()
		if pg.isHomepageLayout {
			pg.initWalletDropdown()
		}

		pg.refreshAvailableTxType()
		go pg.scroll.FetchScrollData(false, pg.ParentWindow())
	}
}

func (pg *TransactionsPage) listenForTxNotifications() {
	if pg.selectedWallet == nil {
		return
	}

	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(transaction *sharedW.Transaction) {
			pg.scroll.FetchScrollData(false, pg.ParentWindow())
			pg.ParentWindow().Reload()
		},
	}
	err := pg.selectedWallet.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, TransactionsPageID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}
}

func (pg *TransactionsPage) stopTxNotificationsListener() {
	if pg.selectedWallet == nil {
		return
	}
	pg.selectedWallet.RemoveTxAndBlockNotificationListener(TransactionsPageID)
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
