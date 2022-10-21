package transaction

import (
	"fmt"
	"image"
	"strings"
	"time"

	"gioui.org/op"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/utils"
	"gitlab.com/raedah/cryptopower/ui/values"
)

const (
	TransactionDetailsPageID = "TransactionDetails"
	viewBlockID              = "viewBlock"
	copyBlockID              = "copyBlock"
)

type transactionWdg struct {
	confirmationIcons    *cryptomaterial.Image
	time, status, wallet cryptomaterial.Label

	copyTextButtons []*cryptomaterial.Clickable
	txStatus        *components.TxStatus
}

type moreItem struct {
	text   string
	id     string
	button *cryptomaterial.Clickable
}

type TxDetailsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	list                        *widget.List
	transactionInputsContainer  layout.List
	transactionOutputsContainer layout.List

	destAddressClickable      *cryptomaterial.Clickable
	associatedTicketClickable *cryptomaterial.Clickable
	hashClickable             *cryptomaterial.Clickable
	rebroadcastClickable      *cryptomaterial.Clickable
	moreOption                *cryptomaterial.Clickable
	outputsCollapsible        *cryptomaterial.Collapsible
	inputsCollapsible         *cryptomaterial.Collapsible
	dot                       *cryptomaterial.Icon
	rebroadcastIcon           *cryptomaterial.Image
	shadowBox                 *cryptomaterial.Shadow

	backButton  cryptomaterial.IconButton
	rebroadcast cryptomaterial.Label

	transaction   *sharedW.Transaction
	ticketSpender *sharedW.Transaction // vote or revoke ticket
	ticketSpent   *sharedW.Transaction // ticket spent in a vote or revoke
	txBackStack   *sharedW.Transaction // track original transaction
	wallet        sharedW.Asset
	dcrImpl       *dcr.DCRAsset

	moreItems  []moreItem
	txnWidgets transactionWdg

	txSourceAccount      string
	txDestinationAddress string
	title                string
	vspHost              string

	moreOptionIsOpen bool
}

func NewTransactionDetailsPage(l *load.Load, transaction *sharedW.Transaction, isTicket bool) *TxDetailsPage {
	impl := l.WL.SelectedWallet.Wallet.(*dcr.DCRAsset)
	if impl == nil {
		log.Error("Only DCR implementation is supported")
		return nil
	}

	rebroadcast := l.Theme.Label(values.TextSize14, values.String(values.StrRebroadcast))
	rebroadcast.TextSize = values.TextSize14
	rebroadcast.Color = l.Theme.Color.Text
	pg := &TxDetailsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(TransactionDetailsPageID),
		list: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		transactionInputsContainer: layout.List{
			Axis: layout.Vertical,
		},
		transactionOutputsContainer: layout.List{
			Axis: layout.Vertical,
		},

		outputsCollapsible: l.Theme.Collapsible(),
		inputsCollapsible:  l.Theme.Collapsible(),

		associatedTicketClickable: l.Theme.NewClickable(true),
		hashClickable:             l.Theme.NewClickable(true),
		destAddressClickable:      l.Theme.NewClickable(true),
		moreOption:                l.Theme.NewClickable(false),
		shadowBox:                 l.Theme.Shadow(),

		dcrImpl: impl,

		transaction:          transaction,
		wallet:               l.WL.SelectedWallet.Wallet,
		rebroadcast:          rebroadcast,
		rebroadcastClickable: l.Theme.NewClickable(true),
		rebroadcastIcon:      l.Theme.Icons.Rebroadcast,
	}

	pg.backButton, _ = components.SubpageHeaderButtons(pg.Load)

	pg.dot = cryptomaterial.NewIcon(l.Theme.Icons.ImageBrightness1)
	pg.dot.Color = l.Theme.Color.Gray1

	pg.moreItems = pg.getMoreItem()

	return pg
}

