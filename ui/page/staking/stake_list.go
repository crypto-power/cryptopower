package staking

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/listeners"
	"github.com/crypto-power/cryptopower/ui/values"
)

func (pg *Page) initTicketList() {
	pg.ticketsList = pg.Theme.NewClickableList(layout.Vertical)
}

func (pg *Page) listenForTxNotifications() {
	if pg.TxAndBlockNotificationListener != nil {
		return
	}

	pg.TxAndBlockNotificationListener = listeners.NewTxAndBlockNotificationListener()
	err := pg.dcrImpl.AddTxAndBlockNotificationListener(pg.TxAndBlockNotificationListener, true, OverviewPageID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-pg.TxAndBlockNotifChan():
				if n.Type == listeners.BlockAttached || n.Type == listeners.NewTransaction {
					pg.ParentWindow().Reload()
				}
			case <-pg.ctx.Done():
				pg.dcrImpl.RemoveTxAndBlockNotificationListener(OverviewPageID)
				pg.CloseTxAndBlockChan()
				pg.TxAndBlockNotificationListener = nil

				return
			}
		}
	}()
}

func (pg *Page) fetchTickets(offset, pageSize int32) ([]*transactionItem, int, bool, error) {
	txs, err := pg.WL.SelectedWallet.Wallet.GetTransactionsRaw(offset, pageSize, dcr.TxFilterTickets, true)
	if err != nil {
		return nil, -1, false, err
	}

	tickets, err := stakeToTransactionItems(pg.Load, txs, true, func(filter int32) bool {
		return filter == dcr.TxFilterTickets
	})
	return tickets, len(tickets), false, err
}

func (pg *Page) ticketListLayout(gtx C) D {
	return layout.Inset{
		Bottom: values.MarginPadding8,
	}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			return layout.Inset{
				Left:   values.MarginPadding26,
				Top:    values.MarginPadding26,
				Bottom: values.MarginPadding26,
			}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {

						txt := pg.Theme.Label(values.TextSize20, values.String(values.StrTickets))
						txt.Font.Weight = font.SemiBold
						return layout.Inset{Bottom: values.MarginPadding18}.Layout(gtx, txt.Layout)

						// txt := pg.Theme.Body1(values.String(values.StrTickets))
						// txt.Color = pg.Theme.Color.GrayText2
						// return layout.Inset{Bottom: values.MarginPadding18}.Layout(gtx, txt.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						if pg.scroll.ItemsCount() <= 0 {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X

							txt := pg.Theme.Body1(values.String(values.StrNoTickets))
							txt.Color = pg.Theme.Color.GrayText3
							txt.Alignment = text.Middle
							return layout.Inset{Top: values.MarginPadding15, Bottom: values.MarginPadding16}.Layout(gtx, txt.Layout)
						}

						tickets := pg.scroll.FetchedData()
						return pg.ticketsList.Layout(gtx, len(tickets), func(gtx C, index int) D {
							ticket := tickets[index]

							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								// gray separator line
								layout.Rigid(func(gtx C) D {
									if index == 0 {
										return D{}
									}
									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									separator := pg.Theme.Separator()
									separator.Width = gtx.Constraints.Max.X
									return layout.Inset{
										Bottom: values.MarginPadding5,
										Left:   values.MarginPadding40,
									}.Layout(gtx, func(gtx C) D {
										return layout.E.Layout(gtx, separator.Layout)
									})
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Bottom: values.MarginPadding5,
									}.Layout(gtx, func(gtx C) D {
										return ticketListLayout(gtx, pg.Load, ticket)
									})
								}),
							)
						})
					}),
				)
			})
		})
	})
}
