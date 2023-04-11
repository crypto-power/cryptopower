package ltc

import (
	"fmt"
	"strings"
	"sync"
	"time"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"

	// "github.com/ltcsuite/ltcwallet/chain"
	// "github.com/ltcsuite/neutrino"
	// "github.com/lightninglabs/neutrino"

	neutrino "github.com/dcrlabs/neutrino-ltc"
	labschain "github.com/dcrlabs/neutrino-ltc/chain"
)

// SyncData holds the data required to sync the wallet.
type SyncData struct {
	mu sync.RWMutex

	bestBlockheight     int32 // Synced peers best block height.
	syncstarted         uint32
	txlistening         uint32
	chainServiceStopped bool

	syncing            bool
	synced             bool
	isRescan           bool
	rescanStartTime    time.Time
	rescanStartHeight  *int32
	isSyncShuttingDown bool

	wg sync.WaitGroup

	// Listeners
	syncProgressListeners map[string]sharedW.SyncProgressListener

	*activeSyncData
}

// reading/writing of properties of this struct are protected by syncData.mu.
type activeSyncData struct {
	syncStage utils.SyncStage

	cfiltersFetchProgress    sharedW.CFiltersFetchProgressReport
	headersFetchProgress     sharedW.HeadersFetchProgressReport
	addressDiscoveryProgress sharedW.AddressDiscoveryProgressReport
	headersRescanProgress    sharedW.HeadersRescanProgressReport
}

const (
	// InvalidSyncStage is the default sync stage.
	InvalidSyncStage = utils.InvalidSyncStage
	// CFiltersFetchSyncStage is the sync stage for fetching cfilters.
	CFiltersFetchSyncStage = utils.CFiltersFetchSyncStage
	// HeadersFetchSyncStage is the sync stage for fetching headers.
	HeadersFetchSyncStage = utils.HeadersFetchSyncStage
	// AddressDiscoverySyncStage is the sync stage for address discovery.
	AddressDiscoverySyncStage = utils.AddressDiscoverySyncStage
	// HeadersRescanSyncStage is the sync stage for headers rescan.
	HeadersRescanSyncStage = utils.HeadersRescanSyncStage
)

func (asset *Asset) initSyncProgressData() {
	cfiltersFetchProgress := sharedW.CFiltersFetchProgressReport{
		GeneralSyncProgress:         &sharedW.GeneralSyncProgress{},
		BeginFetchCFiltersTimeStamp: 0,
		StartCFiltersHeight:         -1,
		CfiltersFetchTimeSpent:      0,
		TotalFetchedCFiltersCount:   0,
	}

	headersFetchProgress := sharedW.HeadersFetchProgressReport{
		GeneralSyncProgress:   &sharedW.GeneralSyncProgress{},
		HeadersFetchTimeSpent: -1,
	}

	addressDiscoveryProgress := sharedW.AddressDiscoveryProgressReport{
		GeneralSyncProgress:       &sharedW.GeneralSyncProgress{},
		AddressDiscoveryStartTime: -1,
		TotalDiscoveryTimeSpent:   -1,
	}

	headersRescanProgress := sharedW.HeadersRescanProgressReport{}
	headersRescanProgress.GeneralSyncProgress = &sharedW.GeneralSyncProgress{}

	asset.syncData.mu.Lock()
	asset.syncData.activeSyncData = &activeSyncData{
		cfiltersFetchProgress:    cfiltersFetchProgress,
		headersFetchProgress:     headersFetchProgress,
		addressDiscoveryProgress: addressDiscoveryProgress,
		headersRescanProgress:    headersRescanProgress,
	}
	asset.syncData.mu.Unlock()
}

// prepareChain sets up the chain service and the chain source
func (asset *Asset) prepareChain() error {
	exists, err := asset.WalletExists()
	if err != nil {
		return fmt.Errorf("error verifying wallet existence: %v", err)
	}
	if !exists {
		return errors.New("wallet not found")
	}

	log.Debug("Starting native LTC wallet sync...")
	chainService, err := asset.loadChainService()
	if err != nil {
		return err
	}

	asset.chainClient = labschain.NewNeutrinoClient(asset.chainParams, chainService, nil) // TODO: Add logger as last param

	return nil
}

func (asset *Asset) loadChainService() (chainService *neutrino.ChainService, err error) {
	// Read config for persistent peers, if set parse and set neutrino's ConnectedPeers
	// persistentPeers.
	var persistentPeers []string
	peerAddresses := asset.ReadStringConfigValueForKey(sharedW.SpvPersistentPeerAddressesConfigKey, "")
	if peerAddresses != "" {
		addresses := strings.Split(peerAddresses, ";")
		for _, address := range addresses {
			peerAddress, err := utils.NormalizeAddress(address, asset.chainParams.DefaultPort)
			if err != nil {
				log.Errorf("SPV peer address(%s) is invalid: %v", peerAddress, err)
			} else {
				persistentPeers = append(persistentPeers, peerAddress)
			}
		}

		if len(persistentPeers) == 0 {
			return chainService, errors.New(utils.ErrInvalidPeers)
		}
	}

	chainService, err = neutrino.NewChainService(neutrino.Config{
		DataDir:       asset.DataDir(),
		Database:      asset.GetWalletDataDb().LTC,
		ChainParams:   *asset.chainParams,
		PersistToDisk: true, // keep cfilter headers on disk for efficient rescanning
		ConnectPeers:  persistentPeers,
		// WARNING: PublishTransaction currently uses the entire duration
		// because if an external bug, but even if the resolved, a typical
		// inv/getdata round trip is ~4 seconds, so we set this so neutrino does
		// not cancel queries too readily.
		BroadcastTimeout: 6 * time.Second,
	})
	if err != nil {
		log.Error(err)
		return nil, fmt.Errorf("couldn't create Neutrino ChainService: %v", err)
	}
	asset.syncData.mu.Lock()
	asset.syncData.chainServiceStopped = false
	asset.syncData.mu.Unlock()

	return chainService, nil
}

// AddSyncProgressListener registers a sync progress listener to the asset.
func (asset *Asset) AddSyncProgressListener(syncProgressListener sharedW.SyncProgressListener, uniqueIdentifier string) error {
	asset.syncData.mu.Lock()
	defer asset.syncData.mu.Unlock()

	if _, ok := asset.syncData.syncProgressListeners[uniqueIdentifier]; ok {
		return errors.New(utils.ErrListenerAlreadyExist)
	}

	asset.syncData.syncProgressListeners[uniqueIdentifier] = syncProgressListener
	return nil
}

// RemoveSyncProgressListener unregisters a sync progress listener from the asset.
func (asset *Asset) RemoveSyncProgressListener(uniqueIdentifier string) {
	log.Error(utils.ErrLTCMethodNotImplemented("RemoveSyncProgressListener"))
}

// CancelSync stops the sync process.
func (asset *Asset) CancelSync() {
	log.Error(utils.ErrLTCMethodNotImplemented("CancelSync"))
}

// SpvSync initiates the full chain sync starting protocols. It attempts to
// restart the chain service if it hasn't been initialized.
func (asset *Asset) SpvSync() (err error) {
	return utils.ErrLTCMethodNotImplemented("SpvSync")
}

// IsConnectedToLitecoinNetwork returns true if the wallet is connected to the
// litecoin network.
func (asset *Asset) IsConnectedToLitecoinNetwork() bool {
	return false
}
