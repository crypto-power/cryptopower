package dcr

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	tkv1 "github.com/decred/politeia/politeiawww/api/ticketvote/v1"
	www "github.com/decred/politeia/politeiawww/api/www/v1"
	"github.com/decred/politeia/politeiawww/client"
)

const (
	PoliteiaMainnetHost = "https://proposals.decred.org/api"
	PoliteiaTestnetHost = "https://test-proposals.decred.org/api"

	ticketVoteApi       = tkv1.APIRoute
	proposalDetailsPath = "/proposals/"

	PoliteiaLastSyncedTimestampConfigKey = "politeia_last_synced_timestamp"
)

var apiPath = www.PoliteiaWWWAPIRoute

func newPoliteiaClient(host string) *politeiaClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	httpClient := &http.Client{
		Transport: tr,
		Timeout:   time.Second * 60,
	}

	return &politeiaClient{
		host:       host,
		httpClient: httpClient,
	}
}

func (p *Politeia) getClient() (*politeiaClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	client := p.Client
	if client == nil {
		client = newPoliteiaClient(p.Host)
		version, err := client.serverVersion()
		if err != nil {
			return nil, err
		}
		client.version = &version

		err = client.loadServerPolicy()
		if err != nil {
			return nil, err
		}

		p.Client = client
	}

	return client, nil
}

func (c *politeiaClient) getRequestBody(method string, body interface{}) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	if method == http.MethodPost {
		if requestBody, ok := body.([]byte); ok {
			return requestBody, nil
		}
	} else if method == http.MethodGet {
		if requestBody, ok := body.(map[string]string); ok {
			params := url.Values{}
			for key, val := range requestBody {
				params.Add(key, val)
			}
			return []byte(params.Encode()), nil
		}
	}

	return nil, errors.New("invalid request body")
}

func (c *politeiaClient) makeRequest(method, apiRoute, path string, body interface{}, dest interface{}) error {
	var err error
	var requestBody []byte

	route := c.host + apiRoute + path
	if body != nil {
		requestBody, err = c.getRequestBody(method, body)
		if err != nil {
			return err
		}
	}

	if method == http.MethodGet && requestBody != nil {
		route += string(requestBody)
	}

	// Create http request
	req, err := http.NewRequest(method, route, nil)
	if err != nil {
		return fmt.Errorf("error creating http request: %s", err.Error())
	}
	if method == http.MethodPost && requestBody != nil {
		req.Body = ioutil.NopCloser(bytes.NewReader(requestBody))
	}

	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}

	// Send request
	r, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		r.Body.Close()
	}()

	responseBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return c.handleError(r.StatusCode, responseBody)
	}

	err = json.Unmarshal(responseBody, dest)
	if err != nil {
		return fmt.Errorf("error unmarshaling response: %s", err.Error())
	}

	return nil
}

func (c *politeiaClient) handleError(statusCode int, responseBody []byte) error {
	switch statusCode {
	case http.StatusNotFound:
		return errors.New("resource not found")
	case http.StatusInternalServerError:
		return errors.New("internal server error")
	case http.StatusForbidden:
		return errors.New(string(responseBody))
	case http.StatusUnauthorized:
		var errResp www.ErrorReply
		if err := json.Unmarshal(responseBody, &errResp); err != nil {
			return err
		}
		return fmt.Errorf("unauthorized: %d", errResp.ErrorCode)
	case http.StatusBadRequest:
		var errResp www.ErrorReply
		if err := json.Unmarshal(responseBody, &errResp); err != nil {
			return err
		}
		return fmt.Errorf("bad request: %d", errResp.ErrorCode)
	}

	return errors.New("unknown error")
}

func (c *politeiaClient) loadServerPolicy() error {
	serverPolicy, err := c.serverPolicy()
	if err != nil {
		return err
	}

	c.policy = &serverPolicy

	return nil
}

func (c *politeiaClient) serverPolicy() (www.PolicyReply, error) {
	var policyReply www.PolicyReply
	err := c.makeRequest(http.MethodGet, apiPath, www.RoutePolicy, nil, &policyReply)
	return policyReply, err
}

func (c *politeiaClient) serverVersion() (www.VersionReply, error) {
	var versionReply www.VersionReply
	err := c.makeRequest(http.MethodGet, apiPath, www.RouteVersion, nil, &versionReply)
	return versionReply, err
}

