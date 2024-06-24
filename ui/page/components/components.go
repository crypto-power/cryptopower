// components contain layout code that are shared by multiple pages but aren't widely used enough to be defined as
// widgets

package components

import (
	"errors"
	"fmt"
	"image/color"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ararog/timeago"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	"github.com/crypto-power/cryptopower/libwallet/txhelper"

	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	Uint32Size    = 32 // 32 or 64 ? shifting 32-bit value by 32 bits will always clear it
	MaxInt32      = 1<<(Uint32Size-1) - 1
	WalletsPageID = "Wallets"
	releaseURL    = "https://api.github.com/repos/crypto-power/cryptopower/releases/latest"
)

type (
	C = layout.Context
	D = layout.Dimensions

	TxStatus struct {
		Title string
		Icon  *cryptomaterial.Image

		// tx purchase only
		TicketStatus       string
		Color              color.NRGBA
		ProgressBarColor   color.NRGBA
		ProgressTrackColor color.NRGBA
		Background         color.NRGBA
	}

	// CummulativeWalletsBalance defines total balance for all available wallets.
	CummulativeWalletsBalance struct {
		Total                   sharedW.AssetAmount
		Spendable               sharedW.AssetAmount
		ImmatureReward          sharedW.AssetAmount
		ImmatureStakeGeneration sharedW.AssetAmount
		LockedByTickets         sharedW.AssetAmount
		VotingAuthority         sharedW.AssetAmount
		UnConfirmed             sharedW.AssetAmount
	}

	DexServer struct {
		SavedHosts map[string][]byte
	}

	FlexOptions struct {
		Axis      layout.Axis
		Spacing   layout.Spacing
		Alignment layout.Alignment
		WeightSum float32
	}

	ReleaseResponse struct {
		TagName string `json:"tag_name"`
		URL     string `json:"html_url"`
	}
)

// Container is simply a wrapper for the Inset type. Its purpose is to differentiate the use of an inset as a padding or
// margin, making it easier to visualize the structure of a layout when reading UI code.
type Container struct {
	Padding layout.Inset
}

func (c Container) Layout(gtx C, w layout.Widget) D {
	return c.Padding.Layout(gtx, w)
}

// HorizontalInset creates an inset with the specified amount applied uniformly to both the left and right edges.
// This function is useful for ensuring consistent horizontal padding or margin around elements, without affecting the vertical spacing.
func HorizontalInset(v unit.Dp) layout.Inset {
	return layout.Inset{Right: v, Left: v}
}

// VerticalInset creates an inset with the specified amount applied uniformly to both the top and bottom edges.
// This function is useful for ensuring consistent vetical padding or margin around elements, without affecting the horizontal spacing.
func VerticalInset(v unit.Dp) layout.Inset {
	return layout.Inset{Top: v, Bottom: v}
}

func UniformPadding(gtx C, body layout.Widget) D {
	width := gtx.Constraints.Max.X

	padding := values.MarginPadding24

	if (width - 2*gtx.Dp(padding)) > gtx.Dp(values.AppWidth) {
		paddingValue := float32(width-gtx.Dp(values.AppWidth)) / 4
		padding = unit.Dp(paddingValue)
	}

	return layout.Inset{
		Top:    values.MarginPadding24,
		Right:  padding,
		Bottom: values.MarginPadding24,
		Left:   padding,
	}.Layout(gtx, body)
}

func UniformHorizontalPadding(gtx C, body layout.Widget) D {
	width := gtx.Constraints.Max.X

	padding := values.MarginPadding24

	if (width - 2*gtx.Dp(padding)) > gtx.Dp(values.AppWidth) {
		paddingValue := float32(width-gtx.Dp(values.AppWidth)) / 3
		padding = unit.Dp(paddingValue)
	}

	return layout.Inset{
		Right: padding,
		Left:  padding,
	}.Layout(gtx, body)
}

func UniformMobile(gtx C, isHorizontal, withList bool, body layout.Widget) D {
	insetRight := values.MarginPadding10
	if withList {
		insetRight = values.MarginPadding0
	}

	insetTop := values.MarginPadding24
	if isHorizontal {
		insetTop = values.MarginPadding0
	}

	return layout.Inset{
		Top:   insetTop,
		Right: insetRight,
		Left:  values.MarginPadding10,
	}.Layout(gtx, body)
}

