package transaction

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/txhelper"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	TransactionsPageID = "Transactions"

	// pageSize defines the number of transactions that can be fetched at ago.
	pageSize = int32(30)
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type multiWalletTx struct {
	*sharedW.Transaction
	walletID int
}

var txTabs = []string{
	values.StrTxRegular,
	values.StrStakingTx,
}

// TransactionsPage shows transactions for a specific wallet or for all wallets.
// TODO: Add support for showing all wallet txs.
type TransactionsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	separator cryptomaterial.Line

	selectedTxCategoryTab int

	statusDropDown *cryptomaterial.DropDown
	orderDropDown  *cryptomaterial.DropDown
	walletDropDown *cryptomaterial.DropDown
	filterBtn      *cryptomaterial.Clickable
	exportBtn      *cryptomaterial.Clickable
	isFilterOpen   bool
	searchEditor   cryptomaterial.Editor

	transactionList *cryptomaterial.ClickableList
	txFilter,
	previousTxFilter int32
	scroll *components.Scroll[*multiWalletTx]

	txCategoryTab *cryptomaterial.SegmentedControl

	materialLoader material.LoaderStyle

	multiWalletLayout bool
	assetWallets      []sharedW.Asset
	selectedWallet    sharedW.Asset

	showLoader,
	dcrWalletExists,
	isShowTitle bool
}

func NewTransactionsPage(l *load.Load, wallet sharedW.Asset) *TransactionsPage {
	pg := &TransactionsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TransactionsPageID),
		separator:        l.Theme.Separator(),
		transactionList:  l.Theme.NewClickableList(layout.Vertical),
		txCategoryTab:    l.Theme.SegmentedControl(txTabs, cryptomaterial.SegmentTypeGroup),
		selectedWallet:   wallet,
		isShowTitle:      true,
	}

	pg.searchEditor = l.Theme.SearchEditor(new(widget.Editor), values.String(values.StrSearch), l.Theme.Icons.SearchIcon)
	pg.searchEditor.Editor.SingleLine = true
	pg.searchEditor.TextSize = pg.ConvertTextSize(l.Theme.TextSize)

	// init the wallet selector if no wallet was pre-selected
	if pg.selectedWallet == nil {
		pg.multiWalletLayout = true
		pg.initWalletSelector()
		pg.isShowTitle = false
	}

	pg.scroll = components.NewScroll(l, pageSize, pg.fetchTransactions)
	pg.filterBtn = l.Theme.NewClickable(false)
	pg.exportBtn = l.Theme.NewClickable(false)
	pg.transactionList.Radius = cryptomaterial.Radius(14)
	pg.transactionList.IsShadowEnabled = true

	pg.orderDropDown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrNewest)},
		{Text: values.String(values.StrOldest)},
	}, values.ProposalDropdownGroup, 1, 0, false)
	pg.orderDropDown.Width = values.MarginPadding100
	pg.materialLoader = material.Loader(pg.Theme.Base)
	pg.orderDropDown.CollapsedLayoutTextDirection = layout.E
	settingCommonDropdown(pg.Theme, pg.orderDropDown)
	pg.orderDropDown.SetConvertTextSize(pg.ConvertTextSize)

	return pg
}

