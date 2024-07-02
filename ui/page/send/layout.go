package send

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget"

	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
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
	pg.advanceOptions = pg.Theme.Collapsible()
	pg.advanceOptions.IconStyle = cryptomaterial.Caret

	_, pg.infoButton = components.SubpageHeaderButtons(pg.Load)

	pg.nextButton = pg.Theme.Button(values.String(values.StrNext))
	pg.nextButton.TextSize = values.TextSize16
	pg.nextButton.Inset = layout.Inset{Top: values.MarginPadding12, Bottom: values.MarginPadding12}
	pg.nextButton.SetEnabled(false)

	pg.toCoinSelection = pg.Theme.NewClickable(false)
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *Page) Layout(gtx C) D {
	if pg.modalLayout == nil {
		return pg.contentLayout(gtx)
	}
	var modalWidth float32 = 450
	if pg.IsMobileView() {
		modalWidth = 0
	}
	modalContent := []layout.Widget{pg.contentLayout}
	return pg.modalLayout.Layout(gtx, modalContent, modalWidth)
}

func (pg *Page) contentLayout(gtx C) D {
	var pageContent []func(gtx C) D
	// Always include the sendLayout
	pageContent = append(pageContent, pg.sendLayout)

	if pg.selectedWallet != nil && pg.selectedWallet.IsSynced() {
		// Include these layouts only if the wallet is synced
		pageContent = append(pageContent, pg.recipientsLayout)
		pageContent = append(pageContent, pg.advanceOptionsLayout)
	} else {
		// Include the notSyncedLayout if the wallet is not synced
		if pg.modalLayout != nil {
			pageContent = append(pageContent, pg.notSyncedLayout)
		}
	}

	cgtx := gtx
	macro := op.Record(cgtx.Ops)
	dims := pg.balanceSection(cgtx)
	call := macro.Stop()
	pageContent = append(pageContent, func(gtx C) D {
		return layout.Spacer{Height: gtx.Metric.PxToDp(dims.Size.Y)}.Layout(gtx)
	})

	return layout.Stack{Alignment: layout.S}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return pg.Theme.List(pg.pageContainer).Layout(gtx, len(pageContent), func(gtx C, i int) D {
				mp := values.MarginPaddingTransform(pg.IsMobileView(), values.MarginPadding32)
				if i == len(pageContent)-1 {
					mp = values.MarginPadding0
				}
				return layout.Inset{Bottom: mp}.Layout(gtx, pageContent[i])
			})
		}),
		layout.Stacked(func(gtx C) D {
			return layout.S.Layout(gtx, func(gtx C) D {
				call.Add(gtx.Ops)
				return dims
			})
		}),
	)
}

func (pg *Page) sendLayout(gtx C) D {
	return pg.sectionWrapper(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Bottom: values.MarginPadding16,
				}.Layout(gtx, pg.titleLayout)
			}),
			layout.Rigid(func(gtx C) D {
				if pg.hideWalletDropdown {
					return D{}
				}
				return layout.Inset{Bottom: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					return pg.walletDropdown.Layout(gtx, values.String(values.StrSourceWallet))
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, func(gtx C) D {
					return pg.accountDropdown.Layout(gtx, values.String(values.StrSourceAccount))
				})
			}),
		)
	})
}

func (pg *Page) titleLayout(gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Right: values.MarginPadding6}.Layout(gtx, func(gtx C) D {
				lbl := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize20), values.String(values.StrSend))
				lbl.Font.Weight = font.SemiBold
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(pg.infoButton.Layout),
	)
}