func TransactionTitleIcon(l *load.Load, wal sharedW.Asset, tx *sharedW.Transaction) *TxStatus {
	var txStatus TxStatus

	switch tx.Direction {
	case txhelper.TxDirectionSent:
		txStatus.Title = values.String(values.StrSent)
		txStatus.Icon = l.Theme.Icons.SendIcon
	case txhelper.TxDirectionReceived:
		txStatus.Title = values.String(values.StrReceived)
		txStatus.Icon = l.Theme.Icons.ReceiveIcon
	default:
		txStatus.Title = values.String(values.StrTransferred)
		txStatus.Icon = l.Theme.Icons.Transferred
	}

	// replace icon for staking tx types
	if wal.TxMatchesFilter(tx, libutils.TxFilterStaking) {
		switch tx.Type {
		case txhelper.TxTypeTicketPurchase:
			{
				if wal.TxMatchesFilter(tx, libutils.TxFilterUnmined) {
					txStatus.Title = values.String(values.StrUmined)
					txStatus.Icon = l.Theme.Icons.TicketUnminedIcon
					txStatus.TicketStatus = dcr.TicketStatusUnmined
					txStatus.Color = l.Theme.Color.LightBlue6
					txStatus.Background = l.Theme.Color.LightBlue
				} else if wal.TxMatchesFilter(tx, libutils.TxFilterImmature) {
					txStatus.Title = values.String(values.StrImmature)
					txStatus.Icon = l.Theme.Icons.TicketImmatureIcon
					txStatus.Color = l.Theme.Color.Yellow
					txStatus.TicketStatus = dcr.TicketStatusImmature
					txStatus.ProgressBarColor = l.Theme.Color.OrangeYellow
					txStatus.ProgressTrackColor = l.Theme.Color.Gray6
					txStatus.Background = l.Theme.Color.Yellow
				} else if wal.TxMatchesFilter(tx, libutils.TxFilterLive) {
					txStatus.Title = values.String(values.StrLive)
					txStatus.Icon = l.Theme.Icons.TicketLiveIcon
					txStatus.Color = l.Theme.Color.Success2
					txStatus.TicketStatus = dcr.TicketStatusLive
					txStatus.ProgressBarColor = l.Theme.Color.Success2
					txStatus.ProgressTrackColor = l.Theme.Color.Success2
					txStatus.Background = l.Theme.Color.Success2
				} else if wal.TxMatchesFilter(tx, libutils.TxFilterExpired) {
					txStatus.Title = values.String(values.StrExpired)
					txStatus.Icon = l.Theme.Icons.TicketExpiredIcon
					txStatus.Color = l.Theme.Color.GrayText2
					txStatus.TicketStatus = dcr.TicketStatusExpired
					txStatus.Background = l.Theme.Color.Gray4
				} else {
					ticketSpender, _ := wal.(*dcr.Asset).TicketSpender(tx.Hash)
					if ticketSpender != nil {
						if ticketSpender.Type == txhelper.TxTypeVote {
							txStatus.Title = values.String(values.StrVoted)
							txStatus.Icon = l.Theme.Icons.TicketVotedIcon
							txStatus.Color = l.Theme.Color.Turquoise700
							txStatus.TicketStatus = dcr.TicketStatusVotedOrRevoked
							txStatus.ProgressBarColor = l.Theme.Color.Turquoise300
							txStatus.ProgressTrackColor = l.Theme.Color.Turquoise100
							txStatus.Background = l.Theme.Color.Success2
						} else {
							txStatus.Title = values.String(values.StrRevoked)
							txStatus.Icon = l.Theme.Icons.TicketRevokedIcon
							txStatus.Color = l.Theme.Color.Orange
							txStatus.TicketStatus = dcr.TicketStatusVotedOrRevoked
							txStatus.ProgressBarColor = l.Theme.Color.Danger
							txStatus.ProgressTrackColor = l.Theme.Color.Orange3
							txStatus.Background = l.Theme.Color.Orange2
						}
					}
				}
			}
		case txhelper.TxTypeVote:
			txStatus.Title = values.String(values.StrVote)
			txStatus.Icon = l.Theme.Icons.TicketVotedIcon
			txStatus.Color = l.Theme.Color.Turquoise700
			txStatus.TicketStatus = dcr.TicketStatusVotedOrRevoked
			txStatus.ProgressBarColor = l.Theme.Color.Turquoise300
			txStatus.ProgressTrackColor = l.Theme.Color.Turquoise100
			txStatus.Background = l.Theme.Color.Success2
		default:
			txStatus.Title = values.String(values.StrRevocation)
			txStatus.Icon = l.Theme.Icons.TicketRevokedIcon
			txStatus.Color = l.Theme.Color.Orange
			txStatus.TicketStatus = dcr.TicketStatusVotedOrRevoked
			txStatus.ProgressBarColor = l.Theme.Color.Danger
			txStatus.ProgressTrackColor = l.Theme.Color.Orange3
			txStatus.Background = l.Theme.Color.Orange2
		}
	} else if tx.Type == txhelper.TxTypeMixed {
		txStatus.Title = values.String(values.StrMixed)
		txStatus.Icon = l.Theme.Icons.MixedTx
	}

	return &txStatus
}