func (pg *TxDetailsPage) getTXSourceAccountAndDirection() {
	// find source account
	for _, input := range pg.transaction.Inputs {
		fmt.Println(input.AccountNumber, "input.AccountNumber")
		if input.AccountNumber != -1 {
			accountName, err := pg.wallet.AccountName(input.AccountNumber)
			if err != nil {
				log.Error(err)
			} else {
				pg.txSourceAccount = accountName
			}
			break
		}
	}

	// find destination address
	for _, output := range pg.transaction.Outputs {
		switch pg.transaction.Direction {
		case dcr.TxDirectionSent:
			// mixed account number
			if pg.transaction.Type == dcr.TxTypeMixed &&
				output.AccountNumber == pg.dcrImpl.UnmixedAccountNumber() {
				accountName, err := pg.wallet.AccountName(output.AccountNumber)
				if err != nil {
					log.Error(err)
				} else {
					pg.txDestinationAddress = accountName
				}
				break
			}
			if output.AccountNumber == -1 {
				pg.txDestinationAddress = output.Address
				break
			}
		case dcr.TxDirectionReceived:
			if output.AccountNumber != -1 {
				accountName, err := pg.wallet.AccountName(output.AccountNumber)
				if err != nil {
					log.Error(err)
				} else {
					pg.txDestinationAddress = accountName
				}
				break
			}
		}
	}
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *TxDetailsPage) OnNavigatedTo() {
	// this tx is a vote transaction
	if pg.transaction.TicketSpentHash != "" {
		pg.ticketSpent, _ = pg.wallet.GetTransactionRaw(pg.transaction.TicketSpentHash)
	}

	if ok, _ := pg.dcrImpl.TicketHasVotedOrRevoked(pg.transaction.Hash); ok {
		pg.ticketSpender, _ = pg.dcrImpl.TicketSpender(pg.transaction.Hash)
	}

	if pg.wallet.TxMatchesFilter(pg.transaction, dcr.TxFilterStaking) {
		go func() {
			info, err := pg.dcrImpl.VSPTicketInfo(pg.transaction.Hash)
			if err != nil {
				log.Errorf("VSPTicketInfo error: %v\n", err)
			}

			pg.vspHost = values.String(values.StrNotAvailable)
			if info != nil {
				pg.vspHost = info.VSP
			}
		}()
	}

	pg.title = values.String(values.StrTransactionDetails)
	if pg.transaction.Type == values.String(values.StrTicket) {
		pg.title = values.String(values.StrTicketDetails)
	}

	pg.getTXSourceAccountAndDirection()
	pg.txnWidgets = initTxnWidgets(pg.Load, pg.transaction)
}

func (pg *TxDetailsPage) getMoreItem() []moreItem {
	return []moreItem{
		{
			text:   values.String(values.StrViewOnExplorer),
			button: pg.Theme.NewClickable(true),
			id:     viewBlockID,
		},
		{
			text:   values.String(values.StrCopyBlockLink),
			button: pg.Theme.NewClickable(true),
			id:     copyBlockID,
		},
	}
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *TxDetailsPage) Layout(gtx C) D {
	body := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      pg.title,
			BackButton: pg.backButton,
			ExtraItem:  pg.moreOption,
			Extra: func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(pg.Theme.Icons.EllipseHoriz.Layout24dp),
						layout.Rigid(func(gtx C) D {
							if pg.moreOptionIsOpen {
								pg.layoutOptionsMenu(gtx)
							}
							return D{}
						}),
					)
				})
			},
			Back: func() {
				if pg.txBackStack == nil {
					pg.ParentNavigator().CloseCurrentPage()
					return
				}
				pg.transaction = pg.txBackStack
				pg.getTXSourceAccountAndDirection()
				pg.txnWidgets = initTxnWidgets(pg.Load, pg.transaction)
				pg.txBackStack = nil
				pg.ParentWindow().Reload()
			},
			Body: func(gtx C) D {
				widgets := []func(gtx C) D{
					// pg.associatedTicket, // TODO currently not part of the v2 update
					pg.txnTypeAndID,
					pg.txnInputs,
					pg.txnOutputs,
				}

				return pg.Theme.Card().Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(pg.txDetailsHeader),
						layout.Rigid(pg.Theme.Separator().Layout),
						layout.Rigid(func(gtx C) D {
							return pg.Theme.List(pg.list).Layout(gtx, len(widgets), func(gtx C, i int) D {
								return layout.Inset{}.Layout(gtx, widgets[i])
							})
						}),
					)
				})
			},
		}

		return sp.CombinedLayout(pg.ParentWindow(), gtx)
	}

	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx, body)
	}
	return pg.layoutDesktop(gtx, body)
}

func (pg *TxDetailsPage) layoutDesktop(gtx C, body layout.Widget) D {
	return components.UniformPadding(gtx, body)
}

