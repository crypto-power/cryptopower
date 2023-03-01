package logger

import (
	"errors"
	"sync"

	"github.com/btcsuite/btclog"
	"github.com/decred/slog"
)

type logger struct {
	subsystemSLoggers map[string]slog.Logger
	subsystemBLoggers map[string]btclog.Logger
}

var instance *logger
var initCtx sync.Once

func New(sLoggers map[string]slog.Logger, bLoggers map[string]btclog.Logger) *logger {
	initCtx.Do(func() {
		instance = &logger{
			subsystemSLoggers: sLoggers,
			subsystemBLoggers: bLoggers,
		}
	})

	return instance
}

func GetLogger() (*logger, error) {
	if instance == nil {
		return nil, errors.New("logger not instantiated")
	}

	return instance, nil
}

// setLogLevel sets the logging level for provided subsystem.  Invalid
// subsystems are ignored.  Uninitialized subsystems are dynamically created as
// needed.
func (l *logger) setLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	logger, ok := l.subsystemSLoggers[subsystemID]
	if !ok {
		return
	}

	level, _ := slog.LevelFromString(logLevel)
	logger.SetLevel(level)
}

func (l *logger) setBTCLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	logger, ok := l.subsystemBLoggers[subsystemID]
	if !ok {
		return
	}
	lvl, _ := btclog.LevelFromString(logLevel)
	logger.SetLevel(lvl)
}

// setLogLevels sets the log level for all subsystem loggers to the passed
// level.  It also dynamically creates the subsystem loggers as needed, so it
// can be used to initialize the logging system.
func SetLogLevels(logLevel string) error {
	if instance == nil {
		return errors.New("cannot set log level on nil logger")
	}
	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for subsystemID := range instance.subsystemSLoggers {
		instance.setLogLevel(subsystemID, logLevel)
	}
	for subsystemID := range instance.subsystemBLoggers {
		instance.setBTCLogLevel(subsystemID, logLevel)
	}
	return nil
}
