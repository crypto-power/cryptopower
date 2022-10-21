// components contain layout code that are shared by multiple pages but aren't widely used enough to be defined as
// widgets

package components

import (
	"fmt"
	"image/color"
	"math"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ararog/timeago"
	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/libwallet/assets/dcr"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/values"
)

const (
	Uint32Size    = 32 // 32 or 64 ? shifting 32-bit value by 32 bits will always clear it
	MaxInt32      = 1<<(Uint32Size-1) - 1
	WalletsPageID = "Wallets"
)

var MaxWidth = unit.Dp(800)

type (
	C              = layout.Context
	D              = layout.Dimensions
	TransactionRow struct {
		Transaction sharedW.Transaction
		Index       int
	}

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
)

// Container is simply a wrapper for the Inset type. Its purpose is to differentiate the use of an inset as a padding or
// margin, making it easier to visualize the structure of a layout when reading UI code.
type Container struct {
	Padding layout.Inset
}

func (c Container) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return c.Padding.Layout(gtx, w)
}

func UniformPadding(gtx layout.Context, body layout.Widget) layout.Dimensions {
	width := gtx.Constraints.Max.X

	padding := values.MarginPadding24

	if (width - 2*gtx.Dp(padding)) > gtx.Dp(MaxWidth) {
		paddingValue := float32(width-gtx.Dp(MaxWidth)) / 2
		padding = unit.Dp(paddingValue)
	}

	return layout.Inset{
		Top:    values.MarginPadding24,
		Right:  padding,
		Bottom: values.MarginPadding24,
		Left:   padding,
	}.Layout(gtx, body)
}

func UniformHorizontalPadding(gtx layout.Context, body layout.Widget) layout.Dimensions {
	width := gtx.Constraints.Max.X

	padding := values.MarginPadding24

	if (width - 2*gtx.Dp(padding)) > gtx.Dp(MaxWidth) {
		paddingValue := float32(width-gtx.Dp(MaxWidth)) / 3
		padding = unit.Dp(paddingValue)
	}

	return layout.Inset{
		Right: padding,
		Left:  padding,
	}.Layout(gtx, body)
}

