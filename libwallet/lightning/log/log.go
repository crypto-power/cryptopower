package log

import (
	"io"
	"sync"

	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/config"
	"github.com/btcsuite/btclog"
	"github.com/lightningnetwork/lnd/build"
)

var (
	initBackend sync.Once
	logWriter   *build.RotatingLogWriter
	initError   error
)

/*
Writer is the implementatino of io.Writer interface required for logging
*/
type Writer struct {
	writer io.Writer
}

func (w *Writer) Write(b []byte) (int, error) {
	//os.Stdout.Write(b)
	if w.writer != nil {
		w.writer.Write(b)
	}
	return len(b), nil
}

/*
GetLogger ensure log backend is initialized and return a logger.
*/
func GetLogger(workingDir string, logger string) (btclog.Logger, error) {
	initLog(workingDir)
	if initError != nil {
		return nil, initError
	}
	return logWriter.GenSubLogger(logger, func() {}), nil
}

/*
GetLogWriter ensure log backend is initialized and return the writer singleton.
This writer is sent to other systems to they can use the same log file.
*/
func GetLogWriter(workingDir string) (*build.RotatingLogWriter, error) {
	initLog(workingDir)
	return logWriter, initError
}

func initLog(workingDir string) {
	initBackend.Do(func() {
		cfg, err := config.GetConfig(workingDir)
		if err != nil {
			initError = err
			return
		}
		buildLogWriter := build.NewRotatingLogWriter()

		filename := workingDir + "/logs/bitcoin/" + cfg.Network + "/lnd.log"
		err = buildLogWriter.InitLogRotator(filename, 10, 3)
		if err != nil {
			initError = err
			return
		}
		logWriter = buildLogWriter
	})
}
