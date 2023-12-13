package dcrdex

import (
	"fmt"
	"image"
	"image/color"
	"net/url"
	"strconv"
	"strings"

	"decred.org/dcrdex/client/core"
	"decred.org/dcrdex/dex"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/dexc"
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
	DEXOnboardingPageID = "dex_onboarding"
	minimumBondStrength = 1
	bondOverlap         = 2
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

	// formWidth is the width for form elements on the onboarding DEX page.
	formWidth = values.MarginPadding450

	dp20 = values.MarginPadding20
	dp16 = values.MarginPadding16
	dp2  = values.MarginPadding2
	dp10 = values.MarginPadding10
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

type bondServerInfo struct {
	url                  string
	cert                 []byte
	exchange             *core.Exchange
	bondAssets           map[libutils.AssetType]*core.BondAsset
	noSupportedBondAsset bool
}

type bondConfirmationInfo struct {
	bondCoinID       string
	requiredBondConf uint16
	currentBondConf  int32
}

type DEXOnboarding struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scrollContainer *widget.List

	dexc dexClient

	currentStep     onboardingStep
	onBoardingSteps map[onboardingStep]dexOnboardingStep

	// Step Set Password
	passwordEditor        cryptomaterial.Editor
	confirmPasswordEditor cryptomaterial.Editor
	dexPass               []byte

	// Step Choose Server
	serverDropDown *cryptomaterial.DropDown
	addServerBtn   *cryptomaterial.Clickable
	bondServer     *bondServerInfo

	// Sub Step Add Server
	wantCustomServer     bool
	serverURLEditor      cryptomaterial.Editor
	serverCertEditor     cryptomaterial.Editor
	goBackToChooseServer *cryptomaterial.Clickable
	// TODO: add a file selector to choose server cert.

	// Step Post Bond
	bondSourceWalletSelector  *components.WalletAndAccountSelector
	bondSourceAccountSelector *components.WalletAndAccountSelector
	bondStrengthEditor        cryptomaterial.Editor
	bondStrengthMoreInfo      *cryptomaterial.Clickable
	newTier                   int

	// Step Wait for Confirmation
	bondConfirmationInfo *bondConfirmationInfo

	goBackBtn cryptomaterial.Button
	nextBtn   cryptomaterial.Button

	materialLoader material.LoaderStyle
	isLoading      bool
}

