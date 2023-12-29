package send

import (
	"context"
	"fmt"
	"sort"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	ManualCoinSelectionPageID = "manual_coin_selection"

	// MaxAddressLen defines the maximum length of address characters displayed
	// on the UI.
	MaxAddressLen = 16
)

// UTXOInfo defines a utxo record associated with a specific row in the table view.
type UTXOInfo struct {
	*sharedW.UnspentOutput
	checkbox    cryptomaterial.CheckBoxStyle
	addressCopy *cryptomaterial.Clickable
}

type AccountUTXOInfo struct {
	Account string
	Details []*UTXOInfo
}

type ManualCoinSelectionPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
	// modalLayout is initialized if this page will be displayed as a modal
	// rather than a full page. A modal display is used and a wallet selector is
	// displayed if this coin selection page is opened from the send page modal.
	modalLayout *cryptomaterial.Modal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	actionButton cryptomaterial.Button
	clearButton  cryptomaterial.Button

	selectedUTXOs cryptomaterial.Label
	txSize        cryptomaterial.Label
	totalAmount   cryptomaterial.Label

	selectedUTXOrows []*sharedW.UnspentOutput
	selectedAmount   float64

	amountLabel        labelCell
	addressLabel       labelCell
	confirmationsLabel labelCell
	dateLabel          labelCell

	// UTXO table sorting buttons. Clickable used because button doesn't support
	// cryptomaterial.Image Icons.
	amountClickable        *cryptomaterial.Clickable
	addressClickable       *cryptomaterial.Clickable
	confirmationsClickable *cryptomaterial.Clickable
	dateClickable          *cryptomaterial.Clickable

	accountUTXOs       AccountUTXOInfo
	UTXOList           *cryptomaterial.ClickableList
	fromCoinSelection  *cryptomaterial.Clickable
	accountCollapsible *cryptomaterial.Collapsible

	listContainer *widget.List
	utxosRow      *widget.List

	lastSortEvent Lastclicked
	clickables    []*cryptomaterial.Clickable
	addressCopy   []*cryptomaterial.Clickable
	properties    []componentProperties

	sortingInProgress bool
	strAssetType      string

	sendPage *Page
}

type componentProperties struct {
	direction layout.Direction
	weight    float32
}

type Lastclicked struct {
	clicked int
	count   int
}

type labelCell struct {
	clickable *cryptomaterial.Clickable
	label     cryptomaterial.Label
}

