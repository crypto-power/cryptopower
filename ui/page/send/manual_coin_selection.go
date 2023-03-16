package send

import (
	"context"
	"fmt"
	"image/color"

	"code.cryptopower.dev/group/cryptopower/app"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	libutils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
)

const (
	ManualCoinSelectionPageID = "backup_success"

	// MaxAddressLen defines the maximum length of address characters displayed
	// on the UI.
	MaxAddressLen = 16
)

type AccountsUTXOInfo struct {
	Account    string
	Details    []*sharedW.UnspentOutput
	checkboxes []cryptomaterial.CheckBoxStyle
}

type ManualCoinSelectionPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	actionButton cryptomaterial.Button
	clearButton  cryptomaterial.Button

	selectedUTXOs cryptomaterial.Label
	txSize        cryptomaterial.Label
	totalAmount   cryptomaterial.Label

	accountsUTXOs     []*AccountsUTXOInfo
	UTXOList          *cryptomaterial.ClickableList
	fromCoinSelection *cryptomaterial.Clickable
	collapsibleList   []*cryptomaterial.Collapsible

	listContainer *widget.List
	utxosRow      *widget.List
}

type componentProperties struct {
	direction layout.Direction
	spacing   layout.Spacing
	weight    float32
}

func NewManualCoinSelectionPage(l *load.Load) *ManualCoinSelectionPage {
	pg := &ManualCoinSelectionPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ManualCoinSelectionPageID),

		actionButton: l.Theme.Button(values.String(values.StrDone)),
		clearButton:  l.Theme.OutlineButton("— " + values.String(values.StrClearSelection)),

		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		utxosRow: &widget.List{
			List: layout.List{Axis: layout.Horizontal},
		},
		UTXOList: l.Theme.NewClickableList(layout.Vertical),
	}
	pg.actionButton.Font.Weight = text.SemiBold
	pg.clearButton.Font.Weight = text.SemiBold
	pg.clearButton.Color = l.Theme.Color.Danger
	pg.clearButton.Inset = layout.UniformInset(values.MarginPadding3)
	c := l.Theme.Color.Danger
	// Background is 8% of the Danger color.
	alphaChan := (127 * 0.8)
	pg.clearButton.Background = color.NRGBA{c.R, c.G, c.B, uint8(alphaChan)}

	pg.selectedUTXOs = pg.Theme.Label(values.TextSize16, "--")
	pg.txSize = pg.Theme.Label(values.TextSize16, "--")
	pg.totalAmount = pg.Theme.Label(values.TextSize16, "--")

	pg.selectedUTXOs.Font.Weight = text.SemiBold
	pg.txSize.Font.Weight = text.SemiBold
	pg.totalAmount.Font.Weight = text.SemiBold

	pg.fromCoinSelection = pg.Theme.NewClickable(false)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ManualCoinSelectionPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	go func() {
		if err := pg.fetchAccountsInfo(); err != nil {
			log.Error(err)
		} else {
			// refresh the display to update the latest changes.
			pg.ParentWindow().Reload()
		}
	}()
}

