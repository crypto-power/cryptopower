package staking

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"github.com/crypto-power/cryptopower/ui/values"
)

func (pg *Page) initStakeStatistics() {
	pg.stakeStatistics = pg.Theme.NewClickable(false)
}

func (pg *Page) stakeStatisticsSection(gtx C) D {
	return pg.pageSections(gtx, func(gtx C) D {
		col := pg.Theme.Color.GrayText2
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				title := pg.Theme.Label(values.TextSize20, values.String(values.StrStatistics))
				title.Font.Weight = font.SemiBold
				return layout.Inset{Bottom: values.MarginPadding18}.Layout(gtx, title.Layout)

			}),
			layout.Rigid(func(gtx C) D {
				leftWg := func(gtx C) D {

					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Bottom: values.MarginPadding18}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,

									layout.Rigid(func(gtx C) D {
										icon := pg.Theme.Icons.StakeyIcon
										return layout.Inset{Right: values.MarginPadding10, Top: values.MarginPadding10}.Layout(gtx, icon.Layout24dp)
									}),

									layout.Rigid(func(gtx C) D {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												txt := pg.Theme.Label(values.TextSize16, values.String(values.StrTotalReward))
												txt.Color = col
												return txt.Layout(gtx)
											}),
											layout.Rigid(func(gtx C) D {
												label := pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", 58))
												return label.Layout(gtx)
											}),
										)
									}),
								)
							})
						}),

						layout.Rigid(func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,

								layout.Rigid(func(gtx C) D {
									icon := pg.Theme.Icons.TicketRevokedIcon
									return layout.Inset{Right: values.MarginPadding10, Top: values.MarginPadding10}.Layout(gtx, icon.Layout24dp)
								}),

								layout.Rigid(func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											txt := pg.Theme.Label(values.TextSize16, values.String(values.StrUmined))
											txt.Color = col
											return txt.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											label := pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", 58))
											return label.Layout(gtx)
										}),
									)
								}),
							)

						}),
					)

				}
				centerWg := func(gtx C) D {

					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Bottom: values.MarginPadding18}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,

									layout.Rigid(func(gtx C) D {
										icon := pg.Theme.Icons.TicketUnminedIcon
										return layout.Inset{Right: values.MarginPadding10, Top: values.MarginPadding10}.Layout(gtx, icon.Layout24dp)
									}),

									layout.Rigid(func(gtx C) D {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												txt := pg.Theme.Label(values.TextSize16, values.String(values.StrUmined))
												txt.Color = col
												return txt.Layout(gtx)
											}),
											layout.Rigid(func(gtx C) D {
												label := pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", 58))
												return label.Layout(gtx)
											}),
										)
									}),
								)
							})
						}),

						layout.Rigid(func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,

								layout.Rigid(func(gtx C) D {
									icon := pg.Theme.Icons.TicketVotedIcon
									return layout.Inset{Right: values.MarginPadding10, Top: values.MarginPadding10}.Layout(gtx, icon.Layout24dp)
								}),

								layout.Rigid(func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											txt := pg.Theme.Label(values.TextSize16, values.String(values.StrVoted))
											txt.Color = col
											return txt.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											label := pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", 58))
											return label.Layout(gtx)
										}),
									)
								}),
							)

						}),
					)

				}
				rightWg := func(gtx C) D {

					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Bottom: values.MarginPadding18}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,

									layout.Rigid(func(gtx C) D {
										icon := pg.Theme.Icons.TicketImmatureIcon
										return layout.Inset{Right: values.MarginPadding10, Top: values.MarginPadding10}.Layout(gtx, icon.Layout24dp)
									}),

									layout.Rigid(func(gtx C) D {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												txt := pg.Theme.Label(values.TextSize16, values.String(values.StrImmature))
												txt.Color = col
												return txt.Layout(gtx)
											}),
											layout.Rigid(func(gtx C) D {
												label := pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", 58))
												return label.Layout(gtx)
											}),
										)
									}),
								)
							})
						}),

						layout.Rigid(func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,

								layout.Rigid(func(gtx C) D {
									icon := pg.Theme.Icons.TicketExpiredIcon
									return layout.Inset{Right: values.MarginPadding10, Top: values.MarginPadding10}.Layout(gtx, icon.Layout24dp)
								}),

								layout.Rigid(func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											txt := pg.Theme.Label(values.TextSize16, values.String(values.StrExpired))
											txt.Color = col
											return txt.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											label := pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", 58))
											return label.Layout(gtx)
										}),
									)
								}),
							)

						}),
					)

				}
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(leftWg),
					layout.Rigid(centerWg),
					layout.Rigid(rightWg),
				)
			}),
		)

	})

}
