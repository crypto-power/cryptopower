package governance

import (
	"context"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	ProposalsPageID = "Proposals"

	// pageSize defines the number of proposals that can be fetched at ago.
	pageSize = int32(20)
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type pFilter struct {
	TypeFilter  int32
	OrderNewest bool
}

type ProposalsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scroll         *components.Scroll[*components.ProposalItem]
	previousFilter pFilter
	statusDropDown *cryptomaterial.DropDown
	orderDropDown  *cryptomaterial.DropDown
	walletDropDown *cryptomaterial.DropDown
	filterBtn      *cryptomaterial.Clickable
	isFilterOpen   bool

	proposalsList *cryptomaterial.ClickableList
	syncButton    *widget.Clickable
	searchEditor  cryptomaterial.Editor

	infoButton  cryptomaterial.IconButton
	updatedIcon *cryptomaterial.Icon

	assetWallets   []sharedW.Asset
	selectedWallet sharedW.Asset

	syncCompleted    bool
	isSyncing        bool
	proposalsFetched bool
}

func NewProposalsPage(l *load.Load) *ProposalsPage {
	pg := &ProposalsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ProposalsPageID),
	}

	pg.searchEditor = l.Theme.SearchEditor(new(widget.Editor), values.String(values.StrSearch), l.Theme.Icons.SearchIcon)
	pg.searchEditor.Editor.SingleLine = true
	pg.searchEditor.TextSize = pg.ConvertTextSize(l.Theme.TextSize)

	pg.updatedIcon = cryptomaterial.NewIcon(pg.Theme.Icons.NavigationCheck)
	pg.updatedIcon.Color = pg.Theme.Color.Success

	pg.syncButton = new(widget.Clickable)
	pg.scroll = components.NewScroll(l, pageSize, pg.fetchProposals)

	pg.proposalsList = pg.Theme.NewClickableList(layout.Vertical)
	pg.proposalsList.IsShadowEnabled = true

	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.infoButton.Size = values.MarginPadding20

	pg.filterBtn = l.Theme.NewClickable(false)

	pg.statusDropDown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrAll)},
		{Text: values.String(values.StrUnderReview)},
		{Text: values.String(values.StrApproved)},
		{Text: values.String(values.StrRejected)},
		{Text: values.String(values.StrAbandoned)},
	}, values.ProposalDropdownGroup, 1, 0, true)

	pg.orderDropDown = l.Theme.DropdownWithCustomPos([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrNewest)},
		{Text: values.String(values.StrOldest)},
	}, values.ProposalDropdownGroup, 1, 0, true)

	if pg.statusDropDown.Reversed() {
		pg.statusDropDown.ExpandedLayoutInset.Right = values.MarginPadding10
	} else {
		pg.statusDropDown.ExpandedLayoutInset.Left = values.MarginPadding10
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

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
// Once proposals update is complete fetchProposals() is automatically called.
func (pg *ProposalsPage) OnNavigatedTo() {
	pg.initWalletSelector()
	if pg.isGovernanceAPIAllowed() {
		pg.syncAndUpdateProposals() // starts a sync listener which is stopped in OnNavigatedFrom().
		pg.proposalsFetched = true
	}
}

func (pg *ProposalsPage) syncAndUpdateProposals() {
	go pg.AssetsManager.Politeia.Sync(context.Background())
	// Only proceed if allowed make Proposals API call.
	pg.listenForSyncNotifications()
	go pg.scroll.FetchScrollData(false, pg.ParentWindow(), false)
	pg.isSyncing = pg.AssetsManager.Politeia.IsSyncing()
}

func (pg *ProposalsPage) isGovernanceAPIAllowed() bool {
	return pg.AssetsManager.IsHTTPAPIPrivacyModeOff(libutils.GovernanceHTTPAPI)
}

// fetchProposals is thread safe and on completing proposals fetch it triggers
// UI update with the new proposals list.
func (pg *ProposalsPage) fetchProposals(offset, pageSize int32) ([]*components.ProposalItem, int, bool, error) {
	var proposalFilter int32
	selectedType := pg.statusDropDown.Selected()
	switch selectedType {
	case values.String(values.StrApproved):
		proposalFilter = libwallet.ProposalCategoryApproved
	case values.String(values.StrRejected):
		proposalFilter = libwallet.ProposalCategoryRejected
	case values.String(values.StrAbandoned):
		proposalFilter = libwallet.ProposalCategoryAbandoned
	default:
		proposalFilter = libwallet.ProposalCategoryAll
	}

	orderNewest := pg.orderDropDown.Selected() != values.String(values.StrOldest)
	searchKey := pg.searchEditor.Editor.Text()
	proposalItems := components.LoadProposals(pg.Load, proposalFilter, offset, pageSize, orderNewest, strings.TrimSpace(searchKey))
	listItems := make([]*components.ProposalItem, 0)

	if selectedType == values.String(values.StrUnderReview) {
		// group 'In discussion' and 'Active' proposals into under review
		for _, item := range proposalItems {
			if item.Proposal.Category == libwallet.ProposalCategoryPre ||
				item.Proposal.Category == libwallet.ProposalCategoryActive {
				listItems = append(listItems, item)
			}
		}
	} else {
		listItems = proposalItems
	}

	return listItems, len(listItems), true, nil
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ProposalsPage) HandleUserInteractions(gtx C) {
	if pg.statusDropDown.Changed(gtx) {
		pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	}

	if pg.orderDropDown.Changed(gtx) {
		pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	}

	if pg.walletDropDown != nil && pg.walletDropDown.Changed(gtx) {
		pg.selectedWallet = pg.assetWallets[pg.walletDropDown.SelectedIndex()]
	}

	if clicked, selectedItem := pg.proposalsList.ItemClicked(); clicked {
		proposalItems := pg.scroll.FetchedData()
		selectedProposal := proposalItems[selectedItem].Proposal
		pg.ParentNavigator().Display(NewProposalDetailsPage(pg.Load, &selectedProposal))
	}

	for pg.syncButton.Clicked(gtx) {
		go pg.AssetsManager.Politeia.Sync(context.Background())
		pg.isSyncing = true

		// TODO: check after 1min if sync does not start, set isSyncing to false and cancel sync
	}

	if !pg.proposalsFetched && pg.isGovernanceAPIAllowed() {
		// TODO: What scenario leads to this??
		pg.syncAndUpdateProposals()
		pg.proposalsFetched = true
	}

	if pg.infoButton.Button.Clicked(gtx) {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrProposal)).
			Body(values.String(values.StrOffChainVote)).
			SetCancelable(true).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(infoModal)
	}

	if pg.syncCompleted {
		time.AfterFunc(time.Second*3, func() {
			pg.syncCompleted = false
			pg.ParentWindow().Reload()
		})
	}

	for {
		event, ok := pg.searchEditor.Editor.Update(gtx)
		if !ok {
			break
		}

		if gtx.Source.Focused(pg.searchEditor.Editor) {
			switch event.(type) {
			case widget.ChangeEvent:
				pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
			}
		}
	}

	// for _, evt := range pg.searchEditor.Editor.Events() {
	// 	if pg.searchEditor.Editor.Focused() {
	// 		switch evt.(type) {
	// 		case widget.ChangeEvent:
	// 			pg.scroll.FetchScrollData(false, pg.ParentWindow(), true)
	// 		}
	// 	}
	// }

	for pg.filterBtn.Clicked(gtx) {
		pg.isFilterOpen = !pg.isFilterOpen
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *ProposalsPage) OnNavigatedFrom() {
	pg.AssetsManager.Politeia.RemoveSyncCallback(ProposalsPageID)
}