// LayoutTransactionRow is a single transaction row on the transactions and overview
// page. It lays out a transaction's direction, balance, status. hideTxAssetInfo
// determines if the transaction should display additional information about the tx
// such as the wallet the tx belong to etc. This is useful on pages where
// the tx is displayed from multi wallets.
func LayoutTransactionRow(gtx C, l *load.Load, wal sharedW.Asset, tx *sharedW.Transaction, hideTxAssetInfo bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	if wal == nil {
		return D{}
	}

	dp16 := values.MarginPaddingTransform(l.IsMobileView(), values.MarginPadding16)
	txStatus := TransactionTitleIcon(l, wal, tx)
	amount := wal.ToAmount(tx.Amount).String()
	assetIcon := CoinImageBySymbol(l, wal.GetAssetType(), wal.IsWatchingOnlyWallet())
	walName := l.Theme.Label(values.TextSize14, wal.GetWalletName())
	grayText := l.Theme.Color.GrayText2
	insetLeft := values.MarginPadding16

	if !hideTxAssetInfo {
		insetLeft = values.MarginPadding8
	}

	return cryptomaterial.LinearLayout{
		Orientation: layout.Horizontal,
		Width:       cryptomaterial.MatchParent,
		Height:      cryptomaterial.WrapContent,
		Alignment:   layout.Middle,
		Padding: layout.Inset{
			Top:    dp16,
			Bottom: values.MarginPadding10,
		},
	}.Layout(gtx,
		layout.Rigid(txStatus.Icon.Layout24dp),
		layout.Rigid(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.WrapContent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Vertical,
				Padding:     layout.Inset{Left: insetLeft},
				Direction:   layout.Center,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if tx.Type == txhelper.TxTypeRegular {
						amount := wal.ToAmount(tx.Amount).String()
						if tx.Direction == txhelper.TxDirectionSent && !strings.Contains(amount, "-") {
							amount = "-" + amount
						}
						return LayoutBalanceCustom(gtx, l, amount, l.ConvertTextSize(values.TextSize18), true)
					}
					return txTitleAndWalletInfoHorizontal(gtx, l, assetIcon, walName, txStatus, hideTxAssetInfo)
				}),
				layout.Rigid(func(gtx C) D {
					if !hideTxAssetInfo && tx.Type == txhelper.TxTypeRegular {
						return walletIconAndName(gtx, assetIcon, walName)
					}
					return cryptomaterial.LinearLayout{
						Width:       cryptomaterial.WrapContent,
						Height:      cryptomaterial.WrapContent,
						Orientation: layout.Horizontal,
						Alignment:   layout.Middle,
						Direction:   layout.W,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							if tx.Type == txhelper.TxTypeRegular {
								return D{}
							}

							if tx.Type == txhelper.TxTypeMixed {
								return txMixedTitle(gtx, l, wal, tx)
							}

							walBalTxt := l.Theme.Label(values.TextSize14, amount)
							walBalTxt.Color = grayText
							return walBalTxt.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							if dcrAsset, ok := wal.(*dcr.Asset); ok {
								if ok, _ := dcrAsset.TicketHasVotedOrRevoked(tx.Hash); ok {
									return layout.Inset{
										Left: values.MarginPadding4,
									}.Layout(gtx, func(gtx C) D {
										ic := cryptomaterial.NewIcon(l.Theme.Icons.DotIcon)
										ic.Color = grayText
										return ic.Layout(gtx, values.MarginPadding6)
									})
								}
							}
							return D{}
						}),
						layout.Rigid(func(gtx C) D {
							var ticketSpender *sharedW.Transaction
							if dcrAsset, ok := wal.(*dcr.Asset); ok {
								ticketSpender, _ = dcrAsset.TicketSpender(tx.Hash)
							}

							if ticketSpender == nil {
								return D{}
							}
							amnt := wal.ToAmount(ticketSpender.VoteReward).ToCoin()
							txt := fmt.Sprintf("%.2f", amnt)
							if amnt > 0 {
								txt = fmt.Sprintf("+%.2f", amnt)
							}
							return layout.Inset{Left: values.MarginPadding4}.Layout(gtx, l.Theme.Label(values.TextSize14, txt).Layout)
						}),
					)
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			txSize := l.ConvertTextSize(values.TextSize16)
			status := l.Theme.Label(l.ConvertTextSize(txSize), values.String(values.StrUnknown))
			var dateTimeLbl cryptomaterial.Label
			txConfirmations := TxConfirmations(wal, tx)
			reqConf := wal.RequiredConfirmations()
			if txConfirmations < 1 {
				status.Text = values.String(values.StrUnconfirmedTx)
				status.Color = l.Theme.Color.GrayText1
			} else if txConfirmations >= reqConf {
				status.Color = grayText
				status.Text = values.String(values.StrComplete)
				date := time.Unix(tx.Timestamp, 0).Format("2006-01-02")
				timeSplit := time.Unix(tx.Timestamp, 0).Format("03:04 PM")
				dateTimeLbl = l.Theme.Label(l.ConvertTextSize(txSize), fmt.Sprintf("%v at %v", date, timeSplit))
				dateTimeLbl.Color = grayText
			} else {
				status = l.Theme.Label(txSize, values.StringF(values.StrTxStatusPending, txConfirmations, reqConf))
				status.Color = l.Theme.Color.GrayText1
			}

			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						statusIcon := l.Theme.Icons.ConfirmIcon
						if TxConfirmations(wal, tx) < wal.RequiredConfirmations() {
							statusIcon = l.Theme.Icons.PendingIcon
						}
						isStaking := false
						tx := tx
						if wal.TxMatchesFilter(tx, libutils.TxFilterStaking) {
							isStaking = true
						}
						isShowDateTime := false
						if status.Text == values.String(values.StrComplete) {
							isShowDateTime = true
						}
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										if !isStaking {
											return status.Layout(gtx)
										}

										if !hideTxAssetInfo {
											return txStakingStatus(gtx, l, wal, tx)
										}

										title := values.String(values.StrRevoke)
										if tx.Type == txhelper.TxTypeVote {
											title = values.String(values.StrVote)
										}

										lbl := l.Theme.Label(l.ConvertTextSize(values.TextSize16), fmt.Sprintf("%dd to %s", tx.DaysToVoteOrRevoke, title))
										lbl.Color = grayText
										return lbl.Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										return layout.Inset{Left: values.MarginPadding7}.Layout(gtx, statusIcon.Layout12dp)
									}),
								)
							}),
							layout.Rigid(func(gtx C) D {
								if !isShowDateTime {
									return D{}
								}
								return dateTimeLbl.Layout(gtx)
							}),
						)
					}),
				)
			})
		}),
	)

}

