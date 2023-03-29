package staking

import (
	"fmt"

	"gioui.org/layout"

	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"github.com/decred/dcrd/dcrutil/v4"
)

func (pg *Page) initStakePriceWidget() *Page {
	pg.stakeSettings = pg.Theme.NewClickable(false)
	_, pg.infoButton = components.SubpageHeaderButtons(pg.Load)

	pg.stake = pg.Theme.Switch()
	return pg
}

func (pg *Page) stakePriceSection(gtx C) D {
	return pg.pageSections(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Bottom: values.MarginPadding11,
				}.Layout(gtx, func(gtx C) D {
					col := pg.Theme.Color.GrayText2
					leftWg := func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										title := pg.Theme.Label(values.TextSize16, values.String(values.StrTicketPrice)+": ")
										title.Color = col
										return title.Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Center.Layout(gtx, func(gtx C) D {
											if pg.WL.SelectedWallet.Wallet.IsSyncing() {
												title := pg.Theme.Label(values.TextSize16, values.String(values.StrLoadingPrice))
												title.Color = col
												return title.Layout(gtx)
											}

											return components.LayoutBalanceSize(gtx, pg.Load, pg.ticketPrice, values.TextSize16)
										})
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{
											Left:  values.MarginPadding8,
											Right: values.MarginPadding4,
										}.Layout(gtx, pg.Theme.Icons.TimerIcon.Layout12dp)
									}),
									layout.Rigid(func(gtx C) D {
										secs, _ := pg.dcrImpl.NextTicketPriceRemaining()
										txt := pg.Theme.Label(values.TextSize16, nextTicketRemaining(int(secs)))
										txt.Color = col

										if pg.WL.SelectedWallet.Wallet.IsSyncing() {
											txt.Text = values.String(values.StrSyncingState)
										}
										return txt.Layout(gtx)
									}),
								)
							}),
							pg.dataRows(values.String(values.StrLiveTickets), pg.ticketOverview.Live),
							pg.dataRows(values.String(values.StrCanBuy), pg.CalculateTotalTicketsCanBuy()),
						)
					}

					rightWg := func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								title := pg.Theme.Label(values.TextSize16, values.String(values.StrStake))
								title.Color = col
								if !pg.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
									return title.Layout(gtx)
								}
								return D{}
							}),
							layout.Rigid(func(gtx C) D {
								if !pg.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
									return layout.Inset{
										Right: values.MarginPadding40,
										Left:  values.MarginPadding4,
									}.Layout(gtx, pg.stake.Layout)
								}
								return D{}
							}),
							layout.Rigid(func(gtx C) D {
								icon := pg.Theme.Icons.HeaderSettingsIcon
								// Todo -- darkmode icons
								// if pg.ticketBuyerWallet.IsAutoTicketsPurchaseActive() {
								// 	icon = pg.Theme.Icons.SettingsInactiveIcon
								// }
								if !pg.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
									return pg.stakeSettings.Layout(gtx, icon.Layout24dp)
								}
								return D{}
							}),
							layout.Rigid(func(gtx C) D {
								pg.infoButton.Inset = layout.UniformInset(values.MarginPadding0)
								pg.infoButton.Size = values.MarginPadding22
								return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, pg.infoButton.Layout)
							}),
						)
					}

					return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Rigid(leftWg),
						layout.Rigid(rightWg),
					)
				})
			}),
			layout.Rigid(pg.balanceProgressBarLayout),
		)
	})
}

func (pg *Page) dataRows(title string, count int) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		return layout.Inset{Top: values.MarginPadding7}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.Label(values.TextSize16, title+":")
					label.Color = pg.Theme.Color.GrayText2
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
						label := pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", count))
						label.Color = pg.Theme.Color.GrayText2
						return label.Layout(gtx)
					})
				}),
			)
		})
	})
}

func (pg *Page) CalculateTotalTicketsCanBuy() int {
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
					return components.LayoutIconAndText(pg.Load, gtx, values.String(values.StrStaked)+": ", totalBalance.LockedByTickets.String(), items[0].Color)
				}),
				layout.Rigid(func(gtx C) D {
					return components.LayoutIconAndText(pg.Load, gtx, values.String(values.StrLabelSpendable)+": ", totalBalance.Spendable.String(), items[1].Color)
				}),
			)
		})
	}
	total := totalBalance.Spendable.ToInt() + totalBalance.LockedByTickets.ToInt()
	pb := pg.Theme.MultiLayerProgressBar(pg.WL.SelectedWallet.Wallet.ToAmount(total).ToCoin(), items)
	pb.Height = values.MarginPadding16
	pb.ShowLedger = true
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
