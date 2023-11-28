package politeia

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/crypto-power/cryptopower/libwallet/utils"
	tkv1 "github.com/decred/politeia/politeiawww/api/ticketvote/v1"
	www "github.com/decred/politeia/politeiawww/api/www/v1"
	"github.com/decred/politeia/politeiawww/client"
)

type politeiaClient struct {
	host    string
	version *www.VersionReply
	policy  *www.PolicyReply
	cookies []*http.Cookie
}

type metadataProposal struct {
	Name   string `json:"name"`
	LinkBy int64  `json:"linkby"`
	LinkTo string `json:"linkto"`
}

const (
	ticketVoteAPI       = tkv1.APIRoute
	proposalDetailsPath = "/proposals/"
)

var apiPath = www.PoliteiaWWWAPIRoute

func (p *Politeia) getClient() error {
	if p.client == nil {
		p.client = &politeiaClient{host: p.host}
		if err := p.client.serverVersion(); err != nil {
			return err
		}

		if err := p.client.serverPolicy(); err != nil {
			return err
		}
	}
	return nil
}

func (c *politeiaClient) makeRequest(method, apiRoute, path string, body interface{}, dest interface{}) error {
	req := &utils.ReqConfig{
		Payload:   body,
		Method:    method,
		HTTPURL:   c.host + apiRoute + path,
		IsRetByte: true,
		Cookies:   c.cookies,
	}

	respBytes := []byte{}
	_, err := utils.HTTPRequest(req, &respBytes)
	if err != nil {
		return err
	}

	err = json.Unmarshal(respBytes, dest)
	if err != nil {
		return fmt.Errorf("error unmarshaling response: %s", err.Error())
	}

	return nil
}

func (c *politeiaClient) serverPolicy() error {
	var policyReply www.PolicyReply
	err := c.makeRequest(http.MethodGet, apiPath, www.RoutePolicy, nil, &policyReply)
	c.policy = &policyReply
	return err
}

func (c *politeiaClient) serverVersion() error {
	var versionReply www.VersionReply
	err := c.makeRequest(http.MethodGet, apiPath, www.RouteVersion, nil, &versionReply)
	c.version = &versionReply
	return err
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

		for _, meta := range proposalRecord.Metadata {
			if meta.Hint == "proposalmetadata" {
				proposal.Type = getProposalType(meta.Payload)
				break
			}
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

func getProposalType(payload string) ProposalType {
	decodedBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		fmt.Println("error decoding proposalmetadata:", err)
		return ProposalTypeNormal
	}

	var meta metadataProposal
	err = json.Unmarshal(decodedBytes, &meta)
	if err != nil {
		log.Error("error unmarshalling metadataProposal:", err)
	}

	if meta.LinkTo != "" {
		return ProposalTypeRFPSubmission
	} else if meta.LinkBy != 0 {
		return ProposalTypeRFPProposal
	} else {
		return ProposalTypeNormal
	}
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

func (c *politeiaClient) getInventory() (*www.TokenInventoryReply, error) {
	var tokenInventoryReply www.TokenInventoryReply
	err := c.makeRequest(http.MethodGet, apiPath, tkv1.RouteInventory, nil, &tokenInventoryReply)
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
	err = c.makeRequest(http.MethodPost, ticketVoteAPI, tkv1.RouteDetails, requestBody, &dr)
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
	err = c.makeRequest(http.MethodPost, ticketVoteAPI, tkv1.RouteResults, requestBody, &resultReply)
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
	err = c.makeRequest(http.MethodPost, ticketVoteAPI, tkv1.RouteCastBallot, b, &reply)
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
