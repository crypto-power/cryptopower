package components

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/widget"

	"gitlab.com/raedah/cryptopower/libwallet"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/values"
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
	wordList := libwallet.PGPWordList()
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
		return seedHex, fmt.Errorf("Seed checksum mismatch")
	}
	seedByte = seedByte[:len(seedByte)-1]

	// minSeedBytes is the minimum number of bytes allowed for a seed.
	minSeedBytes := 16
	// maxSeedBytes is the maximum number of bytes allowed for a seed.
	maxSeedBytes := 64
	if len(seedByte) < minSeedBytes || len(seedByte) > maxSeedBytes {
		return seedHex, fmt.Errorf("Invalid seed bytes length")
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

func translateErr(err error) string {
	switch err.Error() {
	case libwallet.ErrInvalidPassphrase:
		return values.String(values.StrInvalidPassphrase)
	}

	return err.Error()
}

func EditorsNotEmpty(editors ...*widget.Editor) bool {
	for _, e := range editors {
		if e.Text() == "" {
			return false
		}
	}
	return true
}

// getLockWallet returns a list of locked wallets
func getLockedWallets(wallets []*libwallet.Wallet) []*libwallet.Wallet {
	var walletsLocked []*libwallet.Wallet
	for _, wl := range wallets {
		if !wl.HasDiscoveredAccounts && wl.IsLocked() {
			walletsLocked = append(walletsLocked, wl)
		}
	}

	return walletsLocked
}

func computePasswordStrength(pb *cryptomaterial.ProgressBarStyle, th *cryptomaterial.Theme, editors ...*widget.Editor) {
	password := editors[0]
	strength := libwallet.ShannonEntropy(password.Text()) / 4.0
	pb.Progress = float32(strength * 100)
	pb.Color = th.Color.Success
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

func handleSubmitEvent(editors ...*widget.Editor) bool {
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
