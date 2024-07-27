package dcr

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"decred.org/dcrwallet/v4/errors"
	"github.com/asdine/storm"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/wire"
)

const (
	configDBBkt                  = "consensus_agenda_config"
	LastSyncedTimestampConfigKey = "consensus_agenda_last_synced_timestamp"
)

type consensusAgendaSyncCallback func(status utils.AgendaSyncStatus)

type ConsensusAgenda struct {
	db          *storm.DB
	chainParams *chaincfg.Params

	mu         *sync.RWMutex // Pointer required to avoid copying literal values.
	ctx        context.Context
	cancelSync context.CancelFunc

	syncCallbacksMtx *sync.RWMutex // Pointer required to avoid copying literal values.
	syncCallbacks    map[string]consensusAgendaSyncCallback
}

func NewConsensusAgenda(chainParams *chaincfg.Params, db *storm.DB) *ConsensusAgenda {
	return &ConsensusAgenda{
		chainParams:      chainParams,
		db:               db,
		mu:               &sync.RWMutex{},
		syncCallbacksMtx: &sync.RWMutex{},
		syncCallbacks:    make(map[string]consensusAgendaSyncCallback),
	}
}

func (c *ConsensusAgenda) saveLastSyncedTimestamp(lastSyncedTimestamp int64) {
	err := c.db.Set(configDBBkt, LastSyncedTimestampConfigKey, &lastSyncedTimestamp)
	if err != nil {
		log.Errorf("error setting config value for key: %s, error: %v", LastSyncedTimestampConfigKey, err)
	}
}

func (c *ConsensusAgenda) GetLastSyncedTimestamp() (lastSyncedTimestamp int64) {
	err := c.db.Get(configDBBkt, LastSyncedTimestampConfigKey, &lastSyncedTimestamp)
	if err != nil && err != storm.ErrNotFound {
		log.Errorf("error reading config value for key: %s, error: %v", LastSyncedTimestampConfigKey, err)
	}
	return lastSyncedTimestamp
}

func (c *ConsensusAgenda) saveOrOverwiteAgenda(dataAgenda *DcrdataAgenda) error {
	var oldAgenda DcrdataAgenda
	err := c.db.One("Name", dataAgenda.Name, &oldAgenda)
	if err != nil && err != storm.ErrNotFound {
		return errors.Errorf("error checking if agenda was already indexed: %s", err.Error())
	}

	if oldAgenda.Name != "" {
		// delete old record before saving new (if it exists)
		err = c.db.DeleteStruct(oldAgenda)
		if err != nil {
			return err
		}
	}

	return c.db.Save(dataAgenda)
}

// GetAgendaRaw fetches and returns a agenda from the db
func (c *ConsensusAgenda) getAgendaRaw(offset, limit int32, newestFirst bool) ([]DcrdataAgenda, error) {
	query := c.db.Select()

	if offset > 0 {
		query = query.Skip(int(offset))
	}

	if limit > 0 {
		query = query.Limit(int(limit))
	}

	if newestFirst {
		query = query.OrderBy("StartTime").Reverse()
	} else {
		query = query.OrderBy("StartTime")
	}

	var agendas []DcrdataAgenda
	err := query.Find(&agendas)
	if err != nil && err != storm.ErrNotFound {
		return nil, fmt.Errorf("error fetching agendas: %s", err.Error())
	}

	return agendas, nil
}

func (c *ConsensusAgenda) getAgendaDataFromURL() ([]*DcrdataAgenda, error) {
	// Fetch high level agenda detail form dcrdata api.
	var dcrdataAgenda []*DcrdataAgenda
	host := dcrdataAgendasAPIMainnetURL
	if c.chainParams.Net == wire.TestNet3 {
		host = dcrdataAgendasAPITestnetURL
	}

	req := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: host,
	}

	if _, err := utils.HTTPRequest(req, &dcrdataAgenda); err != nil {
		return nil, err
	}
	return dcrdataAgenda, nil
}

func (c *ConsensusAgenda) IsSyncing() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cancelSync != nil
}

