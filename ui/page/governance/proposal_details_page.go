package governance

import (
	"fmt"
	"io"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
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

	proposal       *libwallet.Proposal
	proposalDesRaw string

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

	assetWallets []sharedW.Asset

	walletDropDown    *cryptomaterial.DropDown
	selectedDCRWallet sharedW.Asset

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
		rejectedIcon:      l.Theme.Icons.NavigationCancel,
		successIcon:       l.Theme.Icons.ActionCheckCircle,
		viewInPoliteiaBtn: l.Theme.NewClickable(true),
		copyRedirectURL:   l.Theme.NewClickable(false),
		tempRightHead:     l.Theme.NewClickable(false),
		voteBar:           components.NewVoteBar(l),
	}

	pg.backButton = components.GetBackButton(l)

	pg.vote = l.Theme.Button(values.String(values.StrVote))
	pg.vote.TextSize = l.ConvertTextSize(values.TextSize14)
	pg.vote.Background = l.Theme.Color.Primary
	pg.vote.Color = l.Theme.Color.Surface
	pg.vote.Inset = layout.Inset{
		Top:    values.MarginPadding8,
		Bottom: values.MarginPadding8,
		Left:   values.MarginPadding12,
		Right:  values.MarginPadding12,
	}

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ProposalDetails) OnNavigatedTo() {
	pg.initWalletSelector()
	pg.loadProposalDescription()
	pg.listenForSyncNotifications() // listener is stopped in OnNavigatedFrom()
}

func (pg *ProposalDetails) initWalletSelector() {
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
	if len(pg.assetWallets) > 0 {
		pg.selectedDCRWallet = pg.assetWallets[0].(*dcr.Asset)
	}
	pg.walletDropDown.Width = values.MarginPadding150
	settingCommonDropdown(pg.Theme, pg.walletDropDown)
}

func (pg *ProposalDetails) loadProposalDescription() {
	if pg.proposalDesRaw == "" && !pg.loadingDescription {
		pg.loadingDescription = true
		go func() {
			var proposalDescription string
			if pg.proposal.IndexFile != "" && pg.proposal.IndexFileVersion == pg.proposal.Version {
				proposalDescription = pg.proposal.IndexFile
			} else {
				var err error
				proposalDescription, err = pg.AssetsManager.Politeia.FetchProposalDescription(pg.proposal.Token)
				if err != nil {
					log.Errorf("Error loading proposal description: %v", err)
					time.Sleep(7 * time.Second)
					pg.loadingDescription = false
					return
				}
			}

			pg.proposalDesRaw = proposalDescription
			pg.loadingDescription = false
		}()
	}
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ProposalDetails) Layout(gtx C) D {
	proposal := pg.proposal
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
		ExtraHeader: func(gtx C) D {
			marginTop := values.MarginPadding16
			if pg.IsMobileView() {
				marginTop = values.MarginPaddingMinus8
			}
			return layout.Inset{Bottom: values.MarginPadding16, Top: marginTop}.Layout(gtx, pg.layoutTitle)
		},
		ExtraItem: pg.tempRightHead,
		Extra: func(gtx C) D {
			if pg.IsMobileView() {
				return D{}
			}
			return pg.statusAndTimeLayout(gtx)
		},
	}
	return page.LayoutWithHeadCard(pg.ParentWindow(), gtx)
}

