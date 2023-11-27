package governance

import (
	"fmt"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/renderers"
	"github.com/crypto-power/cryptopower/ui/values"
)

const ProposalDetailsPageID = "proposal_details"

type proposalItemWidgets struct {
	widgets    []layout.Widget
	clickables map[string]*widget.Clickable
}

type ProposalDetails struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	descriptionList *layout.List

	proposal      *libwallet.Proposal
	proposalItems map[string]proposalItemWidgets

	scrollbarList *widget.List
	rejectedIcon  *widget.Icon
	successIcon   *widget.Icon

	redirectIcon *cryptomaterial.Image

	viewInPoliteiaBtn *cryptomaterial.Clickable
	copyRedirectURL   *cryptomaterial.Clickable
	tempRightHead     *cryptomaterial.Clickable

	descriptionCard cryptomaterial.Card
	vote            cryptomaterial.Button
	backButton      cryptomaterial.IconButton

	sourceWalletSelector *components.WalletAndAccountSelector
	selectedWallet       sharedW.Asset

	voteBar            *components.VoteBar
	loadingDescription bool
}

func NewProposalDetailsPage(l *load.Load, proposal *libwallet.Proposal) *ProposalDetails {
	pg := &ProposalDetails{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ProposalDetailsPageID),

		loadingDescription: false,
		proposal:           proposal,
		descriptionCard:    l.Theme.Card(),
		descriptionList:    &layout.List{Axis: layout.Vertical},
		scrollbarList: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		redirectIcon:      l.Theme.Icons.RedirectIcon,
		proposalItems:     make(map[string]proposalItemWidgets),
		rejectedIcon:      l.Theme.Icons.NavigationCancel,
		successIcon:       l.Theme.Icons.ActionCheckCircle,
		viewInPoliteiaBtn: l.Theme.NewClickable(true),
		copyRedirectURL:   l.Theme.NewClickable(false),
		tempRightHead:     l.Theme.NewClickable(false),
		voteBar:           components.NewVoteBar(l),
	}

	pg.backButton, _ = components.SubpageHeaderButtons(l)

	pg.vote = l.Theme.Button(values.String(values.StrVote))
	pg.vote.TextSize = values.TextSize14
	pg.vote.Background = l.Theme.Color.Primary
	pg.vote.Color = l.Theme.Color.Surface
	pg.vote.Inset = layout.Inset{
		Top:    values.MarginPadding8,
		Bottom: values.MarginPadding8,
		Left:   values.MarginPadding12,
		Right:  values.MarginPadding12,
	}

	pg.initWalletSelector()
	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ProposalDetails) OnNavigatedTo() {
	pg.listenForSyncNotifications() // listener is stopped in OnNavigatedFrom()
}

