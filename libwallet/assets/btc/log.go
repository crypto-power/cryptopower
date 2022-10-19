package btc

import (
	"os"

	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
	"github.com/lightninglabs/neutrino"
)

// logWriter implements an io.Writer that outputs to both standard output and
// the write-end pipe of an initialized log rotator.
type logWriter struct{}

func (logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	if logRotator != nil {
		return logRotator.Write(p)

	}
	return len(p), nil
}

var (
	// backendLog is the logging backend used to create all subsystem loggers.
	// The backend must not be used before the log rotator has been initialized,
	// or data races and/or nil pointer dereferences will occur.
	backendLog = btclog.NewBackend(logWriter{})

	// logRotator is one of the logging outputs.  It should be closed on
	// application shutdown.
	logRotator *rotator.Rotator

	log         = backendLog.Logger("BTC")
	neutrinoLog = backendLog.Logger("NTRNO")
	wtxmgrLog   = backendLog.Logger("TXMGR")
	chainLog    = backendLog.Logger("CHAIN")
	walletLog   = backendLog.Logger("WLLT")
)

// SetLogRotator assigns logrotator to be used for logging outputs.
func SetLogRotator(logRotator *rotator.Rotator) {
	logRotator = logRotator
}

// Initialize package-global logger variables.
func init() {
	neutrino.UseLogger(neutrinoLog)
	wtxmgr.UseLogger(wtxmgrLog)
	chain.UseLogger(chainLog)
	wallet.UseLogger(walletLog)
}

// UseLoggers sets the subsystem logs to use the provided loggers.
func UseLoggers(main, walletLog, neutrinoLog, wtxmgrLog, chainLog btclog.Logger) {
	log = main
	neutrino.UseLogger(neutrinoLog)
	wtxmgr.UseLogger(wtxmgrLog)
	chain.UseLogger(chainLog)
	wallet.UseLogger(walletLog)
}

// UseLogger sets the subsystem logs to use the provided logger.
func UseLogger(sLogger slog.Logger) {
	lvlStr := sLogger.Level().String()
	lvl, _ := btclog.LevelFromString(lvlStr)
	logger := backendLog.Logger("BTC")
	logger.SetLevel(lvl)
	UseLoggers(logger, logger, logger, logger, logger)
}
