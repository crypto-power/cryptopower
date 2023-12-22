package seedbackup

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	seedHexFormat  = "HEX"
	seedWordFormat = "Word"
	seedWIFFormat  = "WIF"
	SaveSeedPageID = "save_seed"
)

type saveSeedRow struct {
	rowIndex int
	word1    string
	word2    string
	word3    string
}

type SaveSeedPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	wallet        sharedW.Asset
	pageContainer *widget.List

	backButton   cryptomaterial.IconButton
	actionButton cryptomaterial.Button
	seedList     *widget.List
	hexLabel     cryptomaterial.Label
	copy         cryptomaterial.Button

	infoText string
	seed     string
	rows     []saveSeedRow

	redirectCallback Redirectfunc

	seedFormatRadioGroup *widget.Enum
}

func NewSaveSeedPage(l *load.Load, wallet sharedW.Asset, redirect Redirectfunc) *SaveSeedPage {
	pg := &SaveSeedPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(SaveSeedPageID),
		wallet:           wallet,
		hexLabel:         l.Theme.Label(values.TextSize12, ""),
		copy:             l.Theme.Button(values.String(values.StrCopy)),
		infoText:         values.String(values.StrAskedEnterSeedWords),
		actionButton:     l.Theme.Button(values.String(values.StrWroteAllWords)),
		seedList: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},

		redirectCallback:     redirect,
		seedFormatRadioGroup: new(widget.Enum),
	}

	pg.copy.TextSize = values.TextSize12
	pg.hexLabel.MaxLines = 1
	pg.copy.Background = color.NRGBA{}
	pg.copy.HighlightColor = pg.Theme.Color.SurfaceHighlight
	pg.copy.Color = pg.Theme.Color.Primary
	pg.copy.Inset = layout.UniformInset(values.MarginPadding16)

	pg.backButton, _ = components.SubpageHeaderButtons(l)
	pg.backButton.Icon = l.Theme.Icons.ContentClear

	pg.actionButton.Font.Weight = font.Medium
	pg.pageContainer = &widget.List{
		List: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
	}
	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *SaveSeedPage) OnNavigatedTo() {
	if pg.seedFormatRadioGroup.Value == "" {
		pg.seedFormatRadioGroup.Value = seedHexFormat
	}
	passwordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrConfirmShowSeed)).
		SetPositiveButtonCallback(func(_, password string, m *modal.CreatePasswordModal) bool {
			seed, err := pg.wallet.DecryptSeed(password)
			if err != nil {
				m.SetLoading(false)
				m.SetError(err.Error())
				return false
			}
			m.Dismiss()
			pg.seed = seed

			wordList := strings.Split(seed, " ")
			row1 := wordList[:11]
			row2 := wordList[11:22]
			row3 := wordList[22:]

			// for mobile
			rowMobile1 := wordList[:17]
			rowMobile2 := wordList[17:]
			rows := make([]saveSeedRow, 0)

			if pg.IsMobileView() {
				for i := range rowMobile1 {
					r2 := ""
					if i < len(rowMobile2) {
						r2 = rowMobile2[i]
					}
					rows = append(rows, saveSeedRow{
						rowIndex: i + 1,
						word1:    rowMobile1[i],
						word2:    r2,
					})
				}
			} else {
				for i := range row1 {
					rows = append(rows, saveSeedRow{
						rowIndex: i + 1,
						word1:    row1[i],
						word2:    row2[i],
						word3:    row3[i],
					})
				}
			}
			pg.rows = rows
			return true
		}).
		SetNegativeButtonCallback(func() {
			pg.redirectCallback(pg.Load, pg.ParentWindow())
		}).
		SetCancelable(false)
	pg.ParentWindow().ShowModal(passwordModal)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *SaveSeedPage) HandleUserInteractions() {
	for pg.actionButton.Clicked() {
		pg.ParentNavigator().Display(NewVerifySeedPage(pg.Load, pg.wallet, pg.seed, pg.redirectCallback))
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *SaveSeedPage) OnNavigatedFrom() {}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *SaveSeedPage) Layout(gtx C) D {
	sp := components.SubPage{
		Load:       pg.Load,
		Title:      values.String(values.StrWriteDownSeed),
		SubTitle:   values.String(values.StrStep1),
		BackButton: pg.backButton,
		Back: func() {
			promptToExit(pg.Load, pg.ParentWindow(), pg.redirectCallback)
		},
		Body: func(gtx C) D {
			return pg.Theme.List(pg.pageContainer).Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						label := pg.Theme.Label(values.TextSize16, values.String(values.StrWriteDownAll33Words))
						label.Color = pg.Theme.Color.GrayText1
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						label := pg.Theme.Label(values.TextSize14, values.String(values.StrYourSeedWords))
						label.Color = pg.Theme.Color.GrayText1
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Orientation: layout.Vertical,
							Background:  pg.Theme.Color.Surface,
							Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(8)},
							Margin:      layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding2},
							Padding:     layout.Inset{Top: values.MarginPadding16, Right: values.MarginPadding16, Bottom: values.MarginPadding8, Left: values.MarginPadding16},
						}.Layout(gtx,
							layout.Rigid(label.Layout),
							layout.Rigid(func(gtx C) D {
								return pg.Theme.List(pg.seedList).Layout(gtx, len(pg.rows), func(gtx C, index int) D {
									return pg.desktopSeedRow(gtx, pg.rows[index])
								})
							}),
						)
					}),
					layout.Rigid(pg.hexLayout),
					layout.Rigid(layout.Spacer{Height: values.MarginPadding130}.Layout),
				)
			})
		},
	}
	layout := func(gtx C) D {
		return sp.Layout(pg.ParentWindow(), gtx)
	}
	return container(gtx, pg.IsMobileView(), *pg.Theme, layout, pg.infoText, pg.actionButton, true)
}

