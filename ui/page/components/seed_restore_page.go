package components

import (
	"fmt"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	SeedRestorePageID    = "seed_restore"
	defaultNumberOfSeeds = 32
)

type seedEditors struct {
	focusIndex int
	editors    []*cryptomaterial.RestoreEditor
}

type seedItemMenu struct {
	text   string
	button cryptomaterial.Button
}

type SeedRestore struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the ParentNavigator that displayed this page
	// and the root WindowNavigator. The ParentNavigator is also the root
	// WindowNavigator if this page is displayed from the StartPage, otherwise
	// the ParentNavigator is the MainPage.
	*app.GenericPageModal
	isRestoring     bool
	restoreComplete func()

	seedList *layout.List

	validateSeed    cryptomaterial.Button
	resetSeedFields cryptomaterial.Button
	optionsMenuCard cryptomaterial.Card
	window          app.WindowNavigator

	suggestions []string
	// allSuggestions []string
	seedMenu []seedItemMenu

	seedPhrase string
	walletName string

	openPopupIndex  int
	selected        int
	suggestionLimit int

	seedClicked  bool
	isLastEditor bool

	seedEditors              seedEditors
	nextcurrentCaretPosition int // caret position when seed editor is switched
	currentCaretPosition     int // current caret position
	selectedSeedEditor       int // stores the current focus index of seed editors

	walletType      libutils.AssetType
	getWordSeedType func() sharedW.WordSeedType
}

func NewSeedRestorePage(l *load.Load, walletName string, walletType libutils.AssetType, onRestoreComplete func(), getWordSeedType func() sharedW.WordSeedType) *SeedRestore {
	pg := &SeedRestore{
		Load:            l,
		restoreComplete: onRestoreComplete,
		seedList:        &layout.List{Axis: layout.Vertical},
		suggestionLimit: 3,
		openPopupIndex:  -1,
		walletName:      walletName,
		walletType:      walletType,
		getWordSeedType: getWordSeedType,
	}

	pg.optionsMenuCard = cryptomaterial.Card{Color: pg.Theme.Color.Surface}
	pg.optionsMenuCard.Radius = cryptomaterial.Radius(8)

	pg.validateSeed = l.Theme.Button(values.String(values.StrValidateWalSeed))
	pg.validateSeed.Font.Weight = font.Medium

	pg.resetSeedFields = l.Theme.OutlineButton(values.String(values.StrClearAll))
	pg.resetSeedFields.Font.Weight = font.Medium

	for i := 0; i <= defaultNumberOfSeeds; i++ {
		widgetEditor := new(widget.Editor)
		widgetEditor.SingleLine, widgetEditor.Submit = true, true
		pg.seedEditors.editors = append(pg.seedEditors.editors, l.Theme.RestoreEditor(widgetEditor, "", fmt.Sprintf("%d", i+1)))
	}

	// TODO07 need handle
	// pg.setEditorFocus()

	// init suggestion buttons
	pg.initSeedMenu()

	return pg
}

// ID is a unique string that identifies the page and may be used
// to differentiate this page from other pages.
// Part of the load.Page interface.
func (pg *SeedRestore) ID() string {
	return CreateRestorePageID
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *SeedRestore) OnNavigatedTo() {
	//TODO07 need handle
	// pg.setEditorFocus()
}

func (pg *SeedRestore) SetParentNav(window app.WindowNavigator) {
	pg.window = window
}

func (pg *SeedRestore) setEditorFocus(gtx C) {
	if !pg.IsIOS() {
		pg.seedEditors.focusIndex = -1
		//TODO07
		gtx.Execute(key.FocusCmd{Tag: &pg.seedEditors.editors[0].Edit.Editor})
		// pg.seedEditors.editors[0].Edit.Editor.Focus()
	}
}

