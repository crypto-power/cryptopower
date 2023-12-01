package governance

import (
	"context"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
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
	pageSize = int32(10)
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type ProposalsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	scroll         *components.Scroll[*components.ProposalItem]
	previousFilter int32
	statusDropDown *cryptomaterial.DropDown
	proposalsList  *cryptomaterial.ClickableList
	syncButton     *widget.Clickable
	searchEditor   cryptomaterial.Editor

	infoButton cryptomaterial.IconButton

	updatedIcon *cryptomaterial.Icon

	syncCompleted    bool
	isSyncing        bool
	proposalsFetched bool
}

func NewProposalsPage(l *load.Load) *ProposalsPage {
	pg := &ProposalsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ProposalsPageID),
	}
	pg.searchEditor = l.Theme.IconEditor(new(widget.Editor), values.String(values.StrSearch), l.Theme.Icons.SearchIcon, true)
	pg.searchEditor.Editor.SingleLine, pg.searchEditor.Editor.Submit, pg.searchEditor.Bordered = true, true, false

	pg.updatedIcon = cryptomaterial.NewIcon(pg.Theme.Icons.NavigationCheck)
	pg.updatedIcon.Color = pg.Theme.Color.Success

	pg.syncButton = new(widget.Clickable)
	pg.scroll = components.NewScroll(l, pageSize, pg.fetchProposals)

	pg.proposalsList = pg.Theme.NewClickableList(layout.Vertical)
	pg.proposalsList.IsShadowEnabled = true

	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.infoButton.Size = values.MarginPadding20

	pg.statusDropDown = l.Theme.DropDown([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrAll)},
		{Text: values.String(values.StrUnderReview)},
		{Text: values.String(values.StrApproved)},
		{Text: values.String(values.StrRejected)},
		{Text: values.String(values.StrAbandoned)},
	}, values.ProposalDropdownGroup, 0)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
// Once proposals update is complete fetchProposals() is automatically called.
func (pg *ProposalsPage) OnNavigatedTo() {
	if pg.isGovernanceAPIAllowed() {
		pg.syncAndUpdateProposals() // starts a sync listener which is stopped in OnNavigatedFrom().
		pg.proposalsFetched = true
	}
}

func (pg *ProposalsPage) syncAndUpdateProposals() {
	go pg.AssetsManager.Politeia.Sync(context.Background())
	// Only proceed if allowed make Proposals API call.
	pg.listenForSyncNotifications()
	go pg.scroll.FetchScrollData(false, pg.ParentWindow())
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

	isReset := pg.previousFilter != proposalFilter
	if isReset {
		// reset the offset to zero
		offset = 0
		pg.previousFilter = proposalFilter
	}

	proposalItems := components.LoadProposals(pg.Load, proposalFilter, offset, pageSize, true)
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

	return listItems, len(listItems), isReset, nil
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ProposalsPage) HandleUserInteractions() {
	if pg.statusDropDown.Changed() {
		pg.scroll.FetchScrollData(false, pg.ParentWindow())
	}

	pg.searchEditor.EditorIconButtonEvent = func() {
		// TODO: Proposals search functionality
	}

	if clicked, selectedItem := pg.proposalsList.ItemClicked(); clicked {
		proposalItems := pg.scroll.FetchedData()
		selectedProposal := proposalItems[selectedItem].Proposal
		pg.ParentNavigator().Display(NewProposalDetailsPage(pg.Load, &selectedProposal))
	}

	for pg.syncButton.Clicked() {
		go pg.AssetsManager.Politeia.Sync(context.Background())
		pg.isSyncing = true

		// TODO: check after 1min if sync does not start, set isSyncing to false and cancel sync
	}

	if !pg.proposalsFetched && pg.isGovernanceAPIAllowed() {
		// TODO: What scenario leads to this??
		pg.syncAndUpdateProposals()
		pg.proposalsFetched = true
	}

	if pg.infoButton.Button.Clicked() {
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

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ProposalsPage) Layout(gtx C) D {
	pg.scroll.OnScrollChangeListener(pg.ParentWindow())
	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *ProposalsPage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(pg.layoutSectionHeader),
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding60}.Layout(gtx, pg.layoutContent)
					}),
					layout.Expanded(func(gtx C) D {
						return pg.statusDropDown.Layout(gtx, 10, true)
					}),
				)
			})
		}),
	)
}

func (pg *ProposalsPage) layoutMobile(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.layoutSectionHeader)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding60}.Layout(gtx, pg.layoutContent)
					}),
					layout.Expanded(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.E.Layout(gtx, func(gtx C) D {
							card := pg.Theme.Card()
							card.Radius = cryptomaterial.Radius(8)
							return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
								return card.Layout(gtx, func(gtx C) D {
									return layout.UniformInset(values.MarginPadding8).Layout(gtx, pg.layoutSyncSection)
								})
							})
						})
					}),
					layout.Expanded(func(gtx C) D {
						return pg.statusDropDown.Layout(gtx, 55, true)
					}),
				)
			})
		}),
	)
}

func (pg *ProposalsPage) layoutContent(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return pg.scroll.List().Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return pg.Theme.Card().Layout(gtx, func(gtx C) D {
						if pg.scroll.ItemsCount() <= 0 {
							return components.LayoutNoProposalsFound(gtx, pg.Load, pg.isSyncing, 0)
						}
						proposalItems := pg.scroll.FetchedData()
						return pg.proposalsList.Layout(gtx, len(proposalItems), func(gtx C, i int) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return components.ProposalsList(pg.ParentWindow(), gtx, pg.Load, proposalItems[i])
								}),
								layout.Rigid(func(gtx C) D {
									return pg.Theme.Separator().Layout(gtx)
								}),
							)
						})
					})
				})
			})
		}),
	)
}

func (pg *ProposalsPage) layoutSyncSection(gtx C) D {
	if pg.isSyncing {
		return pg.layoutIsSyncingSection(gtx)
	} else if pg.syncCompleted {
		return pg.updatedIcon.Layout(gtx, values.MarginPadding20)
	}
	return pg.layoutStartSyncSection(gtx)
}

func (pg *ProposalsPage) layoutIsSyncingSection(gtx C) D {
	gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding24)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	loader := material.Loader(pg.Theme.Base)
	loader.Color = pg.Theme.Color.Gray1
	return loader.Layout(gtx)
}

func (pg *ProposalsPage) layoutStartSyncSection(gtx C) D {
	// TODO: use cryptomaterial clickable
	return material.Clickable(gtx, pg.syncButton, pg.Theme.Icons.Restore.Layout24dp)
}

func (pg *ProposalsPage) layoutSectionHeader(gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(pg.Theme.Label(values.TextSize20, values.String(values.StrProposal)).Layout), // Do we really need to display the title? nav is proposals already
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
						if pg.isSyncing {
							text = values.String(values.StrSyncingState)
						} else if pg.syncCompleted {
							text = values.String(values.StrUpdated)
						} else {
							text = values.String(values.StrUpdated) + " " + components.TimeAgo(pg.AssetsManager.Politeia.GetLastSyncedTimeStamp())
						}

						lastUpdatedInfo := pg.Theme.Label(values.TextSize10, text)
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

			go pg.scroll.FetchScrollData(false, pg.ParentWindow())
			pg.ParentWindow().Reload()
		}
	}
	err := pg.AssetsManager.Politeia.AddSyncCallback(proposalSyncCallback, ProposalsPageID)
	if err != nil {
		log.Errorf("Error adding politeia notification listener: %v", err)
		return
	}
}