func NewTransactionsPageWithType(l *load.Load, selectedTab int, wallet sharedW.Asset) *TransactionsPage {
	pg := &TransactionsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TransactionsPageID),
		separator:        l.Theme.Separator(),
		transactionList:  l.Theme.NewClickableList(layout.Vertical),
		txCategoryTab:    l.Theme.SegmentedControl(txTabs, cryptomaterial.SegmentTypeGroup),
		selectedWallet:   wallet,
		isShowTitle:      true,
	}
	pg.selectedTxCategoryTab = selectedTab
	pg.txCategoryTab.SetSelectedSegment(txTabs[selectedTab])
	pg.searchEditor = l.Theme.SearchEditor(new(widget.Editor), values.String(values.StrSearch), l.Theme.Icons.SearchIcon)
	pg.searchEditor.Editor.SingleLine = true
	pg.searchEditor.TextSize = pg.ConvertTextSize(l.Theme.TextSize)
	// init the wallet selector if no wallet was pre-selected
	if pg.selectedWallet == nil {
		pg.multiWalletLayout = true
		pg.initWalletSelector()
		pg.isShowTitle = false
	}
	pg.scroll = components.NewScroll(l, pageSize, pg.fetchTransactions)
	pg.filterBtn = l.Theme.NewClickable(false)
	pg.exportBtn = l.Theme.NewClickable(false)
	pg.transactionList.Radius = cryptomaterial.Radius(14)
	pg.transactionList.IsShadowEnabled = true
	pg.orderDropDown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrNewest)},
		{Text: values.String(values.StrOldest)},
	}, values.ProposalDropdownGroup, 1, 0, false)
	pg.orderDropDown.Width = values.MarginPadding100
	pg.materialLoader = material.Loader(pg.Theme.Base)
	pg.orderDropDown.CollapsedLayoutTextDirection = layout.E
	settingCommonDropdown(pg.Theme, pg.orderDropDown)
	pg.orderDropDown.SetConvertTextSize(pg.ConvertTextSize)
	return pg
}

func (pg *TransactionsPage) DisableUniformTab() {
	pg.txCategoryTab.DisableUniform(true)
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *TransactionsPage) OnNavigatedTo() {
	pg.refreshAvailableTxType()

	pg.listenForTxNotifications() // tx ntfn listener is stopped in OnNavigatedFrom().
	go pg.scroll.FetchScrollData(false, pg.ParentWindow(), false)
}

// initWalletSelector initializes the wallet selector dropdown to enable
// filtering transactions for a specific wallet when this page is used to
// display transactions for multiple wallets.
func (pg *TransactionsPage) initWalletSelector() {
	pg.assetWallets = pg.AssetsManager.AllWallets()
	if pg.txCategoryTab.SelectedSegment() != values.StrTxRegular {
		pg.assetWallets = pg.AssetsManager.AllDCRWallets()
	}

	if len(pg.assetWallets) > 1 {
		items := []cryptomaterial.DropDownItem{
			{
				Icon: pg.Theme.Icons.WalletIcon,
				Text: values.String(values.StrAllWallets),
			},
		}
		for _, wal := range pg.assetWallets {
			if !pg.dcrWalletExists && wal.GetAssetType() == utils.DCRWalletAsset {
				pg.dcrWalletExists = true
			}
			item := cryptomaterial.DropDownItem{
				Text: wal.GetWalletName(),
				Icon: pg.Theme.AssetIcon(wal.GetAssetType()),
			}
			items = append(items, item)
		}

		pg.walletDropDown = pg.Theme.DropDown(items, nil, values.WalletsDropdownGroup, false)
		settingCommonDropdown(pg.Theme, pg.walletDropDown)
		pg.walletDropDown.Width = values.MarginPadding150
		pg.walletDropDown.SetConvertTextSize(pg.ConvertTextSize)
	} else {
		pg.selectedWallet = pg.assetWallets[0]
	}
}

func (pg *TransactionsPage) getAssetType() utils.AssetType {
	if pg.selectedWallet == nil {
		return utils.DCRWalletAsset
	}
	return pg.selectedWallet.GetAssetType()
}

