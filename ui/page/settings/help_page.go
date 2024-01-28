package settings

import (
	"image"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const HelpPageID = "Help"

type cardItem struct {
	Clickable *cryptomaterial.Clickable
	Image     *cryptomaterial.Image
	Title     string
	Link      string
}

type HelpPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	documentation,
	twitterClickable,
	matrixClickable,
	websiteClickable,
	telegramClickable *cryptomaterial.Clickable
	copyRedirectURL *cryptomaterial.Clickable
	shadowBox       *cryptomaterial.Shadow
	backButton      cryptomaterial.IconButton

	pageContainer layout.List
	helpPageCard  []cardItem
}

func NewHelpPage(l *load.Load) *HelpPage {
	pg := &HelpPage{
		Load:              l,
		GenericPageModal:  app.NewGenericPageModal(HelpPageID),
		documentation:     l.Theme.NewClickable(true),
		twitterClickable:  l.Theme.NewClickable(true),
		matrixClickable:   l.Theme.NewClickable(true),
		websiteClickable:  l.Theme.NewClickable(true),
		telegramClickable: l.Theme.NewClickable(true),
		copyRedirectURL:   l.Theme.NewClickable(false),
	}

	pg.shadowBox = l.Theme.Shadow()
	pg.shadowBox.SetShadowRadius(14)

	pg.documentation.Radius = cryptomaterial.Radius(14)
	pg.backButton = components.GetBackButton(l)

	axis := layout.Horizontal
	if l.IsMobileView() {
		axis = layout.Vertical
	}
	pg.pageContainer = layout.List{
		Axis:      axis,
		Alignment: layout.Middle,
	}

	pg.helpPageCard = []cardItem{
		{
			Clickable: pg.documentation,
			Image:     l.Theme.Icons.DocumentationIcon,
			Title:     values.String(values.StrDocumentation),
			Link:      "https://docs.decred.org",
		},
		{
			Clickable: pg.matrixClickable,
			Image:     l.Theme.Icons.MatrixIcon,
			Title:     values.String(values.StrMatrix),
			Link:      "https://matrix.to/#/#cryptopower:decred.org",
		},
		{
			Clickable: pg.twitterClickable,
			Image:     l.Theme.Icons.TwitterIcon,
			Title:     values.String(values.StrTwitter),
			Link:      "https://twitter.com/cryptopowerwlt",
		},
		{
			Clickable: pg.telegramClickable,
			Image:     l.Theme.Icons.TelegramIcon,
			Title:     values.String(values.StrTelegram),
			Link:      "https://t.me/cryptopowerwallet",
		},
		{
			Clickable: pg.websiteClickable,
			Image:     l.Theme.Icons.WebsiteIcon,
			Title:     values.String(values.StrWebsite),
			Link:      "https://cryptopower.dev",
		},
	}

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *HelpPage) OnNavigatedTo() {

}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *HelpPage) Layout(gtx C) D {
	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *HelpPage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(values.MarginPadding20).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.pageHeaderLayout),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					pg.pageContentLayout(),
				)
			}),
		)
	})
}

func (pg *HelpPage) layoutMobile(gtx layout.Context) layout.Dimensions {
	body := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrHelp),
			SubTitle:   values.String(values.StrHelpInfo),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					pg.pageContentLayout(),
				)
			},
		}
		return sp.Layout(pg.ParentWindow(), gtx)
	}
	return components.UniformMobile(gtx, false, false, body)
}

func (pg *HelpPage) pageHeaderLayout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.W.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding16,
							Top:   values.MarginPaddingMinus2,
						}.Layout(gtx, pg.backButton.Layout)
					}),
					layout.Rigid(pg.Theme.Label(values.TextSize20, values.String(values.StrHelp)).Layout),
				)
			})
		}),
	)
}