func (pg *ManualCoinSelectionPage) fetchAccountsInfo() error {
	accounts, err := pg.WL.SelectedWallet.Wallet.GetAccountsRaw()
	if err != nil {
		return fmt.Errorf("querying the accounts names failed: %v", err)
	}

	pg.collapsibleList = make([]*cryptomaterial.Collapsible, 0, len(accounts.Accounts))
	pg.accountsUTXOs = make([]*AccountsUTXOInfo, 0, len(accounts.Accounts))
	for _, account := range accounts.Accounts {
		info, err := pg.WL.SelectedWallet.Wallet.UnspentOutputs(account.Number)
		if err != nil {
			return fmt.Errorf("querying the account (%v) info failed: %v", account.Number, err)
		}

		checkboxes := make([]cryptomaterial.CheckBoxStyle, len(info))
		// create checkboxes for all the utxo available.
		for i := range info {
			checkboxes[i] = pg.Theme.CheckBox(new(widget.Bool), "")
		}

		pg.accountsUTXOs = append(pg.accountsUTXOs, &AccountsUTXOInfo{
			Details:    info,
			Account:    account.Name,
			checkboxes: checkboxes,
		})

		collapsible := pg.Theme.Collapsible()
		collapsible.IconPosition = cryptomaterial.Before
		collapsible.IconStyle = cryptomaterial.Caret
		pg.collapsibleList = append(pg.collapsibleList, collapsible)
	}

	return nil
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ManualCoinSelectionPage) HandleUserInteractions() {
	if pg.actionButton.Clicked() {
		pg.ParentWindow().Display(NewSendPage(pg.Load))
	}

	if pg.fromCoinSelection.Clicked() {
		pg.ParentNavigator().Display(NewSendPage(pg.Load))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *ManualCoinSelectionPage) OnNavigatedFrom() {
	pg.ctxCancel()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ManualCoinSelectionPage) Layout(gtx C) D {
	return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.MatchParent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.MatchParent,
					Height:      cryptomaterial.MatchParent,
					Orientation: layout.Vertical,
				}.Layout(gtx,
					layout.Rigid(pg.topSection),
					layout.Rigid(pg.summarySection),
					layout.Rigid(pg.accountListSection),
				)
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Inset{
					Top: values.MarginPadding5,
				}.Layout(gtx, func(gtx C) D {
					return layout.E.Layout(gtx, pg.actionButton.Layout)
				})
			}),
		)
	})
}

func (pg *ManualCoinSelectionPage) topSection(gtx C) D {
	return layout.Inset{Bottom: values.MarginPadding14}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.WrapContent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Horizontal,
				Alignment:   layout.Start,
				Clickable:   pg.fromCoinSelection,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Top:   values.MarginPadding8,
						Right: values.MarginPadding6,
					}.Layout(gtx, func(gtx C) D {
						return pg.Theme.Icons.ChevronLeft.LayoutSize(gtx, values.MarginPadding8)
					})
				}),
				layout.Rigid(pg.Theme.H6(values.String(values.StrSelectUTXO)).Layout),
			)
		})
	})
}

func (pg *ManualCoinSelectionPage) summarySection(gtx C) D {
	return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			topContainer := layout.UniformInset(values.MarginPadding15)
			return topContainer.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
							textLabel := pg.Theme.Label(values.TextSize16, values.String(values.StrSummary))
							textLabel.Font.Weight = text.SemiBold
							return textLabel.Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(0.3, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrSelectedUTXO)+": ").Layout),
									layout.Flexed(1, func(gtx C) D {
										return layout.W.Layout(gtx, pg.selectedUTXOs.Layout)
									}),
								)
							}),
							layout.Flexed(0.3, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(pg.Theme.Label(values.TextSize16, values.StringF(values.StrTxSize, " : ")).Layout),
									layout.Flexed(1, func(gtx C) D {
										return layout.W.Layout(gtx, pg.txSize.Layout)
									}),
								)
							}),
							layout.Flexed(0.3, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrTotalAmount)+": ").Layout),
									layout.Flexed(1, func(gtx C) D {
										return layout.W.Layout(gtx, pg.totalAmount.Layout)
									}),
								)
							}),
						)
					}),
				)
			})
		})
	})
}

func (pg *ManualCoinSelectionPage) accountListSection(gtx C) D {
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X

		return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					textLabel := pg.Theme.Label(values.TextSize16, values.String(values.StrAccountList))
					textLabel.Font.Weight = text.SemiBold
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(textLabel.Layout),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, pg.clearButton.Layout)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.Theme.List(pg.listContainer).Layout(gtx, len(pg.accountsUTXOs), func(gtx C, i int) D {
						return layout.Inset{
							Left:   values.MarginPadding5,
							Bottom: values.MarginPadding15,
						}.Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									collapsibleHeader := func(gtx C) D {
										t := pg.Theme.Label(values.TextSize16, pg.accountsUTXOs[i].Account)
										t.Font.Weight = text.SemiBold
										return t.Layout(gtx)
									}

									collapsibleBody := func(gtx C) D {
										if len(pg.accountsUTXOs[i].Details) == 0 {
											gtx.Constraints.Min.X = gtx.Constraints.Max.X
											return layout.Center.Layout(gtx,
												pg.Theme.Label(values.TextSize14, values.String(values.StrNoUTXOs)).Layout,
											)
										}
										return pg.accountListItemsSection(gtx, pg.accountsUTXOs[i].Details, pg.accountsUTXOs[i].checkboxes)
									}

									return pg.collapsibleList[i].Layout(gtx, collapsibleHeader, collapsibleBody)
								}),
							)
						})
					})
				}),
			)
		})
	})
}