func NewDEXOnboarding(l *load.Load) *DEXOnboarding {
	th := l.Theme
	pg := &DEXOnboarding{
		Load:                  l,
		GenericPageModal:      app.NewGenericPageModal(DEXOnboardingPageID),
		scrollContainer:       &widget.List{List: layout.List{Axis: layout.Vertical, Alignment: layout.Middle}},
		passwordEditor:        newPasswordEditor(th, values.String(values.StrNewPassword)),
		confirmPasswordEditor: newPasswordEditor(th, values.String(values.StrConfirmPassword)),
		serverDropDown:        th.DropDown(knownDEXServers[l.AssetsManager.NetType()], values.DEXServerDropdownGroup, false),
		addServerBtn:          th.NewClickable(false),
		bondServer:            &bondServerInfo{},
		serverURLEditor:       newTextEditor(th, values.String(values.StrServerURL), values.String(values.StrInputURL), false),
		serverCertEditor:      newTextEditor(th, values.String(values.StrCertificateOPtional), values.String(values.StrInputCertificate), true),
		goBackToChooseServer:  th.NewClickable(false),
		bondStrengthEditor:    newTextEditor(th, values.String(values.StrBondStrength), "1", false),
		bondStrengthMoreInfo:  th.NewClickable(false),
		goBackBtn:             th.Button(values.String(values.StrBack)),
		nextBtn:               th.Button(values.String(values.StrNext)),
		materialLoader:        material.Loader(th.Base),
		dexc:                  l.AssetsManager.DexClient(),
	}

	pg.goBackBtn.Background = pg.Theme.Color.Gray2
	pg.goBackBtn.Color = pg.Theme.Color.Black
	pg.goBackBtn.HighlightColor = pg.Theme.Color.Gray7

	pg.bondStrengthEditor.IsTitleLabel = false
	pg.serverDropDown.Width = formWidth
	pg.serverDropDown.MakeCollapsedLayoutVisibleWhenExpanded = true

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

	pg.currentStep = onboardingSetPassword
	if pg.dexc.IsDEXPasswordSet() {
		pg.currentStep = onboardingChooseServer
	}

	pg.bondStrengthEditor.IsTitleLabel = false
	pg.goBackBtn.Background = pg.Theme.Color.Gray2
	pg.goBackBtn.Color = pg.Theme.Color.Black
	pg.goBackBtn.HighlightColor = pg.Theme.Color.Gray7

	// Set defaults.
	pg.newTier = minimumBondStrength
	pg.bondStrengthEditor.Editor.SetText(fmt.Sprintf("%d", minimumBondStrength))

	pg.isLoading = true
	go func() {
		<-pg.dexc.Ready()
		pg.isLoading = false
	}()

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *DEXOnboarding) OnNavigatedTo() {}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *DEXOnboarding) OnNavigatedFrom() {
	// Empty dex pass
	for i := range pg.dexPass {
		pg.dexPass[i] = 0
	}

	// Remove bond confirmation listener if any.
	if pg.bondSourceWalletSelector != nil {
		pg.bondSourceWalletSelector.SelectedWallet().RemoveTxAndBlockNotificationListener(DEXOnboardingPageID)
	}
}

// Layout draws the page UI components into the provided C
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *DEXOnboarding) Layout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.MatchParent,
		Orientation: layout.Vertical,
		Background:  pg.Theme.Color.Surface,
		Margin: layout.Inset{
			Right: dp20,
			Left:  dp20,
		},
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := pg.Theme.Body1(values.String(values.StrDCRDEXWelcomeMessage))
			txt.Font.Weight = font.Bold
			return pg.centerLayout(gtx, dp16, dp20, txt.Layout)
		}),
		layout.Rigid(pg.onBoardingStepRow),
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
					return layout.Inset{Top: dp10, Bottom: dp10}.Layout(gtx, lb.Layout)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			inset := layout.Inset{Top: dp10, Bottom: dp10}
			if !activeStep {
				return inset.Layout(gtx, semiBoldLabelGrey3(pg.Theme, stepDesc).Layout)
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
	isPassSet := pg.dexc.IsDEXPasswordSet()
	layoutFlex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, values.MarginPadding20, values.MarginPadding12, pg.Theme.H6(values.String(values.StrSetTradePassword)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, 0, 0, pg.Theme.Body1(values.String(values.StrSetTradePasswordDesc)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			pg.passwordEditor.Editor.ReadOnly = isPassSet
			return layout.Inset{Top: dp16}.Layout(gtx, pg.passwordEditor.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			pg.passwordEditor.Editor.ReadOnly = isPassSet
			return layout.Inset{Top: dp16}.Layout(gtx, pg.confirmPasswordEditor.Layout)
		}),
		layout.Rigid(pg.formFooterButtons),
	)

	return layoutFlex
}

