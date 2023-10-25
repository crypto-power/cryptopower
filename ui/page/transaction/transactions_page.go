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

	txTypeDropDown   *cryptomaterial.DropDown
	transactionList  *cryptomaterial.ClickableList
	previousTxFilter int32
	scroll           *components.Scroll[*sharedW.Transaction]

	selectedWallet *load.WalletMapping

	tab *cryptomaterial.SegmentedControl

	materialLoader material.LoaderStyle

	sourceWalletSelector *components.WalletAndAccountSelector
	assetWallets         []sharedW.Asset

	isHomepageLayout,
	showAssetType,
	showLoader bool
}

func NewTransactionsPage(l *load.Load, isHomepageLayout bool) *TransactionsPage {
	pg := &TransactionsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TransactionsPageID),
		separator:        l.Theme.Separator(),
		transactionList:  l.Theme.NewClickableList(layout.Vertical),
		isHomepageLayout: isHomepageLayout,
	}

	pg.tab = l.Theme.SegmentedControl(txTabs)

	// init the asset selector
	if isHomepageLayout {
		pg.initWalletSelector()
		pg.showAssetType = true
	} else {
		pg.selectedWallet = &load.WalletMapping{
			Asset: l.WL.SelectedWallet.Wallet,
		}
	}

	items := []cryptomaterial.DropDownItem{}
	_, keysinfo := components.TxPageDropDownFields(pg.selectedWallet.GetAssetType(), pg.selectedTabIndex)
	for _, name := range keysinfo {
		item := cryptomaterial.DropDownItem{}
		item.Text = name
		items = append(items, item)
	}

	pg.txTypeDropDown = pg.Theme.DropDown(items, values.TxDropdownGroup, 2)

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
	if !pg.selectedWallet.IsSynced() {
		// Events are disabled until the wallet is fully synced.
		return
	}

	pg.listenForTxNotifications() // tx ntfn listener is stopped in OnNavigatedFrom().
	go pg.scroll.FetchScrollData(false, pg.ParentWindow())
}

// initWalletSelector is used by the tx history page, for wallet selection.
func (pg *TransactionsPage) initWalletSelector() {
	// initialize wallet selector
	pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load)
	// reinitialize wallet selector when the page tab changes staking activities
	// this is to prevent wallet selector indexing error, as staking is only
	// supported by decred wallet
	if pg.tab.SelectedSegment() != values.String(values.StrTxOverview) {
		pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, utils.DCRWalletAsset)
	}
	pg.sourceWalletSelector.Title(values.String(values.StrSelectWallet)).
		EnableWatchOnlyWallets(true)
	pg.selectedWallet = pg.sourceWalletSelector.SelectedWallet()

	// Source wallet picker
	pg.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.selectedWallet = selectedWallet
		go pg.refreshAvailableTxType()
		go pg.scroll.FetchScrollData(false, pg.ParentWindow())
	})
}

func (pg *TransactionsPage) sectionNavTab(gtx C) D {
	return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, pg.tab.Layout)
}

func (pg *TransactionsPage) pageTitle(gtx C) D {
	txt := pg.Theme.Label(values.TextSize20, values.String(values.StrTransactions))
	txt.Font.Weight = font.SemiBold
	return txt.Layout(gtx)
}

func (pg *TransactionsPage) refreshAvailableTxType() {
	pg.showLoader = true
	wal := pg.selectedWallet
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
func (pg *TransactionsPage) Layout(gtx layout.Context) layout.Dimensions {
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *TransactionsPage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	pg.scroll.OnScrollChangeListener(pg.ParentWindow())
	wal := pg.WL.SelectedWallet.Wallet

	txlisingView := layout.Flexed(1, func(gtx C) D {
		return layout.Inset{Top: values.MarginPadding0}.Layout(gtx, func(gtx C) D {
			return layout.Stack{Alignment: layout.N}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return layout.Inset{
						Top: values.MarginPadding60,
					}.Layout(gtx, func(gtx C) D {
						return pg.scroll.List().Layout(gtx, 1, func(gtx C, i int) D {
							return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
								return pg.Theme.Card().Layout(gtx, func(gtx C) D {
									return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
										if pg.scroll.ItemsCount() == -1 {
											gtx.Constraints.Min.X = gtx.Constraints.Max.X
											return layout.Center.Layout(gtx, func(gtx C) D {
												return pg.materialLoader.Layout(gtx)
											})
										}

										// return "No transactions yet" text if there are no transactions
										if pg.scroll.ItemsCount() == 0 {
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
														return layout.Dimensions{}
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

								// return "No transactions yet" text if there are no transactions
								if pg.scroll.ItemsCount() == 0 {
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
											return components.LayoutTransactionRow(gtx, pg.Load, pg.selectedWallet, row, true)
										}),
										layout.Rigid(func(gtx C) D {
											// No divider for last row
											if row.Index == len(wallTxs)-1 {
												return layout.Dimensions{}
											}

											return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													return components.LayoutTransactionRow(gtx, pg.Load, row, true, pg.showAssetType)
												}),
												layout.Rigid(func(gtx C) D {
													// No divider for last row
													if row.Index == len(wallTxs)-1 {
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
				}),
				layout.Expanded(func(gtx C) D {
					return pg.txTypeDropDown.Layout(gtx, 0, true)
				}),
				layout.Expanded(func(gtx C) D {
					if pg.isHomepageLayout {
						gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding280)
						gtx.Constraints.Max.Y = gtx.Dp(values.MarginPadding50)
						return pg.sourceWalletSelector.Layout(pg.ParentWindow(), gtx)
					}
					return D{}
				}),
			)
		})
	})

	items := []layout.FlexChild{}
	if pg.selectedWallet.GetAssetType() == utils.DCRWalletAsset {
		// Layouts only supportted by DCR
		items = append(items, layout.Rigid(pg.sectionNavTab))
	}
	items = append(items, txlisingView)
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, items...)
}

func (pg *TransactionsPage) layoutMobile(gtx layout.Context) layout.Dimensions {
	wal := pg.WL.SelectedWallet.Wallet
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

	if clicked, selectedItem := pg.transactionList.ItemClicked(); clicked {
		transactions := pg.scroll.FetchedData()
		pg.ParentNavigator().Display(NewTransactionDetailsPage(pg.Load, pg.WL.SelectedWallet.Wallet, transactions[selectedItem], false))
	}
	cryptomaterial.DisplayOneDropdown(pg.txTypeDropDown)

	if pg.tab.Changed() {
		pg.selectedTabIndex = pg.tab.SelectedIndex()
		pg.initWalletSelector()
		pg.refreshAvailableTxType()
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