func (pg *TxDetailsPage) layoutMobile(gtx C, body layout.Widget) D {
	return components.UniformMobile(gtx, false, false, body)
}

func (pg *TxDetailsPage) txDetailsHeader(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Padding: layout.Inset{
			Left:   values.MarginPadding24,
			Right:  values.MarginPadding24,
			Bottom: values.MarginPadding30,
		},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding22,
			}.Layout(gtx, pg.txnWidgets.txStatus.Icon.Layout24dp)
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							if pg.transaction.Type == dcr.TxTypeTicketPurchase {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrStatus)+": ").Layout),
									layout.Rigid(pg.Theme.Label(values.TextSize16, pg.txnWidgets.txStatus.Title).Layout),
									layout.Rigid(func(gtx C) D {
										// immature tx section
										if pg.txnWidgets.txStatus.TicketStatus == dcr.TicketStatusImmature {
											p := pg.Theme.ProgressBarCirle(pg.getTimeToMatureOrExpire())
											p.Color = pg.txnWidgets.txStatus.ProgressBarColor
											return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
												sz := gtx.Dp(values.MarginPadding22)
												gtx.Constraints.Max = image.Point{X: sz, Y: sz}
												gtx.Constraints.Min = gtx.Constraints.Max
												return p.Layout(gtx)
											})
										}
										return D{}
									}),
								)
							} else {
								// regular transaction
								col := pg.Theme.Color.GrayText2
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										title := dcrutil.Amount(pg.transaction.Amount).String()
										switch pg.transaction.Type {
										case dcr.TxTypeMixed:
											title = dcrutil.Amount(pg.transaction.MixDenomination).String()
										case dcr.TxTypeRegular:
											if pg.transaction.Direction == dcr.TxDirectionSent && !strings.Contains(title, "-") {
												title = "-" + title
											}
										case dcr.TxTypeRevocation, dcr.TxTypeVote:
											return pg.Theme.Label(values.TextSize20, pg.txnWidgets.txStatus.Title).Layout(gtx)
										}
										return components.LayoutBalanceWithUnit(gtx, pg.Load, title)
									}),
									layout.Rigid(func(gtx C) D {
										date := time.Unix(pg.transaction.Timestamp, 0).Format("Jan 2, 2006")
										timeSplit := time.Unix(pg.transaction.Timestamp, 0).Format("03:04:05 PM")
										dateTime := fmt.Sprintf("%v at %v", date, timeSplit)

										lbl := pg.Theme.Label(values.TextSize16, dateTime)
										lbl.Color = col
										return layout.Inset{
											Top:    values.MarginPadding7,
											Bottom: values.MarginPadding7,
										}.Layout(gtx, lbl.Layout)
									}),
									layout.Rigid(func(gtx C) D {
										// immature tx section
										if pg.transaction.Type == dcr.TxTypeVote || pg.transaction.Type == dcr.TxTypeRevocation {
											title := values.String(values.StrRevoke)
											if pg.transaction.Type == dcr.TxTypeVote {
												title = values.String(values.StrVote)
											}

											lbl := pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d days to %s", pg.transaction.DaysToVoteOrRevoke, title))
											lbl.Color = col
											return lbl.Layout(gtx)
										}

										return D{}
									}),
								)
							}
						}),
						layout.Rigid(func(gtx C) D {
							col := pg.Theme.Color.GrayText2

							switch pg.txnWidgets.txStatus.TicketStatus {
							case dcr.TicketStatusImmature:
								maturity := pg.dcrImpl.TicketMaturity()
								blockTime := pg.wallet.TargetTimePerBlockMinutes()
								maturityDuration := time.Duration(maturity*int32(blockTime)) * time.Minute

								lbl := pg.Theme.Label(values.TextSize16, values.StringF(values.StrImmatureInfo,
									pg.getTimeToMatureOrExpire(), maturity, maturityDuration.String()))
								lbl.Color = col
								return lbl.Layout(gtx)

							case dcr.TicketStatusLive:
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										lbl := pg.Theme.Label(values.TextSize16, values.String(values.StrLifeSpan)+": ")
										lbl.Color = col
										return lbl.Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										expiry := pg.dcrImpl.TicketExpiry()
										lbl := pg.Theme.Label(values.TextSize16, values.StringF(values.StrLiveInfoDisc,
											expiry, pg.getTimeToMatureOrExpire(), expiry))
										lbl.Color = col
										return lbl.Layout(gtx)
									}),
								)

							case dcr.TicketStatusVotedOrRevoked:
								if pg.ticketSpender != nil { // voted or revoked
									if pg.ticketSpender.Type == dcr.TxTypeVote {
										return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
											layout.Rigid(func(gtx C) D {
												lbl := pg.Theme.Label(values.TextSize16, values.String(values.StrReward)+": ")
												lbl.Color = col
												return lbl.Layout(gtx)
											}),
											layout.Rigid(func(gtx C) D {
												lbl := pg.Theme.Label(values.TextSize16, dcrutil.Amount(pg.ticketSpender.VoteReward).String())
												lbl.Color = col
												return lbl.Layout(gtx)
											}),
										)
									}
								}
								return D{}
							default:
								return D{}
							}
						}),
						layout.Rigid(func(gtx C) D {
							if pg.transaction.BlockHeight == -1 {
								if !pg.rebroadcastClickable.Enabled() {
									gtx = pg.rebroadcastClickable.SetEnabled(false, &gtx)
								}
								return cryptomaterial.LinearLayout{
									Width:     cryptomaterial.WrapContent,
									Height:    cryptomaterial.WrapContent,
									Clickable: pg.rebroadcastClickable,
									Direction: layout.Center,
									Alignment: layout.Middle,
									Border: cryptomaterial.Border{
										Color:  pg.Theme.Color.Gray2,
										Width:  values.MarginPadding1,
										Radius: cryptomaterial.Radius(10),
									},
									Padding: layout.Inset{
										Top:    values.MarginPadding3,
										Bottom: values.MarginPadding3,
										Left:   values.MarginPadding8,
										Right:  values.MarginPadding8,
									},
									Margin: layout.Inset{Left: values.MarginPadding10},
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return layout.Inset{Right: values.MarginPadding4}.Layout(gtx, pg.rebroadcastIcon.Layout16dp)
									}),
									layout.Rigid(pg.rebroadcast.Layout),
								)
							}
							return D{}
						}),
					)
				}),
			)
		}),
	)
}

