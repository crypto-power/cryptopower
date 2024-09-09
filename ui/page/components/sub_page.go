package components

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

type SubPage struct {
	*load.Load
	Title        string
	SubTitle     string
	Back         func()
	Body         layout.Widget
	InfoTemplate string
	ExtraItem    *cryptomaterial.Clickable
	Extra        layout.Widget
	ExtraText    string
	HandleExtra  func()
	ExtraHeader  layout.Widget

	BackButton cryptomaterial.IconButton
	InfoButton cryptomaterial.IconButton
}

func SubpageHeaderButtons(l *load.Load) (cryptomaterial.IconButton, cryptomaterial.IconButton) {
	backClickable := new(widget.Clickable)
	backButton := l.Theme.NewIconButton(l.Theme.Icons.NavigationArrowBack, backClickable)
	infoButton := l.Theme.IconButton(l.Theme.Icons.ActionInfo)

	size := values.MarginPadding24
	if l.IsMobileView() {
		size = values.MarginPadding16
	}
	backButton.Size, infoButton.Size = size, size

	buttonInset := layout.UniformInset(values.MarginPadding0)
	backButton.Inset, infoButton.Inset = buttonInset, buttonInset

	return backButton, infoButton
}

func GetBackButton(l *load.Load) cryptomaterial.IconButton {
	backClickable := new(widget.Clickable)
	backButton := l.Theme.NewIconButton(l.Theme.Icons.NavigationArrowBack, backClickable)
	size := values.MarginPadding24
	if l.IsMobileView() {
		size = values.MarginPadding20
	}
	backButton.Size = size
	backButton.Inset = layout.UniformInset(values.MarginPadding0)
	l.Theme.AddBackClick(backClickable)
	return backButton
}

func (sp *SubPage) Layout(window app.WindowNavigator, gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: values.MarginPadding22}.Layout(gtx, func(gtx C) D {
				return sp.Header(window, gtx)
			})
		}),
		layout.Flexed(1, sp.Body),
	)
}

func (sp *SubPage) LayoutWithHeadCard(window app.WindowNavigator, gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			horizontalPadding := values.MarginPadding24
			if sp.IsMobileView() {
				horizontalPadding = 16
			}
			return sp.Theme.Card().Layout(gtx, func(gtx C) D {
				return layout.Inset{
					Top:   values.MarginPadding16,
					Left:  horizontalPadding,
					Right: horizontalPadding,
				}.Layout(gtx, func(gtx C) D {
					return sp.Header(window, gtx)
				})
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, sp.Body)
		}),
	)
}

func (sp *SubPage) Header(window app.WindowNavigator, gtx C) D {
	sp.EventHandler(gtx, window)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Right: values.MarginPadding4,
					}.Layout(gtx, sp.BackButton.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(sp.Load.Theme.SemiBoldLabelWithSize(sp.ConvertTextSize(values.TextSize20), sp.Title).Layout),
						layout.Rigid(func(gtx C) D {
							if !utils.StringNotEmpty(sp.SubTitle) {
								return D{}
							}

							sub := sp.Load.Theme.Label(sp.ConvertTextSize(values.TextSize14), sp.SubTitle)
							sub.Color = sp.Load.Theme.Color.GrayText2
							return sub.Layout(gtx)
						}),
					)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						if sp.InfoTemplate != "" {
							return sp.InfoButton.Layout(gtx)
						} else if sp.ExtraItem == nil {
							return D{}
						}
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								if sp.ExtraText != "" {
									return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, sp.Theme.Caption(sp.ExtraText).Layout)
								}
								return D{}
							}),
							layout.Rigid(func(gtx C) D {
								return sp.ExtraItem.Layout(gtx, sp.Extra)
							}),
						)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			if sp.ExtraHeader != nil {
				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, sp.ExtraHeader)
			}
			return D{}
		}),
	)
}

func (sp *SubPage) CombinedLayout(window app.WindowNavigator, gtx C) D {
	return sp.Theme.Card().Layout(gtx, func(gtx C) D {
		return layout.Inset{Bottom: values.MarginPadding24}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.UniformInset(values.MarginPadding24).Layout(gtx, func(gtx C) D {
						return sp.Header(window, gtx)
					})
				}),
				layout.Rigid(sp.Body),
			)
		})
	})
}

func (sp *SubPage) EventHandler(gtx C, window app.WindowNavigator) {
	if sp.InfoTemplate != "" {
		if sp.InfoButton.Button.Clicked(gtx) {
			infoModal := modal.NewCustomModal(sp.Load).
				Title(sp.Title).
				SetupWithTemplate(sp.InfoTemplate).
				SetContentAlignment(layout.W, layout.W, layout.Center).
				SetCancelable(true).
				PositiveButtonStyle(sp.Theme.Color.Primary, sp.Theme.Color.Surface)
			window.ShowModal(infoModal)
		}
	}

	if sp.BackButton.Button.Clicked(gtx) {
		sp.Back()
	}

	if sp.ExtraItem != nil && sp.ExtraItem.Clicked(gtx) && sp.HandleExtra != nil {
		sp.HandleExtra()
	}
}
