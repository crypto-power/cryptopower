package lightning

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/account"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/backup"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/chainservice"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/config"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/data"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/db"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/doubleratchet"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/lnnode"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/log"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/services"
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/swapfunds"
	"github.com/btcsuite/btclog"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/breezbackuprpc"
	"golang.org/x/net/context"
)

// Client represents the lighning client
type Client struct {
	isReady            int32
	started            int32
	stopped            int32
	wg                 sync.WaitGroup
	connectionMu       sync.Mutex
	quitChan           chan struct{}
	log                btclog.Logger
	cfg                *config.Config
	lightningDB        *db.DB
	releaseLightningDB func() error

	//exposed sub services
	AccountService *account.Service
	BackupManager  *backup.Manager
	SwapService    *swapfunds.Service
	ServicesClient *services.Client

	//non exposed services
	lnDaemon *lnnode.Daemon

	//channel for external binding events
	notificationsChan chan data.NotificationEvent

	lspChanStateSyncer *lspChanStateSync
}

// AppServices defined the interface needed by the lighning client in order to function
// right.
type AppServices interface {
	BackupProviderName() string
	BackupProviderSignIn() (string, error)
}

// AuthService is a Specific implementation for backup.Manager
type AuthService struct {
	appServices AppServices
}

// SignIn is the interface function implementation needed for backup.Manager
func (client *AuthService) SignIn() (string, error) {
	return client.appServices.BackupProviderSignIn()
}

// New create a new lightning client with the given configuration
func New(workingDir string, applicationServices AppServices, startBeforeSync bool) (*Client, error) {
	client := &Client{
		quitChan:          make(chan struct{}),
		notificationsChan: make(chan data.NotificationEvent),
	}

	logger, err := log.GetLogger(workingDir, "BRUI")
	if err != nil {
		return nil, err
	}
	client.log = logger

	client.cfg, err = config.GetConfig(workingDir)
	if err != nil {
		return nil, fmt.Errorf("Failed to get config file: %v", err)
	}

	client.ServicesClient, err = services.NewClient(client.cfg)
	if err != nil {
		return nil, fmt.Errorf("Error creating services.Client: %v", err)
	}

	client.log.Infof("New Client")

	client.lightningDB, client.releaseLightningDB, err = db.Get(workingDir)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialze breezDB: %v", err)
	}

	client.log.Infof("New db")
	walletdbPath := client.cfg.WorkingDir + "/data/chain/bitcoin/" + client.cfg.Network + "/wallet.db" // TODO: When functional modify this db path if it is confirmed that cryptopower wallet db can be shared with lighning wallet db.
	walletDBInfo, err := os.Stat(walletdbPath)
	if err == nil {
		client.log.Infof("wallet db size is: %v", walletDBInfo.Size())
	}
	if err = compactWalletDB(client.cfg.WorkingDir+"/data/chain/bitcoin/"+client.cfg.Network, client.log); err != nil {
		return nil, err
	}

	client.lnDaemon, err = lnnode.NewDaemon(client.cfg, client.lightningDB, startBeforeSync)
	if err != nil {
		return nil, fmt.Errorf("Failed to create lnnode.Daemon: %v", err)
	}

	client.log.Infof("New daemon")

	if err := doubleratchet.Start(path.Join(client.cfg.WorkingDir, "sessions_encryption.db")); err != nil {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to start doubleratchet: %v", err)
	}

	backupLogger, err := log.GetLogger(workingDir, "BCKP")
	if err != nil {
		return nil, err
	}
	client.BackupManager, err = backup.NewManager(
		applicationServices.BackupProviderName(),
		&AuthService{appServices: applicationServices},
		client.onServiceEvent,
		client.prepareBackupInfo,
		client.cfg,
		backupLogger,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to start backup manager: %v", err)
	}
	client.log.Infof("New backup")

	client.lspChanStateSyncer = newLSPChanStateSync(client)

	client.AccountService, err = account.NewService(
		client.cfg,
		client.lightningDB,
		client.ServicesClient,
		client.lnDaemon,
		client.RequestBackup,
		client.onServiceEvent,
	)
	client.log.Infof("New AccountService")
	if err != nil {
		return nil, fmt.Errorf("Failed to create AccountService: %v", err)
	}

	client.SwapService, err = swapfunds.NewService(
		client.cfg,
		client.lightningDB,
		client.ServicesClient,
		client.lnDaemon,
		client.AccountService.SendPaymentForRequestV2,
		client.AccountService.AddInvoice,
		client.ServicesClient.LSPList,
		client.AccountService.GetGlobalMaxReceiveLimit,
		client.onServiceEvent,
	)
	client.log.Infof("New SwapService")
	if err != nil {
		return nil, fmt.Errorf("Failed to create SwapService: %v", err)
	}

	client.log.Infof("client initialized")
	return client, nil
}

// extractBackupInfo extracts the information that is needed for the external backup service:
// 1. paths - the files need to be backed up.
// 2. nodeID - the current lightning node id.
func (client *Client) prepareBackupInfo() (paths []string, nodeID string, err error) {
	client.log.Infof("extractBackupInfo started")
	lnclient := client.lnDaemon.APIClient()
	if lnclient == nil {
		return nil, "", errors.New("Daemon is not ready")
	}
	backupClient := client.lnDaemon.BreezBackupClient()
	if backupClient == nil {
		return nil, "", errors.New("Daemon is not ready")
	}

	response, err := backupClient.GetBackup(context.Background(), &breezbackuprpc.GetBackupRequest{})
	if err != nil {
		client.log.Errorf("Couldn't get backup: %v", err)
		return nil, "", err
	}
	info, err := lnclient.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		return nil, "", err
	}

	f, err := client.lightningdbCopy(client.lightningDB)
	if err != nil {
		client.log.Errorf("Couldn't get breez backup file: %v", err)
		return nil, "", err
	}
	files := append(response.Files, f)
	chanBackupFile := client.cfg.WorkingDir + "/data/chain/bitcoin/" + client.cfg.Network + "/channel.backup" // TODO: Move to the right directory once lightning is bootstrapped

	// check if we have channel.backup file
	_, err = os.Stat(chanBackupFile)
	if err != nil {
		client.log.Infof("Not adding channel.backup to the backup: %v", err)
	} else {
		dir, err := ioutil.TempDir("", "chanelbackup")
		if err != nil {
			return nil, "", err
		}
		destFile := fmt.Sprintf("%v/channel.backup", dir)
		if _, err := copyFile(chanBackupFile, destFile); err != nil {
			return nil, "", err
		}
		files = append(files, destFile)
		client.log.Infof("adding channel.backup to the backup")
	}

	client.log.Infof("extractBackupInfo completd")
	return files, info.IdentityPubkey, nil
}

func (client *Client) breezdbCopy(breezDB *db.DB) (string, error) {
	dir, err := ioutil.TempDir("", "backup")
	if err != nil {
		return "", err
	}
	return client.lightningDB.BackupDb(dir)
}

func copyFile(src, dst string) (int64, error) {
	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func compactWalletDB(walletDBDir string, logger btclog.Logger) error {
	dbName := "wallet.db"
	dbPath := path.Join(walletDBDir, dbName)
	f, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if f.Size() <= 10000000 {
		return nil
	}
	newFile, err := ioutil.TempFile(walletDBDir, "cdb-compact")
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
		logger.Criticalf("Error when renaming the new walletdb file: %v", err)
		return err
	}
	logger.Infof("wallet.db was compacted because it's too big")
	return nil
}