func (pg *SaveSeedPage) desktopSeedRow(gtx C, row saveSeedRow) D {
	topMargin := values.MarginPadding8
	if row.rowIndex == 1 {
		topMargin = values.MarginPadding16
	}

	var flexChils []layout.FlexChild
	if pg.IsMobileView() {
		itemWidth := gtx.Constraints.Max.X / 2 // Divide total width into 2 rows for mobile
		flexChils = []layout.FlexChild{
			seedItem(pg.Theme, itemWidth, row.rowIndex, row.word1),
			seedItem(pg.Theme, itemWidth, row.rowIndex+17, row.word2),
		}
	} else {
		itemWidth := gtx.Constraints.Max.X / 3 // Divide total width into 3 rows for deskop
		flexChils = []layout.FlexChild{
			seedItem(pg.Theme, itemWidth, row.rowIndex, row.word1),
			seedItem(pg.Theme, itemWidth, row.rowIndex+11, row.word2),
			seedItem(pg.Theme, itemWidth, row.rowIndex+22, row.word3),
		}
	}
	return cryptomaterial.LinearLayout{
		Width:  cryptomaterial.MatchParent,
		Height: cryptomaterial.WrapContent,
		Margin: layout.Inset{Top: topMargin},
	}.Layout(gtx, flexChils...)
}

func (pg *SaveSeedPage) hexLayout(gtx C) D {
	pg.handleCopyEvent(gtx)
	card := cryptomaterial.Card{
		Color: pg.Theme.Color.Gray4,
	}
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
		Background:  pg.Theme.Color.Surface,
		Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(8)},
		Margin:      layout.Inset{Top: values.MarginPadding0, Bottom: values.MarginPadding16},
		Padding:     layout.Inset{Top: values.MarginPadding5, Right: values.MarginPadding16, Bottom: values.MarginPadding16, Left: values.MarginPadding16},
	}.Layout(gtx,
		layout.Rigid(pg.layoutVoteChoice()),
		layout.Rigid(func(gtx C) D {
			cgtx := gtx
			macro := op.Record(cgtx.Ops)
			copyLayout := pg.copyButtonLayout(cgtx)
			call := macro.Stop()
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Max.X = gtx.Constraints.Max.X - copyLayout.Size.X
					card.Radius = cryptomaterial.CornerRadius{TopRight: 0, TopLeft: 8, BottomRight: 0, BottomLeft: 8}
					return card.Layout(gtx, func(gtx C) D {
						return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
							seedString := pg.seed
							if seedString != "" {
								switch pg.seedFormatRadioGroup.Value {
								case seedHexFormat:
									hexString, _ := components.SeedWordsToHex(pg.seed)
									pg.hexLabel.Text = hexString
								case seedWordFormat:
									pg.hexLabel.Text = pg.seed[:117] + "..."
								}
							}
							return pg.hexLabel.Layout(gtx)
						})
					})
				}),
				layout.Rigid(func(gtx C) D {
					call.Add(gtx.Ops)
					return copyLayout
				}),
			)
		}),
	)
}