// stepChooseServer returns the a dropdown to select known servers.
func (pg *DEXOnboarding) stepChooseServer(gtx C) D {
	layoutFlex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, dp20, values.MarginPadding12, pg.Theme.H6(values.String(values.StrSelectServer)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, 0, 0, pg.Theme.Body1(values.String(values.StrSelectDEXServerDesc)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			l := pg.Theme.Label(values.TextSize16, values.String(values.StrServer))
			l.Font.Weight = font.Bold
			return layout.Inset{Top: dp20}.Layout(gtx, l.Layout)
		}),
		layout.Rigid(pg.serverDropDown.Layout),
		layout.Rigid(components.IconButton(pg.Theme.Icons.ContentAdd, values.String(values.StrAddServer),
			layout.Inset{Top: dp16}, pg.Theme, pg.addServerBtn),
		),
		layout.Rigid(pg.formFooterButtons),
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
				Margin:      layout.Inset{Top: values.MarginPadding20, Bottom: dp16},
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
					return layout.Center.Layout(gtx, pg.Theme.H6(values.String(values.StrAddServer)).Layout)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, 0, 0, pg.Theme.Body1(values.String(values.StrAddServerDesc)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp16}.Layout(gtx, pg.serverURLEditor.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp16}.Layout(gtx, pg.serverCertEditor.Layout)
		}),
		layout.Rigid(pg.formFooterButtons),
	)
}

// formFooterButtons is a convenience function that prepares the required
// buttons for each form page footer.
func (pg *DEXOnboarding) formFooterButtons(gtx C) D {
	addBackBtn, nextBtnEnabled, backBtnEnabled := true, true, true
	switch pg.currentStep {
	case onboardingSetPassword:
		addBackBtn = false
	case onboardingChooseServer, onBoardingStepAddServer:
		backBtnEnabled = !pg.dexc.IsDEXPasswordSet()
	case onboardingPostBond:
		nextBtnEnabled = pg.validateBondStrength() && pg.bondAccountHasEnough() && !pg.isLoading
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
			pg.goBackBtn.SetEnabled(backBtnEnabled)
			return layout.Inset{Right: u10}.Layout(gtx, pg.goBackBtn.Layout)
		}),
		layout.Flexed(nextFlex, func(gtx C) D {
			if pg.isLoading {
				return layout.Center.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding20)
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return pg.materialLoader.Layout(gtx)
				})
			}

			pg.nextBtn.SetEnabled(nextBtnEnabled)
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
			return pg.centerLayout(gtx, dp20, values.MarginPadding12, pg.Theme.H6(values.String(values.StrPostBond)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, 0, 0, pg.Theme.Body1(values.String(values.StrSelectBondWalletMsg)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp20}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrSupportedWallets)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp2}.Layout(gtx, func(gtx C) D {
				if pg.bondSourceWalletSelector == nil {
					return D{} // TODO: return btn to create wallet
				}
				return pg.bondSourceWalletSelector.Layout(pg.ParentWindow(), gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp20}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrAccount)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp2}.Layout(gtx, func(gtx C) D {
				if pg.bondSourceAccountSelector == nil {
					return D{}
				}
				return pg.bondSourceAccountSelector.Layout(pg.ParentWindow(), gtx)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp20 * dp2, Bottom: dp20 * dp2}.Layout(gtx, pg.Theme.Separator().Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.S.Layout(gtx, pg.Theme.Body1(values.String(values.StrSelectBondStrengthMsg)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, dp20, dp16, renderers.RenderHTML(values.String(values.StrPostBondDesc), pg.Theme).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pg.semiBoldLabel(values.String(values.StrCurrentTier)).Layout),
				layout.Rigid(pg.viewOnlyCard(&pg.Theme.Color.Gray2, pg.Theme.Label(values.TextSize16, "0").Layout)),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.5, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: dp16, Right: dp10}.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(pg.semiBoldLabel(values.String(values.StrBondStrength)).Layout),
									layout.Rigid(func(gtx C) D {
										return cryptomaterial.LinearLayout{
											Width:     cryptomaterial.WrapContent,
											Height:    cryptomaterial.WrapContent,
											Clickable: pg.bondStrengthMoreInfo,
											Padding:   layout.Inset{Top: dp2, Left: dp2},
										}.Layout2(gtx, pg.Theme.Icons.InfoAction.Layout16dp)
									}),
								)
							})
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Right: dp10}.Layout(gtx, pg.bondStrengthEditor.Layout)
						}),
					)
				}),
				layout.Flexed(0.5, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: dp16, Left: dp10}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrNewTier)).Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: dp10}.Layout(gtx, pg.viewOnlyCard(nil, pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d", pg.newTier)).Layout))
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
							return layout.Inset{Top: dp16}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrCurrency)).Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return pg.viewOnlyCard(&pg.Theme.Color.Gray2, func(gtx C) D {
								assetType := pg.bondSourceAccountSelector.SelectedWallet().GetAssetType()
								icon := pg.Theme.AssetIcon(assetType)
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										if icon == nil {
											return D{}
										}
										return layout.Inset{Right: 5}.Layout(gtx, icon.Layout20dp)
									}),
									layout.Rigid(pg.Theme.Label(values.TextSize16, assetType.String()).Layout),
								)
							})(gtx)
						}),
					)
				}),
				layout.Flexed(0.7, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: dp16, Left: dp10}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrTotalCost)).Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: dp10}.Layout(gtx, pg.viewOnlyCard(nil, pg.bondAmountInfoDisplay))
						}),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: dp16}.Layout(gtx, pg.formFooterButtons)
		}),
	)

	return layoutFlex
}