func (pg *ProposalDetails) getProposalItemWidgets() *proposalItemWidgets {
	if pg.proposalDesRaw == "" {
		return nil
	}
	r := renderers.RenderMarkdown(pg.Load, pg.Theme, pg.proposalDesRaw)
	proposalWidgets, proposalClickables := r.Layout()
	return &proposalItemWidgets{
		widgets:    proposalWidgets,
		clickables: proposalClickables,
	}
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ProposalDetails) HandleUserInteractions(gtx C) {
	if pg.walletDropDown != nil && pg.walletDropDown.Changed(gtx) {
		pg.selectedDCRWallet = pg.assetWallets[pg.walletDropDown.SelectedIndex()]
		//TODO: implement when selected wallet
	}

	if pg.vote.Clicked(gtx) {
		if len(pg.assetWallets) == 0 {
			pg.displayCreateWalletModal(libutils.DCRWalletAsset)
			return
		}
		pg.ParentWindow().ShowModal(newVoteModal(pg.Load, pg.proposal))
	}

	if pg.viewInPoliteiaBtn.Clicked(gtx) {
		host := "https://proposals.decred.org/record/" + pg.proposal.Token
		if pg.AssetsManager.NetType() == libwallet.Testnet {
			host = "http://45.32.108.164:3000/record/" + pg.proposal.Token
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
													if pg.copyRedirectURL.Clicked(gtx) {
														gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(host))})
														pg.Toast.Notify(values.String(values.StrCopied))
													}
													return pg.copyRedirectURL.Layout(gtx, pg.Theme.NewIcon(pg.Theme.Icons.CopyIcon).Layout24dp)
												})
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
							label.TextSize = pg.ConvertTextSize(values.TextSize14)
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
	proposalSyncCallback := func(_ string, status libutils.ProposalStatus) {
		if status == libutils.ProposalStatusSynced {
			proposal, err := pg.AssetsManager.Politeia.GetProposalRaw(pg.proposal.Token)
			if err == nil {
				pg.proposal = &libwallet.Proposal{Proposal: *proposal}
				pg.ParentWindow().Reload()
			}
		}
	}
	err := pg.AssetsManager.Politeia.AddSyncCallback(proposalSyncCallback, ProposalDetailsPageID)
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
	pg.AssetsManager.Politeia.RemoveSyncCallback(ProposalDetailsPageID)
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