// initWalletSelector initializes the wallet selector dropdown to enable
// filtering proposals
func (pg *ProposalsPage) initWalletSelector() {
	pg.assetWallets = pg.AssetsManager.AllDCRWallets()

	items := []cryptomaterial.DropDownItem{}
	for _, wal := range pg.assetWallets {
		item := cryptomaterial.DropDownItem{
			Text: wal.GetWalletName(),
			Icon: pg.Theme.AssetIcon(wal.GetAssetType()),
		}
		items = append(items, item)
	}

	pg.walletDropDown = pg.Theme.DropdownWithCustomPos(items, values.WalletsDropdownGroup, 2, 0, false)
	pg.walletDropDown.Width = values.MarginPadding150
	settingCommonDropdown(pg.Theme, pg.walletDropDown)
	pg.walletDropDown.SetConvertTextSize(pg.ConvertTextSize)
}

func settingCommonDropdown(t *cryptomaterial.Theme, drodown *cryptomaterial.DropDown) {
	drodown.FontWeight = font.SemiBold
	drodown.Hoverable = false
	drodown.SelectedItemIconColor = &t.Color.Primary
	drodown.ExpandedLayoutInset = layout.Inset{Top: values.MarginPadding35}
	drodown.MakeCollapsedLayoutVisibleWhenExpanded = true
	drodown.Background = &t.Color.Surface
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ProposalsPage) Layout(gtx C) D {
	pg.scroll.OnScrollChangeListener(pg.ParentWindow())
	padding := values.MarginPadding24
	if pg.IsMobileView() {
		padding = values.MarginPadding12
	}
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		inset := layout.Inset{
			Top:    values.MarginPadding16,
			Right:  padding,
			Left:   padding,
			Bottom: values.MarginPadding16,
		}
		return inset.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pg.layoutSectionHeader),
				layout.Flexed(1, func(gtx C) D {
					return layout.Inset{
						Top: values.MarginPadding16,
					}.Layout(gtx, func(gtx C) D {
						return layout.Stack{}.Layout(gtx,
							layout.Expanded(func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										topInset := values.MarginPadding50
										if !pg.isFilterOpen && pg.IsMobileView() {
											return layout.Spacer{Height: topInset}.Layout(gtx)
										}
										if pg.IsMobileView() && pg.isFilterOpen {
											topInset = values.MarginPadding80
										}
										return layout.Inset{
											Top: topInset,
										}.Layout(gtx, pg.searchEditor.Layout)
									}),
									layout.Rigid(pg.layoutContent),
								)
							}),
							layout.Stacked(pg.dropdownLayout),
						)
					})
				}),
			)
		})
	})
}

