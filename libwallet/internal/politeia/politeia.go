package politeia

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"decred.org/dcrwallet/v3/errors"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
)

const (
	PoliteiaMainnetHost = "https://proposals.decred.org/api"
	PoliteiaTestnetHost = "https://test-proposals.decred.org/api"

	configDBBkt                  = "politeia_config"
	LastSyncedTimestampConfigKey = "politeia_last_synced_timestamp"
)

type Politeia struct {
	host string
	db   *storm.DB

	// TODO: Check usages of mu, seems not to always be unlocked.
	mu         *sync.RWMutex // Pointer required to avoid copying literal values.
	ctx        context.Context
	cancelSync context.CancelFunc
	client     *politeiaClient

	notificationListenersMu *sync.RWMutex // Pointer required to avoid copying literal values.
	notificationListeners   map[string]ProposalNotificationListener
}

const (
	ProposalCategoryAll int32 = iota + 1
	ProposalCategoryPre
	ProposalCategoryActive
	ProposalCategoryApproved
	ProposalCategoryRejected
	ProposalCategoryAbandoned
)

func New(host string, db *storm.DB) (*Politeia, error) {
	if err := db.Init(&Proposal{}); err != nil {
		log.Errorf("Error initializing politeia database: %s", err.Error())
		return nil, err
	}

	return &Politeia{
		host: host,
		db:   db,

		mu:                      &sync.RWMutex{},
		notificationListenersMu: &sync.RWMutex{},

		notificationListeners: make(map[string]ProposalNotificationListener),
	}, nil
}

func (p *Politeia) saveLastSyncedTimestamp(lastSyncedTimestamp int64) {
	err := p.db.Set(configDBBkt, LastSyncedTimestampConfigKey, &lastSyncedTimestamp)
	if err != nil {
		log.Errorf("error setting config value for key: %s, error: %v", LastSyncedTimestampConfigKey, err)
	}
}

func (p *Politeia) getLastSyncedTimestamp() (lastSyncedTimestamp int64) {
	err := p.db.Get(configDBBkt, LastSyncedTimestampConfigKey, &lastSyncedTimestamp)
	if err != nil && err != storm.ErrNotFound {
		log.Errorf("error reading config value for key: %s, error: %v", LastSyncedTimestampConfigKey, err)
	}
	return lastSyncedTimestamp
}

func (p *Politeia) saveOrOverwiteProposal(proposal *Proposal) error {
	var oldProposal Proposal
	err := p.db.One("Token", proposal.Token, &oldProposal)
	if err != nil && err != storm.ErrNotFound {
		return errors.Errorf("error checking if proposal was already indexed: %s", err.Error())
	}

	if oldProposal.Token != "" {
		// delete old record before saving new (if it exists)
		p.db.DeleteStruct(oldProposal)
	}

	return p.db.Save(proposal)
}

// GetProposalsRaw fetches and returns a proposals from the db
func (p *Politeia) GetProposalsRaw(category int32, offset, limit int32, newestFirst bool) ([]Proposal, error) {
	return p.getProposalsRaw(category, offset, limit, newestFirst, false)
}

func (p *Politeia) getProposalsRaw(category int32, offset, limit int32, newestFirst bool, skipAbandoned bool) ([]Proposal, error) {
	var query storm.Query
	switch category {
	case ProposalCategoryAll:

		if skipAbandoned {
			query = p.db.Select(
				q.Not(q.Eq("Category", ProposalCategoryAbandoned)),
			)
		} else {
			query = p.db.Select(
				q.True(),
			)
		}
	default:
		query = p.db.Select(
			q.Eq("Category", category),
		)
	}

	if offset > 0 {
		query = query.Skip(int(offset))
	}

	if limit > 0 {
		query = query.Limit(int(limit))
	}

	if newestFirst {
		query = query.OrderBy("PublishedAt").Reverse()
	} else {
		query = query.OrderBy("PublishedAt")
	}

	var proposals []Proposal
	err := query.Find(&proposals)
	if err != nil && err != storm.ErrNotFound {
		return nil, fmt.Errorf("error fetching proposals: %s", err.Error())
	}

	return proposals, nil
}

// GetProposals returns the result of GetProposalsRaw as a JSON string
func (p *Politeia) GetProposals(category int32, offset, limit int32, newestFirst bool) (string, error) {
	result, err := p.GetProposalsRaw(category, offset, limit, newestFirst)
	if err != nil {
		return "", err
	}

	if len(result) == 0 {
		return "[]", nil
	}

	response, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("error marshalling result: %s", err.Error())
	}

	return string(response), nil
}

// GetProposalRaw fetches and returns a single proposal specified by it's censorship record token
func (p *Politeia) GetProposalRaw(censorshipToken string) (*Proposal, error) {
	var proposal Proposal
	err := p.db.One("Token", censorshipToken, &proposal)
	if err != nil {
		return nil, err
	}

	return &proposal, nil
}

// GetProposal returns the result of GetProposalRaw as a JSON string
func (p *Politeia) GetProposal(censorshipToken string) (string, error) {
	return p.marshalResult(p.GetProposalRaw(censorshipToken))
}

// GetProposalByIDRaw fetches and returns a single proposal specified by it's ID
func (p *Politeia) GetProposalByIDRaw(proposalID int) (*Proposal, error) {
	var proposal Proposal
	err := p.db.One("ID", proposalID, &proposal)
	if err != nil {
		return nil, err
	}

	return &proposal, nil
}

// GetProposalByID returns the result of GetProposalByIDRaw as a JSON string
func (p *Politeia) GetProposalByID(proposalID int) (string, error) {
	return p.marshalResult(p.GetProposalByIDRaw(proposalID))
}

// Count returns the number of proposals of a specified category
func (p *Politeia) Count(category int32) (int32, error) {
	var matcher q.Matcher

	if category == ProposalCategoryAll {
		matcher = q.True()
	} else {
		matcher = q.Eq("Category", category)
	}

	count, err := p.db.Select(matcher).Count(&Proposal{})
	if err != nil {
		return 0, err
	}

	return int32(count), nil
}

func (p *Politeia) Overview() (*ProposalOverview, error) {
	pre, err := p.Count(ProposalCategoryPre)
	if err != nil {
		return nil, err
	}

	active, err := p.Count(ProposalCategoryActive)
	if err != nil {
		return nil, err
	}

	approved, err := p.Count(ProposalCategoryApproved)
	if err != nil {
		return nil, err
	}

	rejected, err := p.Count(ProposalCategoryRejected)
	if err != nil {
		return nil, err
	}

	abandoned, err := p.Count(ProposalCategoryApproved)
	if err != nil {
		return nil, err
	}

	return &ProposalOverview{
		All:        pre + active + approved + rejected + abandoned,
		Discussion: pre,
		Voting:     active,
		Approved:   approved,
		Rejected:   rejected,
		Abandoned:  abandoned,
	}, nil
}

func (p *Politeia) ClearSavedProposals() error {
	err := p.db.Drop(&Proposal{})
	if err != nil {
		return translateError(err)
	}

	return p.db.Init(&Proposal{})
}

func (p *Politeia) marshalResult(result interface{}, err error) (string, error) {
	if err != nil {
		return "", translateError(err)
	}

	response, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("error marshalling result: %s", err.Error())
	}

	return string(response), nil
}
