package components

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/color"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/decred/dcrd/dcrutil/v4"
)

// done returns whether the context's Done channel was closed due to
// cancellation or exceeded deadline.
func ContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// RetryFunc implements retry policy for processes that needs to be executed
// after initial failure.
func RetryFunc(retryAttempts int, sleepDur time.Duration, funcDesc string, errFunc func() error) (int, error) {
	var err error
	for i := 0; i < retryAttempts; i++ {
		if i > 0 {
			if i > 1 {
				sleepDur *= 2
			}
			log.Errorf("waiting %s to retry function %s after error: %v\n", sleepDur, funcDesc, err)
			time.Sleep(sleepDur)
		}
		err = errFunc()
		if err == nil {
			return i, nil
		}
	}

	return retryAttempts, fmt.Errorf("last error: %s", err)
}

func SeedWordsToHex(seedWords string) (string, error) {
	var seedHex string
	wordList := dcr.PGPWordList()
	var wordIndexes = make(map[string]uint16, len(wordList))
	for i, word := range wordList {
		wordIndexes[strings.ToLower(word)] = uint16(i)
	}

	words := strings.Split(strings.TrimSpace(seedWords), " ")
	seedByte := make([]byte, len(words))
	idx := 0
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		b, ok := wordIndexes[strings.ToLower(w)]
		if !ok {
			return seedHex, fmt.Errorf("word %v is not in the PGP word list", w)
		}
		if int(b%2) != idx%2 {
			return seedHex, fmt.Errorf("word %v is not valid at position %v, "+
				"check for missing words", w, idx)
		}
		seedByte[idx] = byte(b / 2)
		idx++
	}

	seedByte = seedByte[:idx]
	if checksumByte(seedByte[:len(seedByte)-1]) != seedByte[len(seedByte)-1] {
		return seedHex, fmt.Errorf("seed checksum mismatch")
	}
	seedByte = seedByte[:len(seedByte)-1]

	// minSeedBytes is the minimum number of bytes allowed for a seed.
	minSeedBytes := 16
	// maxSeedBytes is the maximum number of bytes allowed for a seed.
	maxSeedBytes := 64
	if len(seedByte) < minSeedBytes || len(seedByte) > maxSeedBytes {
		return seedHex, fmt.Errorf("invalid seed bytes length")
	}

	seedHex = hex.EncodeToString(seedByte)
	return seedHex, nil
}

// checksumByte returns the checksum byte used at the end of the seed mnemonic
// encoding. The "checksum" is the first byte of the double SHA256.
func checksumByte(data []byte) byte {
	intermediateHash := sha256.Sum256(data)
	return sha256.Sum256(intermediateHash[:])[0]
}

func LayoutIconAndText(l *load.Load, gtx C, title string, val string, col color.NRGBA) D {
	return layout.Inset{Right: values.MarginPadding12}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Right: values.MarginPadding5, Top: values.MarginPadding5}.Layout(gtx, func(gtx C) D {
					ic := cryptomaterial.NewIcon(l.Theme.Icons.ImageBrightness1)
					ic.Color = col
					return ic.Layout(gtx, values.MarginPadding8)
				})
			}),
			layout.Rigid(func(gtx C) D {
				txt := l.Theme.Label(values.TextSize14, title)
				txt.Color = l.Theme.Color.GrayText2
				return txt.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				txt := l.Theme.Label(values.TextSize14, val)
				txt.Color = l.Theme.Color.GrayText2
				return txt.Layout(gtx)
			}),
		)
	})
}

func SetWalletLogo(l *load.Load, gtx C, assetType string, size unit.Dp) D {

	switch strings.ToLower(assetType) {
	case utils.DCRWalletAsset.ToStringLower():
		return l.Theme.Icons.DecredSymbol2.LayoutSize(gtx, size)
	case utils.BTCWalletAsset.ToStringLower():
		return l.Theme.Icons.BTC.LayoutSize(gtx, size)
	case utils.LTCWalletAsset.ToStringLower():
		return l.Theme.Icons.LTC.LayoutSize(gtx, size)
	default:
		return l.Theme.Icons.BTC.LayoutSize(gtx, size)
	}
}

func LayoutOrderAmount(l *load.Load, gtx C, assetType string, amount float64) D {
	if strings.ToLower(assetType) == utils.DCRWalletAsset.ToStringLower() {
		convertedAmount, _ := dcrutil.NewAmount(amount)
		return l.Theme.Label(values.TextSize16, convertedAmount.String()).Layout(gtx)
	}
	convertedAmount, _ := btcutil.NewAmount(amount)
	return l.Theme.Label(values.TextSize16, convertedAmount.String()).Layout(gtx)
}