func UniformMobile(gtx layout.Context, isHorizontal, withList bool, body layout.Widget) layout.Dimensions {
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
	case dcr.TxDirectionSent:
		txStatus.Title = values.String(values.StrSent)
		txStatus.Icon = l.Theme.Icons.SendIcon
	case dcr.TxDirectionReceived:
		txStatus.Title = values.String(values.StrReceived)
		txStatus.Icon = l.Theme.Icons.ReceiveIcon
	default:
		txStatus.Title = values.String(values.StrTransferred)
		txStatus.Icon = l.Theme.Icons.Transferred
	}

	// replace icon for staking tx types
	if wal.TxMatchesFilter(tx, dcr.TxFilterStaking) {
		switch tx.Type {
		case dcr.TxTypeTicketPurchase:
			{
				if wal.TxMatchesFilter(tx, dcr.TxFilterUnmined) {
					txStatus.Title = values.String(values.StrUmined)
					txStatus.Icon = l.Theme.Icons.TicketUnminedIcon
					txStatus.TicketStatus = dcr.TicketStatusUnmined
					txStatus.Color = l.Theme.Color.LightBlue6
					txStatus.Background = l.Theme.Color.LightBlue
				} else if wal.TxMatchesFilter(tx, dcr.TxFilterImmature) {
					txStatus.Title = values.String(values.StrImmature)
					txStatus.Icon = l.Theme.Icons.TicketImmatureIcon
					txStatus.Color = l.Theme.Color.Yellow
					txStatus.TicketStatus = dcr.TicketStatusImmature
					txStatus.ProgressBarColor = l.Theme.Color.OrangeYellow
					txStatus.ProgressTrackColor = l.Theme.Color.Gray6
					txStatus.Background = l.Theme.Color.Yellow
				} else if wal.TxMatchesFilter(tx, dcr.TxFilterLive) {
					txStatus.Title = values.String(values.StrLive)
					txStatus.Icon = l.Theme.Icons.TicketLiveIcon
					txStatus.Color = l.Theme.Color.Success2
					txStatus.TicketStatus = dcr.TicketStatusLive
					txStatus.ProgressBarColor = l.Theme.Color.Success2
					txStatus.ProgressTrackColor = l.Theme.Color.Success2
					txStatus.Background = l.Theme.Color.Success2
				} else if wal.TxMatchesFilter(tx, dcr.TxFilterExpired) {
					txStatus.Title = values.String(values.StrExpired)
					txStatus.Icon = l.Theme.Icons.TicketExpiredIcon
					txStatus.Color = l.Theme.Color.GrayText2
					txStatus.TicketStatus = dcr.TicketStatusExpired
					txStatus.Background = l.Theme.Color.Gray4
				} else {
					ticketSpender, _ := wal.(*dcr.DCRAsset).TicketSpender(tx.Hash)
					if ticketSpender != nil {
						if ticketSpender.Type == dcr.TxTypeVote {
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
		case dcr.TxTypeVote:
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
	} else if tx.Type == dcr.TxTypeMixed {
		txStatus.Title = values.String(values.StrMixed)
		txStatus.Icon = l.Theme.Icons.MixedTx
	}

	return &txStatus
}

// not used anywhere in the code TODO- deprecate
func durationAgo(timestamp int64) string {
	hrsPerYr := 8760.0  // There are 8760 hrs in a year.
	hrsPerMnth := 730.0 // There are 730 hrs in a month.
	hrsPerWk := 168.0   // There are 168 hrs in a Week.
	HrsPerday := 24.0   // There are 24 hrs in a Day.

	getStrDuration := func(opt1, opt2 string, d float64) string {
		if d == 1 {
			return values.StringF(opt1, d)
		} else if d > 1 {
			return values.StringF(opt2, d)
		} else {
			return ""
		}
	}

	d := time.Now().UTC().Sub(time.Unix(timestamp, 0).UTC())
	hrs := d.Hours()

	switch {
	case hrs >= hrsPerYr:
		strDuration := getStrDuration(values.StrYearAgo, values.StrYearsAgo, math.Round(hrs/hrsPerYr))
		if strDuration != "" {
			return strDuration
		}
		fallthrough

	case hrs >= hrsPerMnth:
		strDuration := getStrDuration(values.StrMonthAgo, values.StrMonthsAgo, math.Round(hrs/hrsPerMnth))
		if strDuration != "" {
			return strDuration
		}
		fallthrough

	case hrs >= hrsPerWk:
		strDuration := getStrDuration(values.StrWeekAgo, values.StrWeeksAgo, math.Round(hrs/hrsPerWk))
		if strDuration != "" {
			return strDuration
		}
		fallthrough

	case hrs >= HrsPerday:
		strDuration := getStrDuration(values.StrDayAgo, values.StrDaysAgo, math.Round(hrs/HrsPerday))
		if strDuration != "" {
			return strDuration
		}
		fallthrough

	default:
		if strDuration := getStrDuration(values.StrHourAgo, values.StrHoursAgo, hrs); strDuration != "" {
			return strDuration
		}

		if strDuration := getStrDuration(values.StrMinuteAgo, values.StrMinutesAgo, d.Minutes()); strDuration != "" {
			return strDuration
		}

		return values.StringF(values.String(values.StrSeconds), d.Seconds())
	}
}

// transactionRow is a single transaction row on the transactions and overview page. It lays out a transaction's
// direction, balance, status.
func LayoutTransactionRow(gtx layout.Context, l *load.Load, row TransactionRow) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X

	wal := l.WL.MultiWallet.WalletWithID(row.Transaction.WalletID)
	txStatus := TransactionTitleIcon(l, wal, &row.Transaction)

	return cryptomaterial.LinearLayout{
		Orientation: layout.Horizontal,
		Width:       cryptomaterial.MatchParent,
		Height:      gtx.Dp(values.MarginPadding56),
		Alignment:   layout.Middle,
		Padding:     layout.Inset{Left: values.MarginPadding16, Right: values.MarginPadding16},
	}.Layout(gtx,
		layout.Rigid(txStatus.Icon.Layout24dp),
		layout.Rigid(func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:       cryptomaterial.WrapContent,
				Height:      cryptomaterial.MatchParent,
				Orientation: layout.Vertical,
				Padding:     layout.Inset{Left: values.MarginPadding16},
				Direction:   layout.Center,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if row.Transaction.Type == dcr.TxTypeRegular {
						amount := dcrutil.Amount(row.Transaction.Amount).String()
						if row.Transaction.Direction == dcr.TxDirectionSent && !strings.Contains(amount, "-") {
							amount = "-" + amount
						}
						return LayoutBalanceSize(gtx, l, amount, values.TextSize18)
					}

					return l.Theme.Label(values.TextSize18, txStatus.Title).Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					if row.Transaction.Type == dcr.TxTypeMixed {

						return cryptomaterial.LinearLayout{
							Width:       cryptomaterial.WrapContent,
							Height:      cryptomaterial.WrapContent,
							Orientation: layout.Horizontal,
							Direction:   layout.W,
							Alignment:   layout.Middle,
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								// mix denomination
								mixedDenom := dcrutil.Amount(row.Transaction.MixDenomination).String()
								txt := l.Theme.Label(values.TextSize12, mixedDenom)
								txt.Color = l.Theme.Color.GrayText2
								return txt.Layout(gtx)
							}),
							layout.Rigid(func(gtx C) D {
								// Mixed outputs count
								if row.Transaction.Type == dcr.TxTypeMixed && row.Transaction.MixCount > 1 {
									label := l.Theme.Label(values.TextSize12, fmt.Sprintf("x%d", row.Transaction.MixCount))
									label.Color = l.Theme.Color.GrayText2
									return layout.Inset{Left: values.MarginPadding4}.Layout(gtx, label.Layout)
								}
								return D{}
							}),
						)
					}
					return D{}
				}),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			status := l.Theme.Label(values.TextSize16, values.String(values.StrPending))
			if TxConfirmations(l, row.Transaction) <= 1 {
				status.Color = l.Theme.Color.GrayText1
			} else {
				status.Color = l.Theme.Color.GrayText2
				date := time.Unix(row.Transaction.Timestamp, 0).Format("Jan 2, 2006")
				timeSplit := time.Unix(row.Transaction.Timestamp, 0).Format("03:04:05 PM")
				status.Text = fmt.Sprintf("%v at %v", date, timeSplit)
			}

			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if row.Transaction.Type == dcr.TxTypeVote || row.Transaction.Type == dcr.TxTypeRevocation {
							title := values.String(values.StrRevoke)
							if row.Transaction.Type == dcr.TxTypeVote {
								title = values.String(values.StrVote)
							}

							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									lbl := l.Theme.Label(values.TextSize16, fmt.Sprintf("%dd to %s", row.Transaction.DaysToVoteOrRevoke, title))
									lbl.Color = l.Theme.Color.GrayText2
									return lbl.Layout(gtx)
								}),
								layout.Rigid(func(gtx C) D {
									return layout.Inset{
										Right: values.MarginPadding5,
										Left:  values.MarginPadding5,
									}.Layout(gtx, func(gtx C) D {
										ic := cryptomaterial.NewIcon(l.Theme.Icons.ImageBrightness1)
										ic.Color = l.Theme.Color.GrayText2
										return ic.Layout(gtx, values.MarginPadding6)
									})
								}),
							)
						}

						return D{}
					}),
					layout.Rigid(status.Layout),
					layout.Rigid(func(gtx C) D {
						statusIcon := l.Theme.Icons.ConfirmIcon
						if TxConfirmations(l, row.Transaction) <= 1 {
							statusIcon = l.Theme.Icons.PendingIcon
						}

						return layout.Inset{
							Left: values.MarginPadding15,
							Top:  values.MarginPadding5,
						}.Layout(gtx, statusIcon.Layout12dp)
					}),
				)
			})
		}),
	)
}

