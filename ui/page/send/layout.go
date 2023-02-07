package send

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	libUtil "code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

func (pg *Page) initLayoutWidgets() {
	pg.pageContainer = &widget.List{
		List: layout.List{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		},
	}

	buttonInset := layout.Inset{
		Top:    values.MarginPadding4,
		Right:  values.MarginPadding8,
		Bottom: values.MarginPadding4,
		Left:   values.MarginPadding8,
	}

	pg.nextButton = pg.Theme.Button(values.String(values.StrNext))
	pg.nextButton.TextSize = values.TextSize18
	pg.nextButton.Inset = layout.Inset{Top: values.MarginPadding15, Bottom: values.MarginPadding15}
	pg.nextButton.SetEnabled(false)

	_, pg.infoButton = components.SubpageHeaderButtons(pg.Load)

	pg.retryExchange = pg.Theme.Button(values.String(values.StrRetry))
	pg.retryExchange.Background = pg.Theme.Color.Gray1
	pg.retryExchange.Color = pg.Theme.Color.Surface
	pg.retryExchange.TextSize = values.TextSize12
	pg.retryExchange.Inset = buttonInset

	pg.editRates = pg.Theme.Button(values.String(values.StrEdit))

	if pg.isFeerateAPIApproved() {
		fetchRateBtn := pg.Theme.Button(values.String(values.StrFetchRates))
		fetchRateBtn.TextSize = values.TextSize12
		fetchRateBtn.Inset = buttonInset
		pg.fetchRates = fetchRateBtn
	} else {
		str := values.StringF(values.StrNotAllowed, values.String(values.StrFeeRates))
		fetchRateLabel := pg.Theme.Label(values.TextSize14, str)
		fetchRateLabel.Font.Weight = text.SemiBold
		pg.fetchRates = fetchRateLabel
	}

	pg.editRates.TextSize = values.TextSize12
	pg.editRates.Inset = buttonInset

	pg.ratesEditor = pg.Theme.Editor(new(widget.Editor), "in Sat/kvB")
	pg.ratesEditor.HasCustomButton = false
	pg.ratesEditor.Editor.SingleLine = true
	pg.ratesEditor.TextSize = values.TextSize14

	// Default value for display before fee rate is set.
	pg.editOrDisplay = " - "
	pg.priority = "Unknown"
}

func (pg *Page) topNav(gtx layout.Context) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(pg.Theme.H6(values.String(values.StrSend)+" "+string(pg.WL.SelectedWallet.Wallet.GetAssetType())).Layout),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(pg.infoButton.Layout),
				)
			})
		}),
	)
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *Page) Layout(gtx layout.Context) layout.Dimensions {
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *Page) layoutDesktop(gtx layout.Context) layout.Dimensions {
	pageContent := []func(gtx C) D{
		func(gtx C) D {
			return pg.pageSections(gtx, values.String(values.StrFrom), false, func(gtx C) D {
				return pg.sourceAccountSelector.Layout(pg.ParentWindow(), gtx)
			})
		},
		func(gtx C) D {
			return pg.toSection(gtx)
		},
		func(gtx C) D {
			return pg.coinSelectionSection(gtx)
		},
	}

	// Display the transaction fee rate selection only for btc wallets.
	if pg.selectedWallet.GetAssetType() == libUtil.BTCWalletAsset {
		pageContent = append(pageContent,
			func(gtx C) D { return pg.transactionFeeSection(gtx) },
		)
	}

	dims := layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return layout.Stack{Alignment: layout.NE}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return components.UniformPadding(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, pg.topNav)
							}),
							layout.Rigid(func(gtx C) D {
								return pg.Theme.List(pg.pageContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
									return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
										return layout.Inset{Bottom: values.MarginPadding4, Top: values.MarginPadding4}.Layout(gtx, pageContent[i])
									})
								})
							}),
						)
					})
				}),
			)
		}),
		layout.Stacked(func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return layout.S.Layout(gtx, func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding1}.Layout(gtx, pg.balanceSection)
			})
		}),
	)

	return dims
}

