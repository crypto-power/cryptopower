package dcrdex

import (
	"fmt"
	"image"
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/renderers"
	"github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	DEXAccountOnboardingID = "dex_account_onboarding"
	minimumBondStrength    = 1
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

	u20 = values.MarginPadding20
	u16 = values.MarginPadding16
	u2  = values.MarginPadding2
	u10 = values.MarginPadding10
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
	onBoardingStepWaitForConfirmation
)

type dexOnboardingStep struct {
	parentStep/* optional */ onboardingStep
	stepFn func(gtx C) D
}

type DEXOnboarding struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scrollContainer *widget.List

	currentStep     onboardingStep
	onBoardingSteps map[onboardingStep]dexOnboardingStep

	// Step Set Password
	passwordEditor        cryptomaterial.Editor
	confirmPasswordEditor cryptomaterial.Editor

	// Step: Choose Server
	serverDropDown *cryptomaterial.DropDown
	addServerBtn   *cryptomaterial.Clickable

	// Sub Step: Add Server
	wantCustomServer     bool
	serverURLEditor      cryptomaterial.Editor
	serverCertEditor     cryptomaterial.Editor
	goBackToChooseServer *cryptomaterial.Clickable
	// TODO: add a file selector to choose server cert.

	// Step Post Bond
	bondSourceWalletSelector  *components.WalletAndAccountSelector
	bondSourceAccountSelector *components.WalletAndAccountSelector

	bondStrengthEditor   cryptomaterial.Editor
	bondStrengthMoreInfo *cryptomaterial.Clickable
	newTier              int

	goBackBtn cryptomaterial.Button
	nextBtn   cryptomaterial.Button

	materialLoader material.LoaderStyle
	showLoader     bool
	isLoading      bool
}

func NewDEXOnboarding(l *load.Load) *DEXOnboarding {
	th := l.Theme
	pg := &DEXOnboarding{
		Load:                  l,
		GenericPageModal:      app.NewGenericPageModal(DEXAccountOnboardingID),
		scrollContainer:       &widget.List{List: layout.List{Axis: layout.Vertical, Alignment: layout.Middle}},
		currentStep:           onboardingSetPassword,
		passwordEditor:        newPasswordEditor(th, values.String(values.StrNewPassword)),
		confirmPasswordEditor: newPasswordEditor(th, values.String(values.StrConfirmPassword)),
		serverDropDown:        th.DropDown(knownDEXServers[l.WL.Wallet.Net], values.DEXServerDropdownGroup, 0),
		addServerBtn:          th.NewClickable(false),
		serverURLEditor:       newTextEditor(th, values.String(values.StrServerURL), values.String(values.StrInputURL), false),
		serverCertEditor:      newTextEditor(th, values.String(values.StrCertificateOPtional), values.String(values.StrInputCertificate), true),
		goBackToChooseServer:  th.NewClickable(false),
		bondStrengthEditor:    newTextEditor(th, values.String(values.StrBondStrength), "1", false),
		goBackBtn:             th.Button(values.String(values.StrBack)),
		nextBtn:               th.Button(values.String(values.StrNext)),
		materialLoader:        material.Loader(th.Base),
		bondStrengthMoreInfo:  th.NewClickable(false),
	}

	pg.goBackBtn.Background = pg.Theme.Color.Gray2
	pg.goBackBtn.Color = pg.Theme.Color.Black
	pg.goBackBtn.HighlightColor = pg.Theme.Color.Gray7

	pg.bondStrengthEditor.IsTitleLabel = false

	pg.onBoardingSteps = map[onboardingStep]dexOnboardingStep{
		onboardingSetPassword: {
			stepFn: pg.stepSetPassword,
		},
		onboardingChooseServer: {
			stepFn: pg.stepChooseServer,
		},
		onboardingPostBond: {
			stepFn: pg.stepPostBond,
		},

		// Sub steps:
		onBoardingStepAddServer: {
			parentStep: onboardingChooseServer,
			stepFn:     pg.subStepAddServer,
		},
		onBoardingStepWaitForConfirmation: {
			parentStep: onboardingPostBond,
			stepFn:     pg.stepWaitForBondConfirmation,
		},
	}

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *DEXOnboarding) OnNavigatedTo() {
	pg.showLoader = false
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *DEXOnboarding) OnNavigatedFrom() {}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *DEXOnboarding) Layout(gtx C) D {
	r := 8
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.MatchParent,
		Orientation: layout.Vertical,
		Background:  pg.Theme.Color.Surface,
		Margin: layout.Inset{
			Bottom: values.MarginPadding50,
			Right:  u20,
			Left:   u20,
		},
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(r),
		},
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := pg.Theme.Body1(values.String(values.StrDCRDEXWelcomeMessage))
			txt.Font.Weight = font.Bold
			return pg.centerLayout(gtx, u16, u20, txt.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.onBoardingStepRow(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			return pg.Theme.Separator().Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.Theme.List(pg.scrollContainer).Layout(gtx, 1, func(gtx C, i int) D {
				gtx.Constraints.Max = image.Point{
					X: gtx.Dp(formWidth),
					Y: gtx.Constraints.Max.Y,
				}
				return pg.onBoardingSteps[pg.currentStep].stepFn(gtx)
			})
		}),
	)
}

