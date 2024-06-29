package governance

import (
	"io"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

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

const ConsensusPageID = "Consensus"

type ConsensusPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	assetWallets      []sharedW.Asset
	selectedDCRWallet *dcr.Asset

	consensusItems []*components.ConsensusItem

	listContainer       *widget.List
	syncButton          *widget.Clickable
	viewVotingDashboard *cryptomaterial.Clickable
	copyRedirectURL     *cryptomaterial.Clickable
	redirectIcon        *cryptomaterial.Image

	orderDropDown  *cryptomaterial.DropDown
	statusDropDown *cryptomaterial.DropDown
	walletDropDown *cryptomaterial.DropDown
	consensusList  *cryptomaterial.ClickableList
	filterBtn      *cryptomaterial.Clickable
	isFilterOpen   bool

	infoButton            cryptomaterial.IconButton
	navigateToSettingsBtn cryptomaterial.Button

	syncCompleted bool
	isSyncing     bool
}

func NewConsensusPage(l *load.Load) *ConsensusPage {
	pg := &ConsensusPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ConsensusPageID),
		consensusList:    l.Theme.NewClickableList(layout.Vertical),
		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		syncButton: new(widget.Clickable),

		redirectIcon:        l.Theme.Icons.RedirectIcon,
		viewVotingDashboard: l.Theme.NewClickable(true),
		copyRedirectURL:     l.Theme.NewClickable(false),
	}

	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.infoButton.Size = values.MarginPadding20
	pg.navigateToSettingsBtn = pg.Theme.Button(values.StringF(values.StrEnableAPI, values.String(values.StrGovernance)))
	pg.filterBtn = l.Theme.NewClickable(false)
	pg.orderDropDown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrNewest)},
		{Text: values.String(values.StrOldest)},
	}, values.ConsensusDropdownGroup, 1, 10, true)

	pg.statusDropDown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrAll)},
		{Text: values.String(values.StrUpcoming)},
		{Text: values.String(values.StrInProgress)},
		{Text: values.String(values.StrFailed)},
		{Text: values.String(values.StrLockedIn)},
		{Text: values.String(values.StrFinished)},
	}, values.ConsensusDropdownGroup, 1, 10, true)
	if pg.statusDropDown.Reversed() {
		pg.statusDropDown.ExpandedLayoutInset.Right = values.DP55
	} else {
		pg.statusDropDown.ExpandedLayoutInset.Left = values.DP55
	}

	pg.statusDropDown.CollapsedLayoutTextDirection = layout.E
	pg.orderDropDown.CollapsedLayoutTextDirection = layout.E
	pg.orderDropDown.Width = values.MarginPadding100
	if l.IsMobileView() {
		pg.orderDropDown.Width = values.MarginPadding85
		pg.statusDropDown.Width = values.DP118
	}
	settingCommonDropdown(pg.Theme, pg.statusDropDown)
	settingCommonDropdown(pg.Theme, pg.orderDropDown)
	pg.statusDropDown.SetConvertTextSize(pg.ConvertTextSize)
	pg.orderDropDown.SetConvertTextSize(pg.ConvertTextSize)

	pg.initWalletSelector()
	return pg
}

func (pg *ConsensusPage) OnNavigatedTo() {
	if pg.isAgendaAPIAllowed() {
		// Only query the agendas if the Agenda API is allowed.
		pg.FetchAgendas()
	}
}

func (pg *ConsensusPage) initWalletSelector() {
	pg.assetWallets = pg.AssetsManager.AllDCRWallets()

	items := []cryptomaterial.DropDownItem{}
	for _, wal := range pg.assetWallets {
		item := cryptomaterial.DropDownItem{
			Text: wal.GetWalletName(),
			Icon: pg.Theme.AssetIcon(wal.GetAssetType()),
		}
		items = append(items, item)
	}
	pg.walletDropDown = pg.Theme.DropdownWithCustomPos(items, values.WalletsDropdownGroup, 1, 0, false)
	pg.walletDropDown.Width = values.MarginPadding150
	settingCommonDropdown(pg.Theme, pg.walletDropDown)
	pg.walletDropDown.SetConvertTextSize(pg.ConvertTextSize)
}

