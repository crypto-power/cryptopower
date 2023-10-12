package send

import (
	"fmt"

	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

func (wi *sharedProperties) topNav(gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(wi.Theme.H6(values.String(values.StrSend)+" "+string(wi.selectedWallet.Asset.GetAssetType())).Layout),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(wi.infoButton.Layout),
				)
			})
		}),
	)
}

func (wi *sharedProperties) layoutDesktop(gtx C, window app.WindowNavigator) D {
	if wi.parentWindow == nil {
		wi.parentWindow = window
	}
	pageContent := []func(gtx C) D{
		func(gtx C) D {
			return wi.pageSections(gtx, values.String(values.StrFrom), false, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if wi.isModalLayout {
							return layout.Inset{
								Bottom: values.MarginPadding16,
							}.Layout(gtx, func(gtx C) D {
								return wi.sourceWalletSelector.Layout(wi.parentWindow, gtx)
							})
						}
						return D{}
					}),
					layout.Rigid(func(gtx C) D {
						return wi.sourceAccountSelector.Layout(wi.parentWindow, gtx)
					}),
				)
			})
		},
		wi.toSection,
		wi.coinSelectionSection,
		wi.txLabelSection,
	}

	// Display the transaction fee rate selection only for btc and ltc wallets.
	switch wi.selectedWallet.GetAssetType() {
	case libUtil.BTCWalletAsset, libUtil.LTCWalletAsset:
		pageContent = append(pageContent, wi.feeRateSelector.Layout)
	}

	// Add the bottom spacing section as the last.
	inset := layout.Inset{
		Bottom: values.MarginPadding100,
	}
	pageContent = append(pageContent, func(gtx C) D {
		return inset.Layout(gtx, func(gtx C) D { return D{} })
	})

	dims := layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return layout.Stack{Alignment: layout.NE}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return components.UniformPadding(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, wi.topNav)
							}),
							layout.Rigid(func(gtx C) D {
								return wi.Theme.List(wi.pageContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
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
				return layout.Inset{Left: values.MarginPadding1}.Layout(gtx, wi.balanceSection)
			})
		}),
	)

	return dims
}

func (wi *sharedProperties) layoutMobile(gtx C) D {
	pageContent := []func(gtx C) D{
		func(gtx C) D {
			return wi.pageSections(gtx, values.String(values.StrFrom), false, func(gtx C) D {
				return wi.sourceAccountSelector.Layout(wi.parentWindow, gtx)
			})
		},
		wi.toSection,
		wi.coinSelectionSection,
	}

	dims := layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return layout.Stack{Alignment: layout.NE}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return components.UniformMobile(gtx, false, true, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Bottom: values.MarginPadding16,
									Right:  values.MarginPadding10,
								}.Layout(gtx, wi.topNav)
							}),
							layout.Rigid(func(gtx C) D {
								return wi.Theme.List(wi.pageContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
									return layout.Inset{
										Bottom: values.MarginPadding16,
										Right:  values.MarginPadding2,
									}.Layout(gtx, func(gtx C) D {
										return layout.Inset{
											Bottom: values.MarginPadding4,
											Top:    values.MarginPadding4,
										}.Layout(gtx, pageContent[i])
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
				return layout.Inset{Left: values.MarginPadding1}.Layout(gtx, wi.balanceSection)
			})
		}),
	)

	return dims
}

func (wi *sharedProperties) pageSections(gtx C, title string, showAccountSwitch bool, body layout.Widget) D {
	return wi.Theme.Card().Layout(gtx, func(gtx C) D {
		return layout.UniformInset(values.MarginPadding16).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							inset := layout.Inset{
								Bottom: values.MarginPadding16,
							}
							titleTxt := wi.Theme.Body1(title)
							titleTxt.Color = wi.Theme.Color.Text
							return inset.Layout(gtx, titleTxt.Layout)
						}),
						layout.Flexed(1, func(gtx C) D {
							if showAccountSwitch {
								return layout.E.Layout(gtx, func(gtx C) D {
									inset := layout.Inset{
										Top: values.MarginPaddingMinus5,
									}
									wi.sendDestination.accountSwitch.SetSelectedIndex(wi.sendDestination.selectedIndex)
									return inset.Layout(gtx, wi.sendDestination.accountSwitch.Layout)
								})
							}
							return D{}
						}),
					)
				}),
				layout.Rigid(body),
			)
		})
	})
}

