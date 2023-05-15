package eth

// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

import (
	"fmt"

	"github.com/decred/slog"
	ethlog "github.com/ethereum/go-ethereum/log"
)

var log = slog.Disabled

// UseLoggers sets the subsystem logs to use the provided loggers.
func UseLogger(logger slog.Logger) {
	log = logger
	ethlog.Root().SetHandler(ethHandler())
}

// DisableLog disables all library log output.  Logging output is disabled
// by default until UseLogger is called.
func DisableLog() {
	log = slog.Disabled
	ethlog.DiscardHandler()
}

// ethHandler converts the upstream logging implementation to work with the local
// implementation.
func ethHandler() ethlog.Handler {
	return ethlog.FuncHandler(func(r *ethlog.Record) error {
		ethformatted := r.Msg
		strFormatter := ""
		for i, item := range r.Ctx {
			if i%2 == 0 {
				strFormatter = "%s %v"
			} else {
				strFormatter = "%s=%v"
			}
			ethformatted = fmt.Sprintf(strFormatter, ethformatted, item)
		}
		switch r.Lvl {
		case ethlog.LvlCrit:
			log.Critical(ethformatted)
		case ethlog.LvlError:
			log.Error(ethformatted)
		case ethlog.LvlWarn:
			log.Warn(ethformatted)
		case ethlog.LvlInfo:
			log.Info(ethformatted)
		case ethlog.LvlDebug:
			log.Debug(ethformatted)
		case ethlog.LvlTrace:
			log.Trace(ethformatted)
		}
		return nil
	})
}