func (pg *SaveSeedPage) copyButtonLayout(gtx C) D {
	card := cryptomaterial.Card{
		Color: pg.Theme.Color.Gray4,
	}
	card.Radius = cryptomaterial.CornerRadius{TopRight: 8, TopLeft: 0, BottomRight: 8, BottomLeft: 0}
	return layout.Inset{Left: values.MarginPadding1}.Layout(gtx, func(gtx C) D {
		return card.Layout(gtx, pg.copy.Layout)
	})
}

func (pg *SaveSeedPage) handleCopyEvent(gtx C) {
	if pg.copy.Clicked() {
		if pg.seedFormatRadioGroup.Value == seedWordFormat {
			clipboard.WriteOp{Text: pg.seed}.Add(gtx.Ops)
		} else {
			clipboard.WriteOp{Text: pg.hexLabel.Text}.Add(gtx.Ops)
		}

		pg.copy.Text = values.String(values.StrCopied)
		pg.copy.Color = pg.Theme.Color.Success
		time.AfterFunc(time.Second*3, func() {
			pg.copy.Text = values.String(values.StrCopy)
			pg.copy.Color = pg.Theme.Color.Primary
			pg.ParentWindow().Reload()
		})
	}
}

func seedItem(theme *cryptomaterial.Theme, width, index int, word string) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		if word == "" {
			return D{}
		}
		return cryptomaterial.LinearLayout{
			Width:  width,
			Height: cryptomaterial.WrapContent,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				indexLabel := theme.Label(values.TextSize16, fmt.Sprint(index))
				indexLabel.Color = theme.Color.GrayText1
				indexLabel.Font.Weight = font.Medium
				return cryptomaterial.LinearLayout{
					Width:     gtx.Dp(values.MarginPadding30),
					Height:    gtx.Dp(values.MarginPadding22),
					Direction: layout.Center,
					Margin:    layout.Inset{Right: values.MarginPadding8},
					Border:    cryptomaterial.Border{Radius: cryptomaterial.Radius(9), Color: theme.Color.Gray3, Width: values.MarginPadding1},
				}.Layout2(gtx, indexLabel.Layout)
			}),
			layout.Rigid(layout.Spacer{Width: values.MarginPadding2}.Layout),
			layout.Rigid(func(gtx C) D {
				seedWord := theme.Label(values.TextSize16, word)
				seedWord.Color = theme.Color.GrayText1
				seedWord.Font.Weight = font.Medium
				return seedWord.Layout(gtx)
			}),
		)
	})
}

func (pg *SaveSeedPage) layoutVoteChoice() layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				lbl := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize16), values.String(values.StrCopySeed))
				lbl.Font.Weight = font.SemiBold
				return lbl.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, pg.layoutItems()...)
				})
			}),
		)
	}
}

func (pg *SaveSeedPage) layoutItems() []layout.FlexChild {
	options := make([]layout.FlexChild, 0)

	hexBtn := pg.Theme.RadioButton(pg.seedFormatRadioGroup, seedHexFormat, values.String(values.StrHex), pg.Theme.Color.DeepBlue, pg.Theme.Color.Primary)
	hexRadioItem := layout.Rigid(hexBtn.Layout)
	options = append(options, hexRadioItem)

	wrdBtn := pg.Theme.RadioButton(pg.seedFormatRadioGroup, seedWordFormat, values.String(values.StrWord), pg.Theme.Color.DeepBlue, pg.Theme.Color.Primary)
	wrdRadioItem := layout.Rigid(wrdBtn.Layout)
	options = append(options, wrdRadioItem)

	return options
}
