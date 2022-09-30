package transaction

import (
	"context"
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"

	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet/wallets/dcr"
	"gitlab.com/raedah/cryptopower/listeners"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/values"
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

	selectedCategoryIndex int
	selectedTabIndex      int
	changed               bool

	txTypeDropDown  *cryptomaterial.DropDown
	transactionList *cryptomaterial.ClickableList
	container       *widget.List
	transactions    []dcr.Transaction
	wallets         []*dcr.Wallet

	tabs *cryptomaterial.ClickableList
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
	pg.loadTransactions()
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
	// todo optimize
	txCount, _ := pg.WL.SelectedWallet.Wallet.CountTransactions(dcr.TxFilterAll)
	sentTxCount, _ := pg.WL.SelectedWallet.Wallet.CountTransactions(dcr.TxFilterSent)
	receivedTxCount, _ := pg.WL.SelectedWallet.Wallet.CountTransactions(dcr.TxFilterReceived)
	transferredTxCount, _ := pg.WL.SelectedWallet.Wallet.CountTransactions(dcr.TxFilterTransferred)
	mixedTxCount, _ := pg.WL.SelectedWallet.Wallet.CountTransactions(dcr.TxFilterMixed)
	stakingTxCount, _ := pg.WL.SelectedWallet.Wallet.CountTransactions(dcr.TxFilterStaking)

	items := []cryptomaterial.DropDownItem{
		{
			Text: fmt.Sprintf("%s (%d)", values.String(values.StrAll), txCount),
		},
		{
			Text: fmt.Sprintf("%s (%d)", values.String(values.StrSent), sentTxCount),
		},
		{
			Text: fmt.Sprintf("%s (%d)", values.String(values.StrReceived), receivedTxCount),
		},
		{
			Text: fmt.Sprintf("%s (%d)", values.String(values.StrTransferred), transferredTxCount),
		},
		{
			Text: fmt.Sprintf("%s (%d)", values.String(values.StrMixed), mixedTxCount),
		},
		{
			Text: fmt.Sprintf("%s (%d)", values.String(values.StrStaking), stakingTxCount),
		},
	}

	if pg.selectedTabIndex == 1 {
		items = []cryptomaterial.DropDownItem{
			{
				Text: values.String(values.StrAll),
			},
			{
				Text: values.String(values.StrVote),
			},
			{
				Text: values.String(values.StrRevocation),
			},
		}
	}

	pg.txTypeDropDown = pg.Theme.DropDown(items, values.TxDropdownGroup, 2)
}

func (pg *TransactionsPage) loadTransactions() {
	var txFilter int32

	switch pg.txTypeDropDown.SelectedIndex() {
	case 1:
		txFilter = dcr.TxFilterSent
	case 2:
		txFilter = dcr.TxFilterReceived
	case 3:
		txFilter = dcr.TxFilterTransferred
	case 4:
		txFilter = dcr.TxFilterMixed
	case 5:
		txFilter = dcr.TxFilterStaking
	default:
		txFilter = dcr.TxFilterAll
	}

	if pg.selectedTabIndex == 1 {
		switch pg.txTypeDropDown.SelectedIndex() {
		case 1:
			txFilter = dcr.TxFilterVoted
		case 2:
			txFilter = dcr.TxFilterRevoked
		default:
			txFilter = dcr.TxFilterStaking
		}
	}

	txns := make([]dcr.Transaction, 0)
	txs, err := pg.WL.SelectedWallet.Wallet.GetTransactionsRaw(0, 0, txFilter, true)
	if err != nil {
		// log error and return an empty list.
		log.Errorf("Error loading transactions: %v", err)
		pg.transactions = txns
	} else {
		// remove revoked tickets from staking and all transactions filter
		if txFilter == dcr.TxFilterStaking || txFilter == dcr.TxFilterStaking ||
			txFilter == dcr.TxFilterAll {
			for _, txn := range txs {
				if txn.Type != dcr.TxTypeTicketPurchase /* || txn.Type == dcr.TxTypeVote */ {
					txns = append(txns, txn)
				}
			}
			pg.transactions = txns
		} else {
			pg.transactions = txs
		}
	}
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
	return components.UniformPadding(gtx, func(gtx C) D {
		line := pg.Theme.Separator()
		line.Color = pg.Theme.Color.Gray3
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.pageTitle),
			layout.Rigid(pg.sectionNavTab),
			layout.Rigid(line.Layout),
			layout.Flexed(1, func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					wallTxs := pg.transactions
					return layout.Stack{Alignment: layout.N}.Layout(gtx,
						layout.Expanded(func(gtx C) D {
							return layout.Inset{
								Top: values.MarginPadding60,
							}.Layout(gtx, func(gtx C) D {
								return pg.Theme.List(pg.container).Layout(gtx, 1, func(gtx C, i int) D {
									return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
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
							})
						}),
						layout.Expanded(func(gtx C) D {
							return pg.txTypeDropDown.Layout(gtx, 0, true)
						}),
					)
				})
			}),
		)
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
		pg.loadTransactions()
		break
	}

	if clicked, selectedItem := pg.transactionList.ItemClicked(); clicked {
		pg.ParentNavigator().Display(NewTransactionDetailsPage(pg.Load, &pg.transactions[selectedItem], false))
	}
	cryptomaterial.DisplayOneDropdown(pg.txTypeDropDown)

	if tabItemClicked, clickedTabIndex := pg.tabs.ItemClicked(); tabItemClicked {
		pg.selectedTabIndex = clickedTabIndex
		pg.refreshAvailableTxType()
		pg.loadTransactions()
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
					pg.loadTransactions()
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