func NewManualCoinSelectionPage(l *load.Load, sendPage *Page) *ManualCoinSelectionPage {
	pg := &ManualCoinSelectionPage{
		Load:         l,
		actionButton: l.Theme.Button(values.String(values.StrDone)),
		clearButton:  l.Theme.OutlineButton("— " + values.String(values.StrClearSelection)),
		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		utxosRow: &widget.List{
			List: layout.List{Axis: layout.Horizontal},
		},
		UTXOList:    l.Theme.NewClickableList(layout.Vertical),
		addressCopy: make([]*cryptomaterial.Clickable, 0),
		sendPage:    sendPage,
	}

	if sendPage.modalLayout != nil {
		pg.modalLayout = l.Theme.ModalFloatTitle(values.String(values.StrCoinSelection), pg.IsMobileView())
		pg.GenericPageModal = pg.modalLayout.GenericPageModal
	} else {
		pg.GenericPageModal = app.NewGenericPageModal(ManualCoinSelectionPageID)
	}

	pg.actionButton.Font.Weight = font.SemiBold
	pg.clearButton.Font.Weight = font.SemiBold
	pg.clearButton.Color = l.Theme.Color.Danger
	pg.clearButton.Inset = layout.UniformInset(values.MarginPadding4)
	pg.clearButton.HighlightColor = cryptomaterial.GenHighlightColor(l.Theme.Color.Danger)
	pg.clearButton.TextSize = values.TextSizeTransform(l.IsMobileView(), values.TextSize16)

	pg.txSize = pg.Theme.Label(values.TextSize14, "--")
	pg.totalAmount = pg.Theme.Label(values.TextSize14, "--")
	pg.selectedUTXOs = pg.Theme.Label(values.TextSize14, "--")

	pg.txSize.Font.Weight = font.SemiBold
	pg.totalAmount.Font.Weight = font.SemiBold
	pg.selectedUTXOs.Font.Weight = font.SemiBold

	pg.fromCoinSelection = pg.Theme.NewClickable(false)

	pg.amountClickable = pg.Theme.NewClickable(true)
	pg.addressClickable = pg.Theme.NewClickable(true)
	pg.confirmationsClickable = pg.Theme.NewClickable(true)
	pg.dateClickable = pg.Theme.NewClickable(false)

	pg.strAssetType = sendPage.selectedWallet.GetAssetType().String()
	name := fmt.Sprintf("%v(%v)", values.String(values.StrAmount), pg.strAssetType)

	// UTXO table view titles.
	pg.amountLabel = pg.generateLabel(name, pg.amountClickable)                                                 // Component 2
	pg.addressLabel = pg.generateLabel(values.String(values.StrAddress), pg.addressClickable)                   // Component 3
	pg.confirmationsLabel = pg.generateLabel(values.String(values.StrConfirmations), pg.confirmationsClickable) // component 4
	pg.dateLabel = pg.generateLabel(values.String(values.StrDateCreated), pg.dateClickable)                     // component 5

	pg.accountCollapsible = pg.Theme.Collapsible()
	pg.accountCollapsible.IconPosition = cryptomaterial.Before
	pg.accountCollapsible.IconStyle = cryptomaterial.Caret

	// properties describes the spacing constants set for the display of UTXOs.
	pg.properties = []componentProperties{
		{direction: layout.Center, weight: 0.1}, // Component 1
		{direction: layout.E, weight: 0.17},     // Component 2
		{direction: layout.W, weight: 0.02},     // Spacing Column
		{direction: layout.W, weight: 0.26},     // Component 3
		{direction: layout.W, weight: 0.005},    // Spacing Column
		{direction: layout.E, weight: 0.18},     // Component 4
		{direction: layout.W, weight: 0.02},     // Spacing Column
		{direction: layout.E, weight: 0.22},     // Component 5
	}

	// clickables defines the event handlers mapped to an individual title field.
	pg.clickables = []*cryptomaterial.Clickable{
		pg.amountClickable,        // Component 2
		pg.addressClickable,       // Component 3
		pg.confirmationsClickable, // Component 4
		pg.dateClickable,          // Component 5
	}

	pg.initializeFields()

	return pg
}

func (pg *ManualCoinSelectionPage) initializeFields() {
	pg.lastSortEvent = Lastclicked{clicked: -1}
	pg.selectedUTXOrows = make([]*sharedW.UnspentOutput, 0)
	pg.selectedAmount = 0

	pg.selectedUTXOs.Text = "0"
	pg.txSize.Text = pg.computeUTXOsSize()
	pg.totalAmount.Text = "0 " + pg.strAssetType
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
	account := pg.sendPage.sourceAccountSelector.SelectedAccount()
	info, err := pg.sendPage.selectedWallet.UnspentOutputs(int32(account.AccountNumber))
	if err != nil {
		return fmt.Errorf("querying the account (%v) info failed: %v", account.AccountNumber, err)
	}

	previousUTXOs := make(map[string]struct{}, 0)
	// Use the previous Selection of UTXO if same acccount source has been used.
	if account == pg.sendPage.selectedUTXOs.sourceAccount {
		for _, utxo := range pg.sendPage.selectedUTXOs.selectedUTXOs {
			previousUTXOs[utxo.TxID] = struct{}{}
			pg.selectedAmount += utxo.Amount.ToCoin()
		}
		pg.selectedUTXOrows = pg.sendPage.selectedUTXOs.selectedUTXOs
	}

	rowInfo := make([]*UTXOInfo, len(info))
	// create checkboxes and address copy components for all the utxos available.
	for i, row := range info {
		info := &UTXOInfo{
			UnspentOutput: row,
			checkbox:      pg.Theme.CheckBox(new(widget.Bool), ""),
			addressCopy:   pg.Theme.NewClickable(false),
		}

		info.checkbox.CheckBoxStyle.Size = 20
		// Check if TxID match. If true, set checked to true.
		_, info.checkbox.CheckBox.Value = previousUTXOs[info.TxID]

		rowInfo[i] = info
	}

	pg.accountUTXOs = AccountUTXOInfo{
		Details: rowInfo,
		Account: account.Name,
	}

	pg.accountCollapsible.SetExpanded(len(pg.selectedUTXOrows) > 0)
	pg.updateSummaryInfo()

	return nil
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ManualCoinSelectionPage) HandleUserInteractions() {
	if pg.actionButton.Clicked() {
		pg.sendPage.UpdateSelectedUTXOs(pg.selectedUTXOrows)
		if pg.modalLayout != nil {
			pg.modalLayout.Dismiss()
		} else {
			pg.ParentNavigator().Display(pg.sendPage)
		}
	}

	if pg.fromCoinSelection.Clicked() {
		if pg.modalLayout != nil {
			pg.modalLayout.Dismiss()
		} else {
			pg.ParentNavigator().Display(pg.sendPage)
		}
	}

	if pg.clearButton.Clicked() {
		for i := 0; i < len(pg.accountUTXOs.Details); i++ {
			pg.accountUTXOs.Details[i].checkbox.CheckBox = &widget.Bool{Value: false}
		}
		pg.initializeFields()
	}

	if pg.accountCollapsible.IsExpanded() {
		for pos, component := range pg.clickables {
			if component == nil || !component.Clicked() {
				continue
			}

			if pg.sortingInProgress {
				break
			}

			pg.sortingInProgress = true

			if pos != pg.lastSortEvent.clicked {
				pg.lastSortEvent.clicked = pos
				pg.lastSortEvent.count = 0
			}
			pg.lastSortEvent.count++

			isAscendingOrder := pg.lastSortEvent.count%2 == 0
			sort.SliceStable(pg.accountUTXOs.Details, func(i, j int) bool {
				return sortUTXOrows(i, j, pos, isAscendingOrder, pg.accountUTXOs.Details)
			})

			pg.sortingInProgress = false
			break
		}
	}

	// Update Summary information as the last section when handling events.
	for i := 0; i < len(pg.accountUTXOs.Details); i++ {
		record := pg.accountUTXOs.Details[i]
		if record.checkbox.CheckBox.Changed() {
			if record.checkbox.CheckBox.Value {
				pg.selectedUTXOrows = append(pg.selectedUTXOrows, record.UnspentOutput)
				pg.selectedAmount += record.Amount.ToCoin()
			} else {
				for index, item := range pg.selectedUTXOrows {
					if item.TxID == record.TxID {
						copy(pg.selectedUTXOrows[index:], pg.selectedUTXOrows[index+1:])
						pg.selectedUTXOrows = pg.selectedUTXOrows[:len(pg.selectedUTXOrows)-1]
						break
					}
				}
				pg.selectedAmount -= record.Amount.ToCoin()
			}

			pg.updateSummaryInfo()
		}
	}
}