func (pg *DEXOnboarding) centerLayout(gtx C, top, bottom unit.Dp, content layout.Widget) D {
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Inset{
			Top:    top,
			Bottom: bottom,
		}.Layout(gtx, content)
	})
}

func (pg *DEXOnboarding) onBoardingStepRow(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Margin: layout.Inset{
			Bottom: values.MarginPadding10,
		},
		Direction: layout.Center,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Stack{Alignment: layout.Center}.Layout(gtx,
				layout.Stacked(func(gtx C) D {
					u30 := values.MarginPadding30
					sep := pg.Theme.Separator()
					sep.Width = gtx.Dp(values.MarginPadding500)
					sep.Height = gtx.Dp(values.MarginPadding3)
					return layout.Inset{Bottom: values.MarginPadding35, Right: u30, Left: u30}.Layout(gtx, sep.Layout)
				}),
				layout.Expanded(func(gtx C) D {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Spacing:   layout.SpaceBetween,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.onBoardingStep(gtx, onboardingSetPassword, values.String(values.StrSetPassword))
						}),
						layout.Rigid(func(gtx C) D {
							return pg.onBoardingStep(gtx, onboardingChooseServer, values.String(values.StrSelectServer))
						}),
						layout.Rigid(func(gtx C) D {
							return pg.onBoardingStep(gtx, onboardingPostBond, values.String(values.StrPostBond))
						}),
					)
				}),
			)
		}),
	)
}

func (pg *DEXOnboarding) onBoardingStep(gtx C, step onboardingStep, stepDesc string) D {
	stepColor := pg.Theme.Color.LightBlue4
	textColor := pg.Theme.Color.Black
	currentStep := pg.onBoardingSteps[pg.currentStep]
	activeStep := pg.currentStep == step || currentStep.parentStep == step
	if activeStep {
		stepColor = pg.Theme.Color.Primary
		textColor = pg.Theme.Color.White
	}

	layoutFlex := layout.Flex{
		Axis:      layout.Vertical,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			u40 := gtx.Dp(values.MarginPadding40)
			return cryptomaterial.LinearLayout{
				Width:       u40,
				Height:      u40,
				Background:  stepColor,
				Orientation: layout.Horizontal,
				Direction:   layout.Center,
				Border: cryptomaterial.Border{
					Radius: cryptomaterial.Radius(20),
				},
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					lb := pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", step))
					lb.Color = textColor
					lb.Font.Weight = font.SemiBold
					return layout.Inset{Top: u10, Bottom: u10}.Layout(gtx, lb.Layout)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			inset := layout.Inset{Top: u10, Bottom: u10}
			if !activeStep {
				return inset.Layout(gtx, func(gtx C) D {
					return pg.semiBoldLabelGrey3(gtx, stepDesc)
				})
			}

			lb := pg.semiBoldLabel(stepDesc)
			lb.Color = stepColor
			return inset.Layout(gtx, lb.Layout)
		}),
	)

	return layoutFlex
}