func (pg *ProposalDetails) initWalletSelector() {
	// Source wallet picker
	pg.sourceWalletSelector = components.NewWalletAndAccountSelector(pg.Load, libutils.DCRWalletAsset).
		Title(values.String(values.StrSelectWallet))
	pg.sourceWalletSelector.SetHideBalance(true)
	if pg.sourceWalletSelector.SelectedWallet() != nil {
		pg.selectedWallet = pg.sourceWalletSelector.SelectedWallet().Asset
	}

	pg.sourceWalletSelector.SetLeftAlignment(true)
	pg.sourceWalletSelector.SetBorder(false)

	pg.sourceWalletSelector.WalletSelected(func(selectedWallet *load.WalletMapping) {
		pg.selectedWallet = selectedWallet.Asset
		//TODO: implement when selected wallet
	})
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ProposalDetails) HandleUserInteractions() {
	for token := range pg.proposalItems {
		for location, clickable := range pg.proposalItems[token].clickables {
			if clickable.Clicked() {
				components.GoToURL(location)
			}
		}
	}

	if pg.vote.Clicked() {
		pg.ParentWindow().ShowModal(newVoteModal(pg.Load, pg.proposal))
	}

	for pg.viewInPoliteiaBtn.Clicked() {
		host := "https://proposals.decred.org/record/" + pg.proposal.Token
		if pg.WL.AssetsManager.NetType() == libwallet.Testnet {
			host = "https://test-proposals.decred.org/record/" + pg.proposal.Token
		}

		info := modal.NewCustomModal(pg.Load).
			Title(values.String(values.StrViewOnPoliteia)).
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
												return layout.Inset{Top: values.MarginPadding7}.Layout(gtx, func(gtx C) D {
													if pg.copyRedirectURL.Clicked() {
														clipboard.WriteOp{Text: host}.Add(gtx.Ops)
														pg.Toast.Notify(values.String(values.StrCopied))
													}
													return pg.copyRedirectURL.Layout(gtx, pg.Theme.Icons.CopyIcon.Layout24dp)
												})
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
}

func (pg *ProposalDetails) listenForSyncNotifications() {
	proposalSyncCallback := func(propName string, status libutils.ProposalStatus) {
		if status == libutils.ProposalStatusSynced {
			proposal, err := pg.WL.AssetsManager.Politeia.GetProposalRaw(pg.proposal.Token)
			if err == nil {
				pg.proposal = &libwallet.Proposal{Proposal: *proposal}
				pg.ParentWindow().Reload()
			}
		}
	}
	err := pg.WL.AssetsManager.Politeia.AddSyncCallback(proposalSyncCallback, ProposalDetailsPageID)
	if err != nil {
		log.Errorf("Error adding politeia notification listener: %v", err)
		return
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *ProposalDetails) OnNavigatedFrom() {
	pg.WL.AssetsManager.Politeia.RemoveSyncCallback(ProposalDetailsPageID)
}

// - Layout

func (pg *ProposalDetails) layoutProposalVoteBar(gtx C) D {
	proposal := pg.proposal

	yes := float32(proposal.YesVotes)
	no := float32(proposal.NoVotes)
	quorumPercent := float32(proposal.QuorumPercentage)
	passPercentage := float32(proposal.PassPercentage)
	eligibleTickets := float32(proposal.EligibleTickets)

	return pg.voteBar.
		SetYesNoVoteParams(yes, no).
		SetVoteValidityParams(eligibleTickets, quorumPercent, passPercentage).
		SetProposalDetails(proposal.NumComments, proposal.PublishedAt, proposal.Token).
		SetBottomLayout(pg.sumaryInfo).
		SetDisableInfoTitle(true).
		Layout(gtx)
}

func (pg *ProposalDetails) sumaryInfo(gtx C) D {
	totalVotes := fmt.Sprintf("%d", pg.proposal.YesVotes+pg.proposal.NoVotes)
	quorum := fmt.Sprintf("%d", (pg.proposal.QuorumPercentage/100)*pg.proposal.EligibleTickets)
	discussion := fmt.Sprintf("%d", pg.proposal.NumComments)
	published := libutils.FormatUTCTime(pg.proposal.PublishedAt)
	token := pg.proposal.Token
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					lbl := pg.Theme.Body1(values.String(values.StrTotalVotesTit))
					lbl.Font.Weight = font.SemiBold
					return lbl.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						lbl := pg.Theme.Body2(totalVotes)
						return lbl.Layout(gtx)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					lbl := pg.Theme.Body1(values.String(values.StrQuorumRequite))
					lbl.Font.Weight = font.SemiBold
					return lbl.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						lbl := pg.Theme.Body2(quorum)
						return lbl.Layout(gtx)
					})
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.summaryRow(values.String(values.StrDiscussionsTit), discussion, gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.summaryRow(values.String(values.StrPublished2), published, gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.summaryRow(values.String(values.StrTokenTit), token, gtx)
		}),
	)
}

func (pg *ProposalDetails) summaryRow(title, content string, gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := pg.Theme.Body1(title)
			lbl.Font.Weight = font.SemiBold
			return lbl.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				lbl := pg.Theme.Body2(content)
				return lbl.Layout(gtx)
			})
		}),
	)
}

