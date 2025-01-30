package settings

import (
	"time"

	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const AboutPageID = "About"

type AboutPage struct {
	*load.Load
	*app.GenericPageModal

	card           cryptomaterial.Card
	container      *layout.List
	versionRow     *cryptomaterial.Clickable
	version        cryptomaterial.Label
	versionValue   cryptomaterial.Label
	buildDate      cryptomaterial.Label
	buildDateValue cryptomaterial.Label
	network        cryptomaterial.Label
	networkValue   cryptomaterial.Label
	license        cryptomaterial.Label
	licenseRow     *cryptomaterial.Clickable
	backButton     cryptomaterial.IconButton
	shadowBox      *cryptomaterial.Shadow

	versionTapCount int       // Tap counter
	lastTapTime     time.Time // Track last tap time
}

func NewAboutPage(l *load.Load) *AboutPage {
	pg := &AboutPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(AboutPageID),
		card:             l.Theme.Card(),
		container:        &layout.List{Axis: layout.Vertical},
		versionRow:       l.Theme.NewClickable(true),
		version:          l.Theme.Body1(values.String(values.StrVersion)),
		versionValue:     l.Theme.Body1(l.AppInfo.Version()),
		buildDate:        l.Theme.Body1(values.String(values.StrBuildDate)),
		buildDateValue:   l.Theme.Body1(l.AppInfo.BuildDate().Format("2006-01-02 15:04:05")),
		network:          l.Theme.Body1(values.String(values.StrNetwork)),
		license:          l.Theme.Body1(values.String(values.StrLicense)),
		licenseRow:       l.Theme.NewClickable(true),
		shadowBox:        l.Theme.Shadow(),
		versionTapCount:  0,
		lastTapTime:      time.Time{},
	}

	pg.licenseRow.Radius = cryptomaterial.BottomRadius(14)
	pg.backButton = components.GetBackButton(l)

	pg.versionRow.Hoverable = false
	pg.versionRow.Radius = cryptomaterial.TopRadius(14)

	col := pg.Theme.Color.GrayText2
	pg.versionValue.Color = col
	pg.buildDateValue.Color = col

	netType := pg.AssetsManager.NetType().Display()
	pg.networkValue = l.Theme.Body1(netType)
	pg.networkValue.Color = col

	return pg
}

func (pg *AboutPage) OnNavigatedTo() {}

func (pg *AboutPage) Layout(gtx C) D {
	return pg.layoutDesktop(gtx)
}

func (pg *AboutPage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(values.MarginPadding20).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pg.pageHeaderLayout),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding20}.Layout(gtx, pg.pageContentLayout)
			}),
		)
	})
}

func (pg *AboutPage) layoutMobile(_ layout.Context) layout.Dimensions {
	return layout.Dimensions{}
}

func (pg *AboutPage) pageHeaderLayout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return layout.W.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding16,
							Top:   values.MarginPadding2,
						}.Layout(gtx, pg.backButton.Layout)
					}),
					layout.Rigid(pg.Theme.Label(values.TextSizeTransform(pg.Load.IsMobileView(), values.TextSize20), values.String(values.StrAbout)).Layout),
				)
			})
		}),
	)
}

func (pg *AboutPage) pageContentLayout(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(values.MarginPadding550)
		if pg.Load.IsMobileView() {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
		}
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		return pg.card.Layout(gtx, pg.layoutRows)
	})
}

func (pg *AboutPage) layoutRows(gtx C) D {
	in := layout.Inset{
		Top:    values.MarginPadding20,
		Bottom: values.MarginPadding20,
		Left:   values.MarginPadding16,
		Right:  values.MarginPadding16,
	}
	w := []func(gtx C) D{
		func(gtx C) D {
			return pg.versionRow.Layout(gtx, func(gtx C) D {
				return components.Container{Padding: in}.Layout(gtx, func(gtx C) D {
					return components.EndToEndRow(gtx, pg.version.Layout, pg.versionValue.Layout)
				})
			})
		},
		func(gtx C) D {
			return components.Container{Padding: in}.Layout(gtx, func(gtx C) D {
				return components.EndToEndRow(gtx, pg.buildDate.Layout, pg.buildDateValue.Layout)
			})
		},
		func(gtx C) D {
			return components.Container{Padding: in}.Layout(gtx, func(gtx C) D {
				return components.EndToEndRow(gtx, pg.network.Layout, pg.networkValue.Layout)
			})
		},
		func(gtx C) D {
			return pg.licenseRow.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return in.Layout(gtx, pg.license.Layout)
					}),
					layout.Flexed(1, func(gtx C) D {
						return layout.E.Layout(gtx, func(gtx C) D {
							return in.Layout(gtx, pg.Theme.NewIcon(pg.Theme.Icons.ChevronRight).Layout24dp)
						})
					}),
				)
			})
		},
	}

	return pg.container.Layout(gtx, len(w), func(gtx C, i int) D {
		return layout.Inset{Bottom: values.MarginPadding3}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(w[i]),
				layout.Rigid(func(gtx C) D {
					if i == len(w)-1 {
						return D{}
					}
					return layout.Inset{
						Left: values.MarginPadding16,
					}.Layout(gtx, pg.Theme.Separator().Layout)
				}),
			)
		})
	})
}

func (pg *AboutPage) HandleUserInteractions(gtx C) {
	if pg.licenseRow.Clicked(gtx) {
		pg.ParentNavigator().Display(NewLicensePage(pg.Load))
	}

	if pg.backButton.Button.Clicked(gtx) {
		pg.ParentNavigator().CloseCurrentPage()
	}

	if pg.versionRow.Clicked(gtx) {
		now := time.Now()
		if now.Sub(pg.lastTapTime) > 5*time.Second {
			pg.versionTapCount = 0
		}

		pg.versionTapCount++
		pg.lastTapTime = now

		if pg.versionTapCount >= 5 {
			pg.versionTapCount = 0
			pg.showSecretModal()
		}
	}
}

func (pg *AboutPage) showSecretModal() {
	secretModal := modal.NewCustomModal(pg.Load).
		SetCancelable(true).
		SetupWithTemplate(modal.StakeyImageTemplate).
		SetContentAlignment(layout.W, layout.Center, layout.Center).
		SetPositiveButtonText("") // No positive button

	pg.ParentWindow().ShowModal(secretModal)
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *AboutPage) OnNavigatedFrom() {}
