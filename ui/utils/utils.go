package utils

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"

	// "gioui.org/layout"
	// "gioui.org/text"
	// "gioui.org/unit"
	"gioui.org/widget"
	// "golang.org/x/image/math/fixed"
	"golang.org/x/text/message"
)

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

func FormatDateOrTime(timestamp int64) string {
	utcTime := time.Unix(timestamp, 0).UTC()
	if time.Now().UTC().Sub(utcTime).Hours() < 168 {
		return utcTime.Weekday().String()
	}

	t := strings.Split(utcTime.Format(time.UnixDate), " ")
	t2 := t[2]
	if t[2] == "" {
		t2 = t[3]
	}
	return fmt.Sprintf("%s %s", t[1], t2)
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

func GetUSDExchangeValue(target interface{}) error {
	url := "https://api.bittrex.com/v3/markets/DCR-USDT/ticker"
	res, err := http.Get(url)
	// TODO: include user agent in req header
	if err != nil {
		return err
	}

	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(target)
	if err != nil {
		return err
	}

	return nil
}

func FormatUSDBalance(p *message.Printer, balance float64) string {
	return p.Sprintf("$%.2f", balance)
}

func CryptoToUSD(exchangeRate, coin float64) float64 {
	return coin * exchangeRate
}

func USDToDCR(exchangeRate, usd float64) float64 {
	return usd / exchangeRate
}

func goToURL(url string) {
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
		log.Error(err)
	}
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

func HandleSubmitEvent(editors ...*widget.Editor) bool {
	var submit bool
	for _, editor := range editors {
		for _, e := range editor.Events() {
			if _, ok := e.(widget.SubmitEvent); ok {
				submit = true
			}
		}
	}
	return submit
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