func (pg *ProposalDetails) layoutProposalVoteAction(gtx C) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return pg.vote.Layout(gtx)
}

func (pg *ProposalDetails) layoutInDiscussionState(gtx C) D {
	stateText1 := values.String(values.StrAuthorToAuthorizeVoting)
	stateText2 := values.String(values.StrAdminToTriggerVoting)

	proposal := pg.proposal

	c := func(gtx layout.Context, val int32, info string) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if proposal.VoteStatus == val || proposal.VoteStatus < val {
					c := pg.Theme.Card()
					c.Color = pg.Theme.Color.Primary
					c.Radius = cryptomaterial.Radius(9)
					lbl := pg.Theme.Body1(fmt.Sprint(val))
					lbl.Color = pg.Theme.Color.Surface
					if proposal.VoteStatus < val {
						c.Color = pg.Theme.Color.Gray4
						lbl.Color = pg.Theme.Color.GrayText3
					}
					return c.Layout(gtx, func(gtx C) D {
						m := values.MarginPadding6
						return layout.Inset{Left: m, Right: m}.Layout(gtx, lbl.Layout)
					})
				}
				icon := cryptomaterial.NewIcon(pg.successIcon)
				icon.Color = pg.Theme.Color.Primary
				return layout.Inset{
					Left:   values.MarginPaddingMinus2,
					Right:  values.MarginPaddingMinus2,
					Bottom: values.MarginPaddingMinus2,
				}.Layout(gtx, func(gtx C) D {
					return icon.Layout(gtx, values.MarginPadding24)
				})
			}),
			layout.Rigid(func(gtx C) D {
				col := pg.Theme.Color.Primary
				txt := info + "..."
				if proposal.VoteStatus != val {
					txt = info
					col = pg.Theme.Color.GrayText3
					if proposal.VoteStatus > 1 {
						col = pg.Theme.Color.Text
					}
				}
				lbl := pg.Theme.Body1(txt)
				lbl.Color = col
				return layout.Inset{Left: values.MarginPadding16}.Layout(gtx, lbl.Layout)
			}),
		)
	}

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return c(gtx, 1, stateText1)
		}),
		layout.Rigid(func(gtx C) D {
			height, width := gtx.Dp(values.MarginPadding26), gtx.Dp(values.MarginPadding4)
			line := pg.Theme.Line(height, width)
			if proposal.VoteStatus > 1 {
				line.Color = pg.Theme.Color.Primary
			} else {
				line.Color = pg.Theme.Color.Gray2
			}
			return layout.Inset{Left: values.MarginPadding8}.Layout(gtx, line.Layout)
		}),
		layout.Rigid(func(gtx C) D {
			return c(gtx, 2, stateText2)
		}),
	)
}

func (pg *ProposalDetails) getCategoryText() string {
	proposal := pg.proposal
	categoryTxt := ""
	switch proposal.Category {
	case libwallet.ProposalCategoryApproved:
		categoryTxt = values.String(values.StrApproved)
	case libwallet.ProposalCategoryRejected:
		categoryTxt = values.String(values.StrRejected)
	case libwallet.ProposalCategoryAbandoned:
		categoryTxt = values.String(values.StrAbandoned)
	case libwallet.ProposalCategoryActive:
		categoryTxt = values.String(values.StrVotingInProgress)
	}
	return categoryTxt
}

func (pg *ProposalDetails) layoutNormalTitle(gtx C) D {
	proposal := pg.proposal

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding200)
			return pg.sourceWalletSelector.Layout(pg.ParentWindow(), gtx)
		}),
		layout.Rigid(pg.lineSeparator(layout.Inset{Top: values.MarginPadding10, Bottom: values.MarginPadding10})),
		layout.Rigid(pg.layoutProposalVoteBar),
		layout.Rigid(func(gtx C) D {
			if proposal.Category != libwallet.ProposalCategoryActive {
				return D{}
			}

			if pg.WL.SelectedWallet.Wallet.IsWatchingOnlyWallet() {
				warning := pg.Theme.Label(values.TextSize16, values.String(values.StrWarningVote))
				warning.Color = pg.Theme.Color.Danger
				return warning.Layout(gtx)
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pg.lineSeparator(layout.Inset{Top: values.MarginPadding10, Bottom: values.MarginPadding10})),
				layout.Rigid(pg.layoutProposalVoteAction),
			)
		}),
	)
}

