package components

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

type ProposalItem struct {
	Proposal     libwallet.Proposal
	tooltip      *cryptomaterial.Tooltip
	tooltipLabel cryptomaterial.Label
	voteBar      *VoteBar
}

func ProposalsList(window app.WindowNavigator, gtx C, l *load.Load, prop *ProposalItem) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
		proposal := prop.Proposal
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layoutTitleAndDate(gtx, l, prop)
			}),
			layout.Rigid(func(gtx C) D {
				return layoutAuthor(gtx, l, prop)
			}),
			layout.Rigid(func(gtx C) D {
				if proposal.Category == libwallet.ProposalCategoryActive ||
					proposal.Category == libwallet.ProposalCategoryApproved ||
					proposal.Category == libwallet.ProposalCategoryRejected {
					return layoutProposalVoteBar(window, gtx, prop)
				}
				return D{}
			}),
		)
	})
}

func layoutTitleAndDate(gtx C, l *load.Load, item *ProposalItem) D {
	proposal := item.Proposal
	grayCol := l.Theme.Color.GrayText2
	dotLabel := l.Theme.H4(" . ")
	dotLabel.Color = grayCol

	stateLabel := l.Theme.Body2(fmt.Sprintf("%v /2", proposal.VoteStatus))
	stateLabel.Color = grayCol

	timeAgoLabel := l.Theme.Body2(TimeAgo(proposal.Timestamp))
	timeAgoLabel.Color = grayCol

	var categoryLabel cryptomaterial.Label
	var categoryLabelColor color.NRGBA
	switch proposal.Category {
	case libwallet.ProposalCategoryApproved:
		categoryLabel = l.Theme.Body2(values.String(values.StrApproved))
		categoryLabelColor = l.Theme.Color.Success
	case libwallet.ProposalCategoryActive:
		categoryLabel = l.Theme.Body2(values.String(values.StrVoting))
		categoryLabelColor = l.Theme.Color.Primary
	case libwallet.ProposalCategoryRejected:
		categoryLabel = l.Theme.Body2(values.String(values.StrRejected))
		categoryLabelColor = l.Theme.Color.Danger
	case libwallet.ProposalCategoryAbandoned:
		categoryLabel = l.Theme.Body2(values.String(values.StrAbandoned))
		categoryLabelColor = grayCol
	case libwallet.ProposalCategoryPre:
		categoryLabel = l.Theme.Body2(values.String(values.StrInDiscussion))
		categoryLabelColor = grayCol
	}
	categoryLabel.Color = categoryLabelColor

	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(0.7, func(gtx C) D {
			lbl := l.Theme.H6(proposal.Name)
			lbl.Font.Weight = font.SemiBold
			return layout.Inset{Top: values.MarginPadding4}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(categoryLabel.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPaddingMinus22}.Layout(gtx, dotLabel.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							if item.Proposal.Category == libwallet.ProposalCategoryPre {
								return layout.Inset{
									Right: values.MarginPadding4,
								}.Layout(gtx, stateLabel.Layout)
							}
							return D{}
						}),
						layout.Rigid(func(gtx C) D {
							if item.Proposal.Category == libwallet.ProposalCategoryActive {
								return layout.Inset{
									Right: values.MarginPadding4,
									Top:   values.MarginPadding3,
								}.Layout(gtx, l.Theme.Icons.TimerIcon.Layout12dp)
							}
							return D{}
						}),
						layout.Rigid(timeAgoLabel.Layout),
						layout.Rigid(func(gtx C) D {
							if item.Proposal.Category == libwallet.ProposalCategoryPre {
								return layout.Inset{Left: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
									rect := image.Rectangle{
										Min: gtx.Constraints.Min,
										Max: gtx.Constraints.Max,
									}
									rect.Max.Y = 20
									layoutInfoTooltip(gtx, rect, *item)

									infoIcon := cryptomaterial.NewIcon(l.Theme.Icons.ActionInfo)
									infoIcon.Color = l.Theme.Color.GrayText2
									return infoIcon.Layout(gtx, values.MarginPadding20)
								})
							}
							return D{}
						}),
					)
				}),
			)
		}),
	)
}