func (pg *HelpPage) pageContentLayout() layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
			axis := layout.Horizontal
			if pg.IsMobileView() {
				axis = layout.Vertical
			}
			return layout.Flex{Axis: axis, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return cryptomaterial.UniformPadding(gtx, func(gtx C) D {
						return pg.pageContainer.Layout(gtx, len(pg.helpPageCard), func(gtx C, i int) D {
							return layout.Inset{Left: values.MarginPadding2, Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
								return pg.pageSections(gtx, pg.helpPageCard[i].Image, pg.helpPageCard[i].Clickable, pg.helpPageCard[i].Title)
							})
						})
					})
				}),
			)
		})
	})
}

func (pg *HelpPage) pageSections(gtx C, icon *cryptomaterial.Image, action *cryptomaterial.Clickable, title string) D {
	return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
		width := gtx.Dp(values.MarginPadding140)
		if pg.IsMobileView() {
			width = cryptomaterial.MatchParent
		}
		return cryptomaterial.LinearLayout{
			Orientation: layout.Vertical,
			Width:       width,
			Height:      cryptomaterial.WrapContent,
			Background:  pg.Theme.Color.Surface,
			Clickable:   action,
			Alignment:   layout.Middle,
			Shadow:      pg.shadowBox,
			Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
			Padding:     layout.UniformInset(values.MarginPadding15),
			Margin: layout.Inset{
				Bottom: values.MarginPadding4,
				Top:    values.MarginPadding4,
			},
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return icon.Layout24dp(gtx)
			}),
			layout.Rigid(pg.Theme.Body1(title).Layout),
			layout.Rigid(func(gtx C) D {
				size := image.Point{X: gtx.Constraints.Max.X, Y: gtx.Constraints.Min.Y}
				return D{Size: size}
			}),
		)
	})
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *HelpPage) HandleUserInteractions() {
	for _, cardItem := range pg.helpPageCard {
		if cardItem.Clickable.Clicked() {
			decredURL := cardItem.Link
			info := modal.NewCustomModal(pg.Load).
				Title("View " + cardItem.Title).
				Body(values.String(values.StrCopyLink)).
				SetCancelable(true).
				UseCustomWidget(func(gtx C) D {
					return layout.Stack{}.Layout(gtx,
						layout.Stacked(func(gtx C) D {
							border := widget.Border{Color: pg.Theme.Color.Gray4, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
							wrapper := pg.Theme.Card()
							wrapper.Color = pg.Theme.Color.Gray4
							return border.Layout(gtx, func(gtx C) D {
								return wrapper.Layout(gtx, func(gtx C) D {
									return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
										return layout.Flex{}.Layout(gtx,
											layout.Flexed(0.9, pg.Theme.Body1(decredURL).Layout),
											layout.Flexed(0.1, func(gtx C) D {
												return layout.E.Layout(gtx, func(gtx C) D {
													return layout.Inset{Top: values.MarginPadding7}.Layout(gtx, func(gtx C) D {
														if pg.copyRedirectURL.Clicked() {
															clipboard.WriteOp{Text: decredURL}.Add(gtx.Ops)
															pg.Toast.Notify(values.String(values.StrCopied))
														}
														return pg.copyRedirectURL.Layout(gtx, pg.Theme.Icons.CopyIcon.Layout24dp)
													})
												})
											}),
										)
									})
								})
							})
						}),
						layout.Stacked(func(gtx C) D {
							return layout.Inset{
								Top:  values.MarginPaddingMinus10,
								Left: values.MarginPadding10,
							}.Layout(gtx, func(gtx C) D {
								label := pg.Theme.Body2(values.String(values.StrWebURL))
								label.Color = pg.Theme.Color.GrayText2
								return label.Layout(gtx)
							})
						}),
					)
				}).
				SetPositiveButtonText(values.String(values.StrGotIt))
			pg.ParentWindow().ShowModal(info)
		}
	}

	if pg.backButton.Button.Clicked() {
		pg.ParentNavigator().CloseCurrentPage()
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *HelpPage) OnNavigatedFrom() {}
