package transaction

import (
	"context"
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"code.cryptopower.dev/group/cryptopower/app"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/listeners"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

const TransactionsPageID = "Transactions"

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

	*listeners.TxAndBlockNotificationListener
	ctx       context.Context // page context
	ctxCancel context.CancelFunc
	separator cryptomaterial.Line

	selectedTabIndex int

	txTypeDropDown  *cryptomaterial.DropDown
	transactionList *cryptomaterial.ClickableList
	container       *widget.List
	transactions    []sharedW.Transaction

	tabs *cryptomaterial.ClickableList

	materialLoader material.LoaderStyle

	loading, initialLoadingDone, loadedAll bool
}

func NewTransactionsPage(l *load.Load) *TransactionsPage {
	pg := &TransactionsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TransactionsPageID),
		container: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		separator:       l.Theme.Separator(),
		transactionList: l.Theme.NewClickableList(layout.Vertical),
	}

	pg.tabs = l.Theme.NewClickableList(layout.Horizontal)
	pg.tabs.IsHoverable = false

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
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.refreshAvailableTxType()
	pg.listenForTxNotifications()
	go pg.loadTransactions(false)
}

func (pg *TransactionsPage) sectionNavTab(gtx C) D {
	var selectedTabDims D

	return layout.Inset{
		Top: values.MarginPadding20,
	}.Layout(gtx, func(gtx C) D {
		return pg.tabs.Layout(gtx, len(txTabs), func(gtx C, i int) D {
			return layout.Stack{Alignment: layout.S}.Layout(gtx,
				layout.Stacked(func(gtx C) D {
					return layout.Inset{
						Right:  values.MarginPadding24,
						Bottom: values.MarginPadding14,
					}.Layout(gtx, func(gtx C) D {
						return layout.Center.Layout(gtx, func(gtx C) D {
							lbl := pg.Theme.Label(values.TextSize16, txTabs[i])
							lbl.Color = pg.Theme.Color.GrayText1
							if i == pg.selectedTabIndex {
								lbl.Color = pg.Theme.Color.Primary
								selectedTabDims = lbl.Layout(gtx)
							}

							return lbl.Layout(gtx)
						})
					})
				}),
				layout.Stacked(func(gtx C) D {
					if i != pg.selectedTabIndex {
						return D{}
					}

					tabHeight := gtx.Dp(values.MarginPadding2)
					tabRect := image.Rect(0, 0, selectedTabDims.Size.X, tabHeight)

					return layout.Inset{
						Left: values.MarginPaddingMinus22,
					}.Layout(gtx, func(gtx C) D {
						paint.FillShape(gtx.Ops, pg.Theme.Color.Primary, clip.Rect(tabRect).Op())
						return layout.Dimensions{
							Size: image.Point{X: selectedTabDims.Size.X, Y: tabHeight},
						}
					})
				}),
			)
		})
	})
}

func (pg *TransactionsPage) pageTitle(gtx C) D {
	txt := pg.Theme.Label(values.TextSize20, values.String(values.StrTransactions))
	txt.Font.Weight = text.SemiBold
	return txt.Layout(gtx)
}

func (pg *TransactionsPage) refreshAvailableTxType() {
	wal := pg.WL.SelectedWallet.Wallet
	items := []cryptomaterial.DropDownItem{}
	_, keysinfo := components.TxPageDropDownFields(wal.GetAssetType(), pg.selectedTabIndex)
	for _, name := range keysinfo {
		item := cryptomaterial.DropDownItem{}
		item.Text = name
		items = append(items, item)
	}

	pg.txTypeDropDown = pg.Theme.DropDown(items, values.TxDropdownGroup, 2)

	go func() {
		var countfn = func(fType int32) int {
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
	}()

}

func (pg *TransactionsPage) loadTransactions(loadMore bool) {
	wal := pg.WL.SelectedWallet.Wallet
	mapinfo, _ := components.TxPageDropDownFields(wal.GetAssetType(), pg.selectedTabIndex)
	if len(mapinfo) < 1 {
		log.Warnf("asset type(%v) and tab index(%d) found", wal.GetAssetType(), pg.selectedTabIndex)
		return
	}

	selectedVal, _, _ := strings.Cut(pg.txTypeDropDown.Selected(), " ")
	txFilter, ok := mapinfo[selectedVal]
	if !ok {
		log.Warnf("unsupported field(%v) for asset type(%v) and tab index(%d) found",
			selectedVal, wal.GetAssetType(), pg.selectedTabIndex)
		return
	}

	if pg.loading {
		return
	}

	pg.loading = true

	limit := 20

	offset := 0
	if loadMore {
		offset = len(pg.transactions)
	}

	tempTxs, err := pg.WL.SelectedWallet.Wallet.GetTransactionsRaw(int32(offset), int32(limit), txFilter, true)
	if err != nil {
		// log error and return an empty list.
		log.Errorf("Error loading transactions: %v", err)
		pg.transactions = nil
		return
	}

	pg.initialLoadingDone = true

	if len(tempTxs) == 0 {
		pg.loadedAll = true
		pg.loading = false

		if !loadMore {
			pg.transactions = nil
		}
		return
	}

	if len(tempTxs) < limit {
		pg.loadedAll = true
	}

	if loadMore {
		pg.transactions = append(pg.transactions, tempTxs...)
	} else {
		pg.transactions = tempTxs
	}

	pg.loading = false
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
	pg.onScrollChangeListener()

	return components.UniformPadding(gtx, func(gtx C) D {
		txlisingView := layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding0}.Layout(gtx, func(gtx C) D {
				wallTxs := pg.transactions
				return layout.Stack{Alignment: layout.N}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding60,
						}.Layout(gtx, func(gtx C) D {
							return pg.Theme.List(pg.container).Layout(gtx, 1, func(gtx C, i int) D {
								return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
									return pg.Theme.Card().Layout(gtx, func(gtx C) D {

										if pg.loading {
											gtx.Constraints.Min.X = gtx.Constraints.Max.X
											return layout.Center.Layout(gtx, func(gtx C) D {
												return pg.materialLoader.Layout(gtx)
											})
										}

										// return "No transactions yet" text if there are no transactions
										if len(wallTxs) == 0 {
											padding := values.MarginPadding16
											txt := pg.Theme.Body1(values.String(values.StrNoTransactions))
											txt.Color = pg.Theme.Color.GrayText3
											gtx.Constraints.Min.X = gtx.Constraints.Max.X
											return layout.Center.Layout(gtx, func(gtx C) D {
												return layout.Inset{Top: padding, Bottom: padding}.Layout(gtx, txt.Layout)
											})
										}

										return pg.transactionList.Layout(gtx, len(wallTxs), func(gtx C, index int) D {
											var row = components.TransactionRow{
												Transaction: wallTxs[index],
												Index:       index,
											}

											return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													return components.LayoutTransactionRow(gtx, pg.Load, row)
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
										})
									})
								})
							})
						})
					}),
					layout.Expanded(func(gtx C) D {
						return pg.txTypeDropDown.Layout(gtx, 0, true)
					}),
				)
			})
		})

		items := []layout.FlexChild{layout.Rigid(pg.pageTitle)}
		if pg.WL.SelectedWallet.Wallet.GetAssetType() == utils.DCRWalletAsset {
			// Layouts only supportted by DCR
			line := pg.Theme.Separator()
			line.Color = pg.Theme.Color.Gray3
			items = append(items, layout.Rigid(line.Layout))
			items = append(items, layout.Rigid(pg.sectionNavTab))
		}
		items = append(items, txlisingView)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
	})
}