func walletIconAndName(gtx C, icon *cryptomaterial.Image, name cryptomaterial.Label) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(icon.Layout12dp),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = gtx.Constraints.Max.X / 2
			name.MaxLines = 1
			return layout.Inset{Left: values.MarginPadding4}.Layout(gtx, name.Layout)
		}),
	)
}

func txTitleAndWalletInfoHorizontal(gtx C, l *load.Load, assetIcon *cryptomaterial.Image, walName cryptomaterial.Label, txStatus *TxStatus, hideTxAssetInfo bool) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Direction:   layout.W,
		Alignment:   layout.Baseline,
	}.Layout(gtx,
		layout.Rigid(l.Theme.Label(l.ConvertTextSize(values.TextSize18), txStatus.Title).Layout),
		layout.Rigid(func(gtx C) D {
			if hideTxAssetInfo {
				return D{}
			}
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Inset{Left: values.MarginPadding4}.Layout(gtx, func(gtx C) D {
					return walletIconAndName(gtx, assetIcon, walName)
				})
			})
		}),
	)
}

func txStakingStatus(gtx C, l *load.Load, wal sharedW.Asset, tx *sharedW.Transaction) D {
	durationPrefix := values.String(values.StrVoted)
	if tx.Type == txhelper.TxTypeTicketPurchase {
		if wal.TxMatchesFilter(tx, libutils.TxFilterUnmined) {
			durationPrefix = values.String(values.StrUmined)
		} else if wal.TxMatchesFilter(tx, libutils.TxFilterImmature) {
			durationPrefix = values.String(values.StrImmature)
		} else if wal.TxMatchesFilter(tx, libutils.TxFilterLive) {
			durationPrefix = values.String(values.StrLive)
		} else if wal.TxMatchesFilter(tx, libutils.TxFilterExpired) {
			durationPrefix = values.String(values.StrExpired)
		}
	} else if tx.Type == txhelper.TxTypeRevocation {
		durationPrefix = values.String(values.StrRevoked)
	}

	durationTxt := TimeAgo(tx.Timestamp)
	durationTxt = fmt.Sprintf("%s %s", durationPrefix, durationTxt)
	lbl := l.Theme.Label(values.TextSize14, durationTxt)
	lbl.Color = l.Theme.Color.GrayText2
	return lbl.Layout(gtx)
}

func txMixedTitle(gtx C, l *load.Load, wal sharedW.Asset, tx *sharedW.Transaction) D {
	return cryptomaterial.LinearLayout{
		Width:       cryptomaterial.WrapContent,
		Height:      cryptomaterial.WrapContent,
		Orientation: layout.Horizontal,
		Alignment:   layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			// mix denomination
			mixedDenom := wal.ToAmount(tx.MixDenomination).String()
			txt := l.Theme.Label(values.TextSize14, mixedDenom)
			txt.Color = l.Theme.Color.GrayText2
			return txt.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			// Mixed outputs count
			if tx.MixCount > 1 {
				label := l.Theme.Label(values.TextSize14, fmt.Sprintf("x%d", tx.MixCount))
				label.Color = l.Theme.Color.GrayText2
				return layout.Inset{Left: values.MarginPadding4}.Layout(gtx, label.Layout)
			}
			return D{}
		}),
	)
}

