package eth

// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

import "github.com/decred/slog"

var log = slog.Disabled

// UseLoggers sets the subsystem logs to use the provided loggers.
func UseLogger(logger slog.Logger) {
	log = logger
}

// DisableLog disables all library log output.  Logging output is disabled
// by default until UseLogger is called.
func DisableLog() {
	log = slog.Disabled
}