func (pg *TransactionsPage) refreshAvailableTxType() {
	items := []cryptomaterial.DropDownItem{}
	_, keysInfo := components.TxPageDropDownFields(pg.getAssetType(), pg.selectedTxCategoryTab)
	for _, name := range keysInfo {
		items = append(items, cryptomaterial.DropDownItem{Text: name})
	}
	pg.statusDropDown = pg.Theme.DropdownWithCustomPos(items, values.TxDropdownGroup, 0, 2, false)
	pg.statusDropDown.Width = values.DP118
	pg.statusDropDown.CollapsedLayoutTextDirection = layout.E
	pg.statusDropDown.SetConvertTextSize(pg.ConvertTextSize)
	settingCommonDropdown(pg.Theme, pg.statusDropDown)

	// only show tx count for regular txs, not staking
	if pg.txCategoryTab.SelectedSegment() == values.StrTxOverview {
		pg.showLoader = true

		wallets := pg.assetWallets
		if pg.selectedWallet != nil {
			wallets = []sharedW.Asset{pg.selectedWallet}
		}

		// Do this in background to prevent the app from freezing when counting
		// wallet txs. This is needed in situations where the wallet has lots of
		// txs to be counted.
		go func() {
			items := []cryptomaterial.DropDownItem{}
			mapInfo, keysInfo := components.TxPageDropDownFields(pg.getAssetType(), pg.selectedTxCategoryTab)
			for _, name := range keysInfo {
				var txTypeCount int
				for _, wal := range wallets {
					count, _ := wal.CountTransactions(mapInfo[name])
					txTypeCount += count
				}
				items = append(items, cryptomaterial.DropDownItem{
					Text: fmt.Sprintf("%s (%d)", name, txTypeCount),
				})
			}

			pg.statusDropDown = pg.Theme.DropdownWithCustomPos(items, values.TxDropdownGroup, 0, 2, false)
			pg.statusDropDown.Width = values.DP118
			pg.statusDropDown.CollapsedLayoutTextDirection = layout.E
			pg.statusDropDown.SetConvertTextSize(pg.ConvertTextSize)
			settingCommonDropdown(pg.Theme, pg.statusDropDown)
			pg.showLoader = false
			pg.ParentWindow().Reload()
		}()
	}
}

func (pg *TransactionsPage) fetchTransactions(offset, pageSize int32) (txs []*multiWalletTx, totalTxs int, isReset bool, err error) {
	isReset = pg.previousTxFilter != pg.txFilter
	if isReset {
		// reset the offset to zero
		offset = 0
		pg.previousTxFilter = pg.txFilter
	}

	orderNewest := pg.orderDropDown.Selected() != values.String(values.StrOldest)

	wal := pg.selectedWallet
	if wal == nil {
		txs, totalTxs, err = pg.multiWalletTxns(offset, pageSize, orderNewest)
	} else {
		txs, totalTxs, err = pg.loadTransactions(wal, offset, pageSize, orderNewest)
	}

	return txs, totalTxs, isReset, err
}

func (pg *TransactionsPage) multiWalletTxns(offset, pageSize int32, newestFirst bool) ([]*multiWalletTx, int, error) {
	allTxs := make([]*multiWalletTx, 0)
	for _, wal := range pg.assetWallets {
		if !wal.IsSynced() {
			continue // skip wallets that are not synced
		}

		txs, _, err := pg.loadTransactions(wal, offset, pageSize, newestFirst)
		if err != nil {
			return nil, 0, err
		}
		allTxs = append(allTxs, txs...)
	}

	sort.Slice(allTxs, func(i, j int) bool {
		if newestFirst {
			return allTxs[i].Timestamp > allTxs[j].Timestamp
		}
		return allTxs[i].Timestamp < allTxs[j].Timestamp
	})

	// TODO: remove after PR 268  // Improve infinite scroll list functionality is merged
	if len(allTxs) > int(pageSize) {
		allTxs = allTxs[:int(pageSize)]
	}

	return allTxs, len(allTxs), nil
}

