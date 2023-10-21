package governance

import (
	"context"
	"encoding/hex"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/settings"
	"github.com/crypto-power/cryptopower/ui/values"
)

const TreasuryPageID = "Treasury"

type TreasuryPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	assetsManager *libwallet.AssetsManager

	sourceWalletSelector *components.WalletAndAccountSelector
	selectedWallet       sharedW.Asset

	treasuryItems []*components.TreasuryItem

	listContainer      *widget.List
	viewGovernanceKeys *cryptomaterial.Clickable
	copyRedirectURL    *cryptomaterial.Clickable
	redirectIcon       *cryptomaterial.Image

	searchEditor cryptomaterial.Editor
	infoButton   cryptomaterial.IconButton

	isPolicyFetchInProgress bool
	navigateToSettingsBtn   cryptomaterial.Button

	PiKey string
}

func NewTreasuryPage(l *load.Load) *TreasuryPage {
	pg := &TreasuryPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TreasuryPageID),
		assetsManager:    l.WL.AssetsManager,
		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		redirectIcon:       l.Theme.Icons.RedirectIcon,
		viewGovernanceKeys: l.Theme.NewClickable(true),
		copyRedirectURL:    l.Theme.NewClickable(false),
	}

	pg.searchEditor = l.Theme.IconEditor(new(widget.Editor), values.String(values.StrSearch), l.Theme.Icons.SearchIcon, true)
	pg.searchEditor.Editor.SingleLine, pg.searchEditor.Editor.Submit, pg.searchEditor.Bordered = true, true, false

	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.infoButton.Size = values.MarginPadding20
	pg.navigateToSettingsBtn = pg.Theme.Button(values.StringF(values.StrEnableAPI, values.String(values.StrGovernance)))

	pg.initWalletSelector()
	return pg
}

func (pg *TreasuryPage) ID() string {
	return TreasuryPageID
}

func (pg *TreasuryPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
	// Fetch (or re-fetch) treasury policies in background as this makes
	// a network call. Refresh the window once the call completes.
	pg.PiKey = hex.EncodeToString(pg.WL.AssetsManager.PiKeys()[0])

	if pg.isTreasuryAPIAllowed() && pg.selectedWallet != nil {
		pg.FetchPolicies()
	}
}

func (pg *TreasuryPage) OnNavigatedFrom() {
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}

func (pg *TreasuryPage) isTreasuryAPIAllowed() bool {
	return pg.WL.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
}

func (pg *TreasuryPage) HandleUserInteractions() {
	for i := range pg.treasuryItems {
		if pg.treasuryItems[i].SetChoiceButton.Clicked() {
			pg.updatePolicyPreference(pg.treasuryItems[i])
		}
	}

	if pg.navigateToSettingsBtn.Button.Clicked() {
		pg.ParentWindow().Display(settings.NewSettingsPage(pg.Load))
	}

	if pg.infoButton.Button.Clicked() {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrTreasurySpending)).
			Body(values.String(values.StrTreasurySpendingInfo)).
			SetCancelable(true).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(infoModal)
	}

	for pg.viewGovernanceKeys.Clicked() {
		host := "https://github.com/decred/dcrd/blob/master/chaincfg/mainnetparams.go#L477"
		if pg.WL.AssetsManager.NetType() == libwallet.Testnet {
			host = "https://github.com/decred/dcrd/blob/master/chaincfg/testnetparams.go#L390"
		}

		info := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrVerifyGovernanceKeys)).
			Body(values.String(values.StrCopyLink)).
			SetCancelable(true).
			UseCustomWidget(func(gtx C) D {
				return components.BrowserURLWidget(gtx, pg.Load, host, pg.copyRedirectURL)
			}).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(info)
	}

	if pg.isPolicyFetchInProgress {
		time.AfterFunc(time.Second*1, func() {
			pg.ParentWindow().Reload()
		})
	}

	pg.searchEditor.EditorIconButtonEvent = func() {
		// TODO: treasury search functionality
	}
}

func (pg *TreasuryPage) FetchPolicies() {
	pg.isPolicyFetchInProgress = true

	go func() {
		pg.treasuryItems = components.LoadPolicies(pg.Load, pg.selectedWallet, pg.PiKey)
		pg.isPolicyFetchInProgress = true
		pg.ParentWindow().Reload()
	}()

	// Refresh the window now to signify that the syncing
	// has started with pg.isSyncing set to true above.
	pg.ParentWindow().Reload()
}

func (pg *TreasuryPage) Layout(gtx C) D {
	// If proposals API is not allowed, display the overlay with the message.
	overlay := layout.Stacked(func(gtx C) D { return D{} })
	if !pg.isTreasuryAPIAllowed() {
		gtxCopy := gtx
		overlay = layout.Stacked(func(gtx C) D {
			str := values.StringF(values.StrNotAllowed, values.String(values.StrGovernance))
			return components.DisablePageWithOverlay(pg.Load, nil, gtxCopy, str, &pg.navigateToSettingsBtn)
		})
		// Disable main page from recieving events
		gtx = gtx.Disabled()
	}

	return layout.Stack{}.Layout(gtx, layout.Expanded(pg.layout), overlay)
}

