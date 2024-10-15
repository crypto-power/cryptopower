package governance

import (
	"context"
	"io"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
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
	pageutils "github.com/crypto-power/cryptopower/ui/utils"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	ConsensusPageID = "Consensus"

	// interval to run sync Agendas in minute
	consensusSyncInterval = 30
	// interval to refresh the view in second
	consensusRefreshView = 5
)

type ConsensusPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scroll            *components.Scroll[*components.ConsensusItem]
	assetWallets      []sharedW.Asset
	selectedDCRWallet *dcr.Asset

	listContainer       *widget.List
	syncButton          *cryptomaterial.Clickable
	materialLoader      material.LoaderStyle
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
	agendaFetched bool
	lastSyncTime  int64
	ticker        *time.Ticker
}

func NewConsensusPage(l *load.Load) *ConsensusPage {
	pg := &ConsensusPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ConsensusPageID),
		consensusList:    l.Theme.NewClickableList(layout.Vertical),
		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		redirectIcon:        l.Theme.Icons.RedirectIcon,
		viewVotingDashboard: l.Theme.NewClickable(true),
		copyRedirectURL:     l.Theme.NewClickable(false),
	}

	pg.lastSyncTime, _ = pg.AssetsManager.ConsensusAgenda.GetLastSyncedTimestamp()
	pg.scroll = components.NewScroll(l, pageSize, pg.FetchAgendas)
	pg.syncButton = l.Theme.NewClickable(false)
	pg.materialLoader = material.Loader(l.Theme.Base)

	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.infoButton.Size = values.MarginPadding20
	pg.navigateToSettingsBtn = pg.Theme.Button(values.StringF(values.StrEnableAPI, values.String(values.StrGovernance)))
	pg.filterBtn = l.Theme.NewClickable(false)
	pg.orderDropDown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrNewest)},
		{Text: values.String(values.StrOldest)},
	}, values.ConsensusDropdownGroup, 1, 10, false)

	pg.statusDropDown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrAll)},
		{Text: values.String(values.StrUpcoming)},
		{Text: values.String(values.StrInProgress)},
		{Text: values.String(values.StrFailed)},
		{Text: values.String(values.StrLockedIn)},
		{Text: values.String(values.StrFinished)},
	}, values.ConsensusDropdownGroup, 1, 10, false)
	if pg.statusDropDown.Reversed() {
		pg.statusDropDown.ExpandedLayoutInset.Right = values.DP55
	} else {
		pg.statusDropDown.ExpandedLayoutInset.Left = values.DP55
	}

	pg.statusDropDown.CollapsedLayoutTextDirection = layout.E
	pg.orderDropDown.CollapsedLayoutTextDirection = layout.E
	pg.orderDropDown.Width = values.MarginPadding100
	pg.statusDropDown.Width = values.DP118
	settingCommonDropdown(pg.Theme, pg.statusDropDown)
	settingCommonDropdown(pg.Theme, pg.orderDropDown)
	pg.statusDropDown.SetConvertTextSize(pg.ConvertTextSize)
	pg.orderDropDown.SetConvertTextSize(pg.ConvertTextSize)

	pg.initWalletSelector()

	// ticker to update the page and sync consensus after "consensusSyncInterval" minutes
	pg.ticker = time.NewTicker(time.Second)
	pg.ticker.Stop()
	go pg.refreshPageAndSyncInterval()
	return pg
}

func (pg *ConsensusPage) refreshPageAndSyncInterval() {
	for range pg.ticker.C {
		if pg.syncCompleted {
			pg.syncCompleted = false
		}
		pg.ParentWindow().Reload()
		if !pg.isSyncing && time.Since(time.Unix(pg.lastSyncTime, 0)) > time.Minute*consensusSyncInterval && pg.agendaFetched {
			pg.SyncAgenda()
		}
	}
}

func (pg *ConsensusPage) listenForSyncNotifications() {
	consensusSyncCallback := func(status libutils.AgendaSyncStatus) {
		if status == libutils.AgendaStatusSynced {
			pg.syncCompleted = true
			pg.isSyncing = false
			pg.lastSyncTime, _ = pg.AssetsManager.ConsensusAgenda.GetLastSyncedTimestamp()
			pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
			pg.ParentWindow().Reload()
			// start the ticker to update the page and sync proposals after "consensusRefreshView" minutes
			pg.ticker.Reset(time.Second * consensusRefreshView)
		}
	}
	err := pg.AssetsManager.ConsensusAgenda.AddSyncCallback(consensusSyncCallback, ConsensusPageID)
	if err != nil {
		log.Errorf("Error adding politeia notification listener: %v", err)
		return
	}
}

func (pg *ConsensusPage) OnNavigatedTo() {
	if pg.isAgendaAPIAllowed() {
		pg.syncAndUpdateAgenda()
		pg.agendaFetched = true
	}
}