func (pg *ManualCoinSelectionPage) updateSummaryInfo() {
	pg.txSize.Text = pg.computeUTXOsSize()
	pg.selectedUTXOs.Text = fmt.Sprintf("%d", len(pg.selectedUTXOrows))
	pg.totalAmount.Text = fmt.Sprintf("%f %s", pg.selectedAmount, pg.strAssetType)
}

func (pg *ManualCoinSelectionPage) computeUTXOsSize() string {
	wallet := pg.sendPage.selectedWallet

	// Access to coin selection page is restricted unless destination address is selected.
	destination := pg.sendPage.recipient.destinationAddress()
	feeNSize, err := wallet.ComputeTxSizeEstimation(destination, pg.selectedUTXOrows)
	if err != nil {
		log.Error(err)
	}
	return fmt.Sprintf("%d bytes", feeNSize)
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
	if pg.modalLayout == nil {
		return pg.contentLayout(gtx)
	}
	var modalWidth float32 = 450
	if pg.IsMobileView() {
		modalWidth = 0
	}
	modalContent := []layout.Widget{pg.contentLayout}
	return pg.modalLayout.Layout(gtx, modalContent, modalWidth)
}
func (pg *ManualCoinSelectionPage) contentLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.WrapContent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Vertical,
			}.Layout(gtx,
				layout.Rigid(pg.topSection),
				layout.Rigid(pg.summarySection),
				layout.Rigid(pg.accountListSection),
			)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
				return layout.E.Layout(gtx, pg.actionButton.Layout)
			})
		}),
	)
}

func (pg *ManualCoinSelectionPage) topSection(gtx C) D {
	return layout.Inset{Bottom: values.MarginPadding14}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.WrapContent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Horizontal,
				Alignment:   layout.Middle,
				Clickable:   pg.fromCoinSelection,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Right: values.MarginPadding6,
					}.Layout(gtx, func(gtx C) D {
						return pg.Theme.Icons.ChevronLeft.Layout24dp(gtx)
					})
				}),
				layout.Rigid(func(gtx C) D {
					lbl := pg.Theme.H6(values.String(values.StrCoinSelection))
					lbl.TextSize = values.TextSizeTransform(pg.IsMobileView(), values.TextSize20)
					return lbl.Layout(gtx)
				}),
			)
		})
	})
}