func (pg *TxDetailsPage) getTimeToMatureOrExpire() int {
	progressMax := pg.dcrImpl.TicketMaturity()
	if pg.txnWidgets.txStatus.TicketStatus == dcr.TicketStatusLive {
		progressMax = pg.dcrImpl.TicketExpiry()
	}

	confs := dcr.Confirmations(pg.wallet.GetBestBlockHeight(), *pg.transaction)
	if pg.ticketSpender != nil {
		confs = dcr.Confirmations(pg.wallet.GetBestBlockHeight(), *pg.ticketSpender)
	}

	progress := (float32(confs) / float32(progressMax)) * 100
	return int(progress)
}

func (pg *TxDetailsPage) maturityProgressBar(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Margin:      layout.Inset{Top: values.MarginPadding12},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			t := pg.Theme.Label(values.TextSize14, values.String(values.StrMaturity))
			t.Color = pg.Theme.Color.GrayText2
			return t.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {

			percentageLabel := pg.Theme.Label(values.TextSize14, "25%")
			percentageLabel.Color = pg.Theme.Color.GrayText2

			progress := pg.Theme.ProgressBar(40)
			progress.Color = pg.Theme.Color.LightBlue
			progress.TrackColor = pg.Theme.Color.BlueProgressTint
			progress.Height = values.MarginPadding8
			progress.Width = values.MarginPadding80
			progress.Radius = cryptomaterial.Radius(8)

			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{
					Alignment: layout.Middle,
				}.Layout(gtx,
					layout.Rigid(percentageLabel.Layout),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Left: values.MarginPadding6, Right: values.MarginPadding6}.Layout(gtx, progress.Layout)
					}),
					layout.Rigid(pg.Theme.Label(values.TextSize16, fmt.Sprintf("%d %s", 18, values.String(values.StrHours))).Layout),
				)
			})
		}),
	)
}

func (pg *TxDetailsPage) keyValue(gtx C, key string, value layout.Widget) D {
	return layout.Inset{Bottom: values.MarginPadding18}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(.4, func(gtx C) D {
				return layout.Inset{Right: values.MarginPadding35}.Layout(gtx, func(gtx C) D {
					lbl := pg.Theme.Label(values.TextSize14, key)
					lbl.Color = pg.Theme.Color.GrayText2
					return lbl.Layout(gtx)
				})
			}),
			layout.Flexed(.6, value),
		)
	})
}

