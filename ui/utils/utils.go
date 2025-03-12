package utils

import (
	"fmt"
	"image"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"decred.org/dcrdex/dex/encode"
	"github.com/crypto-power/cryptopower/libwallet/assets/btc"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/notify"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/shirou/gopsutil/mem"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"golang.org/x/text/message"
)

// ZeroBytes use for clearing a password or seed byte slice.
var ZeroBytes = encode.ClearBytes
var NotificationList = make([]notify.Notification, 0)
var Mu sync.Mutex

// the length of name should be 20 characters
func ValidateLengthName(name string) bool {
	trimName := strings.TrimSpace(name)
	return len(trimName) > 0 && len(trimName) <= 20
}

func ValidateHost(host string) bool {
	address := strings.Trim(host, " ")

	if net.ParseIP(address) != nil {
		return true
	}

	_, err := url.ParseRequestURI(address)
	return err == nil

}

func EditorsNotEmpty(editors ...*widget.Editor) bool {
	for _, e := range editors {
		if len(strings.TrimSpace(e.Text())) == 0 {
			return false
		}
	}
	return true
}

// breakBalance takes the balance string and returns it in two slices
func BreakBalance(p *message.Printer, balance string) (b1, b2 string) {
	var isDecimal = true
	balanceParts := strings.Split(balance, ".")
	if len(balanceParts) == 1 {
		isDecimal = false
		balanceParts = strings.Split(balance, " ")
	}

	b1 = balanceParts[0]
	if bal, err := strconv.Atoi(b1); err == nil {
		b1 = p.Sprint(bal)
	}

	b2 = balanceParts[1]
	if isDecimal {
		b1 = b1 + "." + b2[:2]
		b2 = b2[2:]
		return
	}
	b2 = " " + b2
	return
}

func FormatAsUSDString(p *message.Printer, usdAmt float64) string {
	return p.Sprintf("$%.2f", usdAmt)
}

func CryptoToUSD(exchangeRate, coin float64) float64 {
	return coin * exchangeRate
}

func USDToDCR(exchangeRate, usd float64) float64 {
	return usd / exchangeRate
}

func ComputePasswordStrength(pb *cryptomaterial.ProgressBarStyle, th *cryptomaterial.Theme, editors ...*widget.Editor) {
	password := editors[0]
	strength := utils.ShannonEntropy(password.Text()) / 4.0
	pb.Progress = float32(strength)

	//set progress bar color
	switch {
	case pb.Progress <= 0.30:
		pb.Color = th.Color.Danger
	case pb.Progress > 0.30 && pb.Progress <= 0.60:
		pb.Color = th.Color.Yellow
	case pb.Progress > 0.50:
		pb.Color = th.Color.Success
	}
}

// Create new normal notifier (no icon)
func CreateNewNotifier() (notifier notify.Notifier, err error) {
	notifier, err = notify.NewNotifier()
	return
}

// PushAppNotifications: default app title, default app icon
func PushAppNotifications(content string) error {
	return PushNotifications(values.String(values.StrAppWallet), content)
}

// PushAppNotificationsWithIcon: default app title, with customize icon
func PushAppNotificationsWithIcon(content, icon string) error {
	return PushNotificationsWithIcon(values.String(values.StrAppWallet), content, icon)
}

// Push notification with icon (For windows)
func PushNotificationsWithIcon(title, content, iconPath string) error {
	notifier, err := CreateNewNotifierWithIcon(iconPath)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	go notifier.CreateNotification(title, content)
	return nil
}

// Push notification
func PushNotifications(title, content string) error {
	// use icon of cryptopower
	appIcon, err := GetAssetFilePath("ui/assets/decredicons/appicon.png")
	if err != nil {
		log.Error(err.Error())
		return err
	}
	notifier, err := CreateNewNotifierWithIcon(appIcon)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	go notifier.CreateNotification(title, content)
	return nil
}

// Create and push transaction notification
func PostTransactionNotification(notification string, assetType libutils.AssetType) {
	// push notification
	walIcon, err := GetWalletNotifyIconPath(libutils.AssetType(assetType))
	if err != nil {
		log.Error(err.Error())
		return
	}
	PushAppNotificationsWithIcon(notification, walIcon)
}

// Get absolute file path from relative path
func GetAssetFilePath(relativePath string) (absoluteFilePath string, err error) {
	absoluteWdPath, err := GetProjectPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(absoluteWdPath, relativePath), nil
}

func GetAbsolutePath() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("error getting executable path: %s", err.Error())
	}

	exSym, err := filepath.EvalSymlinks(ex)
	if err != nil {
		return "", fmt.Errorf("error getting filepath after evaluating sym links")
	}

	return path.Dir(exSym), nil
}

// Get app absolute path
func GetProjectPath() (string, error) {
	projectPath, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return projectPath, nil
}

func SplitSingleString(text string, index int) string {
	first := text[0 : len(text)-index]
	second := text[len(text)-index:]
	return fmt.Sprintf("%s %s", first, second)
}

func SplitXPUB(text string, index1, index2 int) string {
	first := text[0 : len(text)-index1]
	second := text[len(text)-index1 : len(text)-index2]
	third := text[len(text)-index2:]

	return fmt.Sprintf("%s %s %s", first, second, third)
}

func StringNotEmpty(texts ...string) bool {
	for _, t := range texts {
		if strings.TrimSpace(t) == "" {
			return false
		}
	}

	return true
}

func RadiusLayout(gtx layout.Context, radius int, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	defer clip.UniformRRect(image.Rectangle{Max: dims.Size}, radius).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return dims
}

func USDMarketFromAsset(asset utils.AssetType) (values.Market, error) {
	switch asset {
	case utils.DCRWalletAsset:
		return values.DCRUSDTMarket, nil
	case utils.BTCWalletAsset:
		return values.BTCUSDTMarket, nil
	case utils.LTCWalletAsset:
		return values.LTCUSDTMarket, nil
	default:
		return values.UnknownMarket, fmt.Errorf("unsupported asset type: %s", asset)
	}
}

func IsImportedAccount(assetType libutils.AssetType, acc *sharedW.Account) bool {
	switch assetType {
	case libutils.BTCWalletAsset:
		return acc.AccountNumber == btc.ImportedAccountNumber

	case libutils.DCRWalletAsset:
		return acc.Number == dcr.ImportedAccountNumber

	default:
		return false
	}
}

// GetNumberOfRAM returns the total number of RAM available in gigabytes.
func GetNumberOfRAM() (int, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	// Convert bytes to gigabytes
	return int(vmStat.Total / (1024 * 1024 * 1024)), nil
}

// Get the icon path for the asset type used to display the report.
func GetWalletNotifyIconPath(assetType libutils.AssetType) (string, error) {
	var icon string
	switch assetType.ToStringLower() {
	case libutils.BTCWalletAsset.ToStringLower():
		icon = "logo_btc.png"
	case libutils.DCRWalletAsset.ToStringLower():
		icon = "ic_dcr_qr.png"
	case libutils.LTCWalletAsset.ToStringLower():
		icon = "ltc.png"
	default:
		icon = "#"
	}
	walIcon, err := GetAssetFilePath("ui/assets/decredicons/" + icon)
	if err != nil {
		return "", err
	}
	return walIcon, nil
}
