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

const (
	mainnetParamsHost = "https://github.com/decred/dcrd/blob/master/chaincfg/mainnetparams.go#L477"
	testnetParamsHost = "https://github.com/decred/dcrd/blob/master/chaincfg/testnetparams.go#L390"
)

type TreasuryPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	walletDropDown *cryptomaterial.DropDown

	assetWallets      []sharedW.Asset
	selectedDCRWallet *dcr.Asset

	treasuryItems []*components.TreasuryItem

	listContainer      *widget.List
	viewGovernanceKeys *cryptomaterial.Clickable
	copyRedirectURL    *cryptomaterial.Clickable
	redirectIcon       *cryptomaterial.Image

	searchEditor cryptomaterial.Editor
	infoButton   cryptomaterial.IconButton

	isPolicyFetchInProgress bool
	navigateToSettingsBtn   cryptomaterial.Button
	createWalletBtn         cryptomaterial.Button

	PiKey string
}

func NewTreasuryPage(l *load.Load) *TreasuryPage {
	pg := &TreasuryPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TreasuryPageID),
		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		redirectIcon:       l.Theme.Icons.RedirectIcon,
		viewGovernanceKeys: l.Theme.NewClickable(true),
		copyRedirectURL:    l.Theme.NewClickable(false),
		createWalletBtn:    l.Theme.Button(values.String(values.StrCreateANewWallet)),
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
	pg.initWalletSelector()
	// Fetch (or re-fetch) treasury policies in background as this makes
	// a network call. Refresh the window once the call completes.
	pg.PiKey = hex.EncodeToString(pg.AssetsManager.PiKeys()[0])

	if pg.isTreasuryAPIAllowed() && pg.selectedDCRWallet != nil {
		pg.FetchPolicies()
	}
}

func (pg *TreasuryPage) OnNavigatedFrom() {
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}

func (pg *TreasuryPage) isTreasuryAPIAllowed() bool {
	return pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
}

func (pg *TreasuryPage) initWalletSelector() {
	pg.assetWallets = pg.AssetsManager.AllDCRWallets()

	items := []cryptomaterial.DropDownItem{}
	for _, wal := range pg.assetWallets {
		item := cryptomaterial.DropDownItem{
			Text: wal.GetWalletName(),
			Icon: pg.Theme.AssetIcon(wal.GetAssetType()),
		}
		items = append(items, item)
	}

	pg.walletDropDown = pg.Theme.DropDown(items, nil, values.WalletsDropdownGroup, false)
	if len(pg.assetWallets) > 0 {
		pg.selectedDCRWallet = pg.assetWallets[0].(*dcr.Asset)
	}

	pg.walletDropDown.Width = values.MarginPadding150
	settingCommonDropdown(pg.Theme, pg.walletDropDown)
	pg.walletDropDown.SetConvertTextSize(pg.ConvertTextSize)
}

func (pg *TreasuryPage) HandleUserInteractions(gtx C) {
	for i := range pg.treasuryItems {
		if pg.treasuryItems[i].SetChoiceButton.Clicked(gtx) {
			pg.updatePolicyPreference(pg.treasuryItems[i])
		}
	}

	if pg.walletDropDown != nil && pg.walletDropDown.Changed(gtx) {
		pg.selectedDCRWallet = pg.assetWallets[pg.walletDropDown.SelectedIndex()].(*dcr.Asset)
		pg.FetchPolicies()
	}

	if pg.navigateToSettingsBtn.Button.Clicked(gtx) {
		pg.ParentWindow().Display(settings.NewAppSettingsPage(pg.Load))
	}

	if pg.infoButton.Button.Clicked(gtx) {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrTreasurySpending)).
			Body(values.String(values.StrTreasurySpendingInfo)).
			SetCancelable(true).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(infoModal)
	}

	if pg.viewGovernanceKeys.Clicked(gtx) {
		host := mainnetParamsHost
		if pg.AssetsManager.NetType() == libwallet.Testnet {
			host = testnetParamsHost
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

	if pg.createWalletBtn.Button.Clicked(gtx) {
		pg.ParentWindow().Display(components.NewCreateWallet(pg.Load, func(_ sharedW.Asset) {
			pg.walletCreationSuccessFunc()
		}, libutils.DCRWalletAsset))
	}

	pg.searchEditor.EditorIconButtonEvent = func() {
		// TODO: treasury search functionality
	}
}

func (pg *TreasuryPage) FetchPolicies() {
	pg.isPolicyFetchInProgress = true

	go func() {
		pg.treasuryItems = components.LoadPolicies(pg.Load, pg.selectedDCRWallet, pg.PiKey)
		pg.isPolicyFetchInProgress = true
		pg.ParentWindow().Reload()
	}()

	// Refresh the window now to signify that the syncing
	// has started with pg.isSyncing set to true above.
	pg.ParentWindow().Reload()
}

func (pg *TreasuryPage) Layout(gtx C) D {
	// If proposals API is not allowed, display the overlay with the message.
	overlay := layout.Stacked(func(_ C) D { return D{} })
	if !pg.isTreasuryAPIAllowed() {
		gtxCopy := gtx
		overlay = layout.Stacked(func(_ C) D {
			str := values.StringF(values.StrNotAllowed, values.String(values.StrGovernance))
			return components.DisablePageWithOverlay(pg.Load, nil, gtxCopy, str, "", &pg.navigateToSettingsBtn)
		})
		// Disable main page from receiving events
		gtx = gtx.Disabled()
	}

	return layout.Stack{}.Layout(gtx, layout.Expanded(pg.layout), overlay)
}

func (pg *TreasuryPage) layout(gtx C) D {
	if pg.selectedDCRWallet == nil {
		return pg.decredWalletRequired(gtx)
	}
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		padding := values.MarginPadding24
		if pg.IsMobileView() {
			padding = values.MarginPadding12
		}
		return layout.Inset{
			Left:  padding,
			Top:   values.MarginPadding16,
			Right: padding,
		}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(pg.Theme.Label(pg.ConvertTextSize(values.TextSize20), values.String(values.StrTreasurySpending)).Layout),
								layout.Rigid(pg.infoButton.Layout),
							)
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, pg.layoutVerifyGovernanceKeys)
						}),
					)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
						return layout.Stack{}.Layout(gtx,
							layout.Expanded(func(gtx C) D {
								return layout.Inset{
									Top: values.MarginPadding60,
								}.Layout(gtx, pg.layoutContent)
							}),
							layout.Expanded(pg.dropdownLayout),
						)
					})
				}),
			)
		})
	})
}

