package btc

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcwallet/chain"
	w "github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/jrick/logrotate/rotator"
	"github.com/lightninglabs/neutrino"
)

const (
	logDirName  = "logs"
	logFileName = "neutrino.log"
)

var (
	walletBirthday time.Time
	loggingInited  uint32
)

// logWriter implements an io.Writer that outputs to a rotating log file.
type logWriter struct {
	*rotator.Rotator
}

// logNeutrino initializes logging in the neutrino + wallet packages. Logging
// only has to be initialized once, so an atomic flag is used internally to
// return early on subsequent invocations.
//
// In theory, the the rotating file logger must be Close'd at some point, but
// there are concurrency issues with that since btcd and btcwallet have
// unsupervised goroutines still running after shutdown. So we leave the rotator
// running at the risk of losing some logs.
func logNeutrino(walletDir string) error {
	if !atomic.CompareAndSwapUint32(&loggingInited, 0, 1) {
		return nil
	}

	logSpinner, err := logRotator(walletDir)
	if err != nil {
		return fmt.Errorf("error initializing log rotator: %w", err)
	}

	backendLog := btclog.NewBackend(logWriter{logSpinner})

	logger := func(name string, lvl btclog.Level) btclog.Logger {
		l := backendLog.Logger(name)
		l.SetLevel(lvl)
		return l
	}

	neutrino.UseLogger(logger("NTRNO", btclog.LevelDebug))
	w.UseLogger(logger("BTCW", btclog.LevelInfo))
	wtxmgr.UseLogger(logger("TXMGR", btclog.LevelInfo))
	chain.UseLogger(logger("CHAIN", btclog.LevelInfo))

	return nil
}

// logRotator initializes a rotating file logger.
func logRotator(netDir string) (*rotator.Rotator, error) {
	const maxLogRolls = 8
	logDir := filepath.Join(netDir, logDirName)
	if err := os.MkdirAll(logDir, 0744); err != nil {
		return nil, fmt.Errorf("error creating log directory: %w", err)
	}

	logFilename := filepath.Join(logDir, logFileName)
	return rotator.New(logFilename, 32*1024, false, maxLogRolls)
}
