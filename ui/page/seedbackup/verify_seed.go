package seedbackup

import (
	"math/rand"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const VerifySeedPageID = "verify_seed"

type shuffledSeedWords struct {
	selectedIndex int
	words         []string
	clickables    []*cryptomaterial.Clickable
}

type VerifySeedPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	wallet        sharedW.Asset
	seed          string
	multiSeedList []shuffledSeedWords

	backButton    cryptomaterial.IconButton
	actionButton  cryptomaterial.Button
	listGroupSeed []*layout.List
	list          *widget.List

	redirectCallback Redirectfunc
	toggleSeedInput  *cryptomaterial.Switch
	seedInputEditor  cryptomaterial.Editor
	verifySeedButton cryptomaterial.Button
	wordSeedType     sharedW.WordSeedType
}

func NewVerifySeedPage(l *load.Load, wallet sharedW.Asset, seed string, wordSeedType sharedW.WordSeedType, redirect Redirectfunc) *VerifySeedPage {
	pg := &VerifySeedPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(VerifySeedPageID),
		wallet:           wallet,
		seed:             seed,

		actionButton: l.Theme.Button(values.String(values.StrVerify)),

		redirectCallback: redirect,
		toggleSeedInput:  l.Theme.Switch(),
		wordSeedType:     wordSeedType,
	}
	pg.list = &widget.List{
		List: layout.List{
			Axis: layout.Vertical,
		},
	}

	pg.actionButton.Font.Weight = font.Medium

	pg.backButton = components.GetBackButton(l)
	pg.backButton.Icon = l.Theme.Icons.ContentClear

	pg.seedInputEditor = l.Theme.Editor(new(widget.Editor), values.String(values.StrEnterWalletSeed))
	pg.seedInputEditor.Editor.SingleLine = false
	pg.seedInputEditor.Editor.SetText("")

	pg.verifySeedButton = l.Theme.Button("")
	pg.verifySeedButton.Font.Weight = font.Medium
	pg.verifySeedButton.SetEnabled(false)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *VerifySeedPage) OnNavigatedTo() {
	allSeeds := pg.wordSeedType.AllWords()
	listGroupSeed := make([]*layout.List, 0)
	multiSeedList := make([]shuffledSeedWords, 0)
	seedWords := strings.Split(pg.seed, " ")
	for _, word := range seedWords {
		listGroupSeed = append(listGroupSeed, &layout.List{Axis: layout.Horizontal})
		index := seedPosition(word, allSeeds)
		shuffledSeed := pg.getMultiSeed(index, allSeeds)
		multiSeedList = append(multiSeedList, shuffledSeed)
	}

	pg.multiSeedList = multiSeedList
	pg.listGroupSeed = listGroupSeed
	pg.toggleSeedInput.SetChecked(false)
}

func (pg *VerifySeedPage) getMultiSeed(realSeedIndex int, allSeeds []string) shuffledSeedWords {
	tempAllSeeds := make([]string, len(allSeeds))
	_ = copy(tempAllSeeds, allSeeds)
	shuffledSeed := shuffledSeedWords{
		selectedIndex: -1,
		words:         make([]string, 0),
		clickables:    make([]*cryptomaterial.Clickable, 0),
	}

	clickable := func() *cryptomaterial.Clickable {
		cl := pg.Theme.NewClickable(true)
		cl.Radius = cryptomaterial.Radius(8)
		return cl
	}

	shuffledSeed.words = append(shuffledSeed.words, tempAllSeeds[realSeedIndex])
	shuffledSeed.clickables = append(shuffledSeed.clickables, clickable())
	tempAllSeeds = removeSeed(tempAllSeeds, realSeedIndex)

	for i := 0; i < 3; i++ {
		randomSeed := rand.Intn(len(tempAllSeeds))

		shuffledSeed.words = append(shuffledSeed.words, tempAllSeeds[randomSeed])
		shuffledSeed.clickables = append(shuffledSeed.clickables, clickable())
		tempAllSeeds = removeSeed(tempAllSeeds, randomSeed)
	}

	rand.Shuffle(len(shuffledSeed.words), func(i, j int) {
		shuffledSeed.words[i], shuffledSeed.words[j] = shuffledSeed.words[j], shuffledSeed.words[i]
	})

	return shuffledSeed
}