func (pg *TransactionsPage) loadTransactions(wal sharedW.Asset, offset, pageSize int32, newestFirst bool) ([]*multiWalletTx, int, error) {
	mapInfo, _ := components.TxPageDropDownFields(wal.GetAssetType(), pg.selectedTxCategoryTab)
	if len(mapInfo) < 1 {
		err := fmt.Errorf("unable to resolve asset filters for asset type (%v)", wal.GetAssetType())
		return nil, -1, err
	}

	selectedVal, _, _ := strings.Cut(pg.statusDropDown.Selected(), " ")
	txFilter, ok := mapInfo[selectedVal]
	if !ok {
		err := fmt.Errorf("unsupported field(%v) for asset type(%v) and txCategoryTab index(%d) found",
			selectedVal, wal.GetAssetType(), pg.selectedTxCategoryTab)
		return nil, -1, err
	}
	pg.txFilter = txFilter
	searchKey := pg.searchEditor.Editor.Text()
	walletTxs, err := wal.GetTransactionsRaw(offset, pageSize, txFilter, newestFirst, searchKey)
	if err != nil {
		err = fmt.Errorf("error loading transactions: %v", err)
	}

	txs := make([]*multiWalletTx, 0)
	for i := range walletTxs {
		txs = append(txs, &multiWalletTx{walletTxs[i], wal.GetWalletID()})
	}

	return txs, len(txs), err
}

func settingCommonDropdown(t *cryptomaterial.Theme, dropdown *cryptomaterial.DropDown) {
	dropdown.FontWeight = font.SemiBold
	dropdown.Hoverable = false
	dropdown.SelectedItemIconColor = &t.Color.Primary
	dropdown.ExpandedLayoutInset = layout.Inset{Top: values.MarginPadding35}
	dropdown.MakeCollapsedLayoutVisibleWhenExpanded = true
	dropdown.Background = &t.Color.Gray4
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *TransactionsPage) Layout(gtx C) D {
	isDCRAssetSelected := pg.selectedWallet != nil && pg.selectedWallet.GetAssetType() == utils.DCRWalletAsset
	if isDCRAssetSelected || (pg.dcrWalletExists && pg.selectedWallet == nil) {
		// Only show tx category navigation txCategoryTab for DCR wallets.
		return pg.txCategoryTab.Layout(gtx, pg.layoutContent, pg.IsMobileView())
	}

	return cryptomaterial.UniformPadding(gtx, func(gtx C) D {
		return pg.layoutContent(gtx)
	}, pg.IsMobileView())
}

func (pg *TransactionsPage) walletNotReady() bool {
	return pg.selectedWallet != nil && (!pg.selectedWallet.IsSynced() || pg.selectedWallet.IsRescanning())
}

func (pg *TransactionsPage) layoutContent(gtx C) D {
	if pg.walletNotReady() && pg.walletDropDown == nil {
		return pg.txListLayout(gtx) // nothing else to display on this page at this time
	}

	pageElements := []layout.StackChild{
		layout.Expanded(pg.txListLayout),
	}

	pageElements = append(pageElements, layout.Expanded(pg.dropdownLayout))

	// display tx dropdown if selected wallet is ready and showLoader is false
	if !pg.walletNotReady() && !pg.showLoader {
		pg.ParentWindow().Reload() //refresh UI to display updated txType dropdown
	}

	return layout.Stack{}.Layout(gtx, pageElements...)
}

func (pg *TransactionsPage) dropdownLayout(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: values.MarginPadding40}.Layout(gtx, pg.rightDropdown)
		}),
		layout.Expanded(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return pg.leftDropdown(gtx)
		}),
	)
}