func (pg *SeedRestore) seedEditorsHandle(gtx C) {
	if !pg.Load.IsMobileView() {
		return
	}
	for i := range pg.seedEditors.editors {
		if pg.seedEditors.editors[i].Edit.FirstPressed(gtx) {
			pg.seedList.ScrollTo(i)
			if i >= 28 {
				//TODO07
				// key.SoftKeyboardOp{Show: true}.Add(gtx.Ops)
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			} else {
				gtx.Execute(key.FocusCmd{Tag: &pg.seedEditors.editors[i].Edit.Editor})
				// pg.seedEditors.editors[i].Edit.Editor.Focus()
			}
		}
	}
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *SeedRestore) Layout(gtx C) D {
	body := pg.restore(gtx)
	pg.seedEditorsHandle(gtx)
	pg.resetSeedFields.SetEnabled(pg.updateSeedResetBtn())
	seedValid, _ := pg.validateSeeds()
	pg.validateSeed.SetEnabled(seedValid)

	return body
}

func (pg *SeedRestore) restore(gtx C) D {
	return layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Orientation: layout.Vertical,
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.WrapContent,
				Background:  pg.Theme.Color.Surface,
				Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
				Padding:     layout.UniformInset(values.MarginPadding15),
			}.Layout(gtx,
				layout.Rigid(pg.seedEditorViewDesktop),
				layout.Rigid(layout.Spacer{Height: values.MarginPadding5}.Layout),
				layout.Rigid(pg.resetSeedFields.Layout),
			)
		}),
		layout.Stacked(func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return layout.S.Layout(gtx, func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding1}.Layout(gtx, pg.restoreButtonSection)
			})
		}),
	)
}

func (pg *SeedRestore) restoreButtonSection(gtx C) D {
	card := pg.Theme.Card()
	card.Radius = cryptomaterial.Radius(0)
	return card.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return pg.validateSeed.Layout(gtx)
	})
}

func (pg *SeedRestore) divideIntoColumns(editors []*cryptomaterial.RestoreEditor, numberOfColumns, limit int) [][]layout.FlexChild {
	// Calculate the number of rows needed
	lenEditors := len(editors)
	if limit != 0 {
		lenEditors = limit
	}
	remainingCount := lenEditors % numberOfColumns
	numRows := lenEditors / numberOfColumns
	if remainingCount != 0 {
		numRows++
	} else {
		remainingCount = 1
	}

	// Create a 2D slice to hold the columns
	columns := make([][]layout.FlexChild, numRows)

	// Distribute the words among the columns
	for i := 0; i < lenEditors; i++ {
		editorIndex := i
		rowIndex := editorIndex / numberOfColumns
		editor := editors[editorIndex]
		flexChild := layout.Flexed(1, func(gtx C) D {
			if rowIndex == numRows-1 {
				gtx.Constraints.Max.X = (gtx.Constraints.Max.X * remainingCount / numberOfColumns)
			}
			pg.layoutSeedMenu(gtx, editorIndex)
			return editor.Layout(gtx)
		})
		columns[rowIndex] = append(columns[rowIndex], flexChild)
		if len(columns[rowIndex]) < numberOfColumns*2-1 {
			columns[rowIndex] = append(columns[rowIndex], layout.Rigid(layout.Spacer{Width: values.MarginPadding5}.Layout))
		}
	}

	return columns
}

func (pg *SeedRestore) seedEditorViewDesktop(gtx C) D {
	numberOfColumn := 5
	if pg.IsMobileView() {
		numberOfColumn = 1
	}
	rows := pg.divideIntoColumns(pg.seedEditors.editors, numberOfColumn, pg.getWordSeedType().ToInt())
	columnFlexChilds := make([]layout.FlexChild, 0)
	for i := range rows {
		j := i
		row := rows[j]
		columnFlexChilds = append(columnFlexChilds, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx, row...)
		}))
		if len(rows)-1 != j {
			columnFlexChilds = append(columnFlexChilds, layout.Rigid(layout.Spacer{Height: values.MarginPadding5}.Layout))
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, columnFlexChilds...)
}