func seedPosition(seed string, allSeeds []string) int {
	for i := range allSeeds {
		if allSeeds[i] == seed {
			return i
		}
	}
	return -1
}

func removeSeed(allSeeds []string, index int) []string {
	return append(allSeeds[:index], allSeeds[index+1:]...)
}

func (pg *VerifySeedPage) allSeedsSelected() bool {
	for _, multiSeed := range pg.multiSeedList {
		if multiSeed.selectedIndex == -1 {
			return false
		}
	}

	return true
}

func (pg *VerifySeedPage) selectedSeedPhrase() string {
	var wordList []string
	for _, multiSeed := range pg.multiSeedList {
		if multiSeed.selectedIndex != -1 {
			wordList = append(wordList, multiSeed.words[multiSeed.selectedIndex])
		}
	}

	return strings.Join(wordList, " ")
}

func (pg *VerifySeedPage) verifySeed() {
	passwordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrConfirmToVerifySeed)).
		SetPositiveButtonCallback(func(_, password string, m *modal.CreatePasswordModal) bool {
			seed := pg.seedInputEditor.Editor.Text()
			if !pg.toggleSeedInput.IsChecked() {
				seed = pg.selectedSeedPhrase()
			}
			_, err := pg.wallet.VerifySeedForWallet(seed, password)
			if err != nil {
				if err.Error() == utils.ErrInvalid {
					msg := values.String(values.StrSeedValidationFailed)
					errModal := modal.NewErrorModal(pg.Load, msg, modal.DefaultClickFunc())
					pg.ParentWindow().ShowModal(errModal)
					m.Dismiss()
					return false
				}

				m.SetError(err.Error())
				m.ParentWindow().Reload()
				return false
			}

			pg.ParentNavigator().Display(NewBackupSuccessPage(pg.Load, pg.redirectCallback))
			return true
		})
	pg.ParentWindow().ShowModal(passwordModal)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *VerifySeedPage) HandleUserInteractions(gtx C) {
	if pg.toggleSeedInput.Changed(gtx) {
		pg.ParentWindow().Reload()
	}

	for i, multiSeed := range pg.multiSeedList {
		for j, clickable := range multiSeed.clickables {
			if clickable.Clicked(gtx) {
				pg.multiSeedList[i].selectedIndex = j
			}
		}
	}

	if pg.actionButton.Clicked(gtx) {
		if pg.allSeedsSelected() {
			pg.verifySeed()
		}
	}

	if len(strings.TrimSpace(pg.seedInputEditor.Editor.Text())) != 0 {
		pg.verifySeedButton.SetEnabled(true)
	}

	if pg.verifySeedButton.Clicked(gtx) {
		pg.verifySeed()
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *VerifySeedPage) OnNavigatedFrom() {}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *VerifySeedPage) Layout(gtx C) D {
	textSize16 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize16)
	margin16 := values.MarginPaddingTransform(pg.IsMobileView(), values.MarginPadding16)
	sp := components.SubPage{
		Load:       pg.Load,
		Title:      values.String(values.StrVerifySeed),
		SubTitle:   values.String(values.StrStep2of2),
		BackButton: pg.backButton,
		Back: func() {
			promptToExit(pg.Load, pg.ParentWindow(), pg.redirectCallback)
		},
		Body: func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.toggleSeedInput.Layout)
							}),
							layout.Rigid(pg.Theme.Label(textSize16, values.String(values.StrPasteSeedWords)).Layout),
						)
					})
				}),
				layout.Rigid(func(gtx C) D {
					if pg.toggleSeedInput.IsChecked() {
						return D{}
					}
					label := pg.Theme.Label(textSize16, values.String(values.StrSelectPhrasesToVerify))
					label.Color = pg.Theme.Color.GrayText1
					return label.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: values.MarginPadding16}.Layout),
				layout.Rigid(func(gtx C) D {
					if pg.toggleSeedInput.IsChecked() {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.MatchParent,
							Orientation: layout.Vertical,
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return pg.Theme.Card().Layout(gtx, func(gtx C) D {
									return layout.UniformInset(margin16).Layout(gtx, func(gtx C) D {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(pg.seedInputEditor.Layout),
											layout.Rigid(layout.Spacer{Height: values.MarginPadding16}.Layout),
											layout.Rigid(func(gtx C) D {
												gtx.Constraints.Min.X = gtx.Constraints.Max.X
												pg.verifySeedButton.Text = values.String(values.StrVerify)
												return layout.E.Layout(gtx, pg.verifySeedButton.Layout)
											}),
										)
									})
								})
							}),
						)
					}
					return layout.Inset{
						Bottom: values.MarginPadding96,
					}.Layout(gtx, func(gtx C) D {
						return pg.Theme.List(pg.list).Layout(gtx, len(pg.multiSeedList), func(gtx C, i int) D {
							return pg.seedListRow(gtx, i, pg.multiSeedList[i])
						})
					})
				}),
			)
		},
	}

	pg.actionButton.SetEnabled(pg.allSeedsSelected())
	layout := func(gtx C) D {
		return sp.Layout(pg.ParentWindow(), gtx)
	}
	return container(gtx, pg.IsMobileView(), *pg.Theme, layout, "", pg.actionButton, !pg.toggleSeedInput.IsChecked())
}