func (pg *TxDetailsPage) associatedTicket(gtx C) D {
	if pg.transaction.Type != dcr.TxTypeVote && pg.transaction.Type != dcr.TxTypeRevocation {
		return D{}
	}

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return pg.associatedTicketClickable.Layout(gtx, func(gtx C) D {
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.MatchParent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Horizontal,
					Padding:     layout.Inset{Left: values.MarginPadding16, Top: values.MarginPadding12, Right: values.MarginPadding16, Bottom: values.MarginPadding12},
				}.Layout(gtx,
					layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrViewTicket)).Layout),
					layout.Flexed(1, func(gtx C) D {
						return layout.E.Layout(gtx, pg.Theme.Icons.Next.Layout24dp)
					}),
				)
			})
		}),
		layout.Rigid(pg.Theme.Separator().Layout),
	)
}

//TODO: do this at startup
func (pg *TxDetailsPage) txConfirmations() int32 {
	transaction := pg.transaction
	if transaction.BlockHeight != -1 {
		return (pg.WL.SelectedWallet.Wallet.GetBestBlockHeight() - transaction.BlockHeight) + 1
	}

	return 0
}

func (pg *TxDetailsPage) txnTypeAndID(gtx C) D {
	transaction := pg.transaction
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
		Padding: layout.Inset{
			Top:    values.MarginPadding30,
			Left:   values.MarginPadding70,
			Right:  values.MarginPadding24,
			Bottom: values.MarginPadding18,
		},
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			// hide section for recieved transactions
			if pg.transaction.Type == dcr.TxTypeRegular && pg.transaction.Direction == dcr.TxDirectionReceived {
				return D{}
			}

			label := values.String(values.StrFrom)
			if pg.transaction.Type == dcr.TxTypeTicketPurchase {
				label = values.String(values.StrAccount)
			}
			return pg.keyValue(gtx, label, pg.Theme.Label(values.TextSize14, pg.txSourceAccount).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			if (pg.transaction.Type == dcr.TxTypeRegular && pg.transaction.Direction != dcr.TxDirectionTransferred) || pg.transaction.Type == dcr.TxTypeMixed {
				dim := func(gtx C) D {
					lbl := pg.Theme.Label(values.TextSize14, utils.SplitSingleString(pg.txDestinationAddress, 0))

					if pg.transaction.Direction == dcr.TxDirectionReceived {
						return lbl.Layout(gtx)
					}

					lbl.Color = pg.Theme.Color.Primary
					// copy destination Address
					if pg.destAddressClickable.Clicked() {
						clipboard.WriteOp{Text: pg.txDestinationAddress}.Add(gtx.Ops)
						pg.Toast.Notify(values.String(values.StrTxHashCopied))
					}
					return pg.destAddressClickable.Layout(gtx, lbl.Layout)
				}

				return pg.keyValue(gtx, values.String(values.StrTo), dim)
			}
			return D{}
		}),
		layout.Rigid(func(gtx C) D {
			// hide this section for sent, received and mixed transaction
			if pg.transaction.Type == dcr.TxTypeRegular &&
				pg.transaction.Direction == dcr.TxDirectionSent ||
				pg.transaction.Direction == dcr.TxDirectionReceived ||
				pg.transaction.Type == dcr.TxTypeMixed {
				return D{}
			}

			amount := dcrutil.Amount(pg.transaction.Amount).String()
			if pg.transaction.Type == dcr.TxTypeMixed {
				amount = dcrutil.Amount(pg.transaction.MixDenomination).String()
			} else if pg.transaction.Type == dcr.TxTypeRegular && pg.transaction.Direction == dcr.TxDirectionSent {
				amount = "-" + amount
			}
			return pg.keyValue(gtx, values.String(values.StrTicketPrice), pg.Theme.Label(values.TextSize14, amount).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			// revocation and vote transaction reward
			if pg.transaction.Type == dcr.TxTypeVote {
				return pg.keyValue(gtx, values.String(values.StrReward), pg.Theme.Label(values.TextSize14, dcrutil.Amount(pg.transaction.VoteReward).String()).Layout)
			}
			return D{}
		}),
		layout.Rigid(func(gtx C) D {
			if transaction.BlockHeight != -1 {
				return pg.keyValue(gtx, values.String(values.StrIncludedInBlock), pg.Theme.Label(values.TextSize14, fmt.Sprintf("%d", transaction.BlockHeight)).Layout)
			}
			return D{}
		}),
		layout.Rigid(func(gtx C) D {
			// hide section for tickets
			if pg.transaction.Type == dcr.TxTypeTicketPurchase {
				return D{}
			}
			return pg.keyValue(gtx, values.String(values.StrType), pg.Theme.Label(values.TextSize14, pg.transaction.Type).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			// hide section for non ticket transactions
			if pg.transaction.Type != dcr.TxTypeTicketPurchase {
				return D{}
			}

			if pg.ticketSpender != nil { // voted or revoked
				if pg.ticketSpender.Type == dcr.TxTypeVote {
					return pg.keyValue(gtx, values.String(values.StrVotedOn), pg.Theme.Label(values.TextSize14, timeString(pg.ticketSpender.Timestamp)).Layout)
				} else if pg.ticketSpender.Type == dcr.TxTypeRevocation {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.keyValue(gtx, values.String(values.StrMissedOn), pg.Theme.Label(values.TextSize14, timeString(pg.ticketSpender.Timestamp)).Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return pg.keyValue(gtx, values.String(values.StrRevokeCause), pg.Theme.Label(values.TextSize14, values.String(values.StrMissedTickets)).Layout)
						}),
					)
				}
			}

			if pg.wallet.TxMatchesFilter(pg.transaction, dcr.TxFilterExpired) {
				return pg.keyValue(gtx, values.String(values.StrExpiredOn), pg.Theme.Label(values.TextSize14, timeString(pg.transaction.Timestamp)).Layout)
			}

			// TODO vote transaction progress bar (V2 UI missing)
			// missed tickets currently not implemented on libwallet
			return pg.keyValue(gtx, values.String(values.StrPurchasedOn), pg.Theme.Label(values.TextSize14, timeString(pg.transaction.Timestamp)).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			stat := func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: values.MarginPadding4}.Layout(gtx, pg.txnWidgets.confirmationIcons.Layout12dp)
					}),
					layout.Rigid(func(gtx C) D {
						txt := pg.Theme.Body2("")
						if pg.txConfirmations() > 1 {
							txt.Text = strings.Title(values.String(values.StrConfirmed))
							txt.Color = pg.Theme.Color.Success
						} else {
							txt.Text = strings.Title(values.String(values.StrPending))
							txt.Color = pg.Theme.Color.GrayText2
						}
						return txt.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						m := values.MarginPadding10
						return layout.Inset{
							Left:  m,
							Right: m,
						}.Layout(gtx, func(gtx C) D {
							return pg.dot.Layout(gtx, values.MarginPadding6)
						})
					}),
					layout.Rigid(func(gtx C) D {
						txt := pg.Theme.Body2(values.StringF(values.StrNConfirmations, pg.txConfirmations()))
						txt.Color = pg.Theme.Color.GrayText2
						return txt.Layout(gtx)
					}),
				)
			}

			return pg.keyValue(gtx, values.String(values.StrConfStatus), stat)
		}),
		layout.Rigid(func(gtx C) D {
			return pg.keyValue(gtx, values.String(values.StrTxFee), pg.Theme.Label(values.TextSize14, dcrutil.Amount(transaction.Fee).String()).Layout)
		}),
		layout.Rigid(func(gtx C) D {
			// hide section for non ticket transactions
			if pg.transaction.Type != dcr.TxTypeTicketPurchase {
				return D{}
			}

			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return pg.keyValue(gtx, values.String(values.StrVsp), pg.Theme.Label(values.TextSize14, pg.vspHost).Layout)
				}),
				layout.Rigid(func(gtx C) D {
					return pg.keyValue(gtx, values.String(values.StrVspFee), pg.Theme.Label(values.TextSize14, values.String(values.StrNotAvailable)).Layout)
				}),
			)
		}),
		layout.Rigid(func(gtx C) D {
			dim := func(gtx C) D {
				lbl := pg.Theme.Label(values.TextSize14, utils.SplitSingleString(transaction.Hash, 30))
				lbl.Color = pg.Theme.Color.Primary

				// copy transaction hash
				if pg.hashClickable.Clicked() {
					clipboard.WriteOp{Text: pg.transaction.Hash}.Add(gtx.Ops)
					pg.Toast.Notify(values.String(values.StrTxHashCopied))
				}
				return pg.hashClickable.Layout(gtx, lbl.Layout)
			}
			return pg.keyValue(gtx, values.String(values.StrTransactionID), dim)
		}),
	)
}

