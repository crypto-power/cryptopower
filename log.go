// Copyright (c) 2016, 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/libwallet/assets/btc"
	"github.com/crypto-power/cryptopower/libwallet/assets/dcr"
	"github.com/crypto-power/cryptopower/libwallet/assets/eth"
	"github.com/crypto-power/cryptopower/libwallet/assets/ltc"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/ext"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/logger"
	"github.com/crypto-power/cryptopower/ui"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page"
	account "github.com/crypto-power/cryptopower/ui/page/accounts"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/page/dcrdex"
	"github.com/crypto-power/cryptopower/ui/page/exchange"
	"github.com/crypto-power/cryptopower/ui/page/governance"
	"github.com/crypto-power/cryptopower/ui/page/info"
	"github.com/crypto-power/cryptopower/ui/page/privacy"
	"github.com/crypto-power/cryptopower/ui/page/receive"
	"github.com/crypto-power/cryptopower/ui/page/root"
	"github.com/crypto-power/cryptopower/ui/page/send"
	"github.com/crypto-power/cryptopower/ui/page/staking"
	"github.com/crypto-power/cryptopower/ui/page/transaction"
	"github.com/crypto-power/cryptopower/ui/page/wallet"

	"decred.org/dcrwallet/v3/p2p"
	"decred.org/dcrwallet/v3/spv"
	"decred.org/dcrwallet/v3/ticketbuyer"
	dcrw "decred.org/dcrwallet/v3/wallet"
	"decred.org/dcrwallet/v3/wallet/udb"
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
	// dcrLogger, btcLogger, mainLogger identifies the respective loggers.
	dcrLogger, btcLogger, mainLogger = "dcr.log", "btc.log", libwallet.LogFilename
	ltcLogger, ethLogger             = "ltc.log", "eth.log"
	// backendLog is the logging backend used to create all subsystem loggers.
	// The backend must not be used before the log rotator has been initialized,
	// or data races and/or nil pointer dereferences will occur.
	dcrBackendLog = slog.NewBackend(logWriter{dcrLogger})
	btcBackendLog = btclog.NewBackend(logWriter{btcLogger})
	ltcBackendLog = btclog.NewBackend(logWriter{ltcLogger})
	ethBackendLog = slog.NewBackend(logWriter{ethLogger})
	backendLog    = slog.NewBackend(logWriter{mainLogger})

	// logRotator is one of the logging outputs.
	// NOTE: There are concurrency issues with closing the log rotators used by
	// btcd and btcwallet since they (btcd and btcwallet) have unsupervised
	// goroutines still running after shutdown. So we leave the rotator running
	// at the risk of losing some logs.
	logRotators map[string]*rotator.Rotator

	log          = backendLog.Logger("CRPW")
	sharedWLog   = backendLog.Logger("SHWL")
	winLog       = backendLog.Logger("UI")
	dlwlLog      = backendLog.Logger("DLWL")
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
	ethLog       = ethBackendLog.Logger("ETH")
)

// Initialize package-global logger variables.
func init() {
	sharedW.UseLogger(sharedWLog)
	page.UseLogger(winLog)
	ui.UseLogger(winLog)
	send.UseLogger(winLog)
	root.UseLogger(winLog)
	libwallet.UseLogger(dlwlLog)
	dcr.UseLogger(dcrLog)
	load.UseLogger(log)
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
	dcrdex.UseLogger(winLog)
	account.UseLogger(winLog)
	wallet.UseLogger(winLog)
	receive.UseLogger(winLog)
	eth.UseLogger(ethLog)

	logger.New(subsystemSLoggers, subsystemBLoggers)
	// Neutrino loglevel will always be set to error to control excessive logging.
	ltcNtrn.SetLevel(btclog.LevelError)
	btcNtrn.SetLevel(btclog.LevelError)

	// Similar to BTC and LTC, excessive loggings of dcr spv has been capped to errors.
	dcrSpv.SetLevel(slog.LevelError)
}

// subsystemLoggers maps each subsystem identifier to its associated logger.
var subsystemSLoggers = map[string]slog.Logger{
	"DLWL": dlwlLog,
	"DCR":  dcrLog,
	"UI":   winLog,
	"CRPW": log,
	"EXT":  extLog,
	"AMGR": amgrLog,
	"CMGR": cmgrLog,
	"SYNC": syncLog,
	"TKBY": tkbyLog,
	"WLLT": dcrWalletLog,
	"SHWL": sharedWLog,
	"ETH":  ethLog,
}

var subsystemBLoggers = map[string]btclog.Logger{
	"BTC": btcLog,
	"LTC": ltcLog,
}

// initLogRotator initializes the logging rotater to write logs to logFile and
// create roll files in the same directory.  It must be called before the
// package-global log rotater variables are used.
func initLogRotator(logDir string, maxRolls int) {
	// Close any previously initialized log rotators.
	for _, rotator := range logRotators {
		if rotator != nil {
			rotator.Close() // TODO: what to do with error?
		}
	}

	logRotators = map[string]*rotator.Rotator{
		btcLogger:  nil,
		dcrLogger:  nil,
		ltcLogger:  nil,
		ethLogger:  nil,
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