func (pg *DEXOnboarding) semiBoldLabel(title string) cryptomaterial.Label {
	lb := pg.Theme.Label(values.TextSize16, title)
	lb.Font.Weight = font.SemiBold
	return lb
}

func (pg *DEXOnboarding) viewOnlyCard(bg *color.NRGBA, info func(gtx C) D) func(gtx C) D {
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
				Width:  dp2,
				Color:  pg.Theme.Color.Gray2,
			},
			Margin: layout.Inset{
				Top:    dp2,
				Bottom: dp2,
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
	u12 := values.MarginPadding12
	width := formWidth + values.MarginPadding100
	gtx.Constraints.Max.X = gtx.Dp(width)
	layoutFlex := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, dp20, u12, pg.Theme.H6(values.String(values.StrPostBond)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.centerLayout(gtx, 0, 0, renderers.RenderHTML(values.String(values.StrPostBondDesc), pg.Theme).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       gtx.Dp(width),
				Height:      cryptomaterial.WrapContent,
				Background:  pg.Theme.Color.Gray4,
				Orientation: layout.Vertical,
				Margin: layout.Inset{
					Top:    dp20,
					Bottom: dp20,
				},
				Border: cryptomaterial.Border{
					Radius: cryptomaterial.Radius(8),
				},
				Padding: layout.UniformInset(dp16),
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: dp2}.Layout(gtx, pg.Theme.Icons.TimerIcon.Layout16dp)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Left: dp10}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrWaitingForConfirmation)).Layout)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: 10, Bottom: dp10}.Layout(gtx, pg.Theme.Body1(values.StringF(values.StrDEXBondConfirmationMsg, pg.bondServer.url, pg.bondConfirmationInfo.requiredBondConf)).Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(pg.semiBoldLabel(fmt.Sprintf("%s: ", values.String(values.StrConfirmationStatus))).Layout),
						layout.Rigid(pg.Theme.Label(values.TextSize16, values.StringF(values.StrConfirmationProgressMsg, pg.bondConfirmationInfo.currentBondConf, pg.bondConfirmationInfo.requiredBondConf)).Layout),
					)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: dp20}.Layout(gtx, pg.semiBoldLabel(values.String(values.StrPaymentDetails)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Bottom: values.MarginPadding60}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Flexed(0.33, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: 5}.Layout(gtx, semiBoldLabelGrey3(pg.Theme, values.String(values.StrNewTier)).Layout)
							}),
							layout.Rigid(pg.Theme.Body1(fmt.Sprintf("%d", pg.newTier)).Layout),
						)
					}),
					layout.Flexed(0.33, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: 5}.Layout(gtx, semiBoldLabelGrey3(pg.Theme, values.String(values.StrBondStrength)).Layout)
							}),
							layout.Rigid(pg.Theme.Body1(fmt.Sprintf("%d", pg.newTier)).Layout),
						)
					}),
					layout.Flexed(0.33, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: 5}.Layout(gtx, semiBoldLabelGrey3(pg.Theme, values.String(values.StrTotalCost)).Layout)
							}),
							layout.Rigid(pg.bondAmountInfoDisplay),
						)
					}),
				)
			})
		}),
	)

	return layoutFlex
}