func (pg *TxDetailsPage) txnInputs(gtx C) D {
	transaction := pg.transaction

	collapsibleHeader := func(gtx C) D {
		t := pg.Theme.Label(values.TextSize14, values.StringF(values.StrXInputsConsumed, len(transaction.Inputs)))
		t.Color = pg.Theme.Color.GrayText2
		return t.Layout(gtx)
	}

	collapsibleBody := func(gtx C) D {
		return pg.transactionInputsContainer.Layout(gtx, len(transaction.Inputs), func(gtx C, i int) D {
			input := transaction.Inputs[i]
			addr := utils.SplitSingleString(input.PreviousOutpoint, 20)
			return pg.txnIORow(gtx, input.Amount, input.AccountNumber, addr, i)
		})
	}
	return pg.pageSections(gtx, func(gtx C) D {
		return pg.inputsCollapsible.Layout(gtx, collapsibleHeader, collapsibleBody)
	})
}

func (pg *TxDetailsPage) txnOutputs(gtx C) D {
	transaction := pg.transaction

	collapsibleHeader := func(gtx C) D {
		t := pg.Theme.Label(values.TextSize14, values.StringF(values.StrXOutputCreated, len(transaction.Outputs)))
		t.Color = pg.Theme.Color.GrayText2
		return t.Layout(gtx)
	}

	collapsibleBody := func(gtx C) D {
		x := len(transaction.Inputs)
		return pg.transactionOutputsContainer.Layout(gtx, len(transaction.Outputs), func(gtx C, i int) D {
			output := transaction.Outputs[i]
			return pg.txnIORow(gtx, output.Amount, output.AccountNumber, output.Address, i+x)
		})
	}
	return pg.pageSections(gtx, func(gtx C) D {
		return pg.outputsCollapsible.Layout(gtx, collapsibleHeader, collapsibleBody)
	})
}

