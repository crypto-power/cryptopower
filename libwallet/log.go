// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package libwallet

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/btc"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/dcr"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/eth"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader/ltc"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/politeia"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/vsp"
	"github.com/btcsuite/btclog"
	"github.com/decred/slog"
)

var log = slog.Disabled

// subsystemLoggersA and subsystemLoggersB maps each subsystem identifier to its associated logger.
var (
	subsystemLoggersA []slog.Logger
	subsystemLoggersB []btclog.Logger
)

// UseLoggers sets the subsystems logs to use the provided loggers.
func UseLoggers(logger, dcrlogger, ethLogger, vspLogger, politeiaLogger slog.Logger, btcLogger, ltcLogger btclog.Logger) {
	log = logger
	btc.UseLogger(btcLogger)
	dcr.UseLogger(dcrlogger)
	ltc.UseLogger(ltcLogger)
	eth.UseLogger(ethLogger)
	vsp.UseLogger(vspLogger)
	politeia.UseLogger(politeiaLogger)

	subsystemLoggersA = []slog.Logger{
		logger, dcrlogger, ethLogger, vspLogger, politeiaLogger,
	}

	subsystemLoggersB = []btclog.Logger{
		btcLogger, ltcLogger,
	}
}

// SetLogLevels sets the logging level for all subsystems to the provided level.
func SetLogLevels(logLevel string) {
	level, ok := slog.LevelFromString(logLevel)
	if !ok {
		return
	}

	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for _, subsystem := range subsystemLoggersA {
		subsystem.SetLevel(level)
	}

	levelB, ok := btclog.LevelFromString(logLevel)
	if !ok {
		return
	}

	for _, subsystem := range subsystemLoggersB {
		subsystem.SetLevel(levelB)
	}
}
