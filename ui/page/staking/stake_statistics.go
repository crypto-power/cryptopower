package staking

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

type statisticsItem struct {
	Icon        *cryptomaterial.Image
	Title       string
	ValueText   string
	ValueWidget func(gtx C) D
}

func (pg *Page) stakeStatisticsSection(gtx C) D {
	isMobile := pg.IsMobileView()
	totalRewardDim := func(gtx C) D {
		if pg.totalRewards == "" {
			return D{}
		}
		return components.LayoutBalanceWithStateSemiBold(gtx, pg.Load, pg.totalRewards)
	}
	totalRewardItem := &statisticsItem{Icon: pg.Theme.Icons.StakeyIcon, Title: values.String(values.StrTotalReward), ValueText: pg.totalRewards, ValueWidget: totalRewardDim}
	revokedItem := &statisticsItem{Icon: pg.Theme.Icons.TicketRevokedIcon, Title: values.String(values.StrRevoke), ValueText: fmt.Sprintf("%d", pg.ticketOverview.Revoked)}
	uminedItem := &statisticsItem{Icon: pg.Theme.Icons.TicketUnminedIcon, Title: values.String(values.StrUmined), ValueText: fmt.Sprintf("%d", pg.ticketOverview.Immature)}
	votedItem := &statisticsItem{Icon: pg.Theme.Icons.TicketVotedIcon, Title: values.String(values.StrVoted), ValueText: fmt.Sprintf("%d", pg.ticketOverview.Voted)}
	immatureItem := &statisticsItem{Icon: pg.Theme.Icons.TicketImmatureIcon, Title: values.String(values.StrImmature), ValueText: fmt.Sprintf("%d", pg.ticketOverview.Immature)}
	expiredItem := &statisticsItem{Icon: pg.Theme.Icons.TicketExpiredIcon, Title: values.String(values.StrExpired), ValueText: fmt.Sprintf("%d", pg.ticketOverview.Expired)}

	return pg.pageSections(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				txt := pg.Theme.Label(values.TextSizeTransform(isMobile, values.TextSize20), values.String(values.StrStatistics))
				txt.Font.Weight = font.SemiBold
				return layout.Inset{
					Bottom: values.MarginPaddingTransform(isMobile, values.MarginPadding24),
				}.Layout(gtx, txt.Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if isMobile {
							return pg.dataStatisticsCol(gtx, totalRewardItem, revokedItem, uminedItem, isMobile)
						}
						return pg.dataStatisticsCol(gtx, totalRewardItem, revokedItem, nil, isMobile)
					}),
					layout.Rigid(func(gtx C) D {
						if isMobile {
							return pg.dataStatisticsCol(gtx, votedItem, immatureItem, expiredItem, isMobile)
						}
						return pg.dataStatisticsCol(gtx, uminedItem, votedItem, nil, isMobile)
					}),
					layout.Rigid(func(gtx C) D {
						if isMobile {
							return D{}
						}
						return pg.dataStatisticsCol(gtx, immatureItem, expiredItem, nil, isMobile)
					}),
				)
			}),
		)
	})
}

func (pg *Page) dataStatisticsCol(gtx C, item1, item2, item3 *statisticsItem, isMobile bool) D {
	spacerHeight := values.MarginPaddingTransform(isMobile, values.MarginPadding24)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.dataStatisticsItem(gtx, item1)
		}),
		layout.Rigid(layout.Spacer{Height: spacerHeight}.Layout),
		layout.Rigid(func(gtx C) D {
			return pg.dataStatisticsItem(gtx, item2)
		}),
		layout.Rigid(func(gtx C) D {
			if item3 == nil {
				return D{}
			}
			return layout.Inset{Top: spacerHeight}.Layout(gtx, func(gtx C) D {
				return pg.dataStatisticsItem(gtx, item3)
			})
		}),
	)
}

func (pg *Page) dataStatisticsItem(gtx C, item *statisticsItem) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding10,
			}.Layout(gtx, item.Icon.Layout36dp)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					label := pg.Theme.Label(values.TextSize16, item.Title)
					label.Color = pg.Theme.Color.GrayText2
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					if item.ValueWidget != nil {
						return item.ValueWidget(gtx)
					}
					label := pg.Theme.Label(values.TextSize16, item.ValueText)
					label.Color = pg.Theme.Color.Text
					label.Font.Weight = font.SemiBold
					return label.Layout(gtx)
				}),
			)
		}),
	)
}
