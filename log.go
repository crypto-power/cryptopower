// Copyright (c) 2016, 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gitlab.com/cryptopower/cryptopower/libwallet"
	"gitlab.com/cryptopower/cryptopower/libwallet/assets/btc"
	"gitlab.com/cryptopower/cryptopower/libwallet/assets/dcr"
	"gitlab.com/cryptopower/cryptopower/libwallet/assets/ltc"
	sharedW "gitlab.com/cryptopower/cryptopower/libwallet/assets/wallet"
	"gitlab.com/cryptopower/cryptopower/libwallet/ext"
	"gitlab.com/cryptopower/cryptopower/libwallet/instantswap"
	libutils "gitlab.com/cryptopower/cryptopower/libwallet/utils"
	"gitlab.com/cryptopower/cryptopower/listeners"
	"gitlab.com/cryptopower/cryptopower/logger"
	"gitlab.com/cryptopower/cryptopower/ui"
	"gitlab.com/cryptopower/cryptopower/ui/load"
	"gitlab.com/cryptopower/cryptopower/ui/modal"
	"gitlab.com/cryptopower/cryptopower/ui/page"
	"gitlab.com/cryptopower/cryptopower/ui/page/components"
	"gitlab.com/cryptopower/cryptopower/ui/page/exchange"
	"gitlab.com/cryptopower/cryptopower/ui/page/governance"
	"gitlab.com/cryptopower/cryptopower/ui/page/info"
	"gitlab.com/cryptopower/cryptopower/ui/page/privacy"
	"gitlab.com/cryptopower/cryptopower/ui/page/root"
	"gitlab.com/cryptopower/cryptopower/ui/page/send"
	"gitlab.com/cryptopower/cryptopower/ui/page/staking"
	"gitlab.com/cryptopower/cryptopower/ui/page/transaction"
	"gitlab.com/cryptopower/cryptopower/wallet"

	"decred.org/dcrwallet/v2/p2p"
	"decred.org/dcrwallet/v2/spv"
	"decred.org/dcrwallet/v2/ticketbuyer"
	dcrw "decred.org/dcrwallet/v2/wallet"
	"decred.org/dcrwallet/v2/wallet/udb"
	"github.com/btcsuite/btclog"
	btcC "github.com/btcsuite/btcwallet/chain"
	btcw "github.com/btcsuite/btcwallet/wallet"
	btcWtx "github.com/btcsuite/btcwallet/wtxmgr"
	ltcN "github.com/dcrlabs/neutrino-ltc"
	"github.com/decred/dcrd/addrmgr/v2"
	"github.com/decred/dcrd/connmgr/v3"
	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
	btcN "github.com/lightninglabs/neutrino"
	ltcC "github.com/ltcsuite/ltcwallet/chain"
	ltcw "github.com/ltcsuite/ltcwallet/wallet"
	ltcWtx "github.com/ltcsuite/ltcwallet/wtxmgr"
)

// logWriter implements an io.Writer that outputs to both standard output and
// the write-end pipe of an initialized log rotator.
type logWriter struct {
	loggerID string
}

