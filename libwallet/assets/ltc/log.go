package ltc

import (
	"github.com/btcsuite/btclog"
)

var log = btclog.Disabled

// DisableLog disables all library log output.  Logging output is disabled
// by default until UseLogger is called.
func DisableLog() {
	log = btclog.Disabled
}

// UseLogger sets the subsystem logs to use the provided loggers.
func UseLogger(sLogger btclog.Logger) {
	log = sLogger
}