func (pg *ManualCoinSelectionPage) summarySection(gtx C) D {
	textSize16 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize16)
	margin16 := values.MarginPadding16
	if pg.modalLayout != nil {
		margin16 = values.MarginPadding0
	}
	return layout.Inset{Bottom: margin16}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			topContainer := layout.UniformInset(values.MarginPadding15)
			return topContainer.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
							textLabel := pg.Theme.Label(textSize16, values.String(values.StrSummary))
							textLabel.Font.Weight = font.SemiBold
							return textLabel.Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						axis := layout.Horizontal
						if pg.IsMobileView() || pg.modalLayout != nil {
							axis = layout.Vertical
						}
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Flex{Axis: axis, Spacing: layout.SpaceBetween}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return pg.sumaryContent(gtx, values.String(values.StrSelectedUTXO)+": ", pg.selectedUTXOs)
							}),
							layout.Rigid(func(gtx C) D {
								return pg.sumaryContent(gtx, values.StringF(values.StrTxSize, " : "), pg.txSize)

							}),
							layout.Rigid(func(gtx C) D {
								return pg.sumaryContent(gtx, values.String(values.StrTotalAmount)+": ", pg.totalAmount)
							}),
						)
					}),
				)
			})
		})
	})
}

func (pg *ManualCoinSelectionPage) sumaryContent(gtx C, text string, valueLable cryptomaterial.Label) D {
	textSize14 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize14)
	valueLable.TextSize = textSize14
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(pg.Theme.Label(textSize14, text).Layout),
		layout.Rigid(valueLable.Layout),
	)
}

func (pg *ManualCoinSelectionPage) accountListSection(gtx C) D {
	textSize14 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize14)
	textSize16 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize16)
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					textLabel := pg.Theme.Label(textSize16, values.String(values.StrAccountList))
					textLabel.Font.Weight = font.SemiBold
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(textLabel.Layout),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, pg.clearButton.Layout)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					collapsibleHeader := func(gtx C) D {
						t := pg.Theme.Label(textSize16, pg.accountUTXOs.Account)
						t.Font.Weight = font.SemiBold
						return t.Layout(gtx)
					}

					collapsibleBody := func(gtx C) D {
						if len(pg.accountUTXOs.Details) == 0 {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.Center.Layout(gtx,
								pg.Theme.Label(textSize14, values.String(values.StrNoUTXOs)).Layout,
							)
						}
						return pg.accountListItemsSection(gtx, pg.accountUTXOs.Details)
					}
					return pg.accountCollapsible.Layout(gtx, collapsibleHeader, collapsibleBody)
				}),
			)
		})
	})
}

func (pg *ManualCoinSelectionPage) generateLabel(txt interface{}, clickable *cryptomaterial.Clickable) labelCell {
	txtStr := ""
	switch n := txt.(type) {
	case string:
		txtStr = n
	case float64:
		txtStr = fmt.Sprintf("%8.8f", n)
	case int32, int, int64:
		txtStr = fmt.Sprintf("%d", n)
	}

	lb := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize14), txtStr)
	if len(txtStr) > MaxAddressLen {
		// Only addresses have texts longer than 16 characters.
		lb.Text = txtStr[:MaxAddressLen] + "..."
		lb.Color = pg.Theme.Color.Primary
	}

	if clickable != nil {
		lb.Font.Weight = font.Bold
		lb.Color = pg.Theme.Color.Gray3
	}

	return labelCell{
		label:     lb,
		clickable: clickable,
	}
}

func (pg *ManualCoinSelectionPage) accountListItemsSection(gtx C, utxos []*UTXOInfo) D {
	return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return pg.rowItemsSection(gtx, nil, pg.amountLabel, nil, pg.addressLabel,
					nil, pg.confirmationsLabel, nil, pg.dateLabel)
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return pg.Theme.List(pg.listContainer).Layout(gtx, len(utxos), func(gtx C, index int) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							v := utxos[index]
							checkButton := &v.checkbox                                                            // Component 1
							amountLabel := pg.generateLabel(v.Amount.ToCoin(), nil)                               // component 2
							addresslabel := pg.generateLabel(v.Address, nil)                                      // Component 3
							confirmationsLabel := pg.generateLabel(v.Confirmations, nil)                          // Component 4
							dateLabel := pg.generateLabel(libutils.FormatUTCShortTime(v.ReceiveTime.Unix()), nil) // Component 5

							// copy destination Address
							if v.addressCopy.Clicked() {
								clipboard.WriteOp{Text: v.Address}.Add(gtx.Ops)
								pg.Toast.Notify(values.String(values.StrAddressCopied))
							}

							addressComponent := func(gtx C) D {
								return v.addressCopy.Layout(gtx, addresslabel.label.Layout)
							}
							return pg.rowItemsSection(gtx, checkButton, amountLabel, nil, addressComponent,
								nil, confirmationsLabel, nil, dateLabel)
						}),
						layout.Rigid(func(gtx C) D {
							// No divider for last row
							if index == len(utxos)-1 {
								return D{}
							}
							return layout.Inset{Bottom: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
								return pg.Theme.Separator().Layout(gtx)
							})
						}),
					)
				})
			}),
		)
	})
}

