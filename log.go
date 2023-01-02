// Copyright (c) 2016, 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cryptopower.dev/group/cryptopower/libwallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"
	"code.cryptopower.dev/group/cryptopower/libwallet/ext"
	"code.cryptopower.dev/group/cryptopower/libwallet/spv"
	"code.cryptopower.dev/group/cryptopower/listeners"
	"code.cryptopower.dev/group/cryptopower/ui"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/modal"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/page/governance"
	"code.cryptopower.dev/group/cryptopower/ui/page/info"
	"code.cryptopower.dev/group/cryptopower/ui/page/privacy"
	"code.cryptopower.dev/group/cryptopower/ui/page/root"
	"code.cryptopower.dev/group/cryptopower/ui/page/staking"
	"code.cryptopower.dev/group/cryptopower/ui/page/transaction"
	"code.cryptopower.dev/group/cryptopower/wallet"

	"decred.org/dcrwallet/v2/p2p"
	"decred.org/dcrwallet/v2/ticketbuyer"
	dcrw "decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/udb"
	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcwallet/chain"
	bw "github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/decred/dcrd/addrmgr/v2"
	"github.com/decred/dcrd/connmgr/v3"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
	"github.com/lightninglabs/neutrino"
)

// logWriter implements an io.Writer that outputs to both standard output and
// the write-end pipe of an initialized log rotator.
type logWriter struct{}

// Write writes the data in p to standard out and the log rotator.
func (l logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return logRotator.Write(p)
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
	backendLog    = slog.NewBackend(logWriter{})
	btcBackendLog = btclog.NewBackend(logWriter{})

	// logRotator is one of the logging outputs.  It should be closed on
	// application shutdown.
	logRotator *rotator.Rotator

	log = backendLog.Logger("CRPW")

	walletLog    = backendLog.Logger("WALL")
	winLog       = backendLog.Logger("UI")
	dlwlLog      = backendLog.Logger("DLWL")
	dcrLog       = backendLog.Logger("DCR")
	lstnersLog   = backendLog.Logger("LSTN")
	extLog       = backendLog.Logger("EXT")
	amgrLog      = backendLog.Logger("AMGR")
	cmgrLog      = backendLog.Logger("CMGR")
	syncLog      = backendLog.Logger("SYNC")
	tkbyLog      = backendLog.Logger("TKBY")
	dcrWalletLog = backendLog.Logger("WLLT")
	ntrn         = btcBackendLog.Logger("NTRN")
	btcLog       = btcBackendLog.Logger("BTC")
)

// Initialize package-global logger variables.
func init() {
	wallet.UseLogger(walletLog)
	ui.UseLogger(winLog)
	root.UseLogger(winLog)
	libwallet.UseLogger(dlwlLog)
	dcr.UseLogger(dcrLog)
	load.UseLogger(log)
	listeners.UseLogger(lstnersLog)
	components.UseLogger(winLog)
	transaction.UseLogger(winLog)
	governance.UseLogger(winLog)
	info.UseLogger(winLog)
	staking.UseLogger(winLog)
	privacy.UseLogger(winLog)
	modal.UseLogger(winLog)
	btc.UseLogger(btcLog)
	ext.UseLogger(extLog)
	addrmgr.UseLogger(dcrLog)
	connmgr.UseLogger(dcrLog)
	p2p.UseLogger(syncLog)
	ticketbuyer.UseLogger(tkbyLog)
	udb.UseLogger(dcrWalletLog)
	neutrino.UseLogger(ntrn)
	wtxmgr.UseLogger(btcLog)
	chain.UseLogger(btcLog)
	bw.UseLogger(btcLog)
	dcrw.UseLogger(dcrLog)
	spv.UseLogger(dcrLog)
}

// subsystemLoggers maps each subsystem identifier to its associated logger.
var subsystemSLoggers = map[string]slog.Logger{
	"WALL": walletLog,
	"DLWL": dlwlLog,
	"DCR":  dcrLog,
	"UI":   winLog,
	"CRPW": log,
	"LSTN": lstnersLog,
	"EXT":  extLog,
	"AMGR": amgrLog,
	"CMGR": cmgrLog,
	"SYNC": syncLog,
	"TKBY": tkbyLog,
	"WLLT": walletLog,
}

var subsystemBLoggers = map[string]btclog.Logger{
	"BTC": btcLog,
}

// initLogRotator initializes the logging rotater to write logs to logFile and
// create roll files in the same directory.  It must be called before the
// package-global log rotater variables are used.
func initLogRotator(logFile string, maxRolls int) {
	logDir, _ := filepath.Split(logFile)
	err := os.MkdirAll(logDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
		os.Exit(1)
	}
	r, err := rotator.New(logFile, 32*1024, false, maxRolls)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
		os.Exit(1)
	}

	logRotator = r
}

// setLogLevel sets the logging level for provided subsystem.  Invalid
// subsystems are ignored.  Uninitialized subsystems are dynamically created as
// needed.
func setLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	logger, ok := subsystemSLoggers[subsystemID]
	if !ok {
		return
	}
	libwallet.SetLogLevels("info")
	// Defaults to info if the log level is invalid.
	level, _ := slog.LevelFromString(logLevel)
	logger.SetLevel(level)
}

func setBTCLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	logger, ok := subsystemBLoggers[subsystemID]
	if !ok {
		return
	}
	lvl, _ := btclog.LevelFromString(logLevel)
	logger.SetLevel(lvl)
}

// setLogLevels sets the log level for all subsystem loggers to the passed
// level.  It also dynamically creates the subsystem loggers as needed, so it
// can be used to initialize the logging system.
func setLogLevels(logLevel string) {
	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for subsystemID := range subsystemSLoggers {
		setLogLevel(subsystemID, logLevel)
	}
	for subsystemID := range subsystemBLoggers {
		setBTCLogLevel(subsystemID, logLevel)
	}
	ntrn.SetLevel(btclog.LevelError)
}

func isExistSystem(subsysID string) bool {
	// Validate subsystem.
	_, slExists := subsystemSLoggers[subsysID]
	_, btcExists := subsystemBLoggers[subsysID]
	return slExists || btcExists
}
