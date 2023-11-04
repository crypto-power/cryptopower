package dexc

import (
	"fmt"
	"os"
	"path/filepath"

	"decred.org/dcrdex/dex"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/jrick/logrotate/rotator"
)

var dexLogFile = "dexc.log"

// newDexLogger initializes a new dex.Logger.
func newDexLogger(logDir, lvl string, maxRolls int) (dex.Logger, func(), error) {
	err := os.MkdirAll(logDir, libutils.UserFilePerm)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	r, err := rotator.New(filepath.Join(logDir, dexLogFile), 32*1024, false, maxRolls)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create file rotator: %w", err)
	}

	logCloser := func() {
		r.Close()
	}

	l, err := dex.NewLoggerMaker(r, lvl, true /* TODO: Make configurable */)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to initialize log: %w", err)
	}

	return l.NewLogger("DEXC"), logCloser, nil
}