func (pg *Page) layoutMobile(gtx layout.Context) layout.Dimensions {
	pageContent := []func(gtx C) D{
		func(gtx C) D {
			return pg.pageSections(gtx, values.String(values.StrFrom), false, func(gtx C) D {
				return pg.sourceAccountSelector.Layout(pg.ParentWindow(), gtx)
			})
		},
		func(gtx C) D {
			return pg.toSection(gtx)
		},
		func(gtx C) D {
			return pg.coinSelectionSection(gtx)
		},
	}

	dims := layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return layout.Stack{Alignment: layout.NE}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return components.UniformMobile(gtx, false, true, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: values.MarginPadding16, Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
									return pg.topNav(gtx)
								})
							}),
							layout.Rigid(func(gtx C) D {
								return pg.Theme.List(pg.pageContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
									return layout.Inset{Bottom: values.MarginPadding16, Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
										return layout.Inset{Bottom: values.MarginPadding4, Top: values.MarginPadding4}.Layout(gtx, pageContent[i])
									})
								})
							}),
						)
					})
				}),
			)
		}),
		layout.Stacked(func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return layout.S.Layout(gtx, func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding1}.Layout(gtx, func(gtx C) D {
					return pg.balanceSection(gtx)
				})
			})
		}),
	)

	return dims
}

func (pg *Page) pageSections(gtx layout.Context, title string, showAccountSwitch bool, body layout.Widget) layout.Dimensions {
	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							inset := layout.Inset{
								Bottom: values.MarginPadding16,
							}
							titleTxt := pg.Theme.Body1(title)
							titleTxt.Color = pg.Theme.Color.Text
							return inset.Layout(gtx, titleTxt.Layout)
						}),
						layout.Flexed(1, func(gtx C) D {
							if showAccountSwitch {
								return layout.E.Layout(gtx, func(gtx C) D {
									inset := layout.Inset{
										Top: values.MarginPaddingMinus5,
									}
									return inset.Layout(gtx, pg.sendDestination.accountSwitch.Layout)
								})
							}
							return layout.Dimensions{}
						}),
					)
				}),
				layout.Rigid(body),
			)
		})
	})
}

func (pg *Page) toSection(gtx layout.Context) layout.Dimensions {
	return pg.pageSections(gtx, values.String(values.StrTo), true, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Bottom: values.MarginPadding16,
				}.Layout(gtx, func(gtx C) D {
					if !pg.sendDestination.sendToAddress {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Orientation: layout.Vertical,
							Margin:      layout.Inset{Bottom: values.MarginPadding16},
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Bottom: values.MarginPadding16,
								}.Layout(gtx, func(gtx C) D {
									return pg.sendDestination.destinationWalletSelector.Layout(pg.ParentWindow(), gtx)
								})
							}),
							layout.Rigid(func(gtx C) D {
								return pg.sendDestination.destinationAccountSelector.Layout(pg.ParentWindow(), gtx)
							}),
						)
					}
					return pg.sendDestination.destinationAddressEditor.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if pg.exchangeRate != -1 && pg.usdExchangeSet {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Flexed(0.45, func(gtx C) D {
							return pg.amount.amountEditor.Layout(gtx)
						}),
						layout.Flexed(0.1, func(gtx C) D {
							return layout.Center.Layout(gtx, func(gtx C) D {
								icon := pg.Theme.Icons.CurrencySwapIcon
								return icon.Layout12dp(gtx)
							})
						}),
						layout.Flexed(0.45, func(gtx C) D {
							return pg.amount.usdAmountEditor.Layout(gtx)
						}),
					)
				}
				return pg.amount.amountEditor.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				if pg.exchangeRateMessage == "" {
					return layout.Dimensions{}
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding1)
							return cryptomaterial.Fill(gtx, pg.Theme.Color.Gray1)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								label := pg.Theme.Body2(pg.exchangeRateMessage)
								label.Color = pg.Theme.Color.Danger
								if pg.isFetchingExchangeRate {
									label.Color = pg.Theme.Color.Primary
								}
								return label.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								if pg.isFetchingExchangeRate {
									return layout.Dimensions{}
								}
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return layout.E.Layout(gtx, pg.retryExchange.Layout)
							}),
						)
					}),
				)
			}),
		)
	})
}

func (pg *Page) coinSelectionSection(gtx layout.Context) D {
	m := values.MarginPadding20
	inset := layout.Inset{}
	return inset.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			inset := layout.Inset{
				Top:    values.MarginPadding15,
				Right:  values.MarginPadding15,
				Bottom: values.MarginPadding15,
				Left:   values.MarginPadding15,
			}
			return inset.Layout(gtx, func(gtx C) D {
				textLabel := pg.Theme.Label(values.TextSize16, values.String(values.StrCoinSelection))
				return layout.Inset{}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(textLabel.Layout),
						layout.Flexed(1, func(gtx C) D {
							return layout.E.Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(pg.Theme.Label(values.TextSize16, values.String(values.StrAutomatic)).Layout),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{Left: m}.Layout(gtx, pg.Theme.Icons.ChevronRight.Layout24dp)
									}),
								)
							})
						}),
					)
				})
			})
		})
	})
}