func (pg *ProposalDetails) layoutTitle(gtx C) D {
	proposal := pg.proposal

	return pg.descriptionCard.Layout(gtx, func(gtx C) D {
		if proposal.Category == libwallet.ProposalCategoryPre {
			return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
				return pg.layoutInDiscussionState(gtx)
			})
		}
		return pg.layoutNormalTitle(gtx)
	})
}

func (pg *ProposalDetails) layoutDescription(gtx C) D {
	grayCol := pg.Theme.Color.GrayText2
	proposal := pg.proposal

	dotLabel := pg.Theme.H4(" . ")
	dotLabel.Color = grayCol

	userLabel := pg.Theme.Body2(proposal.Username)
	userLabel.Color = grayCol

	versionLabel := pg.Theme.Body2(values.String(values.StrVersion) + " " + proposal.Version)
	versionLabel.Color = grayCol

	publishedLabel := pg.Theme.Body2(values.String(values.StrPublished2) + " " + components.TimeAgo(proposal.PublishedAt))
	publishedLabel.Color = grayCol

	updatedLabel := pg.Theme.Body2(values.String(values.StrUpdated) + " " + components.TimeAgo(proposal.Timestamp))
	updatedLabel.Color = grayCol

	w := []layout.Widget{
		func(gtx C) D {
			lbl := pg.Theme.H5(proposal.Name)
			lbl.Font.Weight = font.SemiBold
			return lbl.Layout(gtx)
		},
		func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(userLabel.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPaddingMinus22}.Layout(gtx, dotLabel.Layout)
				}),
				layout.Rigid(publishedLabel.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPaddingMinus22}.Layout(gtx, dotLabel.Layout)
				}),
				layout.Rigid(versionLabel.Layout),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: values.MarginPaddingMinus22}.Layout(gtx, dotLabel.Layout)
				}),
				layout.Rigid(updatedLabel.Layout),
			)
		},
		pg.layoutRedirect(values.String(values.StrViewOnPoliteia), pg.redirectIcon, pg.viewInPoliteiaBtn),
		pg.lineSeparator(layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16}),
	}

	_, ok := pg.proposalItems[proposal.Token]
	if ok {
		w = append(w, pg.proposalItems[proposal.Token].widgets...)
	} else {
		loading := func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, layout.Flexed(1, func(gtx C) D {
				return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx C) D {
					return layout.Center.Layout(gtx, material.Loader(pg.Theme.Base).Layout)
				})
			}))
		}

		w = append(w, loading)
	}

	return pg.descriptionCard.Layout(gtx, func(gtx C) D {
		return pg.Theme.List(pg.scrollbarList).Layout(gtx, 1, func(gtx C, i int) D {
			return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
				return pg.descriptionList.Layout(gtx, len(w), func(gtx C, i int) D {
					return layout.UniformInset(unit.Dp(0)).Layout(gtx, w[i])
				})
			})
		})
	})
}

func (pg *ProposalDetails) layoutRedirect(text string, icon *cryptomaterial.Image, btn *cryptomaterial.Clickable) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
			return btn.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
							return layout.E.Layout(gtx, icon.Layout24dp)
						})
					}),
					layout.Rigid(pg.Theme.Body1(text).Layout),
				)
			})
		})
	}
}

