package dcrdex

import (
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	DEXAccountOnboardingID = "dex_account_onboarding"
)

var (
	// knownDEXServers is a map of know DEX servers for supported networks.
	knownDEXServers = map[libutils.NetworkType][]cryptomaterial.DropDownItem{
		libutils.Mainnet: {{
			Text: decredDEXServerMainnet,
		}},
		libutils.Testnet: {{
			Text: decredDEXServerTestnet,
		}},
	}

	// formWidth is the maximum width for form elements on the onboarding DEX
	// page.
	formWidth = values.MarginPadding450
)

// onboardingStep is each step of the flow required for a user to create a DEX
// account with a new DEX server.
type onboardingStep int

const (
	onboardingSetPassword onboardingStep = iota + 1
	onboardingChooseServer
	onboardingPostBond

	// These are sub steps.
	onBoardingStepAddServer
)

type dexOnboardingStep struct {
	parentStep /* optional */, stepN onboardingStep
	stepFn                           func(gtx C) D
}

type DEXOnboarding struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scrollContainer *widget.List

	currentStep *dexOnboardingStep

	// Step Set Password
	passwordEditor        cryptomaterial.Editor
	confirmPasswordEditor cryptomaterial.Editor

	// Step: Choose Server
	serverDropDown *cryptomaterial.DropDown
	addServerBtn   *cryptomaterial.Clickable

	// Sub Step: Add Server
	addBackToChooseServerIcon bool
	serverURLEditor           cryptomaterial.Editor
	serverCertEditor          cryptomaterial.Editor
	goBackToChooseServer      *cryptomaterial.Clickable
	// TODO: add a file selector to choose server cert.

	nextBtn cryptomaterial.Button

	materialLoader material.LoaderStyle
	showLoader     bool
	isLoading      bool
}

func NewDEXOnboarding(l *load.Load) *DEXOnboarding {
	th := l.Theme
	do := &DEXOnboarding{
		GenericPageModal: app.NewGenericPageModal(DEXAccountOnboardingID),
		scrollContainer: &widget.List{
			List: layout.List{
				Axis:      layout.Vertical,
				Alignment: layout.Middle,
			},
		},

		Load:                  l,
		nextBtn:               th.Button(values.String(values.StrNext)),
		passwordEditor:        newPasswordEditor(th, values.String(values.StrNewPassword)),
		confirmPasswordEditor: newPasswordEditor(th, values.String(values.StrConfirmPassword)),
		addServerBtn:          th.NewClickable(false),
		serverDropDown:        th.DropDown(knownDEXServers[l.WL.Wallet.Net], values.DEXServerDropdownGroup, 0),
		serverURLEditor:       newTextEditor(th, values.String(values.StrServerURL), values.String(values.StrInputURL), false),
		serverCertEditor:      newTextEditor(th, values.String(values.StrCertificateOPtional), values.String(values.StrInputCertificate), true),
		goBackToChooseServer:  th.NewClickable(false),
		materialLoader:        material.Loader(th.Base),
	}

	do.currentStep = &dexOnboardingStep{
		stepN:  onboardingSetPassword,
		stepFn: do.stepSetPassword,
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
					X: gtx.Dp(formWidth),
					Y: gtx.Constraints.Max.Y,
				}
				return do.currentStep.stepFn(gtx)
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
					sep.Width = gtx.Dp(values.MarginPadding500)
					sep.Height = gtx.Dp(values.MarginPadding5)
					return layout.Inset{Bottom: values.MarginPadding35, Right: u30, Left: u30}.Layout(gtx, sep.Layout)
				}),
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Spacing:   layout.SpaceBetween,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return do.onBoardingStep(gtx, onboardingSetPassword, values.String(values.StrSetPassword))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return do.onBoardingStep(gtx, onboardingChooseServer, values.String(values.StrSelectServer))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return do.onBoardingStep(gtx, onboardingPostBond, values.String(values.StrPostBond))
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
	if do.currentStep.stepN == step || do.currentStep.parentStep == step {
		color = do.Theme.Color.Primary
		textColor = do.Theme.Color.White
	}

	u10 := values.MarginPadding10

	layoutFlex := layout.Flex{
		Axis:      layout.Vertical,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			u100 := gtx.Dp(values.MarginPadding50)
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
			return layout.Inset{Top: u16, Bottom: u16}.Layout(gtx, do.nextStepBtn)
		}),
	)

	return layoutFlex
}

// stepChooseServer returns the a dropdown to select known servers.
func (do *DEXOnboarding) stepChooseServer(gtx C) D {
	u16 := values.MarginPadding16
	u20 := values.MarginPadding20
	layoutFlex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return do.centerLayout(gtx, u20, values.MarginPadding12, do.Theme.H6(values.String(values.StrSelectServer)).Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return do.centerLayout(gtx, 0, 0, do.Theme.Body1(values.String(values.StrSelectDEXServerDesc)).Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := do.Theme.Label(values.TextSize16, values.String(values.StrServer))
			l.Font.Weight = font.Bold
			return layout.Inset{Top: u20}.Layout(gtx, l.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			do.serverDropDown.Width = gtx.Dp(formWidth)
			return do.serverDropDown.Layout(gtx, 0, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					color := do.Theme.Color.Primary
					return cryptomaterial.LinearLayout{
						Width:       gtx.Dp(values.MarginPadding110),
						Height:      cryptomaterial.WrapContent,
						Orientation: layout.Horizontal,
						Margin:      layout.Inset{Top: u16},
						Direction:   layout.W,
						Alignment:   layout.Middle,
						Clickable:   do.addServerBtn,
					}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							icon := do.Theme.Icons.ContentAdd
							return icon.Layout(gtx, color)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := do.Theme.Label(values.TextSize16, values.String(values.StrAddServer))
							label.Color = color
							label.Font.Weight = font.SemiBold
							return layout.Inset{Left: values.MarginPadding5}.Layout(gtx, label.Layout)
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: u16, Bottom: u16}.Layout(gtx, do.nextStepBtn)
		}),
	)

	return layoutFlex
}