func TxConfirmations(wallet sharedW.Asset, transaction *sharedW.Transaction) int32 {
	if transaction.BlockHeight != -1 {
		return (wallet.GetBestBlockHeight() - transaction.BlockHeight) + 1
	}

	return 0
}

func FormatDateOrTime(timestamp int64) string {
	utcTime := time.Unix(timestamp, 0).UTC()
	currentTime := time.Now().UTC()

	if strconv.Itoa(currentTime.Year()) == strconv.Itoa(utcTime.Year()) && currentTime.Month().String() == utcTime.Month().String() {
		if strconv.Itoa(currentTime.Day()) == strconv.Itoa(utcTime.Day()) {
			if strconv.Itoa(currentTime.Hour()) == strconv.Itoa(utcTime.Hour()) {
				return TimeAgo(timestamp)
			}

			return TimeAgo(timestamp)
		} else if currentTime.Day()-1 == utcTime.Day() {
			yesterday := values.String(values.StrYesterday)
			return yesterday
		}
	}

	t := strings.Split(utcTime.Format(time.UnixDate), " ")
	t2 := t[2]
	year := strconv.Itoa(utcTime.Year())
	if t[2] == "" {
		t2 = t[3]
	}
	return fmt.Sprintf("%s %s, %s", t[1], t2, year)
}

// EndToEndRow layouts out its content on both ends of its horizontal layout.
func EndToEndRow(gtx C, leftWidget, rightWidget func(C) D) D {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(leftWidget),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, rightWidget)
		}),
	)
}

func TimeAgo(timestamp int64) string {
	timeAgo, _ := timeago.TimeAgoWithTime(time.Now(), time.Unix(timestamp, 0))
	return timeAgo
}

func TruncateString(str string, num int) string {
	bnoden := str
	if len(str) > num {
		if num > 3 {
			num -= 3
		}
		bnoden = str[0:num] + "..."
	}
	return bnoden
}

func GoToURL(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Println(err.Error())
	}
}

func TimeFormat(secs int, long bool) string {
	var val string
	if secs > 86399 {
		val = "d"
		if long {
			val = " " + values.String(values.StrDays)
		}
		days := secs / 86400
		return fmt.Sprintf("%d%s", days, val)
	} else if secs > 3599 {
		val = "h"
		if long {
			val = " " + values.String(values.StrHours)
		}
		hours := secs / 3600
		return fmt.Sprintf("%d%s", hours, val)
	} else if secs > 59 {
		val = "s"
		if long {
			val = " " + values.String(values.StrMinutes)
		}
		mins := secs / 60
		return fmt.Sprintf("%d%s", mins, val)
	}

	val = "s"
	if long {
		val = " " + values.String(values.StrSeconds)
	}
	return fmt.Sprintf("%d %s", secs, val)
}

// TxPageDropDownFields returns the fields for the required drop down with the
// transactions view page. Since maps access of items order is always random
// an array of keys is provided to guarantee the dropdown order will always be
// maintained.
func TxPageDropDownFields(wType libutils.AssetType, tabIndex int) (mapInfo map[string]int32, keysInfo []string) {
	switch {
	case wType == libutils.BTCWalletAsset && tabIndex == 0:
		// BTC Transactions Activities dropdown fields.
		mapInfo = map[string]int32{
			values.String(values.StrAll):      libutils.TxFilterAll,
			values.String(values.StrSent):     libutils.TxFilterSent,
			values.String(values.StrReceived): libutils.TxFilterReceived,
		}
		keysInfo = []string{
			values.String(values.StrAll),
			values.String(values.StrSent),
			values.String(values.StrReceived),
		}
	case wType == libutils.LTCWalletAsset && tabIndex == 0:
		// LTC Transactions Activities dropdown fields.
		mapInfo = map[string]int32{
			values.String(values.StrAll):      libutils.TxFilterAll,
			values.String(values.StrSent):     libutils.TxFilterSent,
			values.String(values.StrReceived): libutils.TxFilterReceived,
		}
		keysInfo = []string{
			values.String(values.StrAll),
			values.String(values.StrSent),
			values.String(values.StrReceived),
		}
	case wType == libutils.DCRWalletAsset && tabIndex == 0:
		// DCR Transactions Activities dropdown fields.
		mapInfo = map[string]int32{
			values.String(values.StrAll):         libutils.TxFilterAll,
			values.String(values.StrSent):        libutils.TxFilterSent,
			values.String(values.StrReceived):    libutils.TxFilterReceived,
			values.String(values.StrTransferred): libutils.TxFilterTransferred,
			values.String(values.StrMixed):       libutils.TxFilterMixed,
		}
		keysInfo = []string{
			values.String(values.StrAll),
			values.String(values.StrSent),
			values.String(values.StrReceived),
			values.String(values.StrTransferred),
			values.String(values.StrMixed),
		}
	case wType == libutils.DCRWalletAsset && tabIndex == 1:
		// DCR staking Activities dropdown fields.
		mapInfo = map[string]int32{
			values.String(values.StrAll):        libutils.TxFilterStaking,
			values.String(values.StrVote):       libutils.TxFilterVoted,
			values.String(values.StrRevocation): libutils.TxFilterRevoked,
		}
		keysInfo = []string{
			values.String(values.StrAll),
			values.String(values.StrVote),
			values.String(values.StrRevocation),
		}
	}
	return
}

