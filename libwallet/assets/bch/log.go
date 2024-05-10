package bch

import (
	"github.com/gcash/bchlog"
)

var log = bchlog.Disabled

// DisableLog disables all library log output.  Logging output is disabled
// by default until UseLogger is called.
func DisableLog() {
	log = bchlog.Disabled
}

// UseLogger sets the subsystem logs to use the provided loggers.
func UseLogger(sLogger bchlog.Logger) {
	log = sLogger
}