func (c *ConsensusAgenda) StopSync() {
	c.mu.Lock()
	if c.cancelSync != nil {
		c.cancelSync()
		c.cancelSync = nil
	}
	c.mu.Unlock()
	log.Info("Consensus agenda sync: stopped")
}

func (c *ConsensusAgenda) AllVoteAgendas(chainParams *chaincfg.Params, newestFirst bool) ([]*Agenda, error) {
	if chainParams.Deployments == nil {
		return nil, nil // no agendas to return
	}

	// Check for all agendas from the intital stake version to the
	// current stake version, in order to fetch legacy agendas.
	deployments := make([]chaincfg.ConsensusDeployment, 0)
	for _, v := range chainParams.Deployments {
		deployments = append(deployments, v...)
	}

	dcrdataAgenda, err := c.getAgendaRaw(0, 0, newestFirst) // get all agenda from db
	if err != nil {
		return nil, err
	}

	agendaStatuses := make(map[string]string, len(dcrdataAgenda))
	for _, agenda := range dcrdataAgenda {
		agendaStatuses[agenda.Name] = AgendaStatusFromStr(agenda.Status).String()
	}

	agendas := make([]*Agenda, len(deployments))
	for i := range deployments {
		d := &deployments[i]

		agendas[i] = &Agenda{
			AgendaID:         d.Vote.Id,
			Description:      d.Vote.Description,
			Mask:             uint32(d.Vote.Mask),
			Choices:          d.Vote.Choices,
			VotingPreference: "", // this value can be updated after reading a selected wallet's preferences
			StartTime:        int64(d.StartTime),
			ExpireTime:       int64(d.ExpireTime),
			Status:           agendaStatuses[d.Vote.Id],
		}
	}
	sort.Slice(agendas, func(i, j int) bool {
		if newestFirst {
			return agendas[i].StartTime > agendas[j].StartTime
		}
		return agendas[i].StartTime < agendas[j].StartTime
	})
	return agendas, nil
}

func (c *ConsensusAgenda) AddSyncCallback(syncCallback consensusAgendaSyncCallback, uniqueIdentifier string) error {
	c.syncCallbacksMtx.Lock()
	defer c.syncCallbacksMtx.Unlock()

	if _, ok := c.syncCallbacks[uniqueIdentifier]; ok {
		return errors.New(ErrListenerAlreadyExist)
	}

	c.syncCallbacks[uniqueIdentifier] = syncCallback
	return nil
}

func (c *ConsensusAgenda) RemoveSyncCallback(uniqueIdentifier string) {
	c.syncCallbacksMtx.Lock()
	defer c.syncCallbacksMtx.Unlock()

	delete(c.syncCallbacks, uniqueIdentifier)
}

// Sync fetches all agenda from the server saving them to the db
func (c *ConsensusAgenda) Sync(ctx context.Context) error {
	c.mu.RLock()
	if c.cancelSync != nil {
		c.mu.RUnlock()
		return errors.New(ErrSyncAlreadyInProgress)
	}

	c.ctx, c.cancelSync = context.WithCancel(ctx)
	defer func() {
		c.cancelSync = nil
	}()
	c.mu.RUnlock()

	log.Info("Consensus Agenda sync: started")
	// Check if agenda has been shutdown and exit if true.
	if c.ctx.Err() != nil {
		return c.ctx.Err()
	}

	agendaDatas, err := c.getAgendaDataFromURL()
	if err != nil {
		log.Errorf("Error fetching for agenda server policy: %v", err)
		return err
	}
	for _, agendaData := range agendaDatas {
		if err := c.saveOrOverwiteAgenda(agendaData); err != nil {
			log.Errorf("Error saving agenda: %v", err)
			return err
		}
	}

	log.Info("Consensus Agenda sync: update complete")
	c.saveLastSyncedTimestamp(time.Now().Unix())
	c.publishSynced()
	return nil
}

func (c *ConsensusAgenda) publishSynced() {
	c.syncCallbacksMtx.Lock()
	defer c.syncCallbacksMtx.Unlock()

	for _, syncCallback := range c.syncCallbacks {
		syncCallback(utils.AgendaStatusSynced)
	}
}
