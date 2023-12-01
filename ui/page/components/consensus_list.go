package components

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

type ConsensusItem struct {
	Agenda     *dcr.Agenda
	VoteButton cryptomaterial.Button
}

func AgendaItemWidget(gtx C, l *load.Load, consensusItem *ConsensusItem, hasVotingWallet bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layoutAgendaStatus(gtx, l, consensusItem.Agenda)
		}),
		layout.Rigid(layoutAgendaDetails(l, consensusItem.Agenda.Description)),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(layoutAgendaDetails(l, values.String(values.StrVotingPreference), font.SemiBold)),
				layout.Rigid(layoutAgendaDetails(l, " "+consensusItem.Agenda.VotingPreference)),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layoutAgendaVoteAction(gtx, l, consensusItem, hasVotingWallet)
		}),
	)
}

func layoutAgendaStatus(gtx C, l *load.Load, agenda *dcr.Agenda) D {
	var statusLabel cryptomaterial.Label
	var statusIcon *cryptomaterial.Icon
	var backgroundColor color.NRGBA

	switch agenda.Status {
	case dcr.AgendaStatusFinished.String():
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.GreenText
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.NavigationCheck)
		statusIcon.Color = l.Theme.Color.Green500
		backgroundColor = l.Theme.Color.Green50
	case dcr.AgendaStatusLockedIn.String():
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.GreenText
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.NavigationCheck)
		statusIcon.Color = l.Theme.Color.Green500
		backgroundColor = l.Theme.Color.Green50
	case dcr.AgendaStatusFailed.String():
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.Text
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.NavigationCancel)
		statusIcon.Color = l.Theme.Color.Gray1
		backgroundColor = l.Theme.Color.Gray2
	case dcr.AgendaStatusInProgress.String():
		clr := l.Theme.Color.Primary
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = clr
		statusIcon = cryptomaterial.NewIcon(l.Theme.NavMoreIcon)
		statusIcon.Color = clr
		backgroundColor = l.Theme.Color.LightBlue
	case dcr.AgendaStatusUpcoming.String():
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.Text
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.PlayIcon)
		statusIcon.Color = l.Theme.Color.DeepBlue
		backgroundColor = l.Theme.Color.Gray2
	default:
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.Text
		statusIcon = cryptomaterial.NewIcon(l.Theme.NavMoreIcon)
		statusIcon.Color = l.Theme.Color.Gray1
		backgroundColor = l.Theme.Color.Gray2
	}

	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := l.Theme.Label(values.TextSize20, agenda.AgendaID)
			lbl.Font.Weight = font.SemiBold
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(lbl.Layout),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Background: backgroundColor,
				Width:      cryptomaterial.WrapContent,
				Height:     cryptomaterial.WrapContent,
				Direction:  layout.Center,
				Alignment:  layout.Middle,
				Border:     cryptomaterial.Border{Color: backgroundColor, Width: values.MarginPadding1, Radius: cryptomaterial.Radius(10)},
				Padding:    layout.Inset{Top: values.MarginPadding3, Bottom: values.MarginPadding3, Left: values.MarginPadding8, Right: values.MarginPadding8},
				Margin:     layout.Inset{Left: values.MarginPadding10},
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Right: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
						return statusIcon.Layout(gtx, values.MarginPadding16)
					})
				}),
				layout.Rigid(statusLabel.Layout))
		}),
	)
}

func layoutAgendaDetails(l *load.Load, data string, weight ...font.Weight) layout.Widget {
	return func(gtx C) D {
		lbl := l.Theme.Label(values.TextSize16, data)
		lbl.Font.Weight = font.Light
		if len(weight) > 0 {
			lbl.Font.Weight = weight[0]
		}
		return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, lbl.Layout)
	}
}

func layoutAgendaVoteAction(gtx C, l *load.Load, item *ConsensusItem, hasVotingWallet bool) D {
	if item.Agenda.Status == dcr.AgendaStatusFinished.String() {
		return D{}
	}

	hasVotingStatus := item.Agenda.Status == dcr.AgendaStatusUpcoming.String() || item.Agenda.Status == dcr.AgendaStatusInProgress.String()
	canVote := hasVotingWallet && hasVotingStatus
	item.VoteButton.SetEnabled(canVote)
	if canVote {
		item.VoteButton.Background = l.Theme.Color.Primary
	} else {
		item.VoteButton.Background = l.Theme.Color.Gray3
	}

	gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding150), gtx.Dp(values.MarginPadding200)
	return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, item.VoteButton.Layout)
}

func LayoutNoAgendasFound(gtx C, l *load.Load, syncing bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	text := l.Theme.Body1(values.String(values.StrNoAgendaYet))
	text.Color = l.Theme.Color.GrayText3
	if syncing {
		text = l.Theme.Body1(values.String(values.StrFetchingAgenda))
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{
			Top:    values.MarginPadding10,
			Bottom: values.MarginPadding10,
		}.Layout(gtx, text.Layout)
	})
}

func LoadAgendas(l *load.Load, dcrWallet *dcr.Asset, newestFirst bool) []*ConsensusItem {
	agendas, err := l.AssetsManager.AllVoteAgendas(newestFirst)
	if err != nil {
		return nil
	}

	var walletChoices map[string]string
	if dcrWallet != nil {
		walletChoices, err = dcrWallet.AgendaChoices("")
		if err != nil {
			return nil
		}
	}

	consensusItems := make([]*ConsensusItem, len(agendas))
	for i := range agendas {
		agenda := agendas[i]
		if voteChoice, ok := walletChoices[agenda.AgendaID]; ok {
			agenda.VotingPreference = voteChoice
		} else {
			agenda.VotingPreference = "-"
		}
		consensusItems[i] = &ConsensusItem{
			Agenda:     agenda,
			VoteButton: l.Theme.Button(values.String(values.StrSetChoice)),
		}
	}

	return consensusItems
}