func (pg *TreasuryPage) layout(gtx C) D {
	if pg.selectedWallet == nil {
		return pg.decredWalletRequired(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(pg.Theme.Label(values.TextSize20, values.String(values.StrTreasurySpending)).Layout),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding3}.Layout(gtx, pg.infoButton.Layout)
						}),
					)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, pg.layoutVerifyGovernanceKeys)
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, pg.layoutContent)
		}),
	)
}

func (pg *TreasuryPage) layoutVerifyGovernanceKeys(gtx C) D {
	return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
		return pg.viewGovernanceKeys.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Right: values.MarginPadding10,
					}.Layout(gtx, pg.redirectIcon.Layout16dp)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{
						Top: values.MarginPaddingMinus2,
					}.Layout(gtx, pg.Theme.Label(values.TextSize16, values.String(values.StrVerifyGovernanceKeys)).Layout)
				}),
			)
		})
	})
}

func (pg *TreasuryPage) layoutContent(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			list := layout.List{Axis: layout.Vertical}
			return pg.Theme.List(pg.listContainer).Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return list.Layout(gtx, len(pg.treasuryItems), func(gtx C, i int) D {
						return cryptomaterial.LinearLayout{
							Orientation: layout.Vertical,
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Background:  pg.Theme.Color.Surface,
							Direction:   layout.W,
							Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
							Padding:     layout.UniformInset(values.MarginPadding15),
							Margin:      layout.Inset{Bottom: values.MarginPadding4, Top: values.MarginPadding4},
						}.
							Layout(gtx,
								layout.Rigid(pg.layoutPiKey),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{Top: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
										gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding350)
										return pg.sourceWalletSelector.Layout(pg.ParentWindow(), gtx)
									})
								}),
								layout.Rigid(func(gtx C) D {
									return components.TreasuryItemWidget(gtx, pg.Load, pg.treasuryItems[i])
								}),
							)
					})
				})
			})
		}),
	)
}

func (pg *TreasuryPage) layoutPiKey(gtx C) D {
	backgroundColor := pg.Theme.Color.LightBlue
	if pg.WL.AssetsManager.IsDarkModeOn() {
		backgroundColor = pg.Theme.Color.Background
	}

	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			lbl := pg.Theme.Label(values.TextSize20, values.String(values.StrPiKey))
			lbl.Font.Weight = font.SemiBold
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Background: backgroundColor,
				Width:      cryptomaterial.WrapContent,
				Height:     cryptomaterial.WrapContent,
				Direction:  layout.Center,
				Alignment:  layout.Middle,
				Border: cryptomaterial.Border{
					Radius: cryptomaterial.Radius(4),
				},
				Padding: layout.Inset{
					Top:    values.MarginPadding3,
					Bottom: values.MarginPadding3,
					Left:   values.MarginPadding8,
					Right:  values.MarginPadding8,
				},
			}.Layout2(gtx, pg.Theme.Label(values.TextSize14, pg.PiKey).Layout)
		}),
	)
}

func (pg *TreasuryPage) updatePolicyPreference(treasuryItem *components.TreasuryItem) {
	passwordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrConfirmVote)).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			votingPreference := treasuryItem.OptionsRadioGroup.Value
			err := pg.selectedWallet.(*dcr.Asset).SetTreasuryPolicy(treasuryItem.Policy.PiKey, votingPreference, "", password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}

			pg.FetchPolicies() // re-fetch policies when voting is done.
			infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrPolicySetSuccessful), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(infoModal)

			pm.Dismiss()
			return true
		})
	pg.ParentWindow().ShowModal(passwordModal)
}

func (pg *TreasuryPage) initWalletSelector() {
	// Source wallet picker
	pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, libutils.DCRWalletAsset).
		Title(values.String(values.StrSelectWallet))
	if pg.sourceWalletSelector.SelectedWallet() != nil {
		pg.selectedWallet = pg.sourceWalletSelector.SelectedWallet().Asset
	}

	pg.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.selectedWallet = selectedWallet.Asset
		pg.FetchPolicies()
	})
}

// TODO: Temporary UI. Pending when new designs will be ready for this feature
func (pg *TreasuryPage) decredWalletRequired(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Direction:   layout.Center,
		Alignment:   layout.Middle,
	}.Layout2(gtx, func(gtx C) D {
		txt := "This feature requires that you have a decred wallet."
		lbl := pg.Theme.Label(values.TextSize16, txt)
		lbl.Font.Weight = font.SemiBold
		return lbl.Layout(gtx)
	})
}