// CoinImageBySymbol returns image widget for supported asset coins.
func CoinImageBySymbol(l *load.Load, assetType libutils.AssetType, isWatchOnly bool) *cryptomaterial.Image {
	switch assetType.ToStringLower() {
	case libutils.BTCWalletAsset.ToStringLower():
		if isWatchOnly {
			return l.Theme.Icons.BtcWatchOnly
		}
		return l.Theme.Icons.BTC
	case libutils.DCRWalletAsset.ToStringLower():
		if isWatchOnly {
			return l.Theme.Icons.DcrWatchOnly
		}
		return l.Theme.Icons.DCR
	case libutils.LTCWalletAsset.ToStringLower():
		if isWatchOnly {
			return l.Theme.Icons.LtcWatchOnly
		}
		return l.Theme.Icons.LTC
	}
	return nil
}

// GetTicketPurchaseAccount returns the validly set ticket purchase account if it exists.
func GetTicketPurchaseAccount(selectedWallet *dcr.Asset) (acct *sharedW.Account, err error) {
	tbConfig := selectedWallet.AutoTicketsBuyerConfig()

	isPurchaseAccountSet := tbConfig.PurchaseAccount != -1
	isMixerAccountSet := tbConfig.PurchaseAccount == selectedWallet.MixedAccountNumber()
	isSpendUnmixedAllowed := selectedWallet.ReadBoolConfigValueForKey(sharedW.SpendUnmixedFundsKey, false)
	isAccountMixerConfigSet := selectedWallet.ReadBoolConfigValueForKey(sharedW.AccountMixerConfigSet, false)

	if isPurchaseAccountSet {
		acct, err = selectedWallet.GetAccount(tbConfig.PurchaseAccount)

		if isAccountMixerConfigSet && !isSpendUnmixedAllowed && isMixerAccountSet && err == nil {
			// Mixer account is set and spending from unmixed account is blocked.
			return
		} else if isSpendUnmixedAllowed && err == nil {
			// Spending from unmixed account is allowed. Choose the set account whether its mixed or not.
			return
		}
		// invalid account found. Set it to nil
		acct = nil
	}
	return
}

func CalculateMixedAccountBalance(selectedWallet *dcr.Asset) (*CummulativeWalletsBalance, error) {
	if selectedWallet == nil {
		return nil, errors.New("mixed account only supported by DCR asset")
	}

	// ignore the returned because an invalid purchase account was set previously.
	// Proceed to go and select a valid account if one wasn't provided.
	account, _ := GetTicketPurchaseAccount(selectedWallet)

	var err error
	if account == nil {
		// A valid purchase account hasn't been set. Use default mixed account.
		account, err = selectedWallet.GetAccount(selectedWallet.MixedAccountNumber())
		if err != nil {
			return nil, err
		}
	}

	return &CummulativeWalletsBalance{
		Total:                   account.Balance.Total,
		Spendable:               account.Balance.Spendable,
		ImmatureReward:          account.Balance.ImmatureReward,
		ImmatureStakeGeneration: account.Balance.ImmatureStakeGeneration,
		LockedByTickets:         account.Balance.LockedByTickets,
		VotingAuthority:         account.Balance.VotingAuthority,
		UnConfirmed:             account.Balance.UnConfirmed,
	}, nil
}