func (c *politeiaClient) batchProposals(tokens []string) ([]Proposal, error) {
	b, err := json.Marshal(&www.BatchProposals{Tokens: tokens})
	if err != nil {
		return nil, err
	}

	var batchProposalsReply www.BatchProposalsReply

	err = c.makeRequest(http.MethodPost, apiPath, www.RouteBatchProposals, b, &batchProposalsReply)
	if err != nil {
		return nil, err
	}

	proposals := make([]Proposal, len(batchProposalsReply.Proposals))
	for i, proposalRecord := range batchProposalsReply.Proposals {
		proposal := Proposal{
			Token:       proposalRecord.CensorshipRecord.Token,
			Name:        proposalRecord.Name,
			State:       int32(proposalRecord.State),
			Status:      int32(proposalRecord.Status),
			Timestamp:   proposalRecord.Timestamp,
			UserID:      proposalRecord.UserId,
			Username:    proposalRecord.Username,
			NumComments: int32(proposalRecord.NumComments),
			Version:     proposalRecord.Version,
			PublishedAt: proposalRecord.PublishedAt,
		}

		for _, file := range proposalRecord.Files {
			if file.Name == "index.md" {
				proposal.IndexFile = file.Payload
				break
			}
		}

		proposals[i] = proposal
	}

	return proposals, nil
}

func (c *politeiaClient) proposalDetails(token string) (*www.ProposalDetailsReply, error) {

	route := proposalDetailsPath + token

	var proposalDetailsReply www.ProposalDetailsReply
	err := c.makeRequest(http.MethodGet, apiPath, route, nil, &proposalDetailsReply)
	if err != nil {
		return nil, err
	}

	return &proposalDetailsReply, nil
}

func (c *politeiaClient) tokenInventory() (*www.TokenInventoryReply, error) {
	var tokenInventoryReply www.TokenInventoryReply

	err := c.makeRequest(http.MethodGet, apiPath, www.RouteTokenInventory, nil, &tokenInventoryReply)
	if err != nil {
		return nil, err
	}

	return &tokenInventoryReply, nil
}

func (c *politeiaClient) voteDetails(token string) (*tkv1.DetailsReply, error) {

	requestBody, err := json.Marshal(&tkv1.Details{Token: token})
	if err != nil {
		return nil, err
	}

	var dr tkv1.DetailsReply
	err = c.makeRequest(http.MethodPost, ticketVoteApi, tkv1.RouteDetails, requestBody, &dr)
	if err != nil {
		return nil, err
	}

	// Verify VoteDetails.
	err = client.VoteDetailsVerify(*dr.Vote, c.version.PubKey)
	if err != nil {
		return nil, err
	}

	return &dr, nil
}

func (c *politeiaClient) voteResults(token string) (*tkv1.ResultsReply, error) {

	requestBody, err := json.Marshal(&tkv1.Results{Token: token})
	if err != nil {
		return nil, err
	}

	var resultReply tkv1.ResultsReply
	err = c.makeRequest(http.MethodPost, ticketVoteApi, tkv1.RouteResults, requestBody, &resultReply)
	if err != nil {
		return nil, err
	}

	// Verify CastVoteDetails.
	for _, cvd := range resultReply.Votes {
		err = client.CastVoteDetailsVerify(cvd, c.version.PubKey)
		if err != nil {
			return nil, err
		}
	}

	return &resultReply, nil
}

func (c *politeiaClient) batchVoteSummary(tokens []string) (map[string]www.VoteSummary, error) {
	b, err := json.Marshal(&www.BatchVoteSummary{Tokens: tokens})
	if err != nil {
		return nil, err
	}

	var batchVoteSummaryReply www.BatchVoteSummaryReply

	err = c.makeRequest(http.MethodPost, apiPath, www.RouteBatchVoteSummary, b, &batchVoteSummaryReply)
	if err != nil {
		return nil, err
	}

	return batchVoteSummaryReply.Summaries, nil
}

func (c *politeiaClient) sendVotes(votes []tkv1.CastVote) error {
	b, err := json.Marshal(&tkv1.CastBallot{Votes: votes})
	if err != nil {
		return err
	}

	var reply tkv1.CastBallotReply
	err = c.makeRequest(http.MethodPost, ticketVoteApi, tkv1.RouteCastBallot, b, &reply)
	if err != nil {
		return err
	}

	for _, receipt := range reply.Receipts {
		if receipt.ErrorContext != "" {
			return fmt.Errorf(receipt.ErrorContext)
		}
	}
	return nil
}