func (pg *ConsensusPage) isAgendaAPIAllowed() bool {
	return pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
}

func (pg *ConsensusPage) OnNavigatedFrom() {}

func (pg *ConsensusPage) agendaVoteChoiceModal(agenda *dcr.Agenda) {
	var voteChoices []string
	consensusItems := components.LoadAgendas(pg.Load, pg.selectedDCRWallet, false)
	if len(consensusItems) > 0 {
		consensusItem := consensusItems[0]
		voteChoices = make([]string, len(consensusItem.Agenda.Choices))
		for i := range consensusItem.Agenda.Choices {
			caser := cases.Title(language.Und)
			voteChoices[i] = caser.String(consensusItem.Agenda.Choices[i].Id)
		}
	}

	radiogroupbtns := new(widget.Enum)
	items := make([]layout.FlexChild, 0)
	for i, voteChoice := range voteChoices {
		radioBtn := pg.Load.Theme.RadioButton(radiogroupbtns, voteChoice, voteChoice, pg.Load.Theme.Color.DeepBlue, pg.Load.Theme.Color.Primary)
		radioItem := layout.Rigid(radioBtn.Layout)
		items = append(items, radioItem)

		if i == 0 { // set the first one as the default value.
			radiogroupbtns.Value = voteChoice
		}
	}

	voteModal := modal.NewCustomModal(pg.Load).
		Title(values.String(values.StrVoteChoice)).
		UseCustomWidget(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		}).
		SetCancelable(true).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetPositiveButtonText(values.String(values.StrSave)).
		SetPositiveButtonCallback(func(_ bool, im *modal.InfoModal) bool {
			im.Dismiss()
			voteModal := newAgendaVoteModal(pg.Load, pg.selectedDCRWallet, agenda, radiogroupbtns.Value, func() {
				pg.FetchAgendas() // re-fetch agendas when modal is dismissed
			})
			pg.ParentWindow().ShowModal(voteModal)
			return true
		})
	pg.ParentWindow().ShowModal((voteModal))
}

func (pg *ConsensusPage) HandleUserInteractions(gtx C) {
	for pg.statusDropDown.Changed(gtx) {
		pg.FetchAgendas()
	}

	for pg.orderDropDown.Changed(gtx) {
		pg.FetchAgendas()
	}

	if pg.walletDropDown != nil && pg.walletDropDown.Changed(gtx) {
		pg.selectedDCRWallet = pg.assetWallets[pg.walletDropDown.SelectedIndex()].(*dcr.Asset)
		pg.FetchAgendas()
	}

	if pg.navigateToSettingsBtn.Button.Clicked(gtx) {
		pg.ParentWindow().Display(settings.NewAppSettingsPage(pg.Load))
	}

	for _, item := range pg.consensusItems {
		if item.VoteButton.Clicked(gtx) {
			pg.agendaVoteChoiceModal(item.Agenda)
		}
	}

	for pg.syncButton.Clicked(gtx) {
		pg.FetchAgendas()
	}

	if pg.infoButton.Button.Clicked(gtx) {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrConsensusChange)).
			Body(values.String(values.StrOnChainVote)).
			SetCancelable(true).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(infoModal)
	}

	for pg.viewVotingDashboard.Clicked(gtx) {
		host := "https://voting.decred.org"
		if pg.AssetsManager.NetType() == libwallet.Testnet {
			host = "https://voting.decred.org/testnet"
		}

		info := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrConsensusDashboard)).
			Body(values.String(values.StrCopyLink)).
			SetCancelable(true).
			UseCustomWidget(func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx C) D {
						border := widget.Border{Color: pg.Theme.Color.Gray4, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
						wrapper := pg.Theme.Card()
						wrapper.Color = pg.Theme.Color.Gray4
						return border.Layout(gtx, func(gtx C) D {
							return wrapper.Layout(gtx, func(gtx C) D {
								return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
									return layout.Flex{}.Layout(gtx,
										layout.Flexed(0.9, pg.Theme.Body1(host).Layout),
										layout.Flexed(0.1, func(gtx C) D {
											return layout.E.Layout(gtx, func(gtx C) D {
												if pg.copyRedirectURL.Clicked(gtx) {
													// clipboard.WriteOp{Text: host}.Add(gtx.Ops)
													gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(host))})
													pg.Toast.Notify(values.String(values.StrCopied))
												}
												return pg.copyRedirectURL.Layout(gtx, pg.Theme.NewIcon(pg.Theme.Icons.CopyIcon).Layout24dp)
											})
										}),
									)
								})
							})
						})
					}),
					layout.Stacked(func(gtx C) D {
						return layout.Inset{
							Top:  values.MarginPaddingMinus10,
							Left: values.MarginPadding10,
						}.Layout(gtx, func(gtx C) D {
							label := pg.Theme.Body2(values.String(values.StrWebURL))
							label.Color = pg.Theme.Color.GrayText2
							return label.Layout(gtx)
						})
					}),
				)
			}).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(info)
	}

	if pg.syncCompleted {
		time.AfterFunc(time.Second*1, func() {
			pg.syncCompleted = false
			pg.ParentWindow().Reload()
		})
	}

	for pg.filterBtn.Clicked(gtx) {
		pg.isFilterOpen = !pg.isFilterOpen
	}
}