func (pg *TransactionsPage) leftDropdown(gtx C) D {
	showOverlay := pg.walletNotReady() && pg.multiWalletLayout
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if pg.isShowTitle && pg.IsMobileView() {
				lbl := pg.Theme.Label(values.TextSize16, values.String(values.StrTransactions))
				lbl.Font.Weight = font.Bold
				return layout.Inset{Top: values.MarginPadding4}.Layout(gtx, lbl.Layout)
			}
			if pg.walletDropDown == nil {
				return D{}
			}
			return layout.W.Layout(gtx, pg.walletDropDown.Layout)

		}),
		layout.Rigid(func(gtx C) D {
			if showOverlay {
				return D{}
			}
			icon := pg.Theme.Icons.FilterOffImgIcon
			if pg.isFilterOpen {
				icon = pg.Theme.Icons.FilterImgIcon
			}
			return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						margin := values.MarginPadding20
						if pg.IsMobileView() {
							margin = values.MarginPadding12
						}
						return layout.Inset{Right: margin}.Layout(gtx, func(gtx C) D {
							return pg.buttonWrap(gtx, pg.filterBtn, icon, values.String(values.StrFilter))
						})
					}),
					layout.Rigid(func(gtx C) D {
						// TODO: Enable on mobile
						if pg.IsMobileView() {
							return D{}
						}
						return pg.buttonWrap(gtx, pg.exportBtn, pg.Theme.Icons.ShareIcon, values.String(values.StrExport))
					}),
				)
			})
		}),
	)
}

func (pg *TransactionsPage) buttonWrap(gtx C, clickable *cryptomaterial.Clickable, icon *cryptomaterial.Image, title string) D {
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     cryptomaterial.WrapContent,
		Background: pg.Theme.Color.DefaultThemeColors().SurfaceHighlight,
		Clickable:  clickable,
		Alignment:  layout.Middle,
	}.Layout(gtx,
		layout.Rigid(icon.Layout16dp),
		layout.Rigid(func(gtx C) D {
			if pg.IsMobileView() {
				return D{}
			}
			lbl := pg.Theme.Label(pg.ConvertTextSize(values.TextSize14), title)
			lbl.Font.Weight = font.Bold
			lbl.Color = pg.Theme.Color.GrayText1
			return layout.Inset{Left: values.MarginPadding2}.Layout(gtx, lbl.Layout)
		}),
	)
}

func (pg *TransactionsPage) rightDropdown(gtx C) D {
	if !pg.isFilterOpen {
		return D{}
	}
	return layout.E.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(pg.statusDropDown.Layout),
			layout.Rigid(pg.orderDropDown.Layout),
		)
	})
}

func (pg *TransactionsPage) txListLayout(gtx C) D {
	pg.scroll.OnScrollChangeListener(pg.ParentWindow())
	txListWidget := func(gtx C) D {
		marginTop := values.MarginPadding50
		if pg.isFilterOpen {
			marginTop = values.MarginPadding80
		}
		return layout.Inset{Top: marginTop}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if !pg.isFilterOpen {
						return D{}
					}
					return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, pg.searchEditor.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					itemCount := pg.scroll.ItemsCount()
					card := pg.Theme.Card()
					// return "No transactions yet" text if there are no transactions
					if itemCount == 0 {
						padding := values.MarginPadding16
						txt := pg.Theme.Body1(values.String(values.StrNoTransactions))
						txt.TextSize = pg.ConvertTextSize(values.TextSize14)
						txt.Color = pg.Theme.Color.GrayText3
						return card.Layout(gtx, func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.Center.Layout(gtx, func(gtx C) D {
								return layout.Inset{Top: padding, Bottom: padding}.Layout(gtx, txt.Layout)
							})
						})
					}

					if itemCount == -1 || pg.showLoader {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Center.Layout(gtx, pg.materialLoader.Layout)
					}

					return pg.scroll.List().Layout(gtx, 1, func(gtx C, _ int) D {
						return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
							return card.Layout(gtx, func(gtx C) D {
								padding := values.MarginPaddingTransform(pg.IsMobileView(), values.MarginPadding16)
								return layout.UniformInset(padding).Layout(gtx, func(gtx C) D {
									wallTxs := pg.scroll.FetchedData()
									return pg.transactionList.Layout(gtx, len(wallTxs), func(gtx C, index int) D {
										tx, wal := pg.txAndWallet(wallTxs[index])
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												hideAssetInfo := pg.selectedWallet != nil
												return components.LayoutTransactionRow(gtx, pg.Load, wal, tx, hideAssetInfo)
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
													return layout.Inset{Left: values.MarginPadding32}.Layout(gtx, separator.Layout)
												})
											}),
										)
									})
								})
							})
						})
					})
				}),
			)
		})
	}

	showOverlay := pg.walletNotReady() && pg.multiWalletLayout
	if !showOverlay {
		return txListWidget(gtx)
	}

	gtx = gtx.Disabled()
	return layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Expanded(txListWidget),
		layout.Stacked(func(gtx C) D {
			gtx = gtx.Disabled()
			overlayColor := pg.Theme.Color.Gray3
			overlayColor.A = 220
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y - gtx.Dp(values.MarginPadding60)
			cryptomaterial.Fill(gtx, overlayColor)

			lbl := pg.Theme.Label(pg.ConvertTextSize(values.TextSize20), values.String(values.StrFunctionUnavailable))
			lbl.Font.Weight = font.SemiBold
			lbl.Color = pg.Theme.Color.PageNavText
			lbl.Alignment = text.Middle
			return cryptomaterial.CentralizeWidget(gtx, lbl.Layout)
		}),
	)
}