func CalculateTotalWalletsBalance(wallet sharedW.Asset) (*CummulativeWalletsBalance, error) {
	var totalBalance, spandableBalance, immatureReward, votingAuthority,
		immatureStakeGeneration, lockedByTickets, unConfirmed int64

	accountsResult, err := wallet.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	for _, account := range accountsResult.Accounts {
		totalBalance += account.Balance.Total.ToInt()
		spandableBalance += account.Balance.Spendable.ToInt()
		immatureReward += account.Balance.ImmatureReward.ToInt()

		if wallet.GetAssetType() == libutils.DCRWalletAsset {
			// Fields required only by DCR
			immatureStakeGeneration += account.Balance.ImmatureStakeGeneration.ToInt()
			lockedByTickets += account.Balance.LockedByTickets.ToInt()
			votingAuthority += account.Balance.VotingAuthority.ToInt()
			unConfirmed += account.Balance.UnConfirmed.ToInt()
		}
	}

	cumm := &CummulativeWalletsBalance{
		Total:                   wallet.ToAmount(totalBalance),
		Spendable:               wallet.ToAmount(spandableBalance),
		ImmatureReward:          wallet.ToAmount(immatureReward),
		ImmatureStakeGeneration: wallet.ToAmount(immatureStakeGeneration),
		LockedByTickets:         wallet.ToAmount(lockedByTickets),
		VotingAuthority:         wallet.ToAmount(votingAuthority),
		UnConfirmed:             wallet.ToAmount(unConfirmed),
	}

	return cumm, nil
}

// SecondsToDays takes time in seconds and returns its string equivalent in the format ddhhmm.
func SecondsToDays(totalTimeLeft int64) string {
	q, r := divMod(totalTimeLeft, 24*60*60)
	timeLeft := time.Duration(r) * time.Second
	if q > 0 {
		return fmt.Sprintf("%dd%s", q, timeLeft.String())
	}
	return timeLeft.String()
}

// divMod divides a numerator by a denominator and returns its quotient and remainder.
func divMod(numerator, denominator int64) (quotient, remainder int64) {
	quotient = numerator / denominator // integer division, decimals are truncated
	remainder = numerator % denominator
	return
}

func BrowserURLWidget(gtx C, l *load.Load, url string, copyRedirect *cryptomaterial.Clickable) D {
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx C) D {
			border := widget.Border{Color: l.Theme.Color.Gray4, CornerRadius: values.MarginPadding10, Width: values.MarginPadding2}
			wrapper := l.Theme.Card()
			wrapper.Color = l.Theme.Color.Gray4
			return border.Layout(gtx, func(gtx C) D {
				return wrapper.Layout(gtx, func(gtx C) D {
					return layout.UniformInset(values.MarginPadding10).Layout(gtx, func(gtx C) D {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(0.9, l.Theme.Body1(url).Layout),
							layout.Flexed(0.1, func(gtx C) D {
								return layout.E.Layout(gtx, func(gtx C) D {
									if copyRedirect.Clicked(gtx) {
										gtx.Execute(clipboard.WriteCmd{Data: io.NopCloser(strings.NewReader(url))})
										l.Toast.Notify(values.String(values.StrCopied))
									}
									return copyRedirect.Layout(gtx, l.Theme.Icons.CopyIcon.Layout24dp)
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
				label := l.Theme.Body2(values.String(values.StrWebURL))
				label.Color = l.Theme.Color.GrayText2
				return label.Layout(gtx)
			})
		}),
	)
}

// DisablePageWithOverlay disables the provided page by highlighting a message why
// the page is disabled and adding a background color overlay that blocks any
// page event being triggered.
func DisablePageWithOverlay(l *load.Load, currentPage app.Page, gtx C, title, subtitle string, actionButton *cryptomaterial.Button) D {
	titleLayout := func(gtx C) D {
		if title == "" {
			return D{}
		}

		lbl := l.Theme.Label(values.TextSize20, title)
		lbl.Font.Weight = font.SemiBold
		lbl.Color = l.Theme.Color.PageNavText
		lbl.Alignment = text.Middle
		return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx.Disabled(), lbl.Layout)
	}

	subtitleLayout := func(gtx C) D {
		if subtitle == "" {
			return D{}
		}

		subTitleLbl := l.Theme.Label(values.TextSize14, subtitle)
		subTitleLbl.Font.Weight = font.SemiBold
		subTitleLbl.Color = l.Theme.Color.GrayText2
		return layout.Inset{Bottom: values.MarginPadding20}.Layout(gtx.Disabled(), subTitleLbl.Layout)
	}

	return cryptomaterial.DisableLayout(currentPage, gtx, titleLayout, subtitleLayout, 220, l.Theme.Color.Gray3, actionButton)
}

func WalletHighlightLabel(theme *cryptomaterial.Theme, gtx C, textSize unit.Sp, content string) D {
	indexLabel := theme.Label(textSize, content)
	indexLabel.Color = theme.Color.DeepBlueOrigin
	indexLabel.Font.Weight = font.Medium
	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.WrapContent,
		Height:     gtx.Dp(values.MarginPadding22),
		Direction:  layout.Center,
		Background: theme.Color.Gray8,
		Padding: layout.Inset{
			Left:  values.MarginPadding8,
			Right: values.MarginPadding8},
		Margin: layout.Inset{Right: values.MarginPadding8},
		Border: cryptomaterial.Border{Radius: cryptomaterial.Radius(9), Color: theme.Color.Gray3, Width: values.MarginPadding1},
	}.Layout2(gtx, indexLabel.Layout)
}

// InputsNotEmpty checks if all the provided editors have non-empty text.
func InputsNotEmpty(editors ...*widget.Editor) bool {
	for _, e := range editors {
		if e.Text() == "" {
			return false
		}
	}
	return true
}

func FlexLayout(gtx C, options FlexOptions, widgets []func(gtx C) D) D {
	flexChildren := make([]layout.FlexChild, len(widgets))
	for i, widget := range widgets {
		flexChildren[i] = layout.Rigid(widget)
	}

	return layout.Flex{
		Axis:      options.Axis,
		Alignment: options.Alignment,
	}.Layout(gtx, flexChildren...)
}

// IconButton creates the display for an icon button. The default icon and text
// color is Theme.Color.Primary.
func IconButton(icon *widget.Icon, txt string, inset layout.Inset, th *cryptomaterial.Theme, clickable *cryptomaterial.Clickable) func(gtx C) D {
	return func(gtx C) D {
		return inset.Layout(gtx, func(gtx C) D {
			color := th.Color.Primary
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.MatchParent,
				Height:      cryptomaterial.WrapContent,
				Orientation: layout.Horizontal,
				Alignment:   layout.Start,
				Clickable:   clickable,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return icon.Layout(gtx, color)
				}),
				layout.Rigid(layout.Spacer{Width: values.MarginPadding5}.Layout),
				layout.Rigid(func(gtx C) D {
					label := th.Label(values.TextSize16, txt)
					label.Color = color
					label.Font.Weight = font.SemiBold
					return label.Layout(gtx)
				}),
			)
		})
	}
}