func (pg *Page) recipientsLayout(gtx C) D {
	return pg.sectionWrapper(gtx, func(gtx C) D {
		flexChilds := make([]layout.FlexChild, 0)
		for i := range pg.recipients {
			re := pg.recipients[i]
			j := i
			flexChilds = append(flexChilds, layout.Rigid(func(gtx C) D {
				return re.recipientLayout(j+1, len(pg.recipients) > 1)(gtx)
			}))
		}
		if pg.modalLayout == nil && len(pg.recipients) < 3 {
			flexChilds = append(flexChilds, layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.E.Layout(gtx, pg.addRecipentBtnLayout)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, flexChilds...)
	})
}

func (pg *Page) addRecipentBtnLayout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     cryptomaterial.WrapContent,
		Background: pg.Theme.Color.SurfaceHighlight,
		Clickable:  pg.addRecipentBtn,
		Alignment:  layout.Middle,
	}.Layout(gtx,
		layout.Rigid(pg.Theme.AddIcon().Layout24dp),
		layout.Rigid(func(gtx C) D {
			txt := pg.Theme.Label(values.TextSize16, values.String(values.StrAddRecipient))
			txt.Color = pg.Theme.Color.Primary
			txt.Font.Weight = font.SemiBold
			return layout.Inset{
				Left: values.MarginPadding8,
			}.Layout(gtx, txt.Layout)
		}),
	)
}

func (pg *Page) notSyncedLayout(gtx C) D {
	// If wallet is not synced, display a message and don't display the sections
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	textSize16 := values.TextSizeTransform(pg.IsMobileView(), values.TextSize16)
	return layout.Center.Layout(gtx, func(gtx C) D {
		widgets := []func(gtx C) D{
			func(gtx C) D {
				warning := pg.Theme.Label(textSize16, values.String(values.StrFunctionUnavailable))
				warning.Color = pg.Theme.Color.Danger
				warning.Alignment = text.Middle
				return warning.Layout(gtx)

			},
			func(gtx C) D {
				if pg.selectedWallet != nil && pg.selectedWallet.IsSyncing() {
					syncInfo := components.NewWalletSyncInfo(pg.Load, pg.selectedWallet, func() {}, func(_ sharedW.Asset) {})
					blockHeightFetched := values.StringF(values.StrBlockHeaderFetchedCount, pg.selectedWallet.GetBestBlock().Height, syncInfo.FetchSyncProgress().HeadersToFetchOrScan)
					text := fmt.Sprintf("%s "+blockHeightFetched, values.String(values.StrBlockHeaderFetched))
					blockInfo := pg.Theme.Label(textSize16, text)
					return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, blockInfo.Layout)
				}

				return layout.Inset{Top: values.MarginPadding16}.Layout(gtx, pg.navigateToSyncBtn.Layout)
			},
		}
		options := components.FlexOptions{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
		}
		return components.FlexLayout(gtx, options, widgets)
	})
}

func (pg *Page) advanceOptionsLayout(gtx C) D {
	marginMinus32 := values.MarginPadding0
	if pg.modalLayout != nil {
		marginMinus32 = values.MarginPaddingMinus32
	}
	if pg.IsMobileView() {
		marginMinus32 = values.MarginPaddingMinus16
	}
	return layout.Inset{Top: marginMinus32}.Layout(gtx, func(gtx C) D {
		return pg.sectionWrapper(gtx, func(gtx C) D {
			collapsibleHeader := func(gtx C) D {
				lbl := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize16), values.String(values.StrAdvancedOptions))
				lbl.Font.Weight = font.SemiBold
				return lbl.Layout(gtx)
			}

			collapsibleBody := func(gtx C) D {
				if pg.selectedWallet.GetAssetType() == libutils.DCRWalletAsset {
					return layout.Inset{
						Top: values.MarginPadding16,
					}.Layout(gtx, func(gtx C) D {
						return pg.contentWrapper(gtx, values.String(values.StrCoinSelection), true, pg.coinSelectionSection)
					})
				}

				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return pg.contentWrapper(gtx, "", false, pg.feeRateSelector.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						return pg.contentWrapper(gtx, values.String(values.StrCoinSelection), true, pg.coinSelectionSection)
					}),
				)
			}
			return pg.advanceOptions.Layout(gtx, collapsibleHeader, collapsibleBody)
		})
	})
}

