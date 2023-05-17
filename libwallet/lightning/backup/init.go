package backup

import (
	"encoding/json"
	"fmt"
	"path"
	"sync"

	"github.com/breez/breez/config"
	"github.com/breez/breez/data"
	"github.com/breez/breez/tor"

	"github.com/btcsuite/btclog"
)

type ProviderFactoryInfo struct {
	authService AuthService
	authData    string
	log         btclog.Logger
	torConfig   *tor.TorConfig
}

// ProviderFactory is a factory for create a specific provider.
// This is the function needed to be implemented for a new provider
// to be registered and used.
type ProviderFactory func(providerFactoryInfo ProviderFactoryInfo) (Provider, error)

var providersFactory = map[string]ProviderFactory{
	"gdrive": func(providerFactoryInfo ProviderFactoryInfo) (Provider, error) {
		return NewGoogleDriveProvider(providerFactoryInfo.authService, providerFactoryInfo.log)
	},
	"remoteserver": func(providerFactoryInfo ProviderFactoryInfo) (Provider, error) {
		var providerData ProviderData
		_ = json.Unmarshal([]byte(providerFactoryInfo.authData), &providerData)
		return NewRemoteServerProvider(
			providerData,
			providerFactoryInfo.log,
			providerFactoryInfo.torConfig,
		)
	},
}

type BackupRequest struct {
	BackupNodeData bool
	BackupAppData  bool
}

// Manager holds the data needed for the backup to execute its work.
type Manager struct {
	started           int32
	stopped           int32
	workingDir        string
	db                *backupDB
	provider          Provider
	authService       AuthService
	prepareBackupData DataPreparer
	config            *config.Config
	backupRequestChan chan BackupRequest
	onServiceEvent    func(event data.NotificationEvent)
	quitChan          chan struct{}
	log               btclog.Logger
	encryptionKey     []byte
	encryptionType    string
	mu                sync.Mutex
	wg                sync.WaitGroup

	torConfig *tor.TorConfig
}

// NewManager creates a new Manager
func NewManager(
	providerName string,
	authService AuthService,
	onServiceEvent func(event data.NotificationEvent),
	prepareData DataPreparer,
	config *config.Config,
	log btclog.Logger) (*Manager, error) {

	var provider Provider
	var err error
	if providerName != "" {
		provider, err = createBackupProvider(providerName, ProviderFactoryInfo{authService, "", log, nil})
		if err != nil {
			return nil, err
		}
	}

	db, err := openDB(path.Join(config.WorkingDir, "backup.db"))
	if err != nil {
		return nil, err
	}

	return &Manager{
		db:                db,
		workingDir:        config.WorkingDir,
		onServiceEvent:    onServiceEvent,
		provider:          provider,
		prepareBackupData: prepareData,
		config:            config,
		log:               log,
		authService:       authService,
		backupRequestChan: make(chan BackupRequest, 10),
		quitChan:          make(chan struct{}),
	}, nil
}

// RegisterProvider registers a backup provider with a unique name
func RegisterProvider(providerName string, factory ProviderFactory) {
	providersFactory[providerName] = factory
}

func createBackupProvider(providerName string, providerFactoryInfo ProviderFactoryInfo) (Provider, error) {
	factory, ok := providersFactory[providerName]
	if !ok {
		return nil, fmt.Errorf("provider not found for %v", providerName)
	}
	return factory(providerFactoryInfo)
}