func (pg *ProposalDetails) lineSeparator(inset layout.Inset) layout.Widget {
	return func(gtx C) D {
		return inset.Layout(gtx, pg.Theme.Separator().Layout)
	}
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ProposalDetails) Layout(gtx C) D {
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *ProposalDetails) layoutDesktop(gtx layout.Context) layout.Dimensions {
	proposal := pg.proposal
	_, ok := pg.proposalItems[proposal.Token]
	if !ok && !pg.loadingDescription {
		pg.loadingDescription = true
		go func() {
			var proposalDescription string
			if proposal.IndexFile != "" && proposal.IndexFileVersion == proposal.Version {
				proposalDescription = proposal.IndexFile
			} else {
				var err error
				proposalDescription, err = pg.WL.AssetsManager.Politeia.FetchProposalDescription(proposal.Token)
				if err != nil {
					log.Errorf("Error loading proposal description: %v", err)
					time.Sleep(7 * time.Second)
					pg.loadingDescription = false
					return
				}
			}

			r := renderers.RenderMarkdown(gtx, pg.Theme, proposalDescription)
			proposalWidgets, proposalClickables := r.Layout()
			pg.proposalItems[proposal.Token] = proposalItemWidgets{
				widgets:    proposalWidgets,
				clickables: proposalClickables,
			}
			pg.loadingDescription = false
		}()
	}

	page := components.SubPage{
		Load:       pg.Load,
		Title:      components.TruncateString(proposal.Name, 40),
		BackButton: pg.backButton,
		Back: func() {
			pg.ParentNavigator().CloseCurrentPage()
		},
		Body: func(gtx C) D {
			return pg.layoutDescription(gtx)
		},
		ExtraHeader: func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, pg.layoutTitle)
		},
		ExtraItem: pg.tempRightHead,
		Extra: func(gtx C) D {
			grayCol := pg.Load.Theme.Color.GrayText2
			timeAgoLabel := pg.Load.Theme.Body2(components.TimeAgo(proposal.Timestamp))
			timeAgoLabel.Color = grayCol

			dotLabel := pg.Load.Theme.H4(" . ")
			dotLabel.Color = grayCol

			categoryLabel := pg.Load.Theme.Body2(pg.getCategoryText())
			return layout.Inset{}.Layout(gtx, func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(categoryLabel.Layout),
						layout.Rigid(func(gtx C) D {
							return layout.Inset{Top: values.MarginPaddingMinus22}.Layout(gtx, dotLabel.Layout)
						}),
						layout.Rigid(timeAgoLabel.Layout),
					)
				})
			})
		},
	}
	return page.LayoutWithHeadCard(pg.ParentWindow(), gtx)
}

func (pg *ProposalDetails) layoutMobile(gtx layout.Context) layout.Dimensions {
	proposal := pg.proposal
	_, ok := pg.proposalItems[proposal.Token]
	if !ok && !pg.loadingDescription {
		pg.loadingDescription = true
		go func() {
			var proposalDescription string
			if proposal.IndexFile != "" && proposal.IndexFileVersion == proposal.Version {
				proposalDescription = proposal.IndexFile
			} else {
				var err error
				proposalDescription, err = pg.WL.AssetsManager.Politeia.FetchProposalDescription(proposal.Token)
				if err != nil {
					log.Errorf("Error loading proposal description: %v", err)
					time.Sleep(7 * time.Second)
					pg.loadingDescription = false
					return
				}
			}

			r := renderers.RenderMarkdown(gtx, pg.Theme, proposalDescription)
			proposalWidgets, proposalClickables := r.Layout()
			pg.proposalItems[proposal.Token] = proposalItemWidgets{
				widgets:    proposalWidgets,
				clickables: proposalClickables,
			}
			pg.loadingDescription = false
		}()
	}

	body := func(gtx C) D {
		page := components.SubPage{
			Load:       pg.Load,
			Title:      components.TruncateString(proposal.Name, 30),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, pg.layoutTitle)
					}),
					layout.Rigid(pg.layoutDescription),
				)
			},
			ExtraItem: pg.viewInPoliteiaBtn,
			Extra: func(gtx C) D {
				return layout.Inset{}.Layout(gtx, pg.redirectIcon.Layout24dp)
			},
		}
		return page.Layout(pg.ParentWindow(), gtx)
	}
	return components.UniformMobile(gtx, false, false, body)
}