func (pg *ConsensusPage) FetchAgendas() {
	selectedType := pg.statusDropDown.Selected()
	pg.isSyncing = true

	orderNewest := pg.orderDropDown.Selected() != values.String(values.StrOldest)

	// Fetch (or re-fetch) agendas in background as this makes
	// a network call. Refresh the window once the call completes.
	go func() {
		items := components.LoadAgendas(pg.Load, pg.selectedDCRWallet, orderNewest)
		agenda := dcr.AgendaStatusFromStr(selectedType)
		listItems := make([]*components.ConsensusItem, 0)
		if agenda == dcr.UnknownStatus {
			listItems = items
		} else {
			for _, item := range items {
				if dcr.AgendaStatusType(item.Agenda.Status) == agenda {
					listItems = append(listItems, item)
				}
			}
		}

		pg.isSyncing = false
		pg.syncCompleted = true
		pg.consensusItems = listItems
		pg.ParentWindow().Reload()
	}()

	// Refresh the window now to signify that the syncing
	// has started with pg.isSyncing set to true above.
	pg.ParentWindow().Reload()
}

func (pg *ConsensusPage) Layout(gtx C) D {
	// If Agendas API is not allowed, display the overlay with the message.
	overlay := layout.Stacked(func(_ C) D { return D{} })
	if !pg.isAgendaAPIAllowed() {
		gtxCopy := gtx
		overlay = layout.Stacked(func(_ C) D {
			str := values.StringF(values.StrNotAllowed, values.String(values.StrGovernance))
			return components.DisablePageWithOverlay(pg.Load, nil, gtxCopy, str, "", &pg.navigateToSettingsBtn)
		})
		// Disable main page from receiving events
		gtx = gtx.Disabled()
	}

	mainChild := layout.Expanded(func(gtx C) D {
		return pg.layout(gtx)
	})

	return layout.Stack{}.Layout(gtx, mainChild, overlay)
}