func (wi *sharedProperties) toSection(gtx C) D {
	return wi.pageSections(gtx, values.String(values.StrTo), true, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Bottom: values.MarginPadding16,
				}.Layout(gtx, func(gtx C) D {
					if !wi.sendDestination.sendToAddress {
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
									return wi.sendDestination.destinationWalletSelector.Layout(wi.parentWindow, gtx)
								})
							}),
							layout.Rigid(func(gtx C) D {
								return wi.sendDestination.destinationAccountSelector.Layout(wi.parentWindow, gtx)
							}),
						)
					}
					return wi.sendDestination.destinationAddressEditor.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if wi.exchangeRate != -1 && wi.usdExchangeSet {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Flexed(0.45, wi.amount.amountEditor.Layout),
						layout.Flexed(0.1, func(gtx C) D {
							return layout.Center.Layout(gtx, func(gtx C) D {
								icon := wi.Theme.Icons.CurrencySwapIcon
								return icon.Layout12dp(gtx)
							})
						}),
						layout.Flexed(0.45, wi.amount.usdAmountEditor.Layout),
					)
				}
				return wi.amount.amountEditor.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				if wi.exchangeRateMessage == "" {
					return D{}
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding16, Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							gtx.Constraints.Min.Y = gtx.Dp(values.MarginPadding1)
							return cryptomaterial.Fill(gtx, wi.Theme.Color.Gray1)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								label := wi.Theme.Body2(wi.exchangeRateMessage)
								label.Color = wi.Theme.Color.Danger
								if wi.isFetchingExchangeRate {
									label.Color = wi.Theme.Color.Primary
								}
								return label.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								if wi.isFetchingExchangeRate {
									return D{}
								}
								gtx.Constraints.Min.X = gtx.Constraints.Max.X
								return layout.E.Layout(gtx, wi.retryExchange.Layout)
							}),
						)
					}),
				)
			}),
		)
	})
}

func (wi *sharedProperties) coinSelectionSection(gtx C) D {
	selectedOption := automaticCoinSelection
	sourceAcc := wi.sourceAccountSelector.SelectedAccount()
	if len(wi.selectedUTXOs.selectedUTXOs) > 0 && wi.selectedUTXOs.sourceAccount == sourceAcc {
		selectedOption = manualCoinSelection
	}

	return wi.Theme.Card().Layout(gtx, func(gtx C) D {
		inset := layout.UniformInset(values.MarginPadding15)
		return inset.Layout(gtx, func(gtx C) D {
			textLabel := wi.Theme.Label(values.TextSize16, values.String(values.StrCoinSelection))
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(textLabel.Layout),
				layout.Flexed(1, func(gtx C) D {
					return layout.E.Layout(gtx, func(gtx C) D {
						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Orientation: layout.Horizontal,
							Alignment:   layout.Middle,
							Clickable:   wi.toCoinSelection,
						}.Layout(gtx,
							layout.Rigid(wi.Theme.Label(values.TextSize16, selectedOption).Layout),
							layout.Rigid(wi.Theme.Icons.ChevronRight.Layout24dp),
						)
					})
				}),
			)
		})
	})
}

func (wi *sharedProperties) txLabelSection(gtx C) D {
	return wi.Theme.Card().Layout(gtx, func(gtx C) D {
		topContainer := layout.UniformInset(values.MarginPadding15)
		return topContainer.Layout(gtx, func(gtx C) D {
			textLabel := wi.Theme.Label(values.TextSize16, values.String(values.StrDescriptionNote))
			count := len(wi.txLabelInputEditor.Editor.Text())
			txt := fmt.Sprintf("(%d/%d)", count, wi.txLabelInputEditor.Editor.MaxLen)
			wordsCount := wi.Theme.Label(values.TextSize14, txt)
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
						return wi.txLabelInputEditor.Layout(gtx)
					})
				}),
			)
		})
	})
}

func (wi *sharedProperties) balanceSection(gtx C) D {
	c := wi.Theme.Card()
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
								totalCostText := wi.totalCost
								if wi.exchangeRate != -1 && wi.usdExchangeSet {
									totalCostText = fmt.Sprintf("%s (%s)", wi.totalCost, wi.totalCostUSD)
								}
								return wi.contentRow(gtx, values.String(values.StrTotalCost), totalCostText)
							}),
							layout.Rigid(func(gtx C) D {
								balanceAfterSendText := wi.balanceAfterSend
								if wi.exchangeRate != -1 && wi.usdExchangeSet {
									balanceAfterSendText = fmt.Sprintf("%s (%s)", wi.balanceAfterSend, wi.balanceAfterSendUSD)
								}
								return wi.contentRow(gtx, values.String(values.StrBalanceAfter), balanceAfterSendText)
							}),
						)
					})
				}),
				layout.Flexed(0.3, func(gtx C) D {
					return wi.nextButton.Layout(gtx)
				}),
			)
		})
	})
}

func (wi *sharedProperties) contentRow(gtx C, leftValue, rightValue string) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := wi.Theme.Body2(leftValue)
			txt.Color = wi.Theme.Color.GrayText2
			return txt.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						rightText := wi.Theme.Body1(rightValue)
						rightText.Color = wi.Theme.Color.Text
						return rightText.Layout(gtx)
					}),
				)
			})
		}),
	)
}
