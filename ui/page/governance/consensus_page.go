package governance

import (
	"time"

	"gioui.org/font/gofont"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
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

	// TODO: Currently always nil. Implement a dcr wallet selector.
	selectedDCRWallet *dcr.Asset

	consensusItems []*components.ConsensusItem

	listContainer       *widget.List
	syncButton          *widget.Clickable
	viewVotingDashboard *cryptomaterial.Clickable
	copyRedirectURL     *cryptomaterial.Clickable
	redirectIcon        *cryptomaterial.Image

	statusDropDown *cryptomaterial.DropDown
	consensusList  *cryptomaterial.ClickableList

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

	pg.statusDropDown = l.Theme.DropDown([]cryptomaterial.DropDownItem{
		{Text: values.String(values.StrAll)},
		{Text: values.String(values.StrUpcoming)},
		{Text: values.String(values.StrInProgress)},
		{Text: values.String(values.StrFailed)},
		{Text: values.String(values.StrLockedIn)},
		{Text: values.String(values.StrFinished)},
	}, values.ConsensusDropdownGroup, 0)

	return pg
}

func (pg *ConsensusPage) OnNavigatedTo() {
	if pg.isAgendaAPIAllowed() {
		// Only query the agendas if the Agenda API is allowed.
		pg.FetchAgendas()
	}
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
		SetPositiveButtonCallback(func(isChecked bool, im *modal.InfoModal) bool {
			im.Dismiss()
			voteModal := newAgendaVoteModal(pg.Load, pg.selectedDCRWallet, agenda, radiogroupbtns.Value, func() {
				pg.FetchAgendas() // re-fetch agendas when modal is dismissed
			})
			pg.ParentWindow().ShowModal(voteModal)
			return true
		})
	pg.ParentWindow().ShowModal((voteModal))
}

func (pg *ConsensusPage) HandleUserInteractions() {
	for pg.statusDropDown.Changed() {
		pg.FetchAgendas()
	}

	if pg.navigateToSettingsBtn.Button.Clicked() {
		pg.ParentWindow().Display(settings.NewSettingsPage(pg.Load))
	}

	for _, item := range pg.consensusItems {
		if item.VoteButton.Clicked() {
			pg.agendaVoteChoiceModal(&item.Agenda)
		}
	}

	for pg.syncButton.Clicked() {
		pg.FetchAgendas()
	}

	if pg.infoButton.Button.Clicked() {
		infoModal := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrConsensusChange)).
			Body(values.String(values.StrOnChainVote)).
			SetCancelable(true).
			SetPositiveButtonText(values.String(values.StrGotIt))
		pg.ParentWindow().ShowModal(infoModal)
	}

	for pg.viewVotingDashboard.Clicked() {
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
												if pg.copyRedirectURL.Clicked() {
													clipboard.WriteOp{Text: host}.Add(gtx.Ops)
													pg.Toast.Notify(values.String(values.StrCopied))
												}
												return pg.copyRedirectURL.Layout(gtx, pg.Theme.Icons.CopyIcon.Layout24dp)
											})
										}),
									)
								})
							})
						})
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
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
}

func (pg *ConsensusPage) FetchAgendas() {
	selectedType := pg.statusDropDown.Selected()
	// TODO: pg.selectedDCRWallet is currently always nil. Implement wallet
	// selector. It is impossible to vote on an agenda without a dcr wallet.
	// Also, when the selected wallet changes, this method should be re-called,
	// to fetch and display the newly selected wallet's vote choices.
	pg.isSyncing = true

	// Fetch (or re-fetch) agendas in background as this makes
	// a network call. Refresh the window once the call completes.
	go func() {
		items := components.LoadAgendas(pg.Load, pg.selectedDCRWallet, true)
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
	overlay := layout.Stacked(func(gtx C) D { return D{} })
	if !pg.isAgendaAPIAllowed() {
		gtxCopy := gtx
		overlay = layout.Stacked(func(gtx C) D {
			str := values.StringF(values.StrNotAllowed, values.String(values.StrGovernance))
			return components.DisablePageWithOverlay(pg.Load, nil, gtxCopy, str, &pg.navigateToSettingsBtn)
		})
		// Disable main page from receiving events
		gtx = gtx.Disabled()
	}

	mainChild := layout.Expanded(func(gtx C) D {
		if pg.Load.IsMobileView() {
			return pg.layoutMobile(gtx)
		}
		return pg.layoutDesktop(gtx)
	})

	return layout.Stack{}.Layout(gtx, mainChild, overlay)
}

func (pg *ConsensusPage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(pg.Theme.Label(values.TextSize20, values.String(values.StrConsensusChange)).Layout), // Do we really need to display the title? nav is proposals already
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPadding3}.Layout(gtx, pg.infoButton.Layout)
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
					layout.Expanded(func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding60,
						}.Layout(gtx, pg.layoutContent)
					}),
					layout.Expanded(func(gtx C) D {
						return pg.statusDropDown.Layout(gtx, 10, true)
					}),
				)
			})
		}),
	)
}

func (pg *ConsensusPage) layoutMobile(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(pg.Theme.Label(values.TextSize20, values.String(values.StrConsensusChange)).Layout), // Do we really need to display the title? nav is proposals already
						layout.Rigid(pg.infoButton.Layout),
					)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return layout.Inset{Right: values.MarginPadding10, Top: values.MarginPadding5}.Layout(gtx, pg.layoutRedirectVoting)
					})
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.E.Layout(gtx, func(gtx C) D {
							card := pg.Theme.Card()
							card.Radius = cryptomaterial.Radius(8)
							return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
								return card.Layout(gtx, func(gtx C) D {
									return layout.UniformInset(values.MarginPadding8).Layout(gtx, func(gtx C) D {
										return pg.layoutSyncSection(gtx)
									})
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
						}.Layout(gtx, pg.Theme.Label(values.TextSize16, values.String(values.StrVotingDashboard)).Layout)
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
			return pg.Theme.List(pg.listContainer).Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return list.Layout(gtx, len(pg.consensusItems), func(gtx C, i int) D {
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

func (pg *ConsensusPage) layoutSyncSection(gtx C) D {
	if pg.isSyncing {
		return pg.layoutIsSyncingSection(gtx)
	} else if pg.syncCompleted {
		updatedIcon := cryptomaterial.NewIcon(pg.Theme.Icons.NavigationCheck)
		updatedIcon.Color = pg.Theme.Color.Success
		return updatedIcon.Layout(gtx, values.MarginPadding20)
	}
	return pg.layoutStartSyncSection(gtx)
}

func (pg *ConsensusPage) layoutIsSyncingSection(gtx C) D {
	th := material.NewTheme(gofont.Collection())
	gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding24)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	loader := material.Loader(th)
	loader.Color = pg.Theme.Color.Gray1
	return loader.Layout(gtx)
}

func (pg *ConsensusPage) layoutStartSyncSection(gtx C) D {
	// TODO: use cryptomaterial clickable
	return material.Clickable(gtx, pg.syncButton, pg.Theme.Icons.Restore.Layout24dp)
}
