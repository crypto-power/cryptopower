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

	log  = backendLog.Logger("BTC")
	ntrn = backendLog.Logger("NTRN")
)

// SetLogRotator assigns logrotator to be used for logging outputs.
func SetLogRotator(rotator *rotator.Rotator) {
	logRotator = rotator
}

// UseLoggers sets the subsystem logs to use the provided loggers.
func UseLoggers(logger btclog.Logger) {
	neutrino.UseLogger(ntrn)
	wtxmgr.UseLogger(logger)
	chain.UseLogger(logger)
	wallet.UseLogger(logger)
}

func UseLogger(sLogger slog.Logger) {
	lvlStr := sLogger.Level().String()
	lvl, _ := btclog.LevelFromString(lvlStr)
	log.SetLevel(lvl)

	// Neutrino Info logs are silenced to avoid masking important error with its
	// excessive logging of info messages.
	ntrn.SetLevel(btclog.LevelError)
	UseLoggers(log)
}
