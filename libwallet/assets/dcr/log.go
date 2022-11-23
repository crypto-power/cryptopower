// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dcr

import (
	"os"

	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/spv"
	"decred.org/dcrwallet/v2/p2p"
	"decred.org/dcrwallet/v2/ticketbuyer"
	"decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/udb"
	"github.com/decred/dcrd/addrmgr/v2"
	"github.com/decred/dcrd/connmgr/v3"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

// logWriter implements an io.Writer that outputs to both standard output and
// the write-end pipe of an initialized log rotator.
type logWriter struct{}

func (logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	logRotator.Write(p)
	return len(p), nil
}

// Loggers per subsystem.  A single backend logger is created and all subsytem
// loggers created from it will write to the backend.  When adding new
// subsystems, add the subsystem logger variable here and to the
// subsystemLoggers map.
//
// Loggers can not be used before the log rotator has been initialized with a
// log file.  This must be performed early during application startup by calling
// initLogRotator.
var (
	// backendLog is the logging backend used to create all subsystem loggers.
	// The backend must not be used before the log rotator has been initialized,
	// or data races and/or nil pointer dereferences will occur.
	backendLog = slog.NewBackend(logWriter{})

	// logRotator is one of the logging outputs.  It should be closed on
	// application shutdown.
	logRotator *rotator.Rotator

	log = backendLog.Logger("DLWL")
)

// UseLoggers sets the subsystem logs to use the provided loggers.
func UseLoggers(main, loaderLog, walletLog, tkbyLog,
	syncLog, cmgrLog, amgrLog slog.Logger) {
	log = main
	loader.UseLogger(loaderLog)
	wallet.UseLogger(walletLog)
	udb.UseLogger(walletLog)
	ticketbuyer.UseLogger(tkbyLog)
	spv.UseLogger(syncLog)
	p2p.UseLogger(syncLog)
	connmgr.UseLogger(cmgrLog)
	addrmgr.UseLogger(amgrLog)
}

// UseLogger sets the subsystem logs to use the provided logger.
func UseLogger(logger slog.Logger) {
	UseLoggers(logger, logger, logger, logger, logger, logger, logger)
}

// Log writes a message to the log using LevelInfo.
func Log(m string) {
	log.Info(m)
}

// LogT writes a tagged message to the log using LevelInfo.
func LogT(tag, m string) {
	log.Infof("%s: %s", tag, m)
}
