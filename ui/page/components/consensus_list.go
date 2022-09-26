package components

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/values"
)

type ConsensusItem struct {
	Agenda     libwallet.Agenda
	VoteButton cryptomaterial.Button
}

func AgendaItemWidget(gtx C, l *load.Load, consensusItem *ConsensusItem) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layoutAgendaStatus(gtx, l, consensusItem.Agenda)
		}),
		layout.Rigid(layoutAgendaDetails(l, consensusItem.Agenda.Description)),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(layoutAgendaDetails(l, values.String(values.StrVotingPreference), text.SemiBold)),
				layout.Rigid(layoutAgendaDetails(l, " "+consensusItem.Agenda.VotingPreference)),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layoutAgendaVoteAction(gtx, l, consensusItem)
		}),
	)
}

func layoutAgendaStatus(gtx C, l *load.Load, agenda libwallet.Agenda) D {

	var statusLabel cryptomaterial.Label
	var statusIcon *cryptomaterial.Icon
	var backgroundColor color.NRGBA

	switch agenda.Status {
	case libwallet.AgendaStatusFinished.String():
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.GreenText
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.NavigationCheck)
		statusIcon.Color = l.Theme.Color.Green500
		backgroundColor = l.Theme.Color.Green50
	case libwallet.AgendaStatusLockedIn.String():
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.GreenText
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.NavigationCheck)
		statusIcon.Color = l.Theme.Color.Green500
		backgroundColor = l.Theme.Color.Green50
	case libwallet.AgendaStatusFailed.String():
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.Text
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.NavigationCancel)
		statusIcon.Color = l.Theme.Color.Gray1
		backgroundColor = l.Theme.Color.Gray2
	case libwallet.AgendaStatusInProgress.String():
		clr := l.Theme.Color.Primary
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = clr
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.NavMoreIcon)
		statusIcon.Color = clr
		backgroundColor = l.Theme.Color.LightBlue
	case libwallet.AgendaStatusUpcoming.String():
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.Text
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.PlayIcon)
		statusIcon.Color = l.Theme.Color.DeepBlue
		backgroundColor = l.Theme.Color.Gray2
	default:
		statusLabel = l.Theme.Label(values.TextSize14, agenda.Status)
		statusLabel.Color = l.Theme.Color.Text
		statusIcon = cryptomaterial.NewIcon(l.Theme.Icons.NavMoreIcon)
		statusIcon.Color = l.Theme.Color.Gray1
		backgroundColor = l.Theme.Color.Gray2
	}

	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := l.Theme.Label(values.TextSize20, agenda.AgendaID)
			lbl.Font.Weight = text.SemiBold
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

func layoutAgendaDetails(l *load.Load, data string, weight ...text.Weight) layout.Widget {
	return func(gtx C) D {
		lbl := l.Theme.Label(values.TextSize16, data)
		lbl.Font.Weight = text.Light
		if len(weight) > 0 {
			lbl.Font.Weight = weight[0]
		}
		return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, lbl.Layout)
	}
}

func layoutAgendaVoteAction(gtx C, l *load.Load, item *ConsensusItem) D {
	if item.Agenda.Status == libwallet.AgendaStatusFinished.String() {
		return D{}
	}
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(unit.Dp(150)), gtx.Dp(unit.Dp(200))
	item.VoteButton.Background = l.Theme.Color.Gray3
	item.VoteButton.SetEnabled(false)
	if item.Agenda.Status == libwallet.AgendaStatusUpcoming.String() || item.Agenda.Status == libwallet.AgendaStatusInProgress.String() {
		item.VoteButton.Background = l.Theme.Color.Primary
		item.VoteButton.SetEnabled(true)
	}
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

func LoadAgendas(l *load.Load, selectedWallet *libwallet.Wallet, newestFirst bool) []*ConsensusItem {
	agendas, err := selectedWallet.AllVoteAgendas("", newestFirst)
	if err != nil {
		return nil
	}
	consensusItems := make([]*ConsensusItem, len(agendas))
	for i := 0; i < len(agendas); i++ {
		consensusItems[i] = &ConsensusItem{
			Agenda:     *agendas[i],
			VoteButton: l.Theme.Button(values.String(values.StrSetChoice)),
		}
	}
	return consensusItems
}

// FetchStrAgendaStatus helps in manipulating what statuses to show and how to show
// them on a dropdown.
func FetchStrAgendaStatus() []string {
	strAgendas := make([]string, len(libwallet.AgendasList))
	for i, v := range libwallet.AgendasList {
		if v == libwallet.UnknownStatus {
			strAgendas[i] = "All"
			continue
		}
		// Capitalizes the first letter.
		strAgendas[i] = cases.Title(language.English).String(v.String())
	}
	return strAgendas
}