func (pg *ProposalsPage) dropdownLayout(gtx C) D {
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

func (pg *ProposalsPage) leftDropdown(gtx C) D {
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
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
			return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return pg.filterBtn.Layout(gtx, icon.Layout16dp)
			})
		}),
	)
}

func (pg *ProposalsPage) rightDropdown(gtx C) D {
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

func (pg *ProposalsPage) layoutContent(gtx C) D {
	return pg.scroll.List().Layout(gtx, 1, func(gtx C, i int) D {
		return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
			if pg.scroll.ItemsCount() <= 0 {
				isProposalSyncing := pg.AssetsManager.Politeia.IsSyncing()
				return components.LayoutNoProposalsFound(gtx, pg.Load, isProposalSyncing || pg.scroll.ItemsCount() == -1, 0)
			}
			proposalItems := pg.scroll.FetchedData()
			return pg.proposalsList.Layout(gtx, len(proposalItems), func(gtx C, i int) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return components.ProposalsList(gtx, pg.Load, proposalItems[i])
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Top:    values.MarginPadding7,
							Bottom: values.MarginPadding7,
						}.Layout(gtx, pg.Theme.Separator().Layout)
					}),
				)
			})
		})
	})
}

func (pg *ProposalsPage) layoutSectionHeader(gtx C) D {
	isProposalSyncing := pg.AssetsManager.Politeia.IsSyncing()
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					lb := pg.Theme.Label(pg.ConvertTextSize(values.TextSize20), values.String(values.StrProposal))
					lb.Font.Weight = font.SemiBold
					return lb.Layout(gtx)
				}), // Do we really need to display the title? nav is proposals already
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPadding3}.Layout(gtx, pg.infoButton.Layout)
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			body := func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						var text string
						if isProposalSyncing {
							text = values.String(values.StrSyncingState)
						} else if pg.syncCompleted {
							text = values.String(values.StrUpdated)
						} else {
							text = values.String(values.StrUpdated) + " " + components.TimeAgo(pg.AssetsManager.Politeia.GetLastSyncedTimeStamp())
						}

						lastUpdatedInfo := pg.Theme.Label(pg.ConvertTextSize(values.TextSize10), text)
						lastUpdatedInfo.Color = pg.Theme.Color.GrayText2
						if pg.syncCompleted {
							lastUpdatedInfo.Color = pg.Theme.Color.Success
						}

						return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, lastUpdatedInfo.Layout)
					}),
				)
			}
			return layout.E.Layout(gtx, body)
		}),
	)
}

func (pg *ProposalsPage) listenForSyncNotifications() {
	proposalSyncCallback := func(propName string, status libutils.ProposalStatus) {
		if status == libutils.ProposalStatusSynced {
			pg.syncCompleted = true
			pg.isSyncing = false
			go pg.scroll.FetchScrollData(false, pg.ParentWindow(), false)
			pg.ParentWindow().Reload()
		}
	}
	err := pg.AssetsManager.Politeia.AddSyncCallback(proposalSyncCallback, ProposalsPageID)
	if err != nil {
		log.Errorf("Error adding politeia notification listener: %v", err)
		return
	}
}