// stepSetPassword returns the "Set Password" form.
func (pg *DEXOnboarding) stepSetPassword(gtx C) D {
	layoutFlex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, values.MarginPadding20, values.MarginPadding12, pg.Theme.H6(values.String(values.StrSetTradePassword)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, 0, 0, pg.Theme.Body1(values.String(values.StrSetTradePasswordDesc)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u16}.Layout(gtx, pg.passwordEditor.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u16}.Layout(gtx, pg.confirmPasswordEditor.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.formFooterButtons(gtx)
		}),
	)

	return layoutFlex
}

// stepChooseServer returns the a dropdown to select known servers.
func (pg *DEXOnboarding) stepChooseServer(gtx C) D {
	layoutFlex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, u20, values.MarginPadding12, pg.Theme.H6(values.String(values.StrSelectServer)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, 0, 0, pg.Theme.Body1(values.String(values.StrSelectDEXServerDesc)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			l := pg.Theme.Label(values.TextSize16, values.String(values.StrServer))
			l.Font.Weight = font.Bold
			return layout.Inset{Top: u20}.Layout(gtx, l.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			pg.serverDropDown.Width = gtx.Dp(formWidth)
			return pg.serverDropDown.Layout(gtx, 0, false)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					color := pg.Theme.Color.Primary
					return cryptomaterial.LinearLayout{
						Width:       gtx.Dp(values.MarginPadding110),
						Height:      cryptomaterial.WrapContent,
						Orientation: layout.Horizontal,
						Margin:      layout.Inset{Top: u16},
						Direction:   layout.W,
						Alignment:   layout.Middle,
						Clickable:   pg.addServerBtn,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							icon := pg.Theme.Icons.ContentAdd
							return icon.Layout(gtx, color)
						}),
						layout.Rigid(func(gtx C) D {
							label := pg.Theme.Label(values.TextSize16, values.String(values.StrAddServer))
							label.Color = color
							label.Font.Weight = font.SemiBold
							return layout.Inset{Left: values.MarginPadding5}.Layout(gtx, label.Layout)
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.formFooterButtons(gtx)
		}),
	)

	return layoutFlex
}

// subStepAddServer returns a form to add a server.
func (pg *DEXOnboarding) subStepAddServer(gtx C) D {
	width := gtx.Dp(formWidth)
	if pg.wantCustomServer {
		width = gtx.Dp(formWidth + values.MarginPadding100)
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       width,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Horizontal,
				Margin:      layout.Inset{Top: values.MarginPadding20, Bottom: u16},
				Alignment:   layout.Middle,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if !pg.wantCustomServer {
						return D{}
					}

					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.WrapContent,
						Height:      cryptomaterial.WrapContent,
						Orientation: layout.Horizontal,
						Clickable:   pg.goBackToChooseServer,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.Theme.Icons.NavigationArrowBack.Layout(gtx, pg.Theme.Color.Gray1)
						}),
					)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.Center.Layout(gtx, func(gtx C) D {
						return pg.Theme.H6(values.String(values.StrAddServer)).Layout(gtx)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, 0, 0, pg.Theme.Body1(values.String(values.StrAddServerDesc)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u16}.Layout(gtx, pg.serverURLEditor.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u16}.Layout(gtx, pg.serverCertEditor.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.formFooterButtons(gtx)
		}),
	)
}