func layoutAuthor(gtx C, l *load.Load, item *ProposalItem) D {
	proposal := item.Proposal
	grayCol := l.Theme.Color.GrayText2

	nameLabel := l.Theme.Body2(proposal.Username)
	nameLabel.Color = grayCol

	dotLabel := l.Theme.H4(" . ")
	dotLabel.Color = grayCol

	versionLabel := l.Theme.Body2(values.String(values.StrVersion) + " " + proposal.Version)
	versionLabel.Color = grayCol

	return layout.Flex{}.Layout(gtx,
		layout.Rigid(nameLabel.Layout),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: values.MarginPaddingMinus22}.Layout(gtx, dotLabel.Layout)
		}),
		layout.Rigid(versionLabel.Layout),
	)
}

func layoutProposalVoteBar(window app.WindowNavigator, gtx C, item *ProposalItem) D {
	proposal := item.Proposal
	yes := float32(proposal.YesVotes)
	no := float32(proposal.NoVotes)
	quorumPercent := float32(proposal.QuorumPercentage)
	passPercentage := float32(proposal.PassPercentage)
	eligibleTickets := float32(proposal.EligibleTickets)

	return item.voteBar.
		SetYesNoVoteParams(yes, no).
		SetVoteValidityParams(eligibleTickets, quorumPercent, passPercentage).
		SetProposalDetails(proposal.NumComments, proposal.PublishedAt, proposal.Token).
		Layout(window, gtx)
}

func layoutInfoTooltip(gtx C, rect image.Rectangle, item ProposalItem) {
	inset := layout.Inset{Top: values.MarginPadding20, Left: values.MarginPaddingMinus195}
	item.tooltip.Layout(gtx, rect, inset, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding195)
		gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding195)
		return item.tooltipLabel.Layout(gtx)
	})
}

func LayoutNoProposalsFound(gtx C, l *load.Load, syncing bool, category int32) D {
	var selectedCategory string
	switch category {
	case libwallet.ProposalCategoryApproved:
		selectedCategory = values.String(values.StrApproved)
	case libwallet.ProposalCategoryRejected:
		selectedCategory = values.String(values.StrRejected)
	case libwallet.ProposalCategoryAbandoned:
		selectedCategory = values.String(values.StrAbandoned)
	default:
		selectedCategory = values.String(values.StrUnderReview)
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	text := l.Theme.Body1(values.StringF(values.StrNoProposals, selectedCategory))
	text.Color = l.Theme.Color.GrayText3
	if syncing {
		text = l.Theme.Body1(values.String(values.StrFetchingProposals))
	}

	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{
			Top:    values.MarginPadding10,
			Bottom: values.MarginPadding10,
		}.Layout(gtx, text.Layout)
	})
}

func LoadProposals(l *load.Load, category, offset, pageSize int32, newestFirst bool) []*ProposalItem {
	proposalItems := make([]*ProposalItem, 0)

	proposals, err := l.WL.AssetsManager.Politeia.GetProposalsRaw(category, offset, pageSize, newestFirst)
	if err == nil {
		for i := 0; i < len(proposals); i++ {
			proposal := proposals[i]
			item := &ProposalItem{
				Proposal: libwallet.Proposal{Proposal: proposal},
				voteBar:  NewVoteBar(l),
			}

			if proposal.Category == libwallet.ProposalCategoryPre {
				tooltipLabel := l.Theme.Caption("")
				tooltipLabel.Color = l.Theme.Color.GrayText2
				if proposal.VoteStatus == 1 {
					tooltipLabel.Text = values.String(values.StrWaitingAuthor)
				} else if proposal.VoteStatus == 2 {
					tooltipLabel.Text = values.String(values.StrWaitingForAdmin)
				}

				item.tooltip = l.Theme.Tooltip()
				item.tooltipLabel = tooltipLabel
			}

			proposalItems = append(proposalItems, item)
		}
	}
	return proposalItems
}
