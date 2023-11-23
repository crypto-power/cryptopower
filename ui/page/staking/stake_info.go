package staking

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/decred/dcrd/dcrutil/v4"
)

func (pg *Page) initStakePriceWidget() *Page {
	pg.stakeSettings = pg.Theme.NewClickable(false)
	_, pg.infoButton = components.SubpageHeaderButtons(pg.Load)

	pg.stake = pg.Theme.Switch()
	return pg
}

func (pg *Page) pageHead(gtx C) D {
	txt := pg.Theme.Label(values.TextSize20, values.String(values.StrStakingInfo))
	txt.Font.Weight = font.SemiBold
	return layout.Inset{
		Bottom: values.MarginPadding24,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(txt.Layout),
			layout.Rigid(func(gtx C) D {
				if pg.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
					return D{}
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						title := pg.Theme.Label(values.TextSize16, values.String(values.StrStake))
						title.Color = pg.Theme.Color.GrayText2
						return title.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding24,
							Left:  values.MarginPadding8,
						}.Layout(gtx, pg.stake.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						icon := pg.Theme.Icons.HeaderSettingsIcon
						return pg.stakeSettings.Layout(gtx, icon.Layout24dp)
					}),
				)
			}),
		)
	})
}

func (pg *Page) stakePriceSection(gtx C) D {
	return pg.pageSections(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.pageHead),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Bottom: values.MarginPadding11,
				}.Layout(gtx, func(gtx C) D {
					col := pg.Theme.Color.GrayText2
					leftWg := func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Top:    values.MarginPadding6,
									Bottom: values.MarginPadding6,
								}.Layout(gtx, func(gtx C) D {
									return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											title := pg.Theme.Label(values.TextSize16, values.String(values.StrTicketPrice)+": ")
											title.Color = col
											return title.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											return layout.Center.Layout(gtx, func(gtx C) D {
												if pg.WL.SelectedWallet.Wallet.IsSyncing() || pg.WL.SelectedWallet.Wallet.IsRescanning() || !pg.isTicketsPurchaseAllowed() {
													title := pg.Theme.Label(values.TextSize16, values.String(values.StrLoadingPrice))
													title.Color = col
													return title.Layout(gtx)
												}
												return components.LayoutBalanceSemiBold(gtx, pg.Load, pg.ticketPrice)
											})
										}),
									)
								})
							}),
							layout.Rigid(func(gtx C) D {
								live := fmt.Sprintf("%d", pg.ticketOverview.Live)
								return pg.dataRows(gtx, values.String(values.StrLiveTickets), live)
							}),
						)
					}

					rightWg := func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								secs, _ := pg.dcrImpl.NextTicketPriceRemaining()
								timeleft := nextTicketRemaining(int(secs))
								return pg.dataRows(gtx, values.String(values.StrTimeLeft), timeleft)
							}),
							layout.Rigid(func(gtx C) D {
								canBuy := fmt.Sprintf("%d", pg.CalculateTotalTicketsCanBuy())
								return pg.dataRows(gtx, values.String(values.StrCanBuy), canBuy)
							}),
						)
					}

					return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Rigid(leftWg),
						layout.Rigid(rightWg),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding11, Bottom: values.MarginPadding14}.Layout(gtx, pg.Theme.Separator().Layout)
			}),
			layout.Rigid(pg.balanceProgressBarLayout),
		)
	})
}

func (pg *Page) dataRows(gtx C, title1, value1 string) D {
	return layout.Inset{Top: values.MarginPadding6, Bottom: values.MarginPadding6}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				label := pg.Theme.Label(values.TextSize16, title1)
				label.Color = pg.Theme.Color.GrayText2
				return label.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
					label := pg.Theme.Label(values.TextSize16, value1)
					label.Color = pg.Theme.Color.Text
					label.Font.Weight = font.SemiBold
					return label.Layout(gtx)
				})
			}),
		)
	})
}

func (pg *Page) CalculateTotalTicketsCanBuy() int {
	if !pg.dcrImpl.Synced() {
		return 0
	}

	totalBalance, err := components.CalculateMixedAccountBalance(pg.dcrImpl)
	if err != nil {
		log.Debugf("missing set mixed account error: %v", err)
		return 0
	}

	ticketPrice, err := pg.dcrImpl.TicketPrice()
	if err != nil {
		log.Errorf("ticketPrice error: %v", err)
		return 0
	}
	canBuy := totalBalance.Spendable.ToCoin() / dcrutil.Amount(ticketPrice.TicketPrice).ToCoin()
	if canBuy < 0 {
		canBuy = 0
	}

	return int(canBuy)
}

func (pg *Page) balanceProgressBarLayout(gtx C) D {
	totalBalance, err := components.CalculateMixedAccountBalance(pg.dcrImpl)
	if err != nil {
		return D{}
	}

	items := []cryptomaterial.ProgressBarItem{
		{
			Value: totalBalance.LockedByTickets.ToCoin(),
			Color: pg.Theme.Color.NavyBlue,
		},
		{
			Value: totalBalance.Spendable.ToCoin(),
			Color: pg.Theme.Color.Turquoise300,
		},
	}

	labelWdg := func(gtx C) D {
		return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					text := values.String(values.StrStaked) + ": " + totalBalance.LockedByTickets.String()
					return components.LayoutIconAndTextWithSize(pg.Load, gtx, text, items[0].Color, values.TextSize16, values.MarginPadding10)
				}),
				layout.Rigid(func(gtx C) D {
					text := values.String(values.StrLabelSpendable) + ": " + totalBalance.Spendable.String()
					return components.LayoutIconAndTextWithSize(pg.Load, gtx, text, items[1].Color, values.TextSize16, values.MarginPadding10)
				}),
			)
		})
	}
	total := totalBalance.Spendable.ToInt() + totalBalance.LockedByTickets.ToInt()
	pb := pg.Theme.MultiLayerProgressBar(pg.WL.SelectedWallet.Wallet.ToAmount(total).ToCoin(), items)
	pb.ShowOtherWidgetFirst = true
	pb.Height = values.MarginPadding16
	return pb.Layout(gtx, labelWdg)
}

func (pg *Page) stakingRecordStatistics(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(pg.stakingRecord(pg.totalRewards, fmt.Sprintf("%s %s", values.String(values.StrTotal), values.String(values.StrReward)))),
		layout.Rigid(pg.stakingRecord(fmt.Sprintf("%d", pg.ticketOverview.Voted), values.String(values.StrVoted))),
		layout.Rigid(pg.stakingRecord(fmt.Sprintf("%d", pg.ticketOverview.Revoked), values.String(values.StrRevoked))),
		layout.Rigid(pg.stakingRecord(fmt.Sprintf("%d", pg.ticketOverview.Immature), values.String(values.StrImmature))),
		layout.Rigid(pg.stakingRecord(fmt.Sprintf("%d", pg.ticketOverview.Unmined), values.String(values.StrUmined))),
		layout.Rigid(pg.stakingRecord(fmt.Sprintf("%d", pg.ticketOverview.Expired), values.String(values.StrExpired))),
	)
}

func (pg *Page) stakingRecord(count, status string) layout.Widget {
	return func(gtx C) D {
		return components.EndToEndRow(gtx,
			pg.Theme.Label(values.TextSize14, status).Layout,
			pg.Theme.Label(values.TextSize14, count).Layout,
		)
	}
}