// formFooterButtons is a convenience function that prepares the required
// buttons for each form page.
func (pg *DEXOnboarding) formFooterButtons(gtx C) D {
	var addBackBtn bool
	switch pg.currentStep {
	case onboardingPostBond, onboardingChooseServer, onBoardingStepAddServer:
		addBackBtn = true
	}

	pg.nextBtn.Text = values.String(values.StrNext)
	if pg.currentStep == onBoardingStepAddServer && pg.wantCustomServer {
		pg.nextBtn.Text = values.String(values.StrAdd)
		addBackBtn = false
	}

	u16 := values.MarginPadding16
	u10 := values.MarginPadding10
	var nextFlex float32 = 1.0
	var goBackFlex float32
	if addBackBtn {
		nextFlex = 0.5
		goBackFlex = 0.5
	}

	return cryptomaterial.LinearLayout{
		Width:     gtx.Dp(formWidth),
		Height:    cryptomaterial.WrapContent,
		Spacing:   layout.SpaceBetween,
		Alignment: layout.Middle,
		Margin: layout.Inset{
			Top:    u16,
			Bottom: u16,
		},
	}.Layout(gtx,
		layout.Flexed(goBackFlex, func(gtx C) D {
			if !addBackBtn {
				return D{}
			}
			return layout.Inset{Right: u10}.Layout(gtx, pg.goBackBtn.Layout)
		}),
		layout.Flexed(nextFlex, func(gtx C) D {
			if !addBackBtn {
				return pg.nextBtn.Layout(gtx)
			}
			return layout.Inset{Left: u10}.Layout(gtx, pg.nextBtn.Layout)
		}),
	)
}

func (pg *DEXOnboarding) stepPostBond(gtx C) D {
	layoutFlex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, u20, values.MarginPadding12, pg.Theme.H6(values.String(values.StrPostBond)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, 0, 0, pg.Theme.Body1(values.String(values.StrSelectBondWalletMsg)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u20}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrSupportedWallets)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u2}.Layout(gtx, func(gtx C) D {
				if pg.bondSourceWalletSelector == nil {
					return D{} // TODO: return btn to create wallet
				}
				return pg.bondSourceWalletSelector.Layout(pg.ParentWindow(), gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u20}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrAccount)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u2}.Layout(gtx, func(gtx C) D {
				if pg.bondSourceAccountSelector == nil {
					return D{}
				}
				return pg.bondSourceAccountSelector.Layout(pg.ParentWindow(), gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u20 * u2, Bottom: u20 * u2}.Layout(gtx, pg.Theme.Separator().Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.S.Layout(gtx, pg.Theme.Body1(values.String(values.StrSelectBondStrengthMsg)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, u20, u16, renderers.RenderHTML(values.String(values.StrPostBondDesc), pg.Theme).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return pg.semiBoldLabel(values.String(values.StrCurrentTier)).Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.viewOnlyCard(gtx, &pg.Theme.Color.Gray2, func(gtx C) D {
						return pg.Theme.Label(values.TextSize16, "0").Layout(gtx)
					})(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.5, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: u16, Right: u10}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(pg.semiBoldLabel(values.String(values.StrBondStrength)).Layout),
									layout.Rigid(func(gtx C) D {
										return cryptomaterial.LinearLayout{
											Width:     cryptomaterial.WrapContent,
											Height:    cryptomaterial.WrapContent,
											Clickable: pg.bondStrengthMoreInfo,
											Padding:   layout.Inset{Top: u2, Left: u2},
										}.Layout2(gtx, pg.Theme.Icons.InfoAction.Layout16dp)
									}),
								)
							})
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Right: u10}.Layout(gtx, pg.bondStrengthEditor.Layout)
						}),
					)
				}),
				layout.Flexed(0.5, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: u16, Left: u10}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrNewTier)).Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: u10}.Layout(gtx, pg.viewOnlyCard(gtx, nil, func(gtx C) D {
								return pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", pg.newTier)).Layout(gtx)
							}))
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.3, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: u16}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrCurrency)).Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return pg.viewOnlyCard(gtx, &pg.Theme.Color.Gray2, func(gtx C) D {
								icon, assetType := pg.bondAssetInfo()
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										if icon == nil {
											return D{}
										}
										return layout.Inset{Right: 5}.Layout(gtx, icon.Layout20dp)
									}),
									layout.Rigid(func(gtx C) D {
										return pg.Theme.Label(values.TextSize16, assetType.String()).Layout(gtx)
									}),
								)
							})(gtx)
						}),
					)
				}),
				layout.Flexed(0.7, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: u16, Left: u10}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrTotalCost)).Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: u10}.Layout(gtx, pg.viewOnlyCard(gtx, nil, func(gtx C) D {
								return pg.bondAmountInfoDisplay(gtx)
							}))
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: u16}.Layout(gtx, pg.formFooterButtons)
		}),
	)

	return layoutFlex
}