func (pg *SeedRestore) onSuggestionSeedsClicked(gtx C) {
	index := pg.seedEditors.focusIndex
	if index != -1 {
		for i, b := range pg.seedMenu {
			if pg.seedMenu[i].button.Clicked(gtx) {
				pg.seedEditors.editors[index].Edit.Editor.SetText(b.text)
				pg.seedEditors.editors[index].Edit.Editor.MoveCaret(len(b.text), 0)
				pg.seedClicked = true
				if index != defaultNumberOfSeeds {
					//TODO07
					// pg.seedEditors.editors[index+1].Edit.Editor.Focus()
					gtx.Execute(key.FocusCmd{Tag: &pg.seedEditors.editors[index+1].Edit.Editor})
				}

				if index == defaultNumberOfSeeds {
					pg.isLastEditor = true
				}
			}
		}
	}
}

func (pg *SeedRestore) editorSeedsEventsHandler(gtx C) {
	seedEvent := func(i int, text string) {
		if pg.seedClicked {
			pg.seedEditors.focusIndex = -1
			pg.seedClicked = false
		} else {
			pg.seedEditors.focusIndex = i
		}

		// Remove all unsupported characters.
		trimmedText := libutils.TrimNonAphaNumeric(text)
		if text != trimmedText {
			text = trimmedText
			pg.seedEditors.editors[i].Edit.Editor.SetText(trimmedText)
		}

		if text == "" {
			pg.isLastEditor = false
			pg.openPopupIndex = -1
		} else {
			pg.openPopupIndex = i
		}

		if i != defaultNumberOfSeeds {
			pg.isLastEditor = false
		}
	}

	for i := 0; i < len(pg.seedEditors.editors); i++ {
		editor := pg.seedEditors.editors[i]
		text := editor.Edit.Editor.Text()

		//TODO07
		if gtx.Source.Focused(&editor.Edit.Editor) {
			seedEvent(i, text)
		}

		for {
			event, ok := editor.Edit.Editor.Update(gtx)
			if !ok {
				break
			}
			// for _, e := range editor.Edit.Editor.Events() {
			switch event.(type) {
			case widget.ChangeEvent:
				seedEvent(i, text)

			case widget.SubmitEvent:
				if pg.openPopupIndex != -1 {
					if len(pg.suggestions) > 0 {
						pg.seedEditors.editors[i].Edit.Editor.SetText(pg.seedMenu[pg.selected].text)
					}
				}

				//  Handles Enter and Return keyboard events.
				if i != defaultNumberOfSeeds {
					gtx.Execute(key.FocusCmd{Tag: &pg.seedEditors.editors[i+1].Edit.Editor})
					// pg.seedEditors.editors[i+1].Edit.Editor.Focus()
					pg.selected = 0
				}

				if i == defaultNumberOfSeeds {
					pg.selected = 0
					pg.isLastEditor = true
				}
			}
			// }
		}
	}
}

func (pg *SeedRestore) initSeedMenu() {
	for i := 0; i < pg.suggestionLimit; i++ {
		btn := pg.Theme.Button("")
		btn.Background, btn.Color = color.NRGBA{}, pg.Theme.Color.Text
		pg.seedMenu = append(pg.seedMenu, seedItemMenu{
			text:   "",
			button: btn,
		})
	}
}

func (pg *SeedRestore) suggestionSeedEffect() {
	for k := range pg.suggestions {
		if pg.selected == k || pg.seedMenu[k].button.Hovered() {
			pg.seedMenu[k].button.Background = pg.Theme.Color.Gray4
		} else {
			pg.seedMenu[k].button.Background = color.NRGBA{}
		}
	}
}