func (pg *TransactionsPage) txAndWallet(mtx *multiWalletTx) (*sharedW.Transaction, sharedW.Asset) {
	return mtx.Transaction, pg.AssetsManager.WalletWithID(mtx.walletID)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *TransactionsPage) HandleUserInteractions(gtx C) {
	if pg.statusDropDown.Changed(gtx) {
		go pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	}

	if pg.walletDropDown != nil && pg.walletDropDown.Changed(gtx) {
		assetIndex := pg.walletDropDown.SelectedIndex()
		// The "All Wallets" dropdown item is the first in the dropdown list.
		if assetIndex == 0 {
			pg.selectedWallet = nil
		} else {
			// Assets added as dropdown items have an index of
			// actuallyIndex+1 due to the "All Wallets" dropdown item.
			assetIndex--
			pg.selectedWallet = pg.assetWallets[assetIndex]
		}
		pg.refreshAvailableTxType()
		go pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	}

	if clicked, selectedItem := pg.transactionList.ItemClicked(); clicked {
		transactions := pg.scroll.FetchedData()
		tx, wal := pg.txAndWallet(transactions[selectedItem])
		pg.ParentNavigator().Display(NewTransactionDetailsPage(pg.Load, wal, tx))
	}

	dropDownList := []*cryptomaterial.DropDown{pg.statusDropDown}
	if pg.walletDropDown != nil {
		dropDownList = append(dropDownList, pg.walletDropDown)
	}
	cryptomaterial.DisplayOneDropdown(gtx, dropDownList...)

	if pg.txCategoryTab.Changed() {
		pg.selectedTxCategoryTab = pg.txCategoryTab.SelectedIndex()
		if pg.multiWalletLayout {
			pg.initWalletSelector()
		}

		if pg.walletDropDown != nil && pg.walletDropDown.SelectedIndex() <= 0 {
			pg.selectedWallet = nil
		}

		pg.refreshAvailableTxType()
		go pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	}

	if pg.filterBtn.Clicked(gtx) {
		pg.isFilterOpen = !pg.isFilterOpen
	}

	if pg.exportBtn.Clicked(gtx) {
		exportModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrExportTransaction)).
			Body(values.String(values.StrExportTransactionsMsg)).
			SetNegativeButtonText(values.String(values.StrCancel)).
			SetPositiveButtonText(values.String(values.StrExport)).
			SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
				assets := []sharedW.Asset{pg.selectedWallet}
				if pg.selectedWallet == nil {
					assets = pg.assetWallets
				}
				go func() {
					fileName := filepath.Join(pg.AssetsManager.RootDir(), "exports", fmt.Sprintf("transaction_export_%d.csv", time.Now().Unix()))
					err := exportTxs(assets, fileName)
					if err != nil {
						errModal := modal.NewErrorModal(pg.Load, fmt.Errorf("error exporting your wallet(s) transactions: %v", err).Error(), modal.DefaultClickFunc())
						pg.ParentWindow().ShowModal(errModal)
						return
					}

					infoModal := modal.NewSuccessModal(pg.Load, values.StringF(values.StrExportTransactionSuccessMsg, fileName), modal.DefaultClickFunc())
					pg.ParentWindow().ShowModal(infoModal)
				}()
				return true
			})
		pg.ParentWindow().ShowModal(exportModal)
	}

	if pg.orderDropDown.Changed(gtx) {
		pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	}

	for {
		event, ok := pg.searchEditor.Editor.Update(gtx)
		if !ok {
			break
		}

		if gtx.Source.Focused(pg.searchEditor.Editor) {
			switch event.(type) {
			case widget.ChangeEvent:
				pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
			}
		}
	}
}

