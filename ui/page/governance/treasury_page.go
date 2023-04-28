package governance

import (
	"context"
	"encoding/hex"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/libwallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	libutils "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/page/settings"
	"code.cryptopower.dev/group/cryptopower/ui/values"
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
	wallets       []sharedW.Asset
	treasuryItems []*components.TreasuryItem

	listContainer      *widget.List
	viewGovernanceKeys *cryptomaterial.Clickable
	copyRedirectURL    *cryptomaterial.Clickable
	redirectIcon       *cryptomaterial.Image

	searchEditor cryptomaterial.Editor
	infoButton   cryptomaterial.IconButton

	isPolicyFetchInProgress bool
	navigateToSettingsBtn   cryptomaterial.Button
}

func NewTreasuryPage(l *load.Load) *TreasuryPage {
	pg := &TreasuryPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TreasuryPageID),
		assetsManager:    l.WL.AssetsManager,
		wallets:          l.WL.SortedWalletList(),
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

	return pg
}

func (pg *TreasuryPage) ID() string {
	return TreasuryPageID
}

func (pg *TreasuryPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
	if pg.isTreasuryAPIAllowed() {
		pg.FetchPolicies()
	}
}

func (pg *TreasuryPage) OnNavigatedFrom() {
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}

func (pg *TreasuryPage) isTreasuryAPIAllowed() bool {
	return pg.WL.AssetsManager.IsHttpAPIPrivacyModeOff(libutils.GovernanceHttpAPI)
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
	selectedWallet := pg.WL.SelectedWallet.Wallet

	pg.isPolicyFetchInProgress = true

	// Fetch (or re-fetch) treasury policies in background as this makes
	// a network call. Refresh the window once the call completes.
	key := hex.EncodeToString(pg.WL.AssetsManager.PiKeys()[0])
	go func() {
		pg.treasuryItems = components.LoadPolicies(pg.Load, selectedWallet, key)
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
		overlay = layout.Stacked(func(gtx C) D {
			str := values.StringF(values.StrNotAllowed, values.String(values.StrGovernance))
			return components.DisablePageWithOverlay(pg.Load, nil, gtx, str, &pg.navigateToSettingsBtn)
		})
	}

	return layout.Stack{}.Layout(gtx, layout.Expanded(pg.layout), overlay)
}

func (pg *TreasuryPage) layout(gtx C) D {
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
			return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding10,
						}.Layout(gtx, pg.layoutContent)
					}),
				)
			})
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
	if len(pg.treasuryItems) == 0 {
		return components.LayoutNoPoliciesFound(gtx, pg.Load, pg.isPolicyFetchInProgress)
	}

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
							Layout2(gtx, func(gtx C) D {
								return components.TreasuryItemWidget(gtx, pg.Load, pg.treasuryItems[i])
							})
					})
				})
			})
		}),
	)
}

func (pg *TreasuryPage) updatePolicyPreference(treasuryItem *components.TreasuryItem) {
	passwordModal := modal.NewCreatePasswordModal(pg.Load).
		EnableName(false).
		EnableConfirmPassword(false).
		Title(values.String(values.StrConfirmVote)).
		SetPositiveButtonCallback(func(_, password string, pm *modal.CreatePasswordModal) bool {
			selectedWallet := pg.WL.SelectedWallet.Wallet
			votingPreference := treasuryItem.OptionsRadioGroup.Value
			err := selectedWallet.(*dcr.DCRAsset).SetTreasuryPolicy(treasuryItem.Policy.PiKey, votingPreference, "", password)
			if err != nil {
				pm.SetError(err.Error())
				pm.SetLoading(false)
				return false
			}
			go pg.FetchPolicies() // re-fetch policies when voting is done.
			infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrPolicySetSuccessful), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(infoModal)

			pm.Dismiss()
			return true
		})
	pg.ParentWindow().ShowModal(passwordModal)
}