func (pg *SeedRestore) layoutSeedMenu(gtx C, optionsSeedMenuIndex int) {
	if pg.openPopupIndex != optionsSeedMenuIndex || pg.openPopupIndex != pg.seedEditors.focusIndex ||
		pg.isLastEditor {
		return
	}
	if len(pg.seedMenu) == 0 {
		return
	}

	inset := layout.Inset{
		Top:  values.MarginPadding35,
		Left: values.MarginPadding0,
	}

	m := op.Record(gtx.Ops)
	_, caretPos := pg.seedEditors.editors[pg.seedEditors.focusIndex].Edit.Editor.CaretPos()
	inset.Layout(gtx, func(gtx C) D {
		border := widget.Border{Color: pg.Theme.Color.Gray4, CornerRadius: values.MarginPadding5, Width: values.MarginPadding2}
		if !pg.seedEditorChanged() && pg.nextcurrentCaretPosition != caretPos {
			pg.nextcurrentCaretPosition = -1
			return border.Layout(gtx, func(gtx C) D {
				return pg.optionsMenuCard.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return (&layout.List{Axis: layout.Vertical}).Layout(gtx, len(pg.seedMenu), func(gtx C, i int) D {
						return layout.UniformInset(values.MarginPadding0).Layout(gtx, pg.seedMenu[i].button.Layout)
					})
				})
			})
		}
		return D{}
	})
	op.Defer(gtx.Ops, m.Stop())
}

func (pg SeedRestore) suggestionSeeds(text string) []string {
	var seeds []string
	if text == "" {
		return seeds
	}

	for _, word := range pg.getWordSeedType().AllWords() {
		if strings.HasPrefix(strings.ToLower(word), strings.ToLower(text)) {
			if len(seeds) < pg.suggestionLimit {
				seeds = append(seeds, word)
			}
		}
	}
	return seeds
}

func (pg *SeedRestore) updateSeedResetBtn() bool {
	for _, editor := range pg.seedEditors.editors {
		return editor.Edit.Editor.Text() != ""
	}
	return false
}

func (pg *SeedRestore) validateSeeds() (bool, string) {
	seedPhrase := ""
	allSuggestedWords := strings.Join(pg.getWordSeedType().AllWords(), " ")
	numberOfSeed := pg.getWordSeedType().ToInt()
	for i, editor := range pg.seedEditors.editors {
		if i >= numberOfSeed {
			break
		}
		if editor.Edit.Editor.Text() == "" || !strings.Contains(allSuggestedWords, editor.Edit.Editor.Text()) {
			pg.seedEditors.editors[i].Edit.HintColor = pg.Theme.Color.Danger
			return false, ""
		}

		seedPhrase += editor.Edit.Editor.Text() + " "
	}

	seedPhrase = strings.TrimSpace(seedPhrase)
	return true, seedPhrase
}

func (pg *SeedRestore) verifySeeds() bool {
	isValid, seedphrase := pg.validateSeeds()
	if isValid {
		pg.seedPhrase = seedphrase
		if !sharedW.VerifyMnemonic(pg.seedPhrase, pg.walletType, pg.getWordSeedType()) {
			errModal := modal.NewErrorModal(pg.Load, values.String(values.StrInvalidSeedPhrase), modal.DefaultClickFunc())
			pg.window.ShowModal(errModal)
			return false
		}
	}

	// Compare seed with existing wallets seed. On positive match abort import
	// to prevent duplicate wallet. walletWithSameSeed >= 0 if there is a match.
	walletWithSameSeed, err := pg.AssetsManager.WalletWithSeed(pg.walletType, pg.seedPhrase, pg.getWordSeedType())
	if err != nil {
		log.Error(err)
		return false
	}

	if walletWithSameSeed != -1 {
		errModal := modal.NewErrorModal(pg.Load, values.String(values.StrSeedAlreadyExist), modal.DefaultClickFunc())
		pg.window.ShowModal(errModal)
		return false
	}

	return true
}

func (pg *SeedRestore) resetSeeds() {
	pg.seedEditors.focusIndex = -1
	for i := 0; i < len(pg.seedEditors.editors); i++ {
		pg.seedEditors.editors[i].Edit.Editor.SetText("")
	}
}