func (pg *ProposalDetails) statusAndTimeLayout(gtx C) D {
	grayCol := pg.Load.Theme.Color.GrayText2
	timeAgoLabel := pg.Load.Theme.Body2(components.TimeAgo(pg.proposal.Timestamp))
	timeAgoLabel.Color = grayCol

	dotLabel := pg.Load.Theme.H4(" . ")
	dotLabel.Color = grayCol

	categoryLabel := pg.Load.Theme.Body2(pg.getCategoryText())
	categoryLabel.TextSize = pg.ConvertTextSize(values.TextSize14)
	timeAgoLabel.TextSize = pg.ConvertTextSize(values.TextSize14)
	spacing := layout.SpaceStart
	if pg.IsMobileView() {
		spacing = layout.SpaceBetween
	}
	return layout.Flex{Spacing: spacing}.Layout(gtx,
		layout.Rigid(categoryLabel.Layout),
		layout.Rigid(func(gtx C) D {
			if pg.IsMobileView() {
				return D{}
			}
			return layout.Inset{Top: values.MarginPaddingMinus22}.Layout(gtx, dotLabel.Layout)
		}),
		layout.Rigid(timeAgoLabel.Layout),
	)
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
						lbl.TextSize = pg.ConvertTextSize(values.TextSize14)
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
						lbl.TextSize = pg.ConvertTextSize(values.TextSize14)
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
				lbl.TextSize = pg.ConvertTextSize(values.TextSize14)
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

	c := func(gtx C, val int32, info string) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if proposal.VoteStatus == val || proposal.VoteStatus < val {
					c := pg.Theme.Card()
					c.Color = pg.Theme.Color.Primary
					lbl := pg.Theme.Body1(fmt.Sprint(val))
					lbl.TextSize = pg.ConvertTextSize(values.TextSize18)
					lbl.Color = pg.Theme.Color.Surface
					c.Radius = cryptomaterial.Radius(int(lbl.TextSize)/2 + 1)
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
				lbl.TextSize = pg.ConvertTextSize(values.TextSize18)
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
			if !pg.IsMobileView() {
				return D{}
			}
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Bottom: values.MarginPadding4}.Layout(gtx, pg.statusAndTimeLayout)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx C) D {
					marginTop := values.MarginPadding50
					if pg.IsMobileView() {
						marginTop = values.MarginPadding30
					}
					if len(pg.assetWallets) == 0 {
						marginTop = values.MarginPaddingMinus16
					}
					return layout.Inset{Top: marginTop}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								if len(pg.assetWallets) == 0 {
									return D{}
								}

								return pg.lineSeparator(layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16})(gtx)
							}),
							layout.Rigid(pg.layoutProposalVoteBar),
							layout.Rigid(func(gtx C) D {
								if proposal.Category != libwallet.ProposalCategoryActive {
									return D{}
								}
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(pg.lineSeparator(layout.Inset{Top: values.MarginPadding10, Bottom: values.MarginPadding10})),
									layout.Rigid(pg.layoutProposalVoteAction),
								)
							}),
						)
					})
				}),
				layout.Expanded(func(gtx C) D {
					if len(pg.assetWallets) == 0 {
						return D{}
					}
					return pg.walletDropDown.Layout(gtx)
				}),
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

	userLabel.TextSize = pg.ConvertTextSize(values.TextSize14)
	versionLabel.TextSize = pg.ConvertTextSize(values.TextSize14)
	publishedLabel.TextSize = pg.ConvertTextSize(values.TextSize14)
	updatedLabel.TextSize = pg.ConvertTextSize(values.TextSize14)

	w := []layout.Widget{
		func(gtx C) D {
			lbl := pg.Theme.H5(proposal.Name)
			lbl.TextSize = pg.ConvertTextSize(values.TextSize20)
			lbl.Font.Weight = font.SemiBold
			return lbl.Layout(gtx)
		},
		func(gtx C) D {
			axis := layout.Horizontal
			if pg.IsMobileView() {
				axis = layout.Vertical
			}
			return layout.Flex{Axis: axis}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
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
					)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							if pg.IsMobileView() {
								return D{}
							}
							return layout.Inset{Top: values.MarginPaddingMinus22}.Layout(gtx, dotLabel.Layout)
						}),
						layout.Rigid(updatedLabel.Layout),
					)
				}),
			)
		},
		pg.layoutRedirect(values.String(values.StrViewOnPoliteia), pg.redirectIcon, pg.viewInPoliteiaBtn),
		pg.lineSeparator(layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16}),
	}

	itemWidgets := pg.getProposalItemWidgets()
	if itemWidgets != nil {
		w = append(w, itemWidgets.widgets...)
	} else {
		loading := func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, layout.Flexed(1, func(gtx C) D {
				return components.VerticalInset(values.MarginPadding8).Layout(gtx, func(gtx C) D {
					return layout.Center.Layout(gtx, material.Loader(pg.Theme.Base).Layout)
				})
			}))
		}

		w = append(w, loading)
	}

	return pg.descriptionCard.Layout(gtx, func(gtx C) D {
		return pg.Theme.List(pg.scrollbarList).Layout(gtx, 1, func(gtx C, _ int) D {
			mpSize := values.MarginPadding16
			if pg.IsMobileView() {
				mpSize = values.MarginPadding12
			}
			return layout.UniformInset(mpSize).Layout(gtx, func(gtx C) D {
				return pg.descriptionList.Layout(gtx, len(w), func(gtx C, i int) D {
					return w[i](gtx)
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
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
							return icon.LayoutSize(gtx, pg.ConvertIconSize(values.MarginPadding24))
						})
					}),
					layout.Rigid(func(gtx C) D {
						lb := pg.Theme.Body1(text)
						lb.TextSize = pg.ConvertTextSize(values.TextSize14)
						return lb.Layout(gtx)
					}),
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

func (pg *ProposalDetails) displayCreateWalletModal(asset libutils.AssetType) {
	createWalletModal := modal.NewCustomModal(pg.Load).
		Title(values.String(values.StrCreateWallet)).
		UseCustomWidget(func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding10, Bottom: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return layout.Center.Layout(gtx, pg.Theme.Body2(values.StringF(values.StrCreateAssetWalletToVoteMsg, asset.ToFull())).Layout)
			})
		}).
		SetCancelable(true).
		SetContentAlignment(layout.Center, layout.W, layout.Center).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			pg.ParentNavigator().Display(components.NewCreateWallet(pg.Load, func() {
				pg.walletCreationSuccessFunc()
			}, asset))
			return true
		}).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetPositiveButtonText(values.String(values.StrContinue))
	pg.ParentWindow().ShowModal(createWalletModal)
}

func (pg *ProposalDetails) walletCreationSuccessFunc() {
	pg.ParentNavigator().ClosePagesAfter(ProposalDetailsPageID)
	pg.OnNavigatedTo()
	pg.ParentWindow().Reload()
}