func (pg *TxDetailsPage) txnIORow(gtx C, amount int64, acctNum int32, address string, i int) D {
	accountName := values.String(values.StrExternal)
	if acctNum != -1 {
		name, err := pg.wallet.AccountName(acctNum)
		if err == nil {
			accountName = name
		}
	}

	accountName = fmt.Sprintf("(%s)", accountName)
	amt := dcrutil.Amount(amount).String()

	return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
		card := pg.Theme.Card()
		card.Color = pg.Theme.Color.Gray4
		return card.Layout(gtx, func(gtx C) D {
			return layout.UniformInset(values.MarginPadding15).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(pg.Theme.Label(values.TextSize14, amt).Layout),
							layout.Rigid(func(gtx C) D {
								m := values.MarginPadding5
								return layout.Inset{
									Left:  m,
									Right: m,
								}.Layout(gtx, pg.Theme.Label(values.TextSize14, accountName).Layout)
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						// copy address
						if pg.txnWidgets.copyTextButtons[i].Clicked() {
							clipboard.WriteOp{Text: address}.Add(gtx.Ops)
							pg.Toast.Notify(values.String(values.StrCopied))
						}

						return layout.W.Layout(gtx, func(gtx C) D {
							lbl := pg.Theme.Label(values.TextSize14, address)
							lbl.Color = pg.Theme.Color.Primary
							return pg.txnWidgets.copyTextButtons[i].Layout(gtx, lbl.Layout)
						})
					}),
				)
			})
		})
	})
}

func (pg *TxDetailsPage) layoutOptionsMenu(gtx C) {
	inset := layout.Inset{
		Left: values.MarginPaddingMinus145,
	}

	m := op.Record(gtx.Ops)
	inset.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding168)
		return pg.shadowBox.Layout(gtx, func(gtx C) D {
			optionsMenuCard := cryptomaterial.Card{Color: pg.Theme.Color.Surface}
			optionsMenuCard.Radius = cryptomaterial.Radius(5)
			return optionsMenuCard.Layout(gtx, func(gtx C) D {
				return (&layout.List{Axis: layout.Vertical}).Layout(gtx, len(pg.moreItems), func(gtx C, i int) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return pg.moreItems[i].button.Layout(gtx, func(gtx C) D {
								return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
									redirectURL := pg.WL.Wallet.GetDCRBlockExplorerURL(pg.transaction.Hash)
									if pg.moreItems[i].button.Clicked() {
										switch pg.moreItems[i].id {
										case copyBlockID: // copy the redirect url
											clipboard.WriteOp{Text: redirectURL}.Add(gtx.Ops)
											pg.Toast.Notify(values.String(values.StrCopied))
											pg.moreOptionIsOpen = false
										case viewBlockID: // redirect to browser
											components.GoToURL(redirectURL)
											pg.moreOptionIsOpen = false
										default:
										}
									}

									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									return pg.Theme.Label(values.TextSize14, pg.moreItems[i].text).Layout(gtx)
								})
							})
						}),
					)
				})
			})
		})
	})
	op.Defer(gtx.Ops, m.Stop())
}