func (pg *ManualCoinSelectionPage) rowItemsSection(gtx C, components ...interface{}) D {
	getRowItem := func(index int) layout.Widget {
		var widget layout.Widget
		c := components[index]

		switch n := c.(type) {
		case *cryptomaterial.CheckBoxStyle:
			widget = n.Layout
		case func(gtx C) D:
			widget = n
		case labelCell:
			if n.clickable != nil {
				widget = func(gtx C) D {
					return layout.UniformInset(values.MarginPadding0).Layout(gtx, func(gtx C) D {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Orientation: layout.Horizontal,
							Alignment:   layout.Middle,
							Clickable:   n.clickable,
						}.Layout(gtx,
							layout.Rigid(n.label.Layout),
							layout.Rigid(func(gtx C) D {
								count := pg.lastSortEvent.count
								if pg.lastSortEvent.clicked == (index-1)/2 && count >= 0 {
									m := values.MarginPadding4
									inset := layout.Inset{Left: m}

									if count%2 == 0 { // add ascending icon
										inset.Bottom = m
										return inset.Layout(gtx, pg.Theme.Icons.CaretUp.Layout12dp)
									} // else add descending icon
									inset.Top = m
									return inset.Layout(gtx, pg.Theme.Icons.CaretDown.Layout12dp)
								}
								return D{}
							}),
						)
					})
				}
			} else {
				widget = n.label.Layout
			}
		default:
			// create an empty default placeholder for unsupported widgets.
			widget = func(gtx C) D { return D{} }
		}
		return widget
	}

	max := float32(gtx.Constraints.Max.X)
	return pg.Theme.List(pg.utxosRow).Layout(gtx, len(components), func(gtx C, index int) D {
		c := pg.properties[index]
		gtx.Constraints.Min.X = int(max * c.weight)
		return c.direction.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.End}.Layout(gtx,
				layout.Rigid(getRowItem(index)),
			)
		})
	})
}

func sortUTXOrows(i, j, pos int, ascendingOrder bool, elems []*UTXOInfo) bool {
	switch pos {
	case 0: // component 2 (Amount Component)
		if ascendingOrder {
			return elems[i].Amount.ToInt() > elems[j].Amount.ToInt()
		}
		return elems[i].Amount.ToInt() < elems[j].Amount.ToInt()
	case 1: // component 3 (Address Component)
		addresses := []string{elems[i].Address, elems[j].Address}
		if ascendingOrder {
			sort.Strings(addresses)
			return elems[i].Address == addresses[0]
		}
		sort.Sort(sort.Reverse(sort.StringSlice(addresses)))
		return elems[i].Address == addresses[0]
	case 2: // component 4 (Confirmations Component)
		if ascendingOrder {
			return elems[i].Confirmations > elems[j].Confirmations
		}
		return elems[i].Confirmations < elems[j].Confirmations
	case 3: // component 5 (Date Component)
		if ascendingOrder {
			return elems[i].ReceiveTime.Unix() > elems[j].ReceiveTime.Unix()
		}
		return elems[i].ReceiveTime.Unix() < elems[j].ReceiveTime.Unix()

	default:
		return false
	}
}

// Handle implements app.Modal.
func (pg *ManualCoinSelectionPage) Handle() {
	if pg.modalLayout.BackdropClicked(true) {
		pg.modalLayout.Dismiss()
	} else {
		pg.HandleUserInteractions()
	}
}

// OnDismiss implements app.Modal.
func (pg *ManualCoinSelectionPage) OnDismiss() {
	pg.OnNavigatedFrom()
}

// OnResume implements app.Modal.
func (pg *ManualCoinSelectionPage) OnResume() {
	pg.OnNavigatedTo()
}
