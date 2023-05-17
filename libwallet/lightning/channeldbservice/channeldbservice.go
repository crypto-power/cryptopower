package channeldbservice

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/chainservice"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/config"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/log"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/refcount"
	"github.com/btcsuite/btclog"
	"github.com/lightningnetwork/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/kvdb"
)

const (
	directoryPattern = "data/graph/{{network}}/"
	dbName           = "channel.db"
)

var (
	serviceRefCounter refcount.ReferenceCountable
	chanDB            *channeldb.DB
	logger            btclog.Logger
)

// Get returns a Ch
func Get(workingDir string) (db *channeldb.DB, cleanupFn func() error, err error) {
	service, release, err := serviceRefCounter.Get(
		func() (interface{}, refcount.ReleaseFunc, error) {
			return newService(workingDir)
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return service.(*channeldb.DB), release, err
}

func newService(workingDir string) (db *channeldb.DB, rel refcount.ReleaseFunc, err error) {
	chanDB, err = createService(workingDir)
	if err != nil {
		return nil, nil, err
	}
	return chanDB, release, err
}

func release() error {
	return chanDB.Close()
}

func compactDB(graphDir string) error {
	dbPath := path.Join(graphDir, dbName)
	f, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	logger.Infof("channel db size = %v", f.Size())

	if f.Size() <= 400000000 {
		return nil
	}
	newFile, err := ioutil.TempFile(graphDir, "cdb-compact")
	if err != nil {
		return err
	}
	if err = chainservice.BoltCopy(dbPath, newFile.Name(),
		func(keyPath [][]byte, k []byte, v []byte) bool { return false }); err != nil {
		return err
	}
	if err = os.Rename(dbPath, dbPath+".old"); err != nil {
		return err
	}
	if err = os.Rename(newFile.Name(), dbPath); err != nil {
		logger.Criticalf("Error when renaming the new channeldb file: %v", err)
		return err
	}
	logger.Infof("channel.db was compacted because it's too big")
	return nil
}

func deleteOldDB(graphDir string) error {
	oldDBPath := path.Join(graphDir, dbName+".old")
	err := os.Remove(oldDBPath)
	logger.Infof("os.Remove(%v): %v", oldDBPath, err)
	return err
}

func deleteOldBootstrap(workingDir string) error {
	bootstrap := path.Join(workingDir, "bootstrap")
	err := os.RemoveAll(bootstrap)
	logger.Infof("os.RemoveAll(%v): %v", bootstrap, err)
	return err
}

func createService(workingDir string) (*channeldb.DB, error) {
	config, err := config.GetConfig(workingDir)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger, err = log.GetLogger(workingDir, "CHANNELDB")
		if err != nil {
			return nil, err
		}
		logger.SetLevel(btclog.LevelDebug)
	}

	graphDir := path.Join(workingDir, strings.Replace(directoryPattern, "{{network}}", config.Network, -1))
	if err = compactDB(graphDir); err != nil {
		logger.Errorf("Error in compactDB: %v", err)
	}

	logger.Infof("creating shared channeldb service.")
	cfg := lnd.DefaultConfig()
	dbOptions := []channeldb.OptionModifier{
		channeldb.OptionSetRejectCacheSize(cfg.Caches.RejectCacheSize),
		channeldb.OptionSetChannelCacheSize(cfg.Caches.ChannelCacheSize),
		channeldb.OptionSetBatchCommitInterval(cfg.DB.BatchCommitInterval),
		channeldb.OptionDryRunMigration(cfg.DryRunMigration),
		channeldb.OptionSetUseGraphCache(!cfg.DB.NoGraphCache),
		channeldb.OptionKeepFailedPaymentAttempts(cfg.KeepFailedPaymentAttempts),
		channeldb.OptionPruneRevocationLog(cfg.DB.PruneRevocation),
		channeldb.OptionSetPreAllocCacheNumNodes(channeldb.DefaultPreAllocCacheNumNodes),
	}

	opts := channeldb.DefaultOptions()
	backend, err := kvdb.GetBoltBackend(&kvdb.BoltBackendConfig{
		DBPath:            graphDir,
		DBFileName:        dbName,
		NoFreelistSync:    false,
		AutoCompact:       opts.AutoCompact,
		AutoCompactMinAge: opts.AutoCompactMinAge,
		DBTimeout:         opts.DBTimeout,
	})
	if err != nil {
		return nil, err
	}

	chanDB, err := channeldb.CreateWithBackend(
		backend, dbOptions...,
	)
	if err != nil {
		logger.Errorf("unable to open channeldb: %v", err)
		return nil, err
	}
	deleteOldDB(graphDir)
	deleteOldBootstrap(workingDir)

	logger.Infof("channeldb was opened successfuly")
	return chanDB, err
}