func (pg *TxDetailsPage) pageSections(gtx C, body layout.Widget) D {
	return layout.Inset{
		Left:   values.MarginPadding70,
		Right:  values.MarginPadding24,
		Bottom: values.MarginPadding30,
	}.Layout(gtx, body)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *TxDetailsPage) HandleUserInteractions() {
	for pg.moreOption.Clicked() {
		pg.moreOptionIsOpen = !pg.moreOptionIsOpen
	}

	for pg.associatedTicketClickable.Clicked() {
		if pg.ticketSpent != nil {
			pg.txBackStack = pg.transaction
			pg.transaction = pg.ticketSpent
			pg.getTXSourceAccountAndDirection()
			pg.txnWidgets = initTxnWidgets(pg.Load, pg.transaction)
			pg.ParentWindow().Reload()
		}
	}

	if pg.rebroadcastClickable.Clicked() {
		go func() {
			pg.rebroadcastClickable.SetEnabled(false, nil)
			if !pg.Load.WL.SelectedWallet.Wallet.IsConnectedToNetwork() {
				// if user is not conected to the network, notify the user
				errModal := modal.NewErrorModal(pg.Load, values.String(values.StrNotConnected), modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(errModal)
				if !pg.rebroadcastClickable.Enabled() {
					pg.rebroadcastClickable.SetEnabled(true, nil)
				}
				return
			}

			err := pg.wallet.PublishUnminedTransactions()
			if err != nil {
				// If transactions are not published, notify the user
				errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(errModal)
			} else {
				infoModal := modal.NewSuccessModal(pg.Load, values.String(values.StrRepublished), modal.DefaultClickFunc())
				pg.ParentWindow().ShowModal(infoModal)
			}
			if !pg.rebroadcastClickable.Enabled() {
				pg.rebroadcastClickable.SetEnabled(true, nil)
			}
		}()
	}

	redirectURL := pg.WL.Wallet.GetDCRBlockExplorerURL(pg.transaction.Hash)
	for _, menu := range pg.moreItems {
		if menu.button.Clicked() && menu.id == viewBlockID {
			components.GoToURL(redirectURL)
			pg.moreOptionIsOpen = false
			break
		}
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *TxDetailsPage) OnNavigatedFrom() {}

func initTxnWidgets(l *load.Load, transaction *sharedW.Transaction) transactionWdg {

	var txn transactionWdg
	wal := l.WL.SelectedWallet.Wallet

	t := time.Unix(transaction.Timestamp, 0).UTC()
	txn.time = l.Theme.Body2(t.Format(time.UnixDate))
	txn.status = l.Theme.Body1("")
	txn.wallet = l.Theme.Body2(wal.GetWalletName())

	if components.TxConfirmations(l, *transaction) > 1 {
		txn.status.Text = components.FormatDateOrTime(transaction.Timestamp)
		txn.confirmationIcons = l.Theme.Icons.ConfirmIcon
	} else {
		txn.status.Text = values.String(values.StrPending)
		txn.status.Color = l.Theme.Color.GrayText2
		txn.confirmationIcons = l.Theme.Icons.PendingIcon
	}

	txStatus := components.TransactionTitleIcon(l, wal, transaction)
	txn.txStatus = txStatus

	x := len(transaction.Inputs) + len(transaction.Outputs)
	txn.copyTextButtons = make([]*cryptomaterial.Clickable, x)
	for i := 0; i < x; i++ {
		txn.copyTextButtons[i] = l.Theme.NewClickable(false)
	}

	return txn
}

func timeString(timestamp int64) string {
	return time.Unix(timestamp, 0).Format("Jan 2, 2006 15:04:05 PM")
}
