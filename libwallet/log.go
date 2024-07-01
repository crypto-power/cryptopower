// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package libwallet

import (
	"os"

	"decred.org/dcrwallet/v4/errors"
	"github.com/crypto-power/cryptopower/libwallet/internal/loader"
	"github.com/crypto-power/cryptopower/libwallet/internal/politeia"
	"github.com/crypto-power/cryptopower/libwallet/internal/vsp"
	"github.com/crypto-power/cryptopower/libwallet/utils"
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

	vspcLog     = backendLog.Logger("VSPC")
	politeiaLog = backendLog.Logger("POLT")
)

var log = slog.Disabled

// subsystemLoggers maps each subsystem identifier to its associated logger.
var subsystemLoggers = map[string]slog.Logger{
	"DLWL": log,
	"VSPC": vspcLog,
	"POLT": politeiaLog,
}

// initLogRotator initializes the logging rotater to write logs to logFile and
// create roll files in the same directory.  It must be called before the
// package-global log rotater variables are used.
func initLogRotator(logFile string) error {
	r, err := rotator.New(logFile, 10*1024, false, 3)
	if err != nil {
		return errors.Errorf("failed to create file rotator: %v", err)
	}

	logRotator = r
	return nil
}

// UseLogger sets the subsystem logs to use the provided loggers.
func UseLogger(logger slog.Logger) {
	log = logger
	loader.UseLogger(logger)
	vsp.UseLogger(vspcLog)
	politeia.UseLogger(politeiaLog)
}

// RegisterLogger should be called before logRotator is initialized.
func RegisterLogger(tag string) (slog.Logger, error) {
	if logRotator != nil {
		return nil, errors.E(utils.ErrLogRotatorAlreadyInitialized)
	}

	if _, exists := subsystemLoggers[tag]; exists {
		return nil, errors.E(utils.ErrLoggerAlreadyRegistered)
	}

	logger := backendLog.Logger(tag)
	subsystemLoggers[tag] = logger

	return logger, nil
}

// SetLogLevels sets the logging level for all subsystems to the provided level.
func SetLogLevels(logLevel string) {
	_, ok := slog.LevelFromString(logLevel)
	if !ok {
		return
	}

	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for subsystemID := range subsystemLoggers {
		setLogLevel(subsystemID, logLevel)
	}
}

// setLogLevel sets the logging level for provided subsystem.  Invalid
// subsystems are ignored.  Uninitialized subsystems are dynamically created as
// needed.
func setLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	logger, ok := subsystemLoggers[subsystemID]
	if !ok {
		return
	}

	// Defaults to info if the log level is invalid.
	level, _ := slog.LevelFromString(logLevel)
	logger.SetLevel(level)
}

// Log writes a message to the log using LevelInfo.
func Log(m string) {
	log.Info(m)
}

// LogT writes a tagged message to the log using LevelInfo.
func LogT(tag, m string) {
	log.Infof("%s: %s", tag, m)
}