func exportTxs(assets []sharedW.Asset, fileName string) error {
	if err := os.MkdirAll(filepath.Dir(fileName), utils.UserFilePerm); err != nil {
		return fmt.Errorf("os.MkdirAll error: %w", err)
	}

	var success bool
	defer func() {
		if !success {
			os.Remove(fileName)
		}
	}()

	f, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("os.Create error: %w", err)
	}
	defer f.Close()

	headers := []string{values.String(values.StrTime), values.String(values.StrHash), values.String(values.StrType), values.String(values.StrDirection), values.String(values.StrFee), values.String(values.StrAmount)}

	writer := csv.NewWriter(f)
	writer.UseCRLF = runtime.GOOS == "windows"
	err = writer.Write(headers)
	if err != nil {
		return fmt.Errorf("csv.Writer.Write error: %w", err)
	}

	for _, a := range assets {
		txs, err := a.GetTransactionsRaw(0, math.MaxInt32, utils.TxFilterAll, true, "")
		if err != nil {
			return fmt.Errorf("wallet.GetTransactionsRaw error: %w", err)
		}

		// Write txs to file.
		for _, tx := range txs {
			err := writer.Write([]string{
				time.Unix(tx.Timestamp, 0).String(),
				tx.Hash,
				tx.Type,
				txhelper.TxDirectionString(tx.Direction),
				a.ToAmount(tx.Fee).String(),
				a.ToAmount(tx.Amount).String(),
			})
			if err != nil {
				return fmt.Errorf("csv.Writer.Write error: %v", err)
			}

			writer.Flush()
			if err = writer.Error(); err != nil {
				return fmt.Errorf("csv.Writer error: %w", err)
			}
		}
	}

	success = true
	return nil
}

func (pg *TransactionsPage) listenForTxNotifications() {
	txAndBlockNotificationListener := &sharedW.TxAndBlockNotificationListener{
		OnTransaction: func(walletID int, _ *sharedW.Transaction) {
			// Listen for all new txs but ignore ntfns if the wallet sending the
			// ntfn is not the currently selected wallet.
			if pg.selectedWallet != nil && pg.selectedWallet.GetWalletID() != walletID {
				return // ignore tx
			}

			pg.scroll.FetchScrollData(false, pg.ParentWindow(), false)
		},
	}

	// Listen for ntfns for all wallets.
	for _, w := range pg.assetWallets {
		w.RemoveTxAndBlockNotificationListener(TransactionsPageID)
		err := w.AddTxAndBlockNotificationListener(txAndBlockNotificationListener, TransactionsPageID)
		if err != nil {
			log.Errorf("Error adding tx and block notification listener: %v", err)
			return
		}
	}
}

func (pg *TransactionsPage) stopTxNotificationsListener() {
	for _, w := range pg.assetWallets {
		w.RemoveTxAndBlockNotificationListener(TransactionsPageID)
	}
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