// Write writes the data in p to standard out and the log rotator.
func (l logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	return logRotators[l.loggerID].Write(p)
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
	// dcrLogger, btcLogger, mainLogger indentifies the respective loggers.
	dcrLogger, btcLogger, mainLogger = "dcr.log", "btc.log", "cryptopower.log"
	ltcLogger                        = "ltc.log"
	// backendLog is the logging backend used to create all subsystem loggers.
	// The backend must not be used before the log rotator has been initialized,
	// or data races and/or nil pointer dereferences will occur.
	dcrBackendLog = slog.NewBackend(logWriter{dcrLogger})
	btcBackendLog = btclog.NewBackend(logWriter{btcLogger})
	ltcBackendLog = btclog.NewBackend(logWriter{ltcLogger})
	backendLog    = slog.NewBackend(logWriter{mainLogger})

	// logRotator is one of the logging outputs.  It should be closed on
	// application shutdown.
	logRotators map[string]*rotator.Rotator

	log          = backendLog.Logger("CRPW")
	sharedWLog   = backendLog.Logger("SHWL")
	walletLog    = backendLog.Logger("WALL")
	winLog       = backendLog.Logger("UI")
	dlwlLog      = backendLog.Logger("DLWL")
	lstnersLog   = backendLog.Logger("LSTN")
	extLog       = backendLog.Logger("EXT")
	amgrLog      = backendLog.Logger("AMGR")
	cmgrLog      = backendLog.Logger("CMGR")
	dcrLog       = dcrBackendLog.Logger("DCR")
	syncLog      = dcrBackendLog.Logger("SYNC")
	tkbyLog      = dcrBackendLog.Logger("TKBY")
	dcrWalletLog = dcrBackendLog.Logger("WLLT")
	dcrSpv       = dcrBackendLog.Logger("DCR-S")
	btcNtrn      = btcBackendLog.Logger("B-NTR")
	ltcNtrn      = btcBackendLog.Logger("L-NTR")
	btcLog       = btcBackendLog.Logger("BTC")
	ltcLog       = ltcBackendLog.Logger("LTC")
)

// Initialize package-global logger variables.
func init() {
	sharedW.UseLogger(sharedWLog)
	page.UseLogger(winLog)
	wallet.UseLogger(walletLog)
	ui.UseLogger(winLog)
	send.UseLogger(winLog)
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
	ltc.UseLogger(ltcLog)
	ext.UseLogger(extLog)
	exchange.UseLogger(sharedWLog)
	addrmgr.UseLogger(dcrLog)
	connmgr.UseLogger(dcrLog)
	p2p.UseLogger(syncLog)
	ticketbuyer.UseLogger(tkbyLog)
	udb.UseLogger(dcrWalletLog)
	btcN.UseLogger(btcNtrn)
	ltcN.UseLogger(ltcNtrn)
	ltcWtx.UseLogger(ltcLog)
	btcWtx.UseLogger(btcLog)
	ltcC.UseLogger(ltcLog)
	btcC.UseLogger(btcLog)
	btcw.UseLogger(btcLog)
	ltcw.UseLogger(ltcLog)
	dcrw.UseLogger(dcrLog)
	spv.UseLogger(dcrSpv)
	instantswap.UseLogger(sharedWLog)

	logger.New(subsystemSLoggers, subsystemBLoggers)
	// Neutrino loglevel will always be set to error to control excessive logging.
	ltcNtrn.SetLevel(btclog.LevelError)
	btcNtrn.SetLevel(btclog.LevelError)

	// Similar to BTC and LTC, excessive loggings of dcr spv has been capped to errors.
	dcrSpv.SetLevel(slog.LevelError)
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
	"WLLT": dcrWalletLog,
	"SHWL": sharedWLog,
}

var subsystemBLoggers = map[string]btclog.Logger{
	"BTC": btcLog,
	"LTC": ltcLog,
}

// initLogRotator initializes the logging rotater to write logs to logFile and
// create roll files in the same directory.  It must be called before the
// package-global log rotater variables are used.
func initLogRotator(logDir string, maxRolls int) {
	logRotators = map[string]*rotator.Rotator{
		btcLogger:  nil,
		dcrLogger:  nil,
		ltcLogger:  nil,
		mainLogger: nil,
	}

	err := os.MkdirAll(logDir, libutils.UserFilePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	for logFile := range logRotators {
		r, err := rotator.New(filepath.Join(logDir, logFile), 32*1024, false, maxRolls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
			os.Exit(1)
		}
		logRotators[logFile] = r
	}
}

func isExistSystem(subsysID string) bool {
	// Validate subsystem.
	_, slExists := subsystemSLoggers[subsysID]
	_, btcExists := subsystemBLoggers[subsysID]
	return slExists || btcExists
}
