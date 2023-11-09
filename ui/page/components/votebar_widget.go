package components

import (
	"fmt"
	"image"
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

// VoteBar widget implements voting stat for proposals.
// VoteBar shows the range/percentage of the yes votes and no votes against the total required.
type VoteBar struct {
	*load.Load

	yesVotes           float32
	noVotes            float32
	eligibleVotes      float32
	totalVotes         int
	requiredPercentage float32
	passPercentage     float32

	token       string
	publishedAt int64
	numComment  int32

	isDisableInfoTitle bool

	yesColor color.NRGBA
	noColor  color.NRGBA

	passTooltip   *cryptomaterial.Tooltip
	quorumTooltip *cryptomaterial.Tooltip

	legendIcon *cryptomaterial.Icon
	infoButton cryptomaterial.IconButton

	BottomExtra layout.Widget
}

var voteBarThumbWidth = 2

func NewVoteBar(l *load.Load) *VoteBar {
	vb := &VoteBar{
		Load: l,

		yesColor:      l.Theme.Color.Success,
		noColor:       l.Theme.Color.Danger,
		passTooltip:   l.Theme.Tooltip(),
		quorumTooltip: l.Theme.Tooltip(),
		legendIcon:    cryptomaterial.NewIcon(l.Theme.Icons.ImageBrightness1),
	}

	_, vb.infoButton = SubpageHeaderButtons(l)
	vb.infoButton.Inset = layout.Inset{}
	vb.infoButton.Size = values.MarginPadding20

	return vb
}

func (v *VoteBar) SetYesNoVoteParams(yesVotes, noVotes float32) *VoteBar {
	v.yesVotes = yesVotes
	v.noVotes = noVotes

	v.totalVotes = int(yesVotes + noVotes)

	return v
}

func (v *VoteBar) SetDisableInfoTitle(isDisable bool) *VoteBar {
	v.isDisableInfoTitle = isDisable

	return v
}

func (v *VoteBar) SetVoteValidityParams(eligibleVotes, requiredPercentage, passPercentage float32) *VoteBar {
	v.eligibleVotes = eligibleVotes
	v.passPercentage = passPercentage
	v.requiredPercentage = requiredPercentage

	return v
}

func (v *VoteBar) SetProposalDetails(numComment int32, publishedAt int64, token string) *VoteBar {
	v.numComment = numComment
	v.publishedAt = publishedAt
	v.token = token

	return v
}

func (v *VoteBar) SetBottomLayout(lay layout.Widget) *VoteBar {
	v.BottomExtra = lay
	return v
}

func (v *VoteBar) votebarLayout(gtx C) D {
	var rW, rE int
	r := gtx.Dp(values.MarginPadding4)
	progressBarWidth := gtx.Constraints.Max.X
	quorumRequirement := (v.requiredPercentage / 100) * v.eligibleVotes

	yesVotes := 0
	noVotes := 0
	if quorumRequirement > 0 {
		yesVotes = int((v.yesVotes / quorumRequirement) * 100)
		noVotes = int((v.noVotes / quorumRequirement) * 100)
	}

	yesWidth := (progressBarWidth / 100) * yesVotes
	noWidth := (progressBarWidth / 100) * noVotes

	// progressScale represent the different progress bar layers
	progressScale := func(width int, color color.NRGBA, layer int) layout.Dimensions {
		maxHeight := values.MarginPadding17
		rW, rE = 0, 0
		if layer == 2 {
			if width >= progressBarWidth {
				rE = r
			}
			rW = r
		} else if layer == 3 {
			if v.yesVotes == 0 {
				rW = r
			}
			rE = r
		} else {
			rE, rW = r, r
		}
		d := image.Point{X: width, Y: gtx.Dp(maxHeight)}

		defer clip.RRect{
			Rect: image.Rectangle{Max: image.Point{X: width, Y: gtx.Dp(maxHeight)}},
			NE:   rE, NW: rW, SE: rE, SW: rW,
		}.Push(gtx.Ops).Pop()

		paint.ColorOp{Color: color}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)

		return layout.Dimensions{
			Size: d,
		}
	}

	if yesWidth > progressBarWidth || noWidth > progressBarWidth || (yesWidth+noWidth) > progressBarWidth {
		totalVotes := float32(v.totalVotes)
		yes := (v.yesVotes / totalVotes) * 100
		no := (v.noVotes / totalVotes) * 100
		noWidth = int((float32(progressBarWidth) / 100) * no)
		yesWidth = int((float32(progressBarWidth) / 100) * yes)
		rE = r
	} else if yesWidth < 0 {
		yesWidth, noWidth = 0, 0
	}

	return layout.Stack{Alignment: layout.W}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return progressScale(progressBarWidth, v.Theme.Color.Gray2, 1)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if yesWidth == 0 {
						return D{}
					}
					return progressScale(yesWidth, v.yesColor, 2)
				}),
				layout.Rigid(func(gtx C) D {
					if noWidth == 0 {
						return D{}
					}
					return progressScale(noWidth, v.noColor, 3)
				}),
			)
		}),
		layout.Stacked(v.requiredYesVotesIndicator),
	)
}

