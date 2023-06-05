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

	"github.com/btcsuite/btcd/btcutil"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/ltcsuite/ltcd/ltcutil"
	"gitlab.com/cryptopower/cryptopower/libwallet/assets/dcr"
	libutils "gitlab.com/cryptopower/cryptopower/libwallet/utils"
	"gitlab.com/cryptopower/cryptopower/ui/cryptomaterial"
	"gitlab.com/cryptopower/cryptopower/ui/load"
	"gitlab.com/cryptopower/cryptopower/ui/values"
)

const (
	// MinSeedBytes is the minimum number of bytes allowed for a seed.
	MinSeedBytes = 16

	// MaxSeedBytes is the maximum number of bytes allowed for a seed.
	MaxSeedBytes = 64
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
	wordIndexes := make(map[string]uint16, len(wordList))
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

	if len(seedByte) < MinSeedBytes || len(seedByte) > MaxSeedBytes {
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

func SetWalletLogo(l *load.Load, gtx C, assetType libutils.AssetType, size unit.Dp) D {
	image := CoinImageBySymbol(l, assetType, false)
	if image != nil {
		return image.LayoutSize(gtx, size)
	}
	return D{}
}

func LayoutOrderAmount(l *load.Load, gtx C, assetType string, amount float64) D {
	var convertedAmountStr string

	switch strings.ToLower(assetType) {
	case libutils.DCRWalletAsset.ToStringLower():
		convertedAmount, _ := dcrutil.NewAmount(amount)
		convertedAmountStr = convertedAmount.String()
	case libutils.BTCWalletAsset.ToStringLower():
		convertedAmount, _ := btcutil.NewAmount(amount)
		convertedAmountStr = convertedAmount.String()
	case libutils.LTCWalletAsset.ToStringLower():
		convertedAmount, _ := ltcutil.NewAmount(amount)
		convertedAmountStr = convertedAmount.String()
	default:
		convertedAmountStr = "Unsupported asset type"
	}

	return l.Theme.Label(values.TextSize16, convertedAmountStr).Layout(gtx)
}