func (pg *DEXOnboarding) semiBoldLabel(title string) cryptomaterial.Label {
	lb := pg.Theme.Label(values.TextSize16, title)
	lb.Font.Weight = font.SemiBold
	return lb
}

func (pg *DEXOnboarding) viewOnlyCard(gtx C, bg *color.NRGBA, info func(gtx C) D) func(gtx C) D {
	var cardBg color.NRGBA
	if bg != nil {
		cardBg = *bg
	}
	return func(gtx C) D {
		u12 := values.MarginPadding12
		u15 := values.MarginPadding15
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.WrapContent,
			Background:  cardBg,
			Orientation: layout.Vertical,
			Border: cryptomaterial.Border{
				Radius: cryptomaterial.Radius(8),
				Width:  u2,
				Color:  pg.Theme.Color.Gray2,
			},
			Margin: layout.Inset{
				Top:    u2,
				Bottom: u2,
			},
			Padding: layout.Inset{
				Top:    u12,
				Bottom: u12,
				Left:   u15,
				Right:  u15,
			},
		}.Layout2(gtx, info)
	}
}

func (pg *DEXOnboarding) stepWaitForBondConfirmation(gtx C) D {
	u30 := values.MarginPadding30
	width := formWidth + values.MarginPadding100
	gtx.Constraints.Max.X = gtx.Dp(width)
	layoutFlex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, u20, values.MarginPadding12, pg.Theme.H6(values.String(values.StrPostBond)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, u20, u16, renderers.RenderHTML(values.String(values.StrPostBondDesc), pg.Theme).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       gtx.Dp(width),
				Height:      cryptomaterial.WrapContent,
				Background:  pg.Theme.Color.Gray4,
				Orientation: layout.Vertical,
				Margin: layout.Inset{
					Top:    u30,
					Bottom: u30,
				},
				Border: cryptomaterial.Border{
					Radius: cryptomaterial.Radius(8),
				},
				Padding: layout.UniformInset(u16),
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: u2}.Layout(gtx, pg.Theme.Icons.TimerIcon.Layout16dp)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: u10}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrWaitingForConfirmation)).Layout)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: 10, Bottom: u10}.Layout(gtx, pg.Theme.Body1(values.StringF(values.StrDEXBondConfirmationMsg, "dex.decred.org", 2 /* TODO: use real values */)).Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.semiBoldLabel(fmt.Sprintf("%s: ", values.String(values.StrConfirmationStatus))).Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							return pg.Theme.Label(values.TextSize16, values.StringF(values.StrConfirmationProgressMsg, 1, 2 /* TODO: Use actual tx status */)).Layout(gtx)
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: u20}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrPaymentDetails)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: values.MarginPadding60}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Flexed(0.33, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return pg.semiBoldLabelGrey3(gtx, values.String(values.StrNewTier))
								})
							}),
							layout.Rigid(func(gtx C) D {
								return pg.Theme.Body1(fmt.Sprintf("%d", pg.newTier)).Layout(gtx)
							}),
						)
					}),
					layout.Flexed(0.33, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return pg.semiBoldLabelGrey3(gtx, values.String(values.StrBondStrength))
								})
							}),
							layout.Rigid(func(gtx C) D {
								return pg.Theme.Body1(fmt.Sprintf("%d", pg.newTier /* TODO: Use real value */)).Layout(gtx)
							}),
						)
					}),
					layout.Flexed(0.33, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return pg.semiBoldLabelGrey3(gtx, values.String(values.StrTotalCost))
								})
							}),
							layout.Rigid(func(gtx C) D {
								return pg.bondAmountInfoDisplay(gtx)
							}),
						)
					}),
				)
			})
		}),
	)

	return layoutFlex
}