func (v *VoteBar) votesIndicatorTooltip(gtx C, r image.Rectangle, tipPos float32) {
	insetLeft := tipPos - float32(voteBarThumbWidth/2) - 205
	inset := layout.Inset{Left: unit.Dp(insetLeft), Top: values.MarginPadding25}
	v.passTooltip.Layout(gtx, r, inset, func(gtx C) D {
		txt := values.StringF(values.StrVoteTooltip, int(v.passPercentage))
		return v.Theme.Caption(txt).Layout(gtx)
	})
}

func (v *VoteBar) requiredYesVotesIndicator(gtx C) D {
	thumbLeftPos := (v.passPercentage / 100) * float32(gtx.Constraints.Max.X)
	rect := image.Rectangle{
		Min: image.Point{
			X: int(thumbLeftPos - float32(voteBarThumbWidth/2)),
			Y: -1,
		},
		Max: image.Point{
			X: int(thumbLeftPos) + voteBarThumbWidth,
			Y: 24,
		},
	}
	defer clip.Rect(rect).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, v.Theme.Color.Gray3)
	v.votesIndicatorTooltip(gtx, rect, thumbLeftPos)

	return D{
		Size: rect.Max,
	}
}

func (v *VoteBar) Layout(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding5, Bottom: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								yesLabel := v.Theme.Body1(values.String(values.StrYes) + values.String(values.StrColon))
								return v.layoutIconAndText(gtx, yesLabel, int(v.yesVotes), v.yesColor)
							}),
							layout.Rigid(func(gtx C) D {
								noLabel := v.Theme.Body1(values.String(values.StrNo) + values.String(values.StrColon))
								return v.layoutIconAndText(gtx, noLabel, int(v.noVotes), v.noColor)
							}),
							layout.Flexed(1, func(gtx C) D {
								if v.isDisableInfoTitle {
									return D{}
								}

								return layout.E.Layout(gtx, func(gtx C) D {
									lb := v.Theme.Body1(values.StringF(values.StrVotes, v.totalVotes))
									lb.Font.Weight = font.SemiBold
									return lb.Layout(gtx)
								})
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, v.votebarLayout)
					}),

					layout.Rigid(func(gtx C) D {
						if v.BottomExtra == nil {
							return D{}
						}
						return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, v.BottomExtra)
					}),
				)
			})
		}),
	)
}

func (v *VoteBar) layoutIconAndText(gtx C, lbl cryptomaterial.Label, count int, col color.NRGBA) D {
	return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Right: values.MarginPadding5, Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
					v.legendIcon.Color = col
					return v.legendIcon.Layout(gtx, values.MarginPadding10)
				})
			}),
			layout.Rigid(func(gtx C) D {
				lbl.Font.Weight = font.SemiBold
				return lbl.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				count := float64(count)
				percentage := 0.0
				if v.totalVotes != 0 && count != 0 {
					percentage = (count / float64(v.totalVotes)) * 100
				}
				percentageStr := strconv.FormatFloat(percentage, 'f', 1, 64) + "%"
				countStr := strconv.FormatFloat(count, 'f', 0, 64)

				return v.Theme.Body1(fmt.Sprintf("%s (%s)", countStr, percentageStr)).Layout(gtx)
			}),
		)
	})
}