// switchSeedEditors sets focus on the next seed phrase after moving the
// provided steps either forward or backwards. One the focus get to the last cell
// it start for the initial cell.
// TODO07
func switchSeedEditors(gtx C, editors []*cryptomaterial.RestoreEditor, steps int) {
	for i := 0; i < len(editors); i++ {
		if gtx.Source.Focused(&editors[i].Edit.Editor) {
			nextOnFocus := i + steps
			if (nextOnFocus) < 0 {
				nextOnFocus += len(editors) + 2
				if nextOnFocus >= len(editors) {
					nextOnFocus += steps
				}
			} else if nextOnFocus >= len(editors) {
				nextOnFocus -= 2
			}
			nextOnFocus = nextOnFocus % len(editors)
			gtx.Execute(key.FocusCmd{Tag: &editors[nextOnFocus].Edit.Editor})
			// editors[nextOnFocus].Edit.Editor.Focus()
			return
		}
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *SeedRestore) HandleUserInteractions(gtx C) {
	focus := pg.seedEditors.focusIndex
	if focus != -1 {
		pg.suggestions = pg.suggestionSeeds(pg.seedEditors.editors[focus].Edit.Editor.Text())
		pg.seedMenu = pg.seedMenu[:len(pg.suggestions)]
		if !pg.seedEditorChanged() {
			for k, s := range pg.suggestions {
				pg.seedMenu[k].text, pg.seedMenu[k].button.Text = s, s
			}
		}
	}

	if pg.validateSeed.Clicked(gtx) {
		if !pg.verifySeeds() {
			return
		}

		pg.isRestoring = true
		walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
			Title(values.String(values.StrEnterWalDetails)).
			EnableName(false).
			ShowWalletInfoTip(true).
			SetParent(pg).
			SetPositiveButtonCallback(func(walletName, password string, m *modal.CreatePasswordModal) bool {
				_, err := pg.AssetsManager.RestoreWallet(pg.walletType, pg.walletName, pg.seedPhrase, password, sharedW.PassphraseTypePass, pg.getWordSeedType())
				if err != nil {
					errString := err.Error()
					if err.Error() == libutils.ErrExist {
						errString = values.StringF(values.StrWalletExist, pg.walletName)
					}
					m.SetError(errString)
					pg.isRestoring = false
					return false
				}

				infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrWalletRestored), modal.DefaultClickFunc())
				pg.window.ShowModal(infoModal)
				pg.resetSeeds()
				m.Dismiss()
				pg.window.CloseCurrentPage()
				if pg.restoreComplete != nil {
					pg.restoreComplete()
				}
				return true
			})
		pg.window.ShowModal(walletPasswordModal)
	}

	for pg.resetSeedFields.Clicked(gtx) {
		pg.resetSeeds()
	}

	pg.editorSeedsEventsHandler(gtx)
	pg.onSuggestionSeedsClicked(gtx)
	pg.suggestionSeedEffect()

	if pg.seedEditorChanged() {
		pg.suggestions = nil
		_, caretPos := pg.seedEditors.editors[pg.seedEditors.focusIndex].Edit.Editor.CaretPos()
		pg.currentCaretPosition = caretPos
		pg.nextcurrentCaretPosition = caretPos
	}

	if pg.currentCaretPositionChanged() {
		pg.selected = 0
	}
}

// KeysToHandle returns an expression that describes a set of key combinations
// that this page wishes to capture. The HandleKeyPress() method will only be
// called when any of these key combinations is pressed.
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *SeedRestore) KeysToHandle() []event.Filter {
	if pg.isRestoring {
		return nil // don't capture keys while restoring, problematic?
	}
	//TODO07
	return []event.Filter{key.FocusFilter{Target: pg},
		key.Filter{Focus: pg, Name: key.NameTab, Optional: key.ModShift},
		key.Filter{Focus: pg, Name: key.NameUpArrow},
		key.Filter{Focus: pg, Name: key.NameDownArrow},
		key.Filter{Focus: pg, Name: key.NameLeftArrow},
		key.Filter{Focus: pg, Name: key.NameRightArrow},
		key.Filter{Focus: pg, Name: key.NameReturn},
		key.Filter{Focus: pg, Name: key.NameEnter},
	}
	// Once user starts editing any of the input boxes, the arrow up, down
	// and enter key signals are no longer received.
	// keySet1 := cryptomaterial.AnyKeyWithOptionalModifier(key.ModShift, key.NameTab)
	// keySet2 := cryptomaterial.AnyKey(key.NameUpArrow, key.NameDownArrow,
	// 	key.NameLeftArrow, key.NameRightArrow)
	// keySet3 := cryptomaterial.AnyKey(key.NameReturn, key.NameEnter)
	// return cryptomaterial.AnyKey(string(keySet1), string(keySet2), string(keySet3))
}