func (pg *DEXOnboarding) bondAssetInfo() (*cryptomaterial.Image, libutils.AssetType) {
	s := pg.bondSourceAccountSelector.SelectedWallet()
	assetType := s.GetAssetType()
	var icon *cryptomaterial.Image
	switch assetType {
	case libutils.DCRWalletAsset:
		icon = pg.Theme.Icons.DCR
	case libutils.BTCWalletAsset:
		icon = pg.Theme.Icons.BTC
	case libutils.LTCWalletAsset:
		icon = pg.Theme.Icons.LTC
	}

	return icon, assetType
}

func (pg *DEXOnboarding) bondAmountInfoDisplay(gtx C) D {
	icon, assetType := pg.bondAssetInfo()
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if icon == nil {
				return D{}
			}
			return layout.Inset{Right: 5}.Layout(gtx, icon.Layout20dp)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.Theme.Label(values.TextSize16, fmt.Sprintf("%f", float32(pg.newTier)*20.2222334565 /* TODO: multiple by actual asset bond cost */)).Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: 5}.Layout(gtx, pg.Theme.Label(values.TextSize16, assetType.String()).Layout)
		}),
	)
}

// HandleUserInteractions is called just before Layout() to determine if any
// user interaction recently occurred on the page and may be used to update the
// page's UI components shortly before they are displayed.
// Part of the load.Page interface.
func (pg *DEXOnboarding) HandleUserInteractions() {
	if pg.addServerBtn.Clicked() {
		pg.wantCustomServer = true
		pg.currentStep = onBoardingStepAddServer
	}

	if pg.goBackToChooseServer.Clicked() {
		pg.wantCustomServer = false
		pg.currentStep = onboardingChooseServer
		pg.serverURLEditor.SetError("")
		pg.serverCertEditor.SetError("")
	}

	if pg.goBackBtn.Clicked() {
		switch pg.currentStep {
		case onboardingPostBond:
			if pg.wantCustomServer {
				pg.currentStep = onBoardingStepAddServer
			} else {
				pg.currentStep = onboardingChooseServer
			}
		case onboardingChooseServer, onBoardingStepAddServer:
			pg.currentStep = onboardingSetPassword
		}
	}

	if pg.bondStrengthMoreInfo.Clicked() {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrBondStrength)).
			SetupWithTemplate(modal.BondStrengthInfoTemplate).
			SetCancelable(true).
			SetContentAlignment(layout.W, layout.W, layout.Center).
			SetPositiveButtonText(values.String(values.StrOk))
		pg.ParentWindow().ShowModal(infoModal)
	}

	// editor event listener
	isSubmit, isChanged := cryptomaterial.HandleEditorEvents(pg.passwordEditor.Editor, pg.confirmPasswordEditor.Editor, pg.serverURLEditor.Editor, pg.serverCertEditor.Editor, pg.bondStrengthEditor.Editor)
	if isChanged {
		// reset error when any editor is modified
		pg.passwordEditor.SetError("")
		pg.confirmPasswordEditor.SetError("")
		pg.serverURLEditor.SetError("")
		pg.serverCertEditor.SetError("")
		pg.bondStrengthEditor.SetError("")
		pg.validateBondStrength()
	}

	if pg.nextBtn.Clicked() || isSubmit {
		switch pg.currentStep {
		case onboardingSetPassword:
			ok := pg.validPasswordInputs()
			if !ok {
				return
			}

			pg.currentStep = onBoardingStepAddServer
			knownServers, ok := knownDEXServers[pg.WL.Wallet.Net]
			if ok && len(knownServers) > 0 && !pg.wantCustomServer {
				pg.currentStep = onboardingChooseServer
			}
		case onboardingChooseServer, onBoardingStepAddServer:
			var serverURL string
			var serverCert []byte
			if pg.currentStep == onboardingChooseServer {
				serverURL = pg.serverDropDown.Selected()
				cert, ok := CertStore[serverURL]
				if !ok {
					log.Errorf("Selected DEX server's (%s) cert is missing", serverURL)
					return
				}
				serverCert = cert
			} else if pg.currentStep == onBoardingStepAddServer {
				if utils.EditorsNotEmpty(pg.serverURLEditor.Editor) {
					serverURL = pg.serverURLEditor.Editor.Text()
					serverCert = []byte(pg.serverCertEditor.Editor.Text())
				} else {
					pg.serverURLEditor.SetError(values.String(values.StrDEXServerAddrWarning))
					return
				}
			}

			// TODO: Validate server is reachable and connect.
			_ = serverURL
			_ = serverCert

			pg.currentStep = onboardingPostBond
			pg.bondSourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load /*, supportedAssets...  TODO: Use assets provided by selected DEX server. */).
				Title(values.String(values.StrSelectWallet)).
				WalletSelected(func(wm *load.WalletMapping) {
					if err := pg.bondSourceAccountSelector.SelectFirstValidAccount(wm); err != nil {
						log.Error(err)
					}
				})
			pg.bondSourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load).
				Title(values.String(values.StrSelectAcc)).
				AccountSelected(func(a *sharedW.Account) {
					pg.bondAccountHasEnough()
				}).AccountValidator(func(a *sharedW.Account) bool {
				return !a.IsWatchOnly
			})
			pg.bondSourceAccountSelector.HideLogo = true
			if err := pg.bondSourceAccountSelector.SelectFirstValidAccount(pg.bondSourceWalletSelector.SelectedWallet()); err != nil {
				log.Error(err)
			}

			pg.bondStrengthEditor.Editor.SetText(fmt.Sprintf("%d", minimumBondStrength))
			pg.newTier = minimumBondStrength

		case onboardingPostBond:
			// Validate all input fields.
			hasEnough := pg.bondAccountHasEnough()
			bondStrengthOk := pg.validateBondStrength()
			if !hasEnough || !bondStrengthOk {
				return
			}

			// TODO: Post bond, wait for confirmations and redirect to market page.
			pg.currentStep = onBoardingStepWaitForConfirmation
			// Scroll to the top of the confirmation page after leaving the long
			// post bond form.
			pg.scrollContainer.Position.Offset = 0
		}
	}
}

