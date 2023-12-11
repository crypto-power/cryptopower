package components

import (
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

type TreasuryItem struct {
	Policy            dcr.TreasuryKeyPolicy
	OptionsRadioGroup *widget.Enum
	VoteChoices       [3]string
	SetChoiceButton   cryptomaterial.Button
}

func (t *TreasuryItem) SetVoteChoices(voteChoices [3]string) {
	t.VoteChoices = voteChoices
}

func TreasuryItemWidget(gtx C, l *load.Load, treasuryItem *TreasuryItem) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	axis := layout.Horizontal
	if l.IsMobileView() {
		axis = layout.Vertical
	}
	return layout.Flex{Axis: axis, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := l.Theme.Label(l.ConvertTextSize(values.TextSize18), values.String(values.StrSetTreasuryPolicy))
			lbl.Font.Weight = font.SemiBold
			return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, lbl.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, layoutItems(l, treasuryItem)...)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return layoutPolicyVoteAction(gtx, l, treasuryItem)
				}),
			)
		}),
	)
}

func layoutItems(l *load.Load, treasuryItem *TreasuryItem) []layout.FlexChild {
	voteChoices := [...]string{
		strings.ToLower(values.String(values.StrYes)),
		strings.ToLower(values.String(values.StrNo)),
		strings.ToLower(values.String(values.StrAbstain)),
	}
	items := make([]layout.FlexChild, 0)
	for _, voteChoice := range voteChoices {
		radioBtn := l.Theme.RadioButton(treasuryItem.OptionsRadioGroup, voteChoice, voteChoice, l.Theme.Color.DeepBlue, l.Theme.Color.Primary)
		radioBtn.TextSize = l.ConvertTextSize(values.TextSize16)
		radioItem := layout.Rigid(radioBtn.Layout)
		items = append(items, radioItem)
	}

	return items
}

func layoutPolicyVoteAction(gtx C, l *load.Load, treasuryItem *TreasuryItem) D {
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding100), gtx.Dp(values.MarginPadding150)
	treasuryItem.SetChoiceButton.Background = l.Theme.Color.Gray3
	treasuryItem.SetChoiceButton.SetEnabled(false)

	if treasuryItem.OptionsRadioGroup.Value != "" && treasuryItem.OptionsRadioGroup.Value != treasuryItem.Policy.Policy {
		treasuryItem.SetChoiceButton.Background = l.Theme.Color.Primary
		treasuryItem.SetChoiceButton.SetEnabled(true)
	}
	return treasuryItem.SetChoiceButton.Layout(gtx)
}

func LayoutNoPoliciesFound(gtx C, l *load.Load, syncing bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	text := values.String(values.StrNoPoliciesYet)
	if syncing {
		text = values.String(values.StrFetchingPolicies)
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		lbl := l.Theme.Body1(text)
		lbl.Color = l.Theme.Color.GrayText3
		return layout.Inset{
			Top:    values.MarginPadding10,
			Bottom: values.MarginPadding10,
		}.Layout(gtx, lbl.Layout)
	})
}

func LoadPolicies(l *load.Load, selectedDCRWallet *dcr.Asset, pikey string) []*TreasuryItem {
	policies, err := selectedDCRWallet.TreasuryPolicies(pikey, "")
	if err != nil {
		return nil
	}

	treasuryItems := make([]*TreasuryItem, len(policies))
	for i := 0; i < len(policies); i++ {
		button := l.Theme.Button(values.String(values.StrSetChoice))
		button.TextSize = l.ConvertTextSize(values.TextSize16)
		treasuryItems[i] = &TreasuryItem{
			Policy:            *policies[i],
			OptionsRadioGroup: new(widget.Enum),
			SetChoiceButton:   button,
		}

		treasuryItems[i].OptionsRadioGroup.Value = treasuryItems[i].Policy.Policy
	}
	return treasuryItems
}