func (pg *ManualCoinSelectionPage) genLabel(txt interface{}, isTitle bool) cryptomaterial.Label {
	txtStr := ""
	switch n := txt.(type) {
	case string:
		txtStr = n
	case float64:
		txtStr = fmt.Sprintf("%0.4f", n) // to 4 decimal places
	case int32, int, int64:
		txtStr = fmt.Sprintf("%d", n)
	}

	if len(txtStr) > MaxAddressLen {
		txtStr = txtStr[:MaxAddressLen] + "..."
	}

	lb := pg.Theme.Label(values.TextSize14, txtStr)
	if isTitle {
		lb.Font.Weight = text.Bold
		lb.Color = pg.Theme.Color.Gray3
	}

	return lb
}

func (pg *ManualCoinSelectionPage) accountListItemsSection(gtx C, utxos []*sharedW.UnspentOutput, cb []cryptomaterial.CheckBoxStyle) D {
	properties := []componentProperties{
		{direction: layout.Center, weight: 0.10}, // Component 1
		{direction: layout.W, weight: 0.20},      // Component 2
		{direction: layout.Center, weight: 0.25}, // Component 3
		{direction: layout.Center, weight: 0.20}, // Component 4
		{direction: layout.Center, weight: 0.25}, // Component 5
	}
	return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				amountLabel := pg.genLabel(values.String(values.StrAmount), true)               // Component 2
				addresslabel := pg.genLabel(values.String(values.StrAddress), true)             // Component 3
				confirmationsLabel := pg.genLabel(values.String(values.StrConfirmations), true) // component 4
				dateLabel := pg.genLabel(values.String(values.StrDateCreated), true)            // component 5

				return pg.rowItemsSection(gtx, properties, nil, amountLabel, addresslabel, confirmationsLabel, dateLabel)
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X

				return pg.UTXOList.Layout(gtx, len(utxos), func(gtx C, index int) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
								v := utxos[index]
								checkButton := &cb[index]                                                          // Component 1
								amountLabel := pg.genLabel(v.Amount.ToCoin(), false)                               // component 2
								addresslabel := pg.genLabel(v.Address, false)                                      // Component 3
								confirmationsLabel := pg.genLabel(v.Confirmations, false)                          // Component 4
								dateLabel := pg.genLabel(libutils.FormatUTCShortTime(v.ReceiveTime.Unix()), false) // Component 5

								addresslabel.Color = pg.Theme.Color.Primary

								return pg.rowItemsSection(gtx, properties, checkButton, amountLabel, addresslabel, confirmationsLabel, dateLabel)
							})
						}),
						layout.Rigid(func(gtx C) D {
							// No divider for last row
							if index == len(utxos)-1 {
								D{}
							}
							return pg.Theme.Separator().Layout(gtx)
						}),
					)
				})
			}),
		)
	})
}

func (pg *ManualCoinSelectionPage) rowItemsSection(gtx C, properties []componentProperties, components ...interface{}) D {
	max := float32(gtx.Constraints.Max.X)
	rowItems := make([]layout.Widget, len(components))

	for i, c := range components {
		var widget layout.Widget
		switch n := c.(type) {
		case *cryptomaterial.CheckBoxStyle:
			widget = n.Layout
		case cryptomaterial.Label:
			widget = n.Layout
		default:
			// create a default placeholder widget.
			widget = pg.Theme.Label(values.TextSize10, "").Layout
		}
		// Altering this variable makes flex to overwrite on the same variable
		// causing duplicaton of the last variable.
		w := layout.Rigid(widget)
		rowItems[i] = func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, w)
		}
	}

	return pg.Theme.List(pg.utxosRow).Layout(gtx, len(components), func(gtx C, index int) D {
		c := properties[index]
		gtx.Constraints.Min.X = int(max * c.weight)
		return c.direction.Layout(gtx, func(gtx C) D {
			return rowItems[index](gtx)
		})
	})
}