func (pg *DEXOnboarding) bondAmountInfoDisplay(gtx C) D {
	asset := pg.bondSourceAccountSelector.SelectedWallet()
	assetType := asset.GetAssetType()
	icon := pg.Theme.AssetIcon(assetType)
	bondAsset := pg.bondServer.bondAssets[assetType]
	bondsFeeBuffer := pg.dexc.BondsFeeBuffer(bondAsset.ID)
	amt := uint64(pg.newTier)*bondAsset.Amt*bondOverlap + bondsFeeBuffer
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if icon == nil {
				return D{}
			}
			return layout.Inset{Right: 5}.Layout(gtx, icon.Layout20dp)
		}),
		layout.Rigid(pg.Theme.Label(values.TextSize16, fmt.Sprintf("%v", asset.ToAmount(int64(amt)))).Layout),
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

		// Clear the add server form
		pg.serverURLEditor.Editor.SetText("")
		pg.serverCertEditor.Editor.SetText("")
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
	}

	if pg.nextBtn.Clicked() || isSubmit && !pg.isLoading {
		switch pg.currentStep {
		case onboardingSetPassword:
			ok := pg.validPasswordInputs()
			if !ok {
				return
			}

			pg.currentStep = onBoardingStepAddServer
			knownServers, ok := knownDEXServers[pg.AssetsManager.NetType()]
			if ok && len(knownServers) > 0 && !pg.wantCustomServer {
				pg.currentStep = onboardingChooseServer
			}
		case onboardingChooseServer, onBoardingStepAddServer:
			serverInfo := new(bondServerInfo)
			if pg.currentStep == onboardingChooseServer {
				serverInfo.url = pg.serverDropDown.Selected()
				cert, ok := CertStore[serverInfo.url]
				if !ok {
					log.Errorf("Selected DEX server's (%s) cert is missing", serverInfo.url)
					return
				}
				serverInfo.cert = cert
			} else if pg.currentStep == onBoardingStepAddServer {
				if utils.EditorsNotEmpty(pg.serverURLEditor.Editor) {
					serverURL := pg.serverURLEditor.Editor.Text()
					if _, err := url.ParseRequestURI(serverURL); err != nil {
						pg.serverURLEditor.SetError(values.String(values.StrDEXServerAddrWarning))
						return
					}
					serverInfo.url = serverURL
					serverInfo.cert = []byte(pg.serverCertEditor.Editor.Text())
				} else {
					pg.serverURLEditor.SetError(values.String(values.StrDEXServerAddrWarning))
					return
				}
			}

			pg.bondServer = serverInfo
			pg.connectServerAndPrepareForBonding()

		case onboardingPostBond:
			// Validate all input fields.
			hasEnough := pg.bondAccountHasEnough()
			bondStrengthOk := pg.validateBondStrength()
			if !hasEnough || !bondStrengthOk {
				break
			}

			if !pg.bondSourceWalletSelector.SelectedWallet().IsSynced() { // Only fully synced wallets should post bonds.
				pg.notifyError(values.String(values.StrWalletNotSynced))
				return
			}

			// Initialize with password now, if dex password has not been
			// initialized.
			if !pg.dexc.IsDEXPasswordSet() {
				pg.isLoading = true
				go func() {
					pg.dexPass = []byte(pg.passwordEditor.Editor.Text())
					if err := pg.dexc.InitWithPassword(pg.dexPass, nil); err != nil {
						pg.isLoading = false
						pg.notifyError(err.Error())
						return
					}

					pg.isLoading = false
					pg.postBond()
				}()
			}

			// dexc password has been sent already.
			dexPasswordModal := modal.NewCreatePasswordModal(pg.Load).
				EnableName(false).
				EnableConfirmPassword(false).
				Title(values.String(values.StrDexPassword)).
				SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
					pg.dexPass = []byte(password)
					err := pg.dexc.Login(pg.dexPass)
					if err != nil {
						pm.SetError(err.Error())
						pm.SetLoading(false)
						return false
					}

					go pg.postBond()
					return true
				})
			dexPasswordModal.SetPasswordTitleVisibility(false)
			pg.ParentWindow().ShowModal(dexPasswordModal)

		case onBoardingStepWaitForConfirmation:
			if pg.bondConfirmationInfo.currentBondConf >= int32(pg.bondConfirmationInfo.requiredBondConf) {
				pg.ParentNavigator().Display(NewDEXMarketPage(pg.Load))
			}
		}
	}
}

