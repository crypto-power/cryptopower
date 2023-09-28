package dcrdex

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	DEXAccountOnboardingID = "dex_account_onboarding"
)

// onboardingStep is each step of the flow required for a user to create a DEX
// account with a new DEX server.
type onboardingStep int

const (
	onboardingStep1 onboardingStep = iota + 1
	onboardingStep2
	onboardingStep3
)

type DEXOnboarding struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scrollContainer *widget.List

	passwordEditor        cryptomaterial.Editor
	confirmPasswordEditor cryptomaterial.Editor
	materialLoader        material.LoaderStyle

	nextBtn cryptomaterial.Button

	currentStep onboardingStep
	showLoader  bool
	isLoading   bool
}

func NewDEXOnboarding(l *load.Load) *DEXOnboarding {
	do := &DEXOnboarding{
		GenericPageModal: app.NewGenericPageModal(DEXAccountOnboardingID),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},

		Load:                  l,
		nextBtn:               l.Theme.Button(values.String(values.StrNext)),
		passwordEditor:        newPasswordEditor(l.Theme, values.String(values.StrNewPassword)),
		confirmPasswordEditor: newPasswordEditor(l.Theme, values.String(values.StrConfirmPassword)),
		materialLoader:        material.Loader(l.Theme.Base),
		currentStep:           onboardingStep1,
	}

	return do
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (do *DEXOnboarding) OnNavigatedTo() {
	do.showLoader = false
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (do *DEXOnboarding) OnNavigatedFrom() {}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (do *DEXOnboarding) Layout(gtx C) D {
	r := 8
	u20 := values.MarginPadding20
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.MatchParent,
		Orientation: layout.Vertical,
		Background:  do.Theme.Color.Surface,
		Margin: layout.Inset{
			Bottom: values.MarginPadding50,
			Right:  u20,
			Left:   u20,
		},
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(r),
		},
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(values.MarginPadding16).Layout(gtx, do.Theme.Body1(values.String(values.StrDCRDEXWelcomeMessage)).Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return do.onBoardingStepRow(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return do.Theme.Separator().Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// TODO: Add Step Select Server and Step Registration Fee
			return do.Theme.List(do.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
				gtx.Constraints.Max = image.Point{
					X: int(cryptomaterial.MaxWidth),
					Y: gtx.Constraints.Max.Y,
				}
				return do.stepSetPassword(gtx)
			})
		}),
	)
}

func (do *DEXOnboarding) centerLayout(gtx C, top, bottom unit.Dp, content layout.Widget) D {
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{
			Top:    top,
			Bottom: bottom,
		}.Layout(gtx, content)
	})
}

func (do *DEXOnboarding) onBoardingStepRow(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Margin: layout.Inset{
			Bottom: values.MarginPadding10,
		},
		Direction: layout.Center,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{Alignment: layout.Center}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					u30 := values.MarginPadding30
					sep := do.Theme.Separator()
					sep.Width = values.Length1000
					sep.Height = int(values.MarginPadding5)
					return layout.Inset{Bottom: values.MarginPadding35, Right: u30, Left: u30}.Layout(gtx, sep.Layout)
				}),
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Spacing:   layout.SpaceBetween,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return do.onBoardingStep(gtx, onboardingStep1, values.String(values.StrSetPassword))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return do.onBoardingStep(gtx, onboardingStep2, values.String(values.StrSelectServer))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return do.onBoardingStep(gtx, onboardingStep3, values.String(values.StrPostBond))
						}),
					)
				}),
			)
		}),
	)
}

func (do *DEXOnboarding) onBoardingStep(gtx C, step onboardingStep, stepDesc string) D {
	color := do.Theme.Color.LightBlue4
	textColor := do.Theme.Color.Black
	if do.currentStep == step {
		color = do.Theme.Color.Primary
		textColor = do.Theme.Color.White
	}

	u10 := values.MarginPadding10

	layoutFlex := layout.Flex{
		Axis:      layout.Vertical,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			u100 := int(values.MarginPadding100)
			return cryptomaterial.LinearLayout{
				Width:       u100,
				Height:      u100,
				Background:  color,
				Orientation: layout.Horizontal,
				Direction:   layout.Center,
				Border: cryptomaterial.Border{
					Radius: cryptomaterial.Radius(25),
				},
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lb := do.Theme.Label(values.TextSize20, fmt.Sprintf("%d", step))
					lb.Color = textColor
					return layout.Inset{Top: u10, Bottom: u10}.Layout(gtx, lb.Layout)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: u10, Bottom: u10}.Layout(gtx, do.Theme.Body1(stepDesc).Layout)
		}),
	)

	return layoutFlex
}

// stepSetPassword returns the "Set Password" form.
func (do *DEXOnboarding) stepSetPassword(gtx C) D {
	u16 := values.MarginPadding16
	layoutFlex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return do.centerLayout(gtx, values.MarginPadding20, values.MarginPadding12, do.Theme.H6(values.String(values.StrSetTradePassword)).Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return do.centerLayout(gtx, 0, 0, do.Theme.Body1(values.String(values.StrSetTradePasswordDesc)).Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: u16}.Layout(gtx, do.passwordEditor.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: u16}.Layout(gtx, do.confirmPasswordEditor.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: u16, Bottom: u16}.Layout(gtx, do.nextBtn.Layout)
		}),
	)

	return layoutFlex
}

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (do *DEXOnboarding) HandleUserInteractions() {
	// editor event listener
	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(do.passwordEditor.Editor, do.confirmPasswordEditor.Editor)
	if isChanged {
		// reset error when any editor is modified
		do.passwordEditor.SetError("")
		do.confirmPasswordEditor.SetError("")
	}

	if do.nextBtn.Clicked() || isSubmit {
		switch do.currentStep {
		case onboardingStep1:
			ok := do.validPasswordInputs()
			if !ok {
				return
			}
			do.currentStep = onboardingStep2
		case onboardingStep2:
			// TODO: Validate the DEX server and connect.
			do.currentStep = onboardingStep3
		case onboardingStep3:
			// TODO: Post bond and redirect to Markets page
		}
	}
}

func (do *DEXOnboarding) passwordsMatch(editors ...*widget.Editor) bool {
	if len(editors) != 2 {
		return false
	}

	password := editors[0]
	matching := editors[1]

	if password.Text() != matching.Text() {
		do.confirmPasswordEditor.SetError(values.String(values.StrPasswordNotMatch))
		return false
	}

	do.confirmPasswordEditor.SetError("")
	return true
}

func (do *DEXOnboarding) validPasswordInputs() bool {
	validPassword := utils.EditorsNotEmpty(do.confirmPasswordEditor.Editor)
	if !validPassword {
		return false
	}

	passwordsMatch := do.passwordsMatch(do.passwordEditor.Editor, do.confirmPasswordEditor.Editor)
	return validPassword && passwordsMatch
}

func newPasswordEditor(th *cryptomaterial.Theme, title string) cryptomaterial.Editor {
	passE := th.EditorPassword(new(widget.Editor), title)
	passE.Editor.SingleLine, passE.Editor.Submit = true, true
	passE.Hint = title
	return passE
}