// bondAccountHasEnough checks if the selected bond account has enough to cover
// the bond costs.
func (pg *DEXOnboarding) bondAccountHasEnough() bool {
	ac := pg.bondSourceAccountSelector.SelectedAccount()
	if ac.Balance.Spendable.ToCoin() < float64(pg.newTier)*20 /* TODO: Use actual bond cost + reservations + fees */ {
		pg.bondSourceAccountSelector.SetError(values.String(values.StrInsufficientFundsInAccount))
		return false
	}
	pg.bondSourceAccountSelector.SetError("")
	return true
}

func (pg *DEXOnboarding) validateBondStrength() bool {
	var ok bool
	pg.newTier = 0
	bondStrengthStr := pg.bondStrengthEditor.Editor.Text()
	if bondStrength, err := strconv.Atoi(bondStrengthStr); err != nil {
		pg.bondStrengthEditor.SetError(values.String(values.StrBondStrengthErrMsg))
	} else if bondStrength < minimumBondStrength {
		pg.bondStrengthEditor.SetError(values.StringF(values.StrMinimumBondStrength, minimumBondStrength))
	} else {
		ok = true
		pg.newTier = bondStrength
	}
	return ok
}

func (pg *DEXOnboarding) semiBoldLabelGrey3(gtx C, text string) D {
	lb := pg.Theme.Label(values.TextSize16, text)
	lb.Color = pg.Theme.Color.GrayText3
	lb.Font.Weight = font.SemiBold
	return lb.Layout(gtx)
}

func (pg *DEXOnboarding) passwordsMatch(editors ...*widget.Editor) bool {
	if len(editors) != 2 {
		return false
	}

	password := editors[0]
	matching := editors[1]

	if password.Text() != matching.Text() {
		pg.confirmPasswordEditor.SetError(values.String(values.StrPasswordNotMatch))
		return false
	}

	pg.confirmPasswordEditor.SetError("")
	return true
}

func (pg *DEXOnboarding) validPasswordInputs() bool {
	validPassword := utils.EditorsNotEmpty(pg.confirmPasswordEditor.Editor)
	if !validPassword {
		return false
	}

	passwordsMatch := pg.passwordsMatch(pg.passwordEditor.Editor, pg.confirmPasswordEditor.Editor)
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
