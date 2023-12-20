package staking

import (
	"fmt"
	"image"
	"sort"
	"time"

	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

type transactionItem struct {
	transaction   *sharedW.Transaction
	ticketSpender *sharedW.Transaction
	status        *components.TxStatus
	confirmations int32
	progress      float32
	showProgress  bool
	showTime      bool
	purchaseTime  string
	ticketAge     string

	statusTooltip     *cryptomaterial.Tooltip
	walletNameTooltip *cryptomaterial.Tooltip
	dateTooltip       *cryptomaterial.Tooltip
	daysBehindTooltip *cryptomaterial.Tooltip
	durationTooltip   *cryptomaterial.Tooltip
}

func (pg *Page) stakeToTransactionItems(txs []*sharedW.Transaction, newestFirst bool, hasFilter func(int32) bool) ([]*transactionItem, error) {
	tickets := make([]*transactionItem, 0)
	for _, tx := range txs {
		bestBlockHeight := pg.dcrWallet.GetBestBlockHeight()

		ticketSpender, err := pg.dcrWallet.TicketSpender(tx.Hash)
		if err != nil {
			return nil, err
		}

		// Apply voted and revoked tx filter
		if (hasFilter(dcr.TxFilterVoted) || hasFilter(dcr.TxFilterRevoked)) && ticketSpender == nil {
			continue
		}

		if hasFilter(dcr.TxFilterVoted) && ticketSpender.Type != dcr.TxTypeVote {
			continue
		}

		if hasFilter(dcr.TxFilterRevoked) && ticketSpender.Type != dcr.TxTypeRevocation {
			continue
		}

		// This fixes a libwallet bug where live tickets transactions
		// do not have updated data of Stake spender.
		if hasFilter(dcr.TxFilterLive) && ticketSpender != nil {
			continue
		}

		ticketCopy := tx
		txStatus := components.TransactionTitleIcon(pg.Load, pg.dcrWallet, tx)
		confirmations := dcr.Confirmations(bestBlockHeight, tx)
		var ticketAge string

		showProgress := txStatus.TicketStatus == dcr.TicketStatusImmature || txStatus.TicketStatus == dcr.TicketStatusLive
		if ticketSpender != nil { /// voted or revoked
			showProgress = dcr.Confirmations(bestBlockHeight, ticketSpender) <= pg.dcrWallet.TicketMaturity()
			ticketAge = fmt.Sprintf("%d days", ticketSpender.DaysToVoteOrRevoke)
		} else if txStatus.TicketStatus == dcr.TicketStatusImmature ||
			txStatus.TicketStatus == dcr.TicketStatusLive {

			ticketAgeDuration := time.Since(time.Unix(tx.Timestamp, 0)).Seconds()
			ticketAge = components.TimeFormat(int(ticketAgeDuration), true)
		}

		showTime := showProgress && txStatus.TicketStatus != dcr.TicketStatusLive

		var progress float32
		if showProgress {
			progressMax := pg.dcrWallet.TicketMaturity()
			if txStatus.TicketStatus == dcr.TicketStatusLive {
				progressMax = pg.dcrWallet.TicketExpiry()
			}

			confs := confirmations
			if ticketSpender != nil {
				confs = dcr.Confirmations(bestBlockHeight, ticketSpender)
			}

			progress = (float32(confs) / float32(progressMax)) * 100
		}

		tickets = append(tickets, &transactionItem{
			transaction:   ticketCopy,
			ticketSpender: ticketSpender,
			status:        txStatus,
			confirmations: dcr.Confirmations(bestBlockHeight, tx),
			progress:      progress,
			showProgress:  showProgress,
			showTime:      showTime,
			purchaseTime:  time.Unix(tx.Timestamp, 0).Format("Jan 2, 2006 15:04:05 PM"),
			ticketAge:     ticketAge,

			statusTooltip:     pg.Theme.Tooltip(),
			walletNameTooltip: pg.Theme.Tooltip(),
			dateTooltip:       pg.Theme.Tooltip(),
			daysBehindTooltip: pg.Theme.Tooltip(),
			durationTooltip:   pg.Theme.Tooltip(),
		})
	}

	// bring vote and revoke tx forward
	sort.Slice(tickets[:], func(i, j int) bool {
		timeStampI := tickets[i].transaction.Timestamp
		timeStampJ := tickets[j].transaction.Timestamp

		if tickets[i].ticketSpender != nil {
			timeStampI = tickets[i].ticketSpender.Timestamp
		}

		if tickets[j].ticketSpender != nil {
			timeStampJ = tickets[j].ticketSpender.Timestamp
		}

		if newestFirst {
			return timeStampI > timeStampJ
		}
		return timeStampI < timeStampJ
	})

	return tickets, nil
}

func TicketStatusDetails(gtx C, l *load.Load, dcrWallet *dcr.Asset, tx *transactionItem) D {
	date := time.Unix(tx.transaction.Timestamp, 0).Format("Jan 2, 2006")
	timeSplit := time.Unix(tx.transaction.Timestamp, 0).Format("03:04:05 PM")
	dateTime := fmt.Sprintf("%v at %v", date, timeSplit)
	bestBlock := dcrWallet.GetBestBlock()
	col := l.Theme.Color.GrayText3
	textSize16 := values.TextSizeTransform(l.IsMobileView(), values.TextSize16)

	switch tx.status.TicketStatus {
	case dcr.TicketStatusUnmined:
		lbl := l.Theme.Label(textSize16, values.StringF(values.StrUnminedInfo, components.TimeAgo(tx.transaction.Timestamp)))
		lbl.Color = col
		return lbl.Layout(gtx)
	case dcr.TicketStatusImmature:
		maturity := dcrWallet.TicketMaturity()
		blockTime := dcrWallet.TargetTimePerBlockMinutes()
		maturityDuration := time.Duration(maturity*int32(blockTime)) * time.Minute
		blockRemaining := (bestBlock.Height - tx.transaction.BlockHeight)

		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				lbl := l.Theme.Label(textSize16, values.StringF(values.StrImmatureInfo, blockRemaining, maturity,
					maturityDuration.String()))
				lbl.Color = col
				return lbl.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				p := l.Theme.ProgressBarCircle(int(tx.progress))
				p.Color = tx.status.ProgressBarColor
				return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
					sz := gtx.Dp(values.MarginPadding22)
					gtx.Constraints.Max = image.Point{X: sz, Y: sz}
					gtx.Constraints.Min = gtx.Constraints.Max
					return p.Layout(gtx)
				})
			}),
		)
	case dcr.TicketStatusLive:
		expiry := dcrWallet.TicketExpiry()
		lbl := l.Theme.Label(textSize16, values.StringF(values.StrLiveInfoDisc, expiry, getTimeToMatureOrExpire(dcrWallet, tx), expiry))
		lbl.Color = col
		return lbl.Layout(gtx)
	case dcr.TicketStatusVotedOrRevoked:
		if tx.ticketSpender.Type == dcr.TxTypeVote {
			return multiContent(gtx, l, dateTime, fmt.Sprintf("%s %v", values.String(values.StrVoted), components.TimeAgo(tx.transaction.Timestamp)))
		}
		lbl := l.Theme.Label(textSize16, dateTime)
		lbl.Color = col
		return lbl.Layout(gtx)
	case dcr.TicketStatusExpired:
		return multiContent(gtx, l, dateTime, fmt.Sprintf("%s %v", values.String(values.StrExpired), components.TimeAgo(tx.transaction.Timestamp)))
	default:
		return D{}
	}
}