// HandleKeyPress is called when one or more keys are pressed on the current
// window that match any of the key combinations returned by KeysToHandle().
// Satisfies the load.KeyEventHandler interface for receiving key events.
func (pg *SeedRestore) HandleKeyPress(gtx C, evt *key.Event) {
	if pg.isRestoring {
		return
	}
	if evt.Name == key.NameTab && evt.Modifiers != key.ModShift && evt.State == key.Press && pg.openPopupIndex == -1 {
		if len(pg.suggestions) > 0 {
			pg.seedClicked = true
		}
		switchSeedEditors(gtx, pg.seedEditors.editors, 1)
	}

	// If seed suggestion list is opened and tab key is pressed select
	// the highlighted option and move the cusor to the next next seed editor.
	if evt.Name == key.NameTab && evt.State == key.Press && pg.openPopupIndex != -1 && len(pg.suggestions) != 0 {
		if pg.seedEditors.focusIndex == -1 && len(pg.suggestions) == 1 {
			return
		}
		pg.seedMenu[pg.selected].button.Click()
	}

	if evt.Name == key.NameTab && evt.Modifiers == key.ModShift && evt.State == key.Press && pg.openPopupIndex == -1 {
		switchSeedEditors(gtx, pg.seedEditors.editors, -1)
	}

	if evt.Name == key.NameDownArrow && evt.State == key.Press {
		if pg.openPopupIndex != -1 {
			pg.selected++
			if pg.selected > len(pg.suggestions)-1 {
				pg.selected = 0
			}
			return
		}
		if len(pg.suggestions) > 0 {
			pg.seedClicked = true
		}
		switchSeedEditors(gtx, pg.seedEditors.editors, 5)
	}

	if evt.Name == key.NameUpArrow && evt.State == key.Press {
		if pg.openPopupIndex != -1 {
			pg.selected--
			if pg.selected < 0 {
				pg.selected = len(pg.suggestions) - 1
			}
			return
		}
		switchSeedEditors(gtx, pg.seedEditors.editors, -5)
	}

	if evt.Name == key.NameLeftArrow && evt.State == key.Press && pg.openPopupIndex == -1 {
		if len(pg.suggestions) > 0 {
			pg.seedClicked = true
		}
		switchSeedEditors(gtx, pg.seedEditors.editors, -1)
	}

	if evt.Name == key.NameRightArrow && evt.State == key.Press && pg.openPopupIndex == -1 {
		if len(pg.suggestions) > 0 {
			pg.seedClicked = true
		}
		switchSeedEditors(gtx, pg.seedEditors.editors, 1)
	}

	if (evt.Name == key.NameReturn || evt.Name == key.NameEnter) && pg.openPopupIndex != -1 && evt.State == key.Press && len(pg.suggestions) != 0 {
		if pg.seedEditors.focusIndex == -1 && len(pg.suggestions) == 1 {
			return
		}
		pg.seedMenu[pg.selected].button.Click()
	}
}

func (pg *SeedRestore) currentCaretPositionChanged() bool {
	focus := pg.seedEditors.focusIndex
	if !pg.seedEditorChanged() {
		if focus == -1 {
			return false
		}
		_, caretPos := pg.seedEditors.editors[pg.seedEditors.focusIndex].Edit.Editor.CaretPos()
		if pg.currentCaretPosition != caretPos {
			pg.currentCaretPosition = caretPos
			return true
		}
	}

	return false
}

func (pg *SeedRestore) seedEditorChanged() bool {
	focus := pg.seedEditors.focusIndex
	if pg.selectedSeedEditor != focus {
		if focus == -1 {
			return false
		}
		pg.selectedSeedEditor = focus
		return true
	}

	return false
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *SeedRestore) OnNavigatedFrom() {}