func (pg *ConsensusPage) layout(gtx C) D {
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
					return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Flex{}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									lb := pg.Theme.Label(pg.ConvertTextSize(values.TextSize20), values.String(values.StrConsensusChange))
									lb.Font.Weight = font.SemiBold
									return lb.Layout(gtx)
								}), // Do we really need to display the title? nav is proposals already
								layout.Rigid(func(gtx C) D {
									return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, pg.infoButton.Layout)
								}),
							)
						}),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, pg.layoutRedirectVoting)
						}),
					)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
						return layout.Stack{}.Layout(gtx,
							layout.Stacked(func(gtx C) D {
								topInset := values.MarginPadding50
								if pg.IsMobileView() && pg.isFilterOpen {
									topInset = values.MarginPadding80
								}
								return layout.Inset{
									Top: topInset,
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

func (pg *ConsensusPage) dropdownLayout(gtx C) D {
	if pg.IsMobileView() {
		return layout.Stack{}.Layout(gtx,
			layout.Stacked(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Inset{Top: values.MarginPadding40}.Layout(gtx, pg.rightDropdown)
			}),
			layout.Expanded(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return pg.leftDropdown(gtx)
			}),
		)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(pg.leftDropdown),
		layout.Rigid(pg.rightDropdown),
	)
}

func (pg *ConsensusPage) leftDropdown(gtx C) D {
	return layout.Flex{Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if pg.walletDropDown == nil {
				return D{}
			}
			if len(pg.assetWallets) == 0 {
				return D{}
			}
			return layout.W.Layout(gtx, pg.walletDropDown.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			if !pg.IsMobileView() {
				return D{}
			}
			icon := pg.Theme.Icons.FilterOffImgIcon
			if pg.isFilterOpen {
				icon = pg.Theme.Icons.FilterImgIcon
			}
			return layout.Inset{Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
				return pg.filterBtn.Layout(gtx, icon.Layout16dp)
			})
		}),
	)
}

func (pg *ConsensusPage) rightDropdown(gtx C) D {
	if !pg.isFilterOpen && pg.IsMobileView() {
		return D{}
	}
	return layout.E.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(pg.statusDropDown.Layout),
			layout.Rigid(pg.orderDropDown.Layout),
		)
	})
}

func (pg *ConsensusPage) layoutRedirectVoting(gtx C) D {
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.viewVotingDashboard.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Right: values.MarginPadding10,
						}.Layout(gtx, pg.redirectIcon.Layout16dp)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPaddingMinus2,
						}.Layout(gtx, pg.Theme.Label(pg.ConvertTextSize(values.TextSize16), values.String(values.StrVotingDashboard)).Layout)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx C) D {
			var text string
			if pg.isSyncing {
				text = values.String(values.StrSyncingState)
			} else if pg.syncCompleted {
				text = values.String(values.StrUpdated)
			}

			lastUpdatedInfo := pg.Theme.Label(values.TextSize10, text)
			lastUpdatedInfo.Color = pg.Theme.Color.GrayText2
			if pg.syncCompleted {
				lastUpdatedInfo.Color = pg.Theme.Color.Success
			}

			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, lastUpdatedInfo.Layout)
			})
		}),
	)
}

func (pg *ConsensusPage) layoutContent(gtx C) D {
	if len(pg.consensusItems) == 0 {
		return components.LayoutNoAgendasFound(gtx, pg.Load, pg.isSyncing)
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			list := layout.List{Axis: layout.Vertical}
			return pg.Theme.List(pg.listContainer).Layout(gtx, 1, func(gtx C, _ int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return list.Layout(gtx, len(pg.consensusItems), func(gtx C, i int) D {
						return cryptomaterial.LinearLayout{
							Orientation: layout.Vertical,
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Background:  pg.Theme.Color.Surface,
							Direction:   layout.W,
							Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
							Padding:     layout.Inset{Bottom: values.MarginPadding15, Top: values.MarginPadding15},
							Margin:      layout.Inset{Bottom: values.MarginPadding4, Top: values.MarginPadding4},
						}.
							Layout2(gtx, func(gtx C) D {
								// TODO: Implement dcr wallet selector to enable
								// voting.
								hasVotingWallet := pg.selectedDCRWallet != nil // Vote button will be disabled if nil.
								return components.AgendaItemWidget(gtx, pg.Load, pg.consensusItems[i], hasVotingWallet)
							})
					})
				})
			})
		}),
	)
}