func TxConfirmations(l *load.Load, transaction sharedW.Transaction) int32 {
	if transaction.BlockHeight != -1 {
		return (l.WL.MultiWallet.WalletWithID(transaction.WalletID).GetBestBlockHeight() - transaction.BlockHeight) + 1
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
func EndToEndRow(gtx layout.Context, leftWidget, rightWidget func(C) D) layout.Dimensions {
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

func CreateOrderDropDown(l *load.Load, grp uint, pos uint) *cryptomaterial.DropDown {
	return l.Theme.DropDown([]cryptomaterial.DropDownItem{{Text: values.String(values.StrNewest)},
		{Text: values.String(values.StrOldest)}}, grp, pos)
}

// CoinImageBySymbol returns image widget for supported asset coins.
func CoinImageBySymbol(l *load.Load, coinName string) *cryptomaterial.Image {
	switch strings.ToLower(coinName) {
	case "btc":
		return l.Theme.Icons.BTC
	case "dcr":
		return l.Theme.Icons.DCR
	}
	return nil
}

func CalculateTotalWalletsBalance(l *load.Load) (*CummulativeWalletsBalance, error) {
	var totalBalance, spandableBalance, immatureReward, votingAuthority,
		immatureStakeGeneration, lockedByTickets, unConfirmed int64

	accountsResult, err := l.WL.SelectedWallet.Wallet.GetAccountsRaw()
	if err != nil {
		return nil, err
	}

	for _, account := range accountsResult.Accounts {
		totalBalance += account.Balance.Total.ToInt()
		spandableBalance += account.Balance.Spendable.ToInt()
		immatureReward += account.Balance.ImmatureReward.ToInt()
		immatureStakeGeneration += account.Balance.ImmatureStakeGeneration.ToInt()
		lockedByTickets += account.Balance.LockedByTickets.ToInt()
		votingAuthority += account.Balance.VotingAuthority.ToInt()
		unConfirmed += account.Balance.UnConfirmed.ToInt()
	}

	toAmount := func(v int64) sharedW.AssetAmount {
		return l.WL.SelectedWallet.Wallet.ToAmount(v)
	}

	cumm := &CummulativeWalletsBalance{
		Total:                   toAmount(totalBalance),
		Spendable:               toAmount(spandableBalance),
		ImmatureReward:          toAmount(immatureReward),
		ImmatureStakeGeneration: toAmount(immatureStakeGeneration),
		LockedByTickets:         toAmount(lockedByTickets),
		VotingAuthority:         toAmount(votingAuthority),
		UnConfirmed:             toAmount(unConfirmed),
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