func (pg *TransactionsPage) layoutMobile(gtx layout.Context) layout.Dimensions {
	container := func(gtx C) D {
		wallTxs := pg.transactions
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Stack{Alignment: layout.N}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding60,
						}.Layout(gtx, func(gtx C) D {
							return pg.Theme.List(pg.container).Layout(gtx, 1, func(gtx C, i int) D {
								return pg.Theme.Card().Layout(gtx, func(gtx C) D {

									// return "No transactions yet" text if there are no transactions
									if len(wallTxs) == 0 {
										padding := values.MarginPadding16
										txt := pg.Theme.Body1(values.String(values.StrNoTransactions))
										txt.Color = pg.Theme.Color.GrayText3
										gtx.Constraints.Min.X = gtx.Constraints.Max.X
										return layout.Center.Layout(gtx, func(gtx C) D {
											return layout.Inset{Top: padding, Bottom: padding}.Layout(gtx, txt.Layout)
										})
									}

									return pg.transactionList.Layout(gtx, len(wallTxs), func(gtx C, index int) D {
										var row = components.TransactionRow{
											Transaction: wallTxs[index],
											Index:       index,
										}

										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												return components.LayoutTransactionRow(gtx, pg.Load, row)
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
		go pg.loadTransactions(false)
		break
	}

	if clicked, selectedItem := pg.transactionList.ItemClicked(); clicked {
		pg.ParentNavigator().Display(NewTransactionDetailsPage(pg.Load, &pg.transactions[selectedItem], false))
	}
	cryptomaterial.DisplayOneDropdown(pg.txTypeDropDown)

	if tabItemClicked, clickedTabIndex := pg.tabs.ItemClicked(); tabItemClicked {
		pg.selectedTabIndex = clickedTabIndex
		pg.refreshAvailableTxType()
		go pg.loadTransactions(false)
	}
}

func (pg *TransactionsPage) listenForTxNotifications() {
	if pg.TxAndBlockNotificationListener != nil {
		return
	}
	pg.TxAndBlockNotificationListener = listeners.NewTxAndBlockNotificationListener()
	err := pg.WL.SelectedWallet.Wallet.AddTxAndBlockNotificationListener(pg.TxAndBlockNotificationListener, true, TransactionsPageID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-pg.TxAndBlockNotifChan:
				if n.Type == listeners.NewTransaction {
					pg.loadTransactions(false)
					pg.ParentWindow().Reload()
				}
			case <-pg.ctx.Done():
				pg.WL.SelectedWallet.Wallet.RemoveTxAndBlockNotificationListener(TransactionsPageID)
				close(pg.TxAndBlockNotifChan)
				pg.TxAndBlockNotificationListener = nil

				return
			}
		}
	}()
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *TransactionsPage) OnNavigatedFrom() {
	pg.ctxCancel()
}

func (pg *TransactionsPage) onScrollChangeListener() {
	if len(pg.transactions) < 5 || !pg.initialLoadingDone {
		return
	}

	// The first check is for when the list is scrolled to the bottom using the scroll bar.
	// The second check is for when the list is scrolled to the bottom using the mouse wheel.
	// OffsetLast is 0 if we've scrolled to the last item on the list. Position.Length > 0
	// is to check if the page is still scrollable.
	if (pg.container.List.Position.OffsetLast >= -50 && pg.container.List.Position.BeforeEnd) || (pg.container.List.Position.OffsetLast == 0 && pg.container.List.Position.Length > 0) {
		if !pg.loadedAll {
			pg.loadTransactions(true)
		}
	}
}