func (pg *DEXOnboarding) connectServerAndPrepareForBonding() {
	pg.isLoading = true
	defer func() {
		pg.isLoading = false
	}()

	xc, err := pg.dexc.GetDEXConfig(pg.bondServer.url, pg.bondServer.cert)
	if err != nil {
		pg.notifyError(fmt.Errorf("Error discovering account: %w", err).Error())
		return
	}
	pg.bondServer.exchange = xc

	pg.bondServer.bondAssets = make(map[libutils.AssetType]*core.BondAsset)
	var supportedBondAssets []libutils.AssetType
	for _, asset := range xc.BondAssets {
		assetSym := dex.BipIDSymbol(asset.ID)
		var assetType libutils.AssetType
		switch {
		case strings.EqualFold(assetSym, libutils.DCRWalletAsset.String()):
			assetType = libutils.DCRWalletAsset
		case strings.EqualFold(assetSym, libutils.BTCWalletAsset.String()):
			assetType = libutils.BTCWalletAsset
		case strings.EqualFold(assetSym, libutils.LTCWalletAsset.String()):
			assetType = libutils.LTCWalletAsset
		default:
			continue // unsupported asset
		}

		supportedBondAssets = append(supportedBondAssets, assetType)
		pg.bondServer.bondAssets[assetType] = asset
	}

	pg.bondServer.noSupportedBondAsset = len(supportedBondAssets) == 0
	if pg.bondServer.noSupportedBondAsset {
		pg.notifyError(values.StringF(values.StrNoSupportedBondAsset, pg.bondServer.url))
		return
	}

	pg.currentStep = onboardingPostBond
	// TODO: pg.bondSourceWalletSelector should be an asset type
	// selector so users can easily create missing wallets and fund
	// it with the required bond amount.
	pg.bondSourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, supportedBondAssets...).
		Title(values.String(values.StrSelectWallet)).
		WalletSelected(func(asset sharedW.Asset) {
			if err := pg.bondSourceAccountSelector.SelectFirstValidAccount(asset); err != nil {
				log.Error(err)
			}
		})
	pg.bondSourceAccountSelector = components.NewWalletAndAccountSelector(pg.Load, supportedBondAssets...).
		Title(values.String(values.StrSelectAcc)).
		AccountValidator(func(a *sharedW.Account) bool {
			return !a.IsWatchOnly
		}).
		AccountSelected(func(a *sharedW.Account) {
			pg.bondAccountHasEnough()
		})
	pg.bondSourceAccountSelector.HideLogo = true
	if err := pg.bondSourceAccountSelector.SelectFirstValidAccount(pg.bondSourceWalletSelector.SelectedWallet()); err != nil {
		log.Error(err)
	}

	pg.bondStrengthEditor.Editor.SetText(fmt.Sprintf("%d", minimumBondStrength))
	pg.newTier = minimumBondStrength
	pg.ParentWindow().Reload()
}