// ConditionalFlexedRigidLayout decides whether to use layout.Rigid or layout.Flexed
func ConditionalFlexedRigidLayout(flexWeight float32, isMobileView bool, content layout.Widget) layout.FlexChild {
	if isMobileView {
		return layout.Rigid(content)
	}
	return layout.Flexed(flexWeight, content)
}

// GetServerIcon returns the icon for the provided server name.
func GetServerIcon(theme *cryptomaterial.Theme, serverName string) *cryptomaterial.Image {
	switch serverName {
	case instantswap.Changelly.ToString():
		return theme.Icons.ChangellyIcon
	case instantswap.ChangeNow.ToString():
		return theme.Icons.ChangeNowIcon
	case instantswap.CoinSwitch.ToString():
		return theme.Icons.CoinSwitchIcon
	case instantswap.FlypMe.ToString():
		return theme.Icons.FlypMeIcon
	case instantswap.GoDex.ToString():
		return theme.Icons.GodexIcon
	case instantswap.SimpleSwap.ToString():
		return theme.Icons.SimpleSwapIcon
	case instantswap.SwapZone.ToString():
		return theme.Icons.SwapzoneIcon
	case instantswap.Trocador.ToString():
		return theme.Icons.TrocadorIcon

	default:
		return theme.Icons.AddExchange
	}
}

// CheckForUpdate checks if a new version of the app is available
// by comparing the current version with the latest release version
// available on GitHub.
func CheckForUpdate(l *load.Load) *ReleaseResponse {
	req := &libutils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: releaseURL,
	}

	releaseResponse := new(ReleaseResponse)
	if _, err := libutils.HTTPRequest(req, &releaseResponse); err != nil {
		log.Error("checking for update failed:", err)
		return nil
	}

	isUpdateAvaialble := compareVersions(releaseResponse.TagName, l.Version())
	if !isUpdateAvaialble {
		return nil
	}

	return releaseResponse
}

// compareVersions compares two semantic version strings and returns
// true if version1 is greater than version2, otherwise returns false.
func compareVersions(version1, version2 string) bool {
	// Remove the 'v' prefix, if it exists
	v1 := strings.TrimPrefix(version1, "v")
	v2 := strings.TrimPrefix(version2, "v")

	// Split the version strings into their components
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Iterate through the version parts and compare
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		// Convert string parts to integers for comparison
		num1, err1 := strconv.Atoi(parts1[i])
		num2, err2 := strconv.Atoi(parts2[i])

		// Handle potential errors in conversion
		if err1 != nil || err2 != nil {
			log.Error("Error converting version numbers:", err1, err2)
			return false
		}

		// Compare the version numbers
		if num1 > num2 {
			return true // version1 is greater
		} else if num1 < num2 {
			return false // version2 is greater
		}
	}

	// If one version has more numbers than the other, compare those
	if len(parts1) > len(parts2) {
		return true
	} else if len(parts1) < len(parts2) {
		return false
	}

	return false // versions are equal
}
