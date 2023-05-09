// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dcr

import (
	"github.com/decred/slog"
)

var log = slog.Disabled

// UseLoggers sets the subsystem logs to use the provided loggers.
func UseLogger(logger slog.Logger) {
	log = logger
}

// Log writes a message to the log using LevelInfo.
func Log(m string) {
	log.Info(m)
}

// LogT writes a tagged message to the log using LevelInfo.
func LogT(tag, m string) {
	log.Infof("%s: %s", tag, m)
}