func (pg *DEXOnboarding) postBond() {
	asset := pg.bondSourceWalletSelector.SelectedWallet()
	bondAsset := pg.bondServer.bondAssets[asset.GetAssetType()]
	postBond := &core.PostBondForm{
		Addr:      pg.bondServer.url,
		AppPass:   pg.dexPass,
		Asset:     &bondAsset.ID,
		Bond:      uint64(pg.newTier) * bondAsset.Amt,
		Cert:      pg.bondServer.cert,
		FeeBuffer: pg.dexc.BondsFeeBuffer(bondAsset.ID),
	}

	// postBondFn sends the actual request to post bond.
	postBondFn := func(walletPass string) {
		defer func() {
			pg.isLoading = false
		}()

		// Add bond wallet to core if it does not exist.
		if !pg.dexc.HasWallet(int32(bondAsset.ID)) {
			cfg := map[string]string{
				dexc.DexDcrWalletIDConfigKey:          fmt.Sprintf("%d", asset.GetWalletID()),
				dexc.DexDcrWalletAccountNameConfigKey: pg.bondSourceAccountSelector.SelectedAccount().AccountName,
			}

			err := pg.dexc.AddWallet(*postBond.Asset, cfg, pg.dexPass, []byte(walletPass))
			if err != nil {
				pg.notifyError(fmt.Sprintf("Failed to prepare bond wallet: %v", err))
				return
			}
		}

		res, err := pg.dexc.PostBond(postBond)
		if err != nil {
			pg.notifyError(fmt.Sprintf("Failed to post bond: %v", err))
			return
		}

		pg.bondConfirmationInfo = &bondConfirmationInfo{
			requiredBondConf: res.ReqConfirms,
			bondCoinID:       res.BondID,
		}

		// Listen for new block updates. This listener is removed in OnNavigateFrom().
		asset.AddTxAndBlockNotificationListener(&sharedW.TxAndBlockNotificationListener{
			OnBlockAttached: func(_ int, _ int32) {
				pg.bondConfirmationInfo.currentBondConf++
				pg.ParentWindow().Reload()
			},
		}, DEXOnboardingPageID)

		pg.currentStep = onBoardingStepWaitForConfirmation
		pg.scrollContainer.Position.Offset = 0 // Scroll to the top of the confirmation page after leaving the long post bond form.
		pg.ParentWindow().Reload()
	}

	// Request for wallet password before attempting to post bond.
	walletPasswordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrEnterSpendingPassword)).
		SetPositiveButtonCallback(func(_, walletPass string, pm *modal.CreatePasswordModal) bool {
			pg.isLoading = true
			go postBondFn(walletPass)
			return true
		})
	pg.ParentWindow().ShowModal(walletPasswordModal)
}

func (pg *DEXOnboarding) notifyError(errMsg string) {
	errModal := modal.NewErrorModal(pg.Load, errMsg, modal.DefaultClickFunc())
	pg.ParentWindow().ShowModal(errModal)
}

// bondAccountHasEnough checks if the selected bond account has enough to cover
// the bond costs.
func (pg *DEXOnboarding) bondAccountHasEnough() bool {
	ac := pg.bondSourceAccountSelector.SelectedAccount()
	asset := pg.bondSourceWalletSelector.SelectedWallet()
	bondAsset := pg.bondServer.bondAssets[asset.GetAssetType()]
	bondsFeeBuffer := pg.dexc.BondsFeeBuffer(bondAsset.ID)
	bondCost := uint64(pg.newTier)*bondAsset.Amt*bondOverlap + bondsFeeBuffer
	bondAmt := asset.ToAmount(int64(bondCost))
	if ac.Balance.Spendable.ToInt() < bondAmt.ToInt() {
		pg.bondSourceAccountSelector.SetError(values.StringF(values.StrInsufficientBondAmount, bondAmt.String()))
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

func semiBoldLabelGrey3(th *cryptomaterial.Theme, text string) cryptomaterial.Label {
	lb := th.Label(values.TextSize16, text)
	lb.Color = th.Color.GrayText3
	lb.Font.Weight = font.SemiBold
	return lb
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