func (pg *TreasuryPage) dropdownLayout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if pg.walletDropDown == nil {
				return D{}
			}
			if len(pg.assetWallets) < 2 {
				return D{}
			}
			return layout.W.Layout(gtx, pg.walletDropDown.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, pg.Theme.Separator().Layout)
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
					}.Layout(gtx, pg.Theme.Label(pg.ConvertTextSize(values.TextSize16), values.String(values.StrVerifyGovernanceKeys)).Layout)
				}),
			)
		})
	})
}

func (pg *TreasuryPage) layoutContent(gtx C) D {
	return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
		list := layout.List{Axis: layout.Vertical}
		return pg.Theme.List(pg.listContainer).Layout(gtx, 1, func(gtx C, _ int) D {
			return list.Layout(gtx, len(pg.treasuryItems), func(gtx C, i int) D {
				return layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(pg.layoutPiKey),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding24}.Layout(gtx, func(gtx C) D {
								return components.TreasuryItemWidget(gtx, pg.Load, pg.treasuryItems[i])
							})
						}),
					)
				})
			})
		})
	})
}

func (pg *TreasuryPage) layoutPiKey(gtx C) D {
	backgroundColor := pg.Theme.Color.LightBlue
	if pg.AssetsManager.IsDarkModeOn() {
		backgroundColor = pg.Theme.Color.Background
	}
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := pg.Theme.Label(pg.ConvertTextSize(values.TextSize18), values.String(values.StrPiKey))
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
			}.Layout2(gtx, pg.Theme.Label(pg.ConvertTextSize(values.TextSize14), pg.PiKey).Layout)
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
			err := pg.selectedDCRWallet.SetTreasuryPolicy(treasuryItem.Policy.PiKey, votingPreference, "", password)
			if err != nil {
				pm.SetError(err.Error())
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

// TODO: Temporary UI. Pending when new designs will be ready for this feature
func (pg *TreasuryPage) decredWalletRequired(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
		Direction:   layout.Center,
		Alignment:   layout.Middle,
	}.Layout2(gtx, func(gtx C) D {
		options := components.FlexOptions{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		}
		widgets := []func(gtx C) D{
			func(gtx C) D {
				txt := "This feature requires that you have a decred wallet."
				lbl := pg.Theme.Label(values.TextSize16, txt)
				lbl.Font.Weight = font.SemiBold
				return lbl.Layout(gtx)
			},
			func(gtx C) D {
				return layout.Inset{
					Top: values.MarginPadding20,
				}.Layout(gtx, func(gtx C) D {
					return pg.createWalletBtn.Layout(gtx)
				})
			},
		}
		return components.FlexLayout(gtx, options, widgets)
	})
}

func (pg *TreasuryPage) walletCreationSuccessFunc() {
	pg.OnNavigatedTo()
	pg.ParentWindow().CloseCurrentPage()
	pg.ParentWindow().Reload()
}