func (pg *Page) coinSelectionSection(gtx C) D {
	selectedOption := automaticCoinSelection
	sourceAcc := pg.accountDropdown.SelectedAccount()
	if len(pg.selectedUTXOs.selectedUTXOs) > 0 && pg.selectedUTXOs.sourceAccount == sourceAcc {
		selectedOption = manualCoinSelection
	}

	border := widget.Border{
		Color:        pg.Theme.Color.Gray4,
		CornerRadius: values.MarginPadding10,
		Width:        values.MarginPadding2,
	}
	return border.Layout(gtx, func(gtx C) D {
		return pg.Theme.Card().Layout(gtx, func(gtx C) D {
			inset := layout.UniformInset(values.MarginPadding15)
			return inset.Layout(gtx, func(gtx C) D {
				textLabel := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize16), selectedOption)
				textLabel.Font.Weight = font.SemiBold
				return cryptomaterial.LinearLayout{
					Width:       cryptomaterial.WrapContent,
					Height:      cryptomaterial.WrapContent,
					Orientation: layout.Horizontal,
					Alignment:   layout.Middle,
					Clickable:   pg.toCoinSelection,
				}.Layout2(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Rigid(textLabel.Layout),
						layout.Rigid(pg.Theme.NewIcon(pg.Theme.Icons.ChevronRight).Layout20dp),
					)
				})

			})
		})
	})
}

func (pg *Page) balanceSection(gtx C) D {
	return pg.sectionWrapper(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				inset := layout.Inset{
					Bottom: values.MarginPadding16,
					Left:   values.MarginPadding5,
					Right:  values.MarginPadding5,
				}
				return inset.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							totalCostText := pg.totalCost
							if pg.exchangeRate != -1 && pg.usdExchangeSet {
								totalCostText = fmt.Sprintf("%s (%s)", pg.totalCost, pg.totalCostUSD)
							}
							inset := layout.Inset{
								Bottom: values.MarginPadding12,
							}
							return inset.Layout(gtx, func(gtx C) D {
								return pg.contentRow(gtx, values.String(values.StrTotalCost), totalCostText)
							})
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
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return pg.nextButton.Layout(gtx)
			}),
		)
	})
}

func (pg *Page) sectionWrapper(gtx C, body layout.Widget) D {
	margin16 := values.MarginPadding16
	if pg.modalLayout != nil {
		margin16 = values.MarginPadding0
	}
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Vertical,
		Padding:     layout.UniformInset(margin16),
		Background:  pg.Theme.Color.Surface,
		Border: cryptomaterial.Border{
			Radius: cryptomaterial.Radius(8),
		},
	}.Layout2(gtx, body)
}

func (pg *Page) contentWrapper(gtx C, title string, zeroBottomPadding bool, content layout.Widget) D {
	padding := values.MarginPadding16
	if zeroBottomPadding {
		padding = values.MarginPadding0
	}
	return layout.Inset{
		Bottom: padding,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				lbl := pg.Theme.Label(values.TextSizeTransform(pg.IsMobileView(), values.TextSize16), title)
				lbl.Font.Weight = font.SemiBold
				return layout.Inset{
					Bottom: values.MarginPadding4,
				}.Layout(gtx, lbl.Layout)
			}),
			layout.Rigid(content),
		)
	})
}

func (pg *Page) contentRow(gtx C, leftValue, rightValue string) D {
	textSize := values.TextSizeTransform(pg.IsMobileView(), values.TextSize16)
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			lbl := pg.Theme.Label(textSize, leftValue)
			lbl.Color = pg.Theme.Color.GrayText2
			return lbl.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						lbl := pg.Theme.Label(textSize, rightValue)
						lbl.Color = pg.Theme.Color.Text
						lbl.Font.Weight = font.SemiBold
						return lbl.Layout(gtx)
					}),
				)
			})
		}),
	)
}