func multiContent(gtx C, l *load.Load, leftText, rightText string) D {
	textSize16 := values.TextSizeTransform(l.IsMobileView(), values.TextSize16)
	col := l.Theme.Color.GrayText3
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := l.Theme.Label(textSize16, leftText)
			lbl.Color = col
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding5,
				Left:  values.MarginPadding5,
			}.Layout(gtx, func(gtx C) D {
				ic := cryptomaterial.NewIcon(l.Theme.Icons.DotIcon)
				ic.Color = col
				return ic.Layout(gtx, values.MarginPadding6)
			})
		}),
		layout.Rigid(func(gtx C) D {
			lbl := l.Theme.Label(textSize16, rightText)
			lbl.Color = col
			return lbl.Layout(gtx)
		}),
	)
}

func ticketListLayout(gtx C, l *load.Load, wallet *dcr.Asset, ticket *transactionItem) layout.Dimensions {
	return layout.Inset{
		Right: values.MarginPadding26,
	}.Layout(gtx, func(gtx C) D {
		return components.EndToEndRow(gtx,
			func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						wrapIcon := l.Theme.Card()
						wrapIcon.Color = ticket.status.Background
						wrapIcon.Radius = cryptomaterial.Radius(8)
						dims := wrapIcon.Layout(gtx, func(gtx C) D {
							return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
								return ticket.status.Icon.LayoutTransform(gtx, l.IsMobileView(), values.MarginPadding24)
							})
						})

						return layout.Inset{
							Right: values.MarginPadding16,
						}.Layout(gtx, func(gtx C) D {
							return dims
						})
					}),
					layout.Rigid(l.Theme.Label(values.TextSizeTransform(l.IsMobileView(), values.TextSize18), ticket.status.Title).Layout),
				)
			},
			func(gtx C) D {
				return TicketStatusDetails(gtx, l, wallet, ticket)
			})
	})
}

func nextTicketRemaining(allsecs int) string {
	if allsecs == 0 {
		return "imminent"
	}
	str := ""
	if allsecs > 604799 {
		weeks := allsecs / 604800
		allsecs %= 604800
		str += fmt.Sprintf("%dw ", weeks)
	}
	if allsecs > 86399 {
		days := allsecs / 86400
		allsecs %= 86400
		str += fmt.Sprintf("%dd ", days)
	}
	if allsecs > 3599 {
		hours := allsecs / 3600
		allsecs %= 3600
		str += fmt.Sprintf("%dh ", hours)
	}
	if allsecs > 59 {
		mins := allsecs / 60
		allsecs %= 60
		str += fmt.Sprintf("%dm ", mins)
	}
	if allsecs > 0 {
		str += fmt.Sprintf("%ds ", allsecs)
	}
	return str
}

func getTimeToMatureOrExpire(dcrWallet *dcr.Asset, tx *transactionItem) int {
	progressMax := dcrWallet.TicketMaturity()
	if tx.status.TicketStatus == dcr.TicketStatusLive {
		progressMax = dcrWallet.TicketExpiry()
	}

	bestBlockHeight := dcrWallet.GetBestBlockHeight()
	confs := dcr.Confirmations(bestBlockHeight, tx.transaction)
	if tx.ticketSpender != nil {
		confs = dcr.Confirmations(bestBlockHeight, tx.ticketSpender)
	}

	progress := (float32(confs) / float32(progressMax)) * 100
	return int(progress)
}