func (pg *VerifySeedPage) seedListRow(gtx C, index int, multiSeed shuffledSeedWords) D {
	marginPading16 := values.MarginPaddingTransform(pg.IsMobileView(), values.MarginPadding16)
	text := "-"
	if multiSeed.selectedIndex != -1 {
		text = multiSeed.words[multiSeed.selectedIndex]
	}
	seedItem := seedItem(pg.Theme, gtx.Constraints.Max.X, index+1, text)
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
		Background:  pg.Theme.Color.Surface,
		Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(8)},
		Margin:      components.VerticalInset(values.MarginPadding4),
		Padding:     layout.Inset{Top: marginPading16, Right: marginPading16, Bottom: values.MarginPadding8, Left: marginPading16},
	}.Layout(gtx,
		seedItem,
		layout.Rigid(layout.Spacer{Height: marginPading16}.Layout),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			widgets := []layout.Widget{
				func(gtx C) D { return pg.seedButton(gtx, 0, multiSeed) },
				layout.Spacer{Width: values.MarginPadding5}.Layout,
				func(gtx C) D { return pg.seedButton(gtx, 1, multiSeed) },
				layout.Spacer{Width: values.MarginPadding5}.Layout,
				func(gtx C) D { return pg.seedButton(gtx, 2, multiSeed) },
				layout.Spacer{Width: values.MarginPadding5}.Layout,
				func(gtx C) D { return pg.seedButton(gtx, 3, multiSeed) },
			}
			return pg.listGroupSeed[index].Layout(gtx, len(widgets), func(gtx C, i int) D {
				return widgets[i](gtx)
			})
		}),
	)
}

func (pg *VerifySeedPage) seedButton(gtx C, index int, multiSeed shuffledSeedWords) D {
	borderColor := pg.Theme.Color.Gray2
	textColor := pg.Theme.Color.GrayText2
	if index == multiSeed.selectedIndex {
		borderColor = pg.Theme.Color.Primary
		textColor = pg.Theme.Color.Primary
	}

	return multiSeed.clickables[index].Layout(gtx, func(gtx C) D {
		width := values.MarginPadding100
		height := values.MarginPadding40
		if pg.IsMobileView() {
			width = values.MarginPadding85
			height = values.MarginPadding30
		}
		return cryptomaterial.LinearLayout{
			Width:      gtx.Dp(width),
			Height:     gtx.Dp(height),
			Background: pg.Theme.Color.Surface,
			Direction:  layout.Center,
			Border:     cryptomaterial.Border{Radius: cryptomaterial.Radius(8), Color: borderColor, Width: values.MarginPadding2},
		}.Layout2(gtx, func(gtx C) D {
			label := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize16), multiSeed.words[index])
			label.Color = textColor
			return label.Layout(gtx)
		})
	})
}
