package components

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/color"
	"strings"
	"time"

	"decred.org/dcrwallet/v3/pgpwordlist"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/btcsuite/btcd/btcutil"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/ltcsuite/ltcd/ltcutil"
	"github.com/tyler-smith/go-bip39"
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

func SeedWordsToHex(seedWords string, wordSeedType sharedW.WordSeedType) (string, error) {
	var seedHex string
	var seedByte []byte
	var err error
	if wordSeedType == sharedW.WordSeed33 {
		words := strings.Split(strings.TrimSpace(seedWords), " ")
		seedByte, err = pgpwordlist.DecodeMnemonics(words)
		if checksumByte(seedByte[:len(seedByte)-1]) != seedByte[len(seedByte)-1] {
			return seedHex, fmt.Errorf("seed checksum mismatch")
		}
		seedByte = seedByte[:len(seedByte)-1]

		if len(seedByte) < MinSeedBytes || len(seedByte) > MaxSeedBytes {
			return seedHex, fmt.Errorf("invalid seed bytes length")
		}
	} else {
		seedByte, err = bip39.EntropyFromMnemonic(seedWords)
	}

	if err != nil {
		return "", err
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

func LayoutIconAndTextWithSize(l *load.Load, gtx C, text string, col color.NRGBA, size unit.Sp, iconSize unit.Dp) D {
	return layout.Inset{Right: values.MarginPadding12}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Right: values.MarginPadding5,
				}.Layout(gtx, func(gtx C) D {
					ic := cryptomaterial.NewIcon(l.Theme.Icons.DotIcon)
					ic.Color = col
					return ic.Layout(gtx, iconSize)
				})
			}),
			layout.Rigid(func(gtx C) D {
				txt := l.Theme.Label(size, text)
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

	return l.Theme.Label(l.ConvertTextSize(values.TextSize16), convertedAmountStr).Layout(gtx)
}

func GetWordSeedTypeDropdownItems() []cryptomaterial.DropDownItem {
	return []cryptomaterial.DropDownItem{
		{Text: values.String(values.Str12WordSeed)},
		{Text: values.String(values.Str24WordSeed)},
		{Text: values.String(values.Str33WordSeed)},
	}
}

func GetWordSeedType(val string) sharedW.WordSeedType {
	switch val {
	case values.String(values.Str12WordSeed):
		return sharedW.WordSeed12
	case values.String(values.Str24WordSeed):
		return sharedW.WordSeed24
	case values.String(values.Str33WordSeed):
		return sharedW.WordSeed33
	default:
		return 0
	}
}