// transactionFeeSection only supports btc fee rate setting.
func (pg *Page) transactionFeeSection(gtx layout.Context) D {
	inset := layout.Inset{
		Bottom: values.MarginPadding100,
	}

	return inset.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			topContainer := layout.UniformInset(values.MarginPadding15)
			return topContainer.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X // use maximum width

				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						textLabel := pg.Theme.Label(values.TextSize16, values.String(values.StrTxFee))
						return layout.Inset{Bottom: values.MarginPadding10}.Layout(gtx, textLabel.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						card := pg.Theme.Card()
						card.Color = pg.Theme.Color.Background

						return card.Layout(gtx, func(gtx C) D {
							return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
								gtx.Constraints.Min.X = gtx.Constraints.Max.X // use maximum width

								feeText := pg.txFee
								if pg.exchangeRate != -1 && pg.usdExchangeSet {
									feeText = fmt.Sprintf("%s (%s)", pg.txFee, pg.txFeeUSD)
								}

								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									pg.widgetsRow(gtx, pg.editOrDisplay, pg.editRates, pg.fetchRates),
									pg.widgetsRow(gtx, values.StringF(values.StrPriority, " : "), pg.priority),
									pg.widgetsRow(gtx, values.StringF(values.StrTxSize, " : "), pg.estSignedSize),
									pg.widgetsRow(gtx, values.StringF(values.StrCost, " : "), feeText),
								)
							})
						})
					}),
				)
			})
		})
	})
}

func (pg *Page) balanceSection(gtx layout.Context) layout.Dimensions {
	c := pg.Theme.Card()
	c.Radius = cryptomaterial.Radius(0)
	return c.Layout(gtx, func(gtx C) D {
		inset := layout.Inset{
			Top:    values.MarginPadding10,
			Bottom: values.MarginPadding10,
			Left:   values.MarginPadding15,
			Right:  values.MarginPadding15,
		}
		return inset.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.6, func(gtx C) D {
					inset := layout.Inset{
						Right: values.MarginPadding15,
					}
					return inset.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {

								totalCostText := pg.totalCost
								if pg.exchangeRate != -1 && pg.usdExchangeSet {
									totalCostText = fmt.Sprintf("%s (%s)", pg.totalCost, pg.totalCostUSD)
								}
								return pg.contentRow(gtx, values.String(values.StrTotalCost), totalCostText)
							}),
							layout.Rigid(func(gtx C) D {
								balanceAfterSendText := pg.balanceAfterSend
								if pg.exchangeRate != -1 && pg.usdExchangeSet {
									balanceAfterSendText = fmt.Sprintf("%s (%s)", pg.balanceAfterSend, pg.balanceAfterSendUSD)
								}
								return pg.contentRow(gtx, values.String(values.StrBalanceAfter), balanceAfterSendText)
							}),
						)
					})
				}),
				layout.Flexed(0.3, func(gtx C) D {
					return pg.nextButton.Layout(gtx)
				}),
			)
		})
	})
}

func (pg *Page) contentRow(gtx layout.Context, leftValue, rightValue string) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := pg.Theme.Body2(leftValue)
			txt.Color = pg.Theme.Color.GrayText2
			return txt.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						rightText := pg.Theme.Body1(rightValue)
						rightText.Color = pg.Theme.Color.Text
						return rightText.Layout(gtx)
					}),
				)
			})
		}),
	)
}

func (pg *Page) widgetsRow(gtx layout.Context, items ...interface{}) layout.FlexChild {
	widgets := make([]layout.FlexChild, 0, len(items))
	for _, item := range items {
		switch n := item.(type) {
		case string:
			w := pg.Theme.Label(values.TextSize14, n)
			if len(widgets) == 0 {
				w.Font.Weight = text.SemiBold
			} else {
				w.Font.Style = text.Italic
			}
			widgets = append(widgets, layout.Rigid(w.Layout))
		case cryptomaterial.Button:
			widgets = append(widgets, layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding10}.Layout(gtx, n.Layout)
			}))
		case cryptomaterial.Label:
			widgets = append(widgets, layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Center.Layout(gtx, n.Layout)
			}))
		case cryptomaterial.Editor:
			widgets = append(widgets, layout.Rigid(func(gtx C) D {
				n.Editor.Focus()
				// Resize the height to fit 1/5 of the original height.
				gtx.Constraints.Max.X = gtx.Constraints.Max.X / 5
				return layout.Inset{Bottom: values.MarginPadding5}.Layout(gtx, n.Layout)
			}))
		default:
			continue
		}
	}

	return layout.Rigid(func(gtx C) D {
		return layout.Inset{Bottom: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, widgets...)
		})
	})
}