// subStepAddServer returns a for to add a server.
func (do *DEXOnboarding) subStepAddServer(gtx C) D {
	u16 := values.MarginPadding16
	width := gtx.Dp(formWidth)
	if do.addBackToChooseServerIcon {
		width = gtx.Dp(formWidth + values.MarginPadding100)
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return cryptomaterial.LinearLayout{
				Width:       width,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Horizontal,
				Margin:      layout.Inset{Top: values.MarginPadding20, Bottom: u16},
				Alignment:   layout.Middle,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !do.addBackToChooseServerIcon {
						return D{}
					}

					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.WrapContent,
						Height:      cryptomaterial.WrapContent,
						Orientation: layout.Horizontal,
						Clickable:   do.goBackToChooseServer,
					}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return do.Theme.Icons.NavigationArrowBack.Layout(gtx, do.Theme.Color.Gray1)
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx C) D {
						return do.Theme.H6(values.String(values.StrAddServer)).Layout(gtx)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return do.centerLayout(gtx, 0, 0, do.Theme.Body1(values.String(values.StrAddServerDesc)).Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: u16}.Layout(gtx, do.serverURLEditor.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: u16}.Layout(gtx, do.serverCertEditor.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: u16, Bottom: u16}.Layout(gtx, do.nextStepBtn)
		}),
	)
}

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (do *DEXOnboarding) HandleUserInteractions() {
	if do.addServerBtn.Clicked() {
		if do.currentStep.stepN == onboardingChooseServer {
			do.addBackToChooseServerIcon = true
		}
		do.currentStep = &dexOnboardingStep{
			parentStep: onboardingChooseServer,
			stepN:      onBoardingStepAddServer,
			stepFn:     do.subStepAddServer,
		}
	}

	if do.goBackToChooseServer.Clicked() {
		do.addBackToChooseServerIcon = false
		do.serverURLEditor.SetError("")
		do.serverCertEditor.SetError("")
		do.currentStep = &dexOnboardingStep{
			stepN:  onboardingChooseServer,
			stepFn: do.stepChooseServer,
		}
	}

	// editor event listener
	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(do.passwordEditor.Editor, do.confirmPasswordEditor.Editor, do.serverURLEditor.Editor, do.serverCertEditor.Editor)
	if isChanged {
		// reset error when any editor is modified
		do.passwordEditor.SetError("")
		do.confirmPasswordEditor.SetError("")
		do.serverURLEditor.SetError("")
		do.serverCertEditor.SetError("")
	}

	if do.nextBtn.Clicked() || isSubmit {
		step := do.currentStep.stepN
		switch step {
		case onboardingSetPassword:
			ok := do.validPasswordInputs()
			if !ok {
				return
			}

			do.currentStep = &dexOnboardingStep{
				parentStep: onboardingChooseServer,
				stepN:      onBoardingStepAddServer,
				stepFn:     do.subStepAddServer,
			}

			knownServers, ok := knownDEXServers[do.WL.Wallet.Net]
			if ok && len(knownServers) > 0 {
				do.currentStep = &dexOnboardingStep{
					parentStep: onboardingChooseServer,
					stepN:      onboardingChooseServer,
					stepFn:     do.stepChooseServer,
				}
			}
		case onboardingChooseServer, onBoardingStepAddServer:
			var serverURL string
			var serverCert []byte
			if step == onboardingChooseServer {
				serverURL = do.serverDropDown.Selected()
				cert, ok := CertStore[serverURL]
				if !ok {
					log.Errorf("Selected DEX server's (%s) cert is missing", serverURL)
					return
				}
				serverCert = cert
			} else if step == onBoardingStepAddServer {
				if utils.EditorsNotEmpty(do.serverURLEditor.Editor) {
					serverURL = do.serverURLEditor.Editor.Text()
					serverCert = []byte(do.serverCertEditor.Editor.Text())
				} else {
					do.serverURLEditor.SetError(values.String(values.StrDEXServerAddrWarning))
					return
				}
			}

			// TODO: Validate server is reachable and connect.
			_ = serverURL
			_ = serverCert

			do.currentStep = &dexOnboardingStep{
				stepN:  onboardingPostBond,
				stepFn: do.subStepAddServer, // TODO: Add post bond step
			}
		case onboardingPostBond:
			// TODO: Post bond and redirect to Markets page
		}
	}
}

// nextStepBtn is a convenience function that changes the nextStep button text
// based on the current step. TODO: If the designs changes the text for
// onBoardingStepAddServer, remove this function and use do.nextBtn directly.
func (do *DEXOnboarding) nextStepBtn(gtx C) D {
	if do.currentStep.stepN == onBoardingStepAddServer {
		do.nextBtn.Text = values.String(values.StrAdd)
	} else {
		do.nextBtn.Text = values.String(values.StrNext)
	}
	return do.nextBtn.Layout(gtx)
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

func newTextEditor(th *cryptomaterial.Theme, title string, hint string, multiLine bool) cryptomaterial.Editor {
	e := th.Editor(new(widget.Editor), title)
	e.Editor.SingleLine = !multiLine
	e.Editor.Submit = true
	e.Hint = hint
	return e
}
