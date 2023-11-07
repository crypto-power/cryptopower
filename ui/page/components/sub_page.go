package components

import (
	"gioui.org/layout"
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
	backButton := l.Theme.IconButton(l.Theme.Icons.NavigationArrowBack)
	infoButton := l.Theme.IconButton(l.Theme.Icons.ActionInfo)

	m24 := values.MarginPadding24
	backButton.Size, infoButton.Size = m24, m24

	buttonInset := layout.UniformInset(values.MarginPadding0)
	backButton.Inset, infoButton.Inset = buttonInset, buttonInset

	return backButton, infoButton
}

func (sp *SubPage) Layout(window app.WindowNavigator, gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: values.MarginPadding22}.Layout(gtx, func(gtx C) D {
				return sp.Header(window, gtx)
			})
		}),
		layout.Rigid(sp.Body),
	)
}

func (sp *SubPage) LayoutWithHeadCard(window app.WindowNavigator, gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return sp.Theme.Card().Layout(gtx, func(gtx C) D {
				inset := layout.Inset{
					Top:   values.MarginPadding16,
					Left:  values.MarginPadding24,
					Right: values.MarginPadding24,
				}
				return inset.Layout(gtx, func(gtx C) D {
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
	sp.EventHandler(window)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Right: values.MarginPadding20,
					}.Layout(gtx, sp.BackButton.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(sp.Load.Theme.LabelSemiBold(values.TextSize20, sp.Title).Layout),
						layout.Rigid(func(gtx C) D {
							if !utils.StringNotEmpty(sp.SubTitle) {
								return D{}
							}

							sub := sp.Load.Theme.Label(values.TextSize14, sp.SubTitle)
							sub.Color = sp.Load.Theme.Color.GrayText2
							return sub.Layout(gtx)
						}),
					)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						if sp.InfoTemplate != "" {
							return sp.InfoButton.Layout(gtx)
						} else if sp.ExtraItem != nil {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									if sp.ExtraText != "" {
										return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
											return sp.Theme.Caption(sp.ExtraText).Layout(gtx)
										})
									}
									return D{}
								}),
								layout.Rigid(func(gtx C) D {
									return sp.ExtraItem.Layout(gtx, sp.Extra)
								}),
							)
						}
						return D{}
					})
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			if sp.ExtraHeader != nil {
				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					return sp.ExtraHeader(gtx)
				})
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

func (sp *SubPage) EventHandler(window app.WindowNavigator) {
	if sp.InfoTemplate != "" {
		if sp.InfoButton.Button.Clicked() {
			infoModal := modal.NewCustomModal(sp.Load).
				Title(sp.Title).
				SetupWithTemplate(sp.InfoTemplate).
				SetContentAlignment(layout.W, layout.W, layout.Center).
				SetCancelable(true).
				PositiveButtonStyle(sp.Theme.Color.Primary, sp.Theme.Color.Surface)
			window.ShowModal(infoModal)
		}
	}

	if sp.BackButton.Button.Clicked() {
		sp.Back()
	}

	if sp.ExtraItem != nil && sp.ExtraItem.Clicked() {
		sp.HandleExtra()
	}
}