func (pg *ConsensusPage) syncAndUpdateAgenda() {
	// Only proceed if allowed make Agenda API call.
	pg.listenForSyncNotifications()
	pg.scroll.FetchScrollData(false, pg.ParentWindow(), false)
	pg.SyncAgenda()
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

func (pg *ConsensusPage) OnNavigatedFrom() {
	pg.ticker.Stop()
	pg.AssetsManager.ConsensusAgenda.RemoveSyncCallback(ConsensusPageID)
}

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
				pg.scroll.FetchScrollData(false, pg.ParentWindow(), true) // re-fetch agendas when modal is dismissed
			})
			pg.ParentWindow().ShowModal(voteModal)
			return true
		})
	pg.ParentWindow().ShowModal((voteModal))
}

func (pg *ConsensusPage) HandleUserInteractions(gtx C) {
	for pg.statusDropDown.Changed(gtx) {
		pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	}

	for pg.orderDropDown.Changed(gtx) {
		pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	}

	if pg.walletDropDown != nil && pg.walletDropDown.Changed(gtx) {
		pg.selectedDCRWallet = pg.assetWallets[pg.walletDropDown.SelectedIndex()].(*dcr.Asset)
		pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	}

	if pg.navigateToSettingsBtn.Button.Clicked(gtx) {
		pg.ParentWindow().Display(settings.NewAppSettingsPage(pg.Load))
	}

	for _, item := range pg.scroll.FetchedData() {
		if item.VoteButton.Clicked(gtx) {
			pg.agendaVoteChoiceModal(item.Agenda)
		}
	}

	if pg.syncButton.Clicked(gtx) {
		if pg.isSyncing {
			return
		}
		pg.SyncAgenda()
	}

	if pg.infoButton.Button.Clicked(gtx) {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrConsensusChange)).
			Body(values.String(values.StrOnChainVote)).
			SetCancelable(true).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(infoModal)
	}

	if pg.viewVotingDashboard.Clicked(gtx) {
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

	if pg.filterBtn.Clicked(gtx) {
		pg.isFilterOpen = !pg.isFilterOpen
	}
}

func (pg *ConsensusPage) SyncAgenda() {
	pg.isSyncing = true
	pg.syncCompleted = false
	go func() { _ = pg.AssetsManager.ConsensusAgenda.Sync(context.Background()) }()
	pg.ParentWindow().Reload()
}

func (pg *ConsensusPage) FetchAgendas(_, _ int32) ([]*components.ConsensusItem, int, bool, error) {
	selectedType := pg.statusDropDown.Selected()
	orderNewest := pg.orderDropDown.Selected() != values.String(values.StrOldest)

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
	return listItems, len(listItems), true, nil
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
			if len(pg.assetWallets) < 2 {
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
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					var text string
					if pg.isSyncing {
						text = values.String(values.StrSyncingState)
					} else if pg.syncCompleted {
						text = values.String(values.StrUpdated)
					} else {
						text = values.String(values.StrUpdated) + " " + pageutils.TimeAgo(pg.lastSyncTime)
					}

					lastUpdatedInfo := pg.Theme.Label(values.TextSize12, text)
					lastUpdatedInfo.Color = pg.Theme.Color.GrayText2
					if pg.syncCompleted {
						lastUpdatedInfo.Color = pg.Theme.Color.Success
					}

					return layout.Inset{Bottom: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
						return lastUpdatedInfo.Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx C) D {
					return cryptomaterial.LinearLayout{
						Width:     cryptomaterial.WrapContent,
						Height:    cryptomaterial.WrapContent,
						Direction: layout.E,
						Alignment: layout.End,
						Margin:    layout.Inset{Left: values.MarginPadding2},
						Clickable: pg.syncButton,
					}.Layout2(gtx, func(gtx C) D {
						if pg.isSyncing {
							gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding20)
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.Inset{Left: values.MarginPadding5, Bottom: values.MarginPadding2}.Layout(gtx, pg.materialLoader.Layout)
						}
						return layout.Inset{Left: values.MarginPadding5}.Layout(gtx, pg.Theme.NewIcon(pg.Theme.Icons.NavigationRefresh).Layout18dp)
					})
				}),
			)
		}),
	)
}

func (pg *ConsensusPage) layoutContent(gtx C) D {
	return pg.scroll.List().Layout(gtx, 1, func(gtx C, _ int) D {
		return layout.Inset{Right: values.MarginPadding2, Top: values.MarginPadding15, Bottom: values.MarginPadding15}.Layout(gtx, func(gtx C) D {
			if pg.scroll.ItemsCount() <= 0 {
				return components.LayoutNoAgendasFound(gtx, pg.Load, pg.isSyncing)
			}
			consensusItems := pg.scroll.FetchedData()
			return pg.listContainer.Layout(gtx, len(consensusItems), func(gtx C, i int) D {
				return layout.Inset{
					Top:    values.MarginPadding5,
					Bottom: values.MarginPadding30,
				}.Layout(gtx, func(gtx C) D {
					hasVotingWallet := pg.selectedDCRWallet != nil
					return components.AgendaItemWidget(gtx, pg.Load, consensusItems[i], hasVotingWallet)
				})
			})
		})
	})
}
