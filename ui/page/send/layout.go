package send

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
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

	pg.txLabelInputEditor = pg.Theme.Editor(new(widget.Editor), values.String(values.StrNote))
	pg.txLabelInputEditor.Editor.SingleLine = false
	pg.txLabelInputEditor.Editor.SetText("")
	// Set the maximum characters the editor can accept.
	pg.txLabelInputEditor.Editor.MaxLen = MaxTxLabelSize

	pg.toCoinSelection = pg.Theme.NewClickable(false)
}

func (pg *Page) topNav(gtx layout.Context) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(pg.Theme.H6(values.String(values.StrSend)+" "+string(pg.selectedWallet.GetAssetType())).Layout),
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
	if pg.Load.IsMobileView() {
		return pg.layoutMobile(gtx)
	}

	if pg.modalLayout != nil {
		modalContent := []layout.Widget{pg.layoutDesktop}
		return pg.modalLayout.Layout(gtx, modalContent, 450)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *Page) layoutDesktop(gtx layout.Context) D {
	pageContent := []func(gtx C) D{
		func(gtx C) D {
			return pg.pageSections(gtx, values.String(values.StrFrom), false, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if pg.modalLayout != nil {
							return layout.Inset{
								Bottom: values.MarginPadding16,
							}.Layout(gtx, func(gtx C) D {
								return pg.sourceWalletSelector.Layout(pg.ParentWindow(), gtx)
							})
						}
						return D{}
					}),
					layout.Rigid(func(gtx C) D {
						return pg.sourceAccountSelector.Layout(pg.ParentWindow(), gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if pg.selectedWallet.IsSynced() {
							return D{}
						}
						txt := pg.Theme.Label(values.TextSize14, values.String(values.StrFunctionUnavailable))
						txt.Font.Weight = font.SemiBold
						txt.Color = pg.Theme.Color.Danger
						return txt.Layout(gtx)
					}),
				)
			})
		},
		func(gtx C) D {
			// disable this section if the layout is a modal layout
			// and the selected wallet is not synced.
			if pg.modalLayout != nil && !pg.selectedWallet.IsSynced() {
				gtx = gtx.Disabled()
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pg.toSection),
				layout.Rigid(func(gtx C) D {
					if pg.modalLayout != nil {
						// coin selection not allowed on the send modal
						return D{}
					}
					return pg.coinSelectionSection(gtx)
				}),
				layout.Rigid(pg.txLabelSection),
			)
		},
	}

	// Display the transaction fee rate selection only for btc and ltc wallets.
	switch pg.selectedWallet.GetAssetType() {
	case libUtil.BTCWalletAsset, libUtil.LTCWalletAsset:
		pageContent = append(pageContent, pg.feeRateSelector.Layout)
	}

	// Add the bottom spacing section as the last.
	inset := layout.Inset{
		Bottom: values.MarginPadding50,
	}
	pageContent = append(pageContent, func(gtx C) D {
		return inset.Layout(gtx, func(gtx C) D { return D{} })
	})

	dims := layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return layout.Stack{Alignment: layout.NE}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return layout.Inset{
						Left:   values.MarginPadding24,
						Right:  values.MarginPadding24,
						Bottom: values.MarginPadding24,
					}.Layout(gtx, func(gtx C) D {
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
									pg.sendDestination.accountSwitch.SetSelectedIndex(pg.sendDestination.selectedIndex)
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
	return layout.Inset{Bottom: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
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
	})
}

func (pg *Page) coinSelectionSection(gtx layout.Context) D {
	selectedOption := automaticCoinSelection
	sourceAcc := pg.sourceAccountSelector.SelectedAccount()
	if len(pg.selectedUTXOs.selectedUTXOs) > 0 && pg.selectedUTXOs.sourceAccount == sourceAcc {
		selectedOption = manualCoinSelection
	}

	return pg.Theme.Card().Layout(gtx, func(gtx C) D {
		inset := layout.UniformInset(values.MarginPadding15)
		return inset.Layout(gtx, func(gtx C) D {
			textLabel := pg.Theme.Label(values.TextSize16, values.String(values.StrCoinSelection))
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(textLabel.Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Orientation: layout.Horizontal,
							Alignment:   layout.Middle,
							Clickable:   pg.toCoinSelection,
						}.Layout(gtx,
							layout.Rigid(pg.Theme.Label(values.TextSize16, selectedOption).Layout),
							layout.Rigid(pg.Theme.Icons.ChevronRight.Layout24dp),
						)
					})
				}),
			)
		})
	})
}

func (pg *Page) txLabelSection(gtx layout.Context) D {
	return layout.Inset{Top: values.MarginPadding8}.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			topContainer := layout.UniformInset(values.MarginPadding15)
			return topContainer.Layout(gtx, func(gtx C) D {
				textLabel := pg.Theme.Label(values.TextSize16, values.String(values.StrDescriptionNote))
				count := len(pg.txLabelInputEditor.Editor.Text())
				txt := fmt.Sprintf("(%d/%d)", count, pg.txLabelInputEditor.Editor.MaxLen)
				wordsCount := pg.Theme.Label(values.TextSize14, txt)
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(textLabel.Layout),
							layout.Flexed(1,
								func(gtx C) D {
									return layout.Inset{
										Top:  values.MarginPadding2,
										Left: values.MarginPadding5,
									}.Layout(gtx, wordsCount.Layout)
								}),
						)
					}),

					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Top: values.MarginPadding10,
						}.Layout(gtx, func(gtx C) D {
							return pg.txLabelInputEditor.Layout(gtx)
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
