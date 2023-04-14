package staking

import (
	"math"

	"gioui.org/layout"
	"gioui.org/text"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	"code.cryptopower.dev/group/cryptopower/listeners"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

const pageSize int32 = 20

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

func (pg *Page) fetchTickets(reverse bool) {
	if pg.loadingTickets || pg.loadedAllTickets {
		return
	}
	defer func() {
		pg.loadingTickets = false
		if reverse {
			pg.list.Position.Offset = int(math.Abs(float64(pg.list.Position.OffsetLast + 4)))
			pg.list.Position.OffsetLast = -4
		} else {
			pg.list.Position.Offset = 4
			pg.list.Position.OffsetLast = pg.list.Position.OffsetLast + 4
		}

	}()
	pg.loadingTickets = true
	switch reverse {
	case true:
		pg.ticketOffset -= pageSize
	default:
		if len(pg.tickets) != 0 {
			pg.ticketOffset += pageSize
		}
	}
	txs, err := pg.WL.SelectedWallet.Wallet.GetTransactionsRaw(pg.ticketOffset, pageSize, dcr.TxFilterTickets, true)
	if err != nil {
		errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		return
	}

	tickets, err := stakeToTransactionItems(pg.Load, txs, true, func(filter int32) bool {
		return filter == dcr.TxFilterTickets
	})
	if err != nil {
		errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
		pg.ParentWindow().ShowModal(errModal)
		return
	}

	switch ticketLen := len(tickets); {
	case ticketLen > 0:
		if len(tickets) < int(pageSize) {
			pg.tickets = append(pg.tickets, tickets...) // if the last batch is too small append to existing.
		} else {
			pg.tickets = tickets
		}
	default:
		pg.loadedAllTickets = true
	}
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
						txt := pg.Theme.Body1(values.String(values.StrTickets))
						txt.Color = pg.Theme.Color.GrayText2
						return layout.Inset{Bottom: values.MarginPadding18}.Layout(gtx, txt.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						tickets := pg.tickets

						if len(tickets) == 0 {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X

							txt := pg.Theme.Body1(values.String(values.StrNoTickets))
							txt.Color = pg.Theme.Color.GrayText3
							txt.Alignment = text.Middle
							return layout.Inset{Top: values.MarginPadding15, Bottom: values.MarginPadding16}.Layout(gtx, txt.Layout)
						}

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

func (pg *Page) onScrollChangeListener() {
	if len(pg.tickets) < int(pageSize) {
		return
	}

	if pg.list.List.Position.OffsetLast == 0 && !pg.list.List.Position.BeforeEnd {
		go pg.fetchTickets(false)
	}

	// Fetches preceeding pagesize tickets if the list scrollbar is at the beginning.
	if pg.list.List.Position.BeforeEnd && pg.list.Position.Offset == 0 && pg.ticketOffset >= pageSize {
		if pg.loadedAllTickets {
			pg.loadedAllTickets = false
		}
		go pg.fetchTickets(true)
	}
}
