// Copyright (c) 2015 The btcsuite developers
// Copyright (c) 2017-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ltc

import (
	"github.com/btcsuite/btclog"
)

// log is a logger that is initialized with no output filters.  This
// means the package will not perform any logging by default until the caller
// requests it.
var log = btclog.Disabled

// UseLogger uses a specified Logger to output package logging info.
// This should be used in preference to SetLogWriter if the caller is also
// using slog.
func UseLogger(logger btclog.Logger) {
	log = logger
}
