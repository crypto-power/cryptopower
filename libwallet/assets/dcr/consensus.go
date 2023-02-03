package dcr

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"

	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/wire"
)

// AgendaStatusType defines the various agenda statuses.
type AgendaStatusType string

const (
	dcrdataAgendasAPIMainnetUrl = "https://dcrdata.decred.org/api/agendas"
	dcrdataAgendasAPITestnetUrl = "https://testnet.decred.org/api/agendas"

	// AgendaStatusUpcoming used to define an agenda yet to vote.
	AgendaStatusUpcoming AgendaStatusType = "upcoming"

	// AgendaStatusInProgress used to define an agenda with voting ongoing.
	AgendaStatusInProgress AgendaStatusType = "in progress"

	// AgendaStatusFailed used to define an agenda when the votes tally does not
	// attain the minimum threshold set. Activation height is not set for such an
	// agenda.
	AgendaStatusFailed AgendaStatusType = "failed"

	// AgendaStatusLockedIn used to define an agenda that has passed after attaining
	// the minimum set threshold.
	AgendaStatusLockedIn AgendaStatusType = "locked in"

	// AgendaStatusFinished used to define an agenda that has finished voting.
	AgendaStatusFinished AgendaStatusType = "finished"

	// UnknownStatus is used when a status string is not recognized.
	UnknownStatus AgendaStatusType = "unknown"
)

func (a AgendaStatusType) String() string {
	switch a {
	case AgendaStatusUpcoming:
		return "upcoming"
	case AgendaStatusInProgress:
		return "in progress"
	case AgendaStatusLockedIn:
		return "locked in"
	case AgendaStatusFailed:
		return "failed"
	case AgendaStatusFinished:
		return "finished"
	default:
		return "unknown"
	}
}

// AgendaStatusFromStr creates an agenda status from a string. If "UnknownStatus"
// is returned then an invalid status string has been passed.
func AgendaStatusFromStr(status string) AgendaStatusType {
	switch strings.ToLower(status) {
	case "defined", "upcoming":
		return AgendaStatusUpcoming
	case "started", "in progress":
		return AgendaStatusInProgress
	case "failed":
		return AgendaStatusFailed
	case "lockedin", "locked in":
		return AgendaStatusLockedIn
	case "active", "finished":
		return AgendaStatusFinished
	default:
		return UnknownStatus
	}
}

// SetVoteChoice sets a voting choice for the specified agenda. If a ticket
// hash is provided, the voting choice is also updated with the VSP controlling
// the ticket. If a ticket hash isn't provided, the vote choice is saved to the
// local wallet database and the VSPs controlling all unspent, unexpired tickets
// are updated to use the specified vote choice.
func (asset *DCRAsset) SetVoteChoice(agendaID, choiceID, hash, passphrase string) error {
	var ticketHash *chainhash.Hash
	if hash != "" {
		hash, err := chainhash.NewHashFromStr(hash)
		if err != nil {
			return fmt.Errorf("inavlid hash: %w", err)
		}
		ticketHash = hash
	}

	// The wallet will need to be unlocked to sign the API
	// request(s) for setting this vote choice with the VSP.
	err := asset.UnlockWallet(passphrase)
	if err != nil {
		return utils.TranslateError(err)
	}
	defer asset.LockWallet()

	ctx, _ := asset.ShutdownContextWithCancel()

	// get choices
	choices, _, err := asset.Internal().DCR.AgendaChoices(ctx, ticketHash) // returns saved prefs for current agendas
	if err != nil {
		return err
	}

	currentChoice := w.AgendaChoice{
		AgendaID: agendaID,
		ChoiceID: "abstain", // default to abstain as current choice if not found in wallet
	}

	for i := range choices {
		if choices[i].AgendaID == agendaID {
			currentChoice.ChoiceID = choices[i].ChoiceID
			break
		}
	}

	newChoice := w.AgendaChoice{
		AgendaID: agendaID,
		ChoiceID: choiceID,
	}

	_, err = asset.Internal().DCR.SetAgendaChoices(ctx, ticketHash, newChoice)
	if err != nil {
		return err
	}

	var vspPreferenceUpdateSuccess bool
	defer func() {
		if !vspPreferenceUpdateSuccess {
			// Updating the agenda voting preference with the vsp failed,
			// revert the locally saved voting preference for the agenda.
			_, revertError := asset.Internal().DCR.SetAgendaChoices(ctx, ticketHash, currentChoice)
			if revertError != nil {
				log.Errorf("unable to revert locally saved voting preference: %v", revertError)
			}
		}
	}()

	// If a ticket hash is provided, set the specified vote choice with
	// the VSP associated with the provided ticket. Otherwise, set the
	// vote choice with the VSPs associated with all "votable" tickets.
	ticketHashes := make([]*chainhash.Hash, 0)
	if ticketHash != nil {
		ticketHashes = append(ticketHashes, ticketHash)
	} else {
		err = asset.Internal().DCR.ForUnspentUnexpiredTickets(ctx, func(hash *chainhash.Hash) error {
			ticketHashes = append(ticketHashes, hash)
			return nil
		})
		if err != nil {
			return fmt.Errorf("unable to fetch hashes for all unspent, unexpired tickets: %v", err)
		}
	}

	// Never return errors from this for loop, so all tickets are tried.
	// The first error will be returned to the caller.
	var firstErr error
	for _, tHash := range ticketHashes {
		vspTicketInfo, err := asset.Internal().DCR.VSPTicketInfo(ctx, tHash)
		if err != nil {
			// Ignore NotExist error, just means the ticket is not
			// registered with a VSP, nothing more to do here.
			if firstErr == nil && !errors.Is(errors.NotExist, err) {
				firstErr = err
			}
			continue // try next tHash
		}

		// Update the vote choice for the ticket with the associated VSP.
		vspClient, err := asset.VSPClient(vspTicketInfo.Host, vspTicketInfo.PubKey)
		if err != nil && firstErr == nil {
			firstErr = err
			continue // try next tHash
		}
		err = vspClient.SetVoteChoice(ctx, tHash, []w.AgendaChoice{newChoice}, nil, nil)
		if err != nil && firstErr == nil {
			firstErr = err
			continue // try next tHash
		}
	}

	vspPreferenceUpdateSuccess = firstErr == nil
	return firstErr
}

// AllVoteAgendas returns all agendas of all stake versions for the active
// network and this version of the software. Also returns any saved vote
// preferences for the agendas of the current stake version. Vote preferences
// for older agendas cannot currently be retrieved.
func (asset *DCRAsset) AllVoteAgendas(hash string, newestFirst bool) ([]*Agenda, error) {
	if asset.chainParams.Deployments == nil {
		return nil, nil // no agendas to return
	}

	var ticketHash *chainhash.Hash
	if hash != "" {
		hash, err := chainhash.NewHashFromStr(hash)
		if err != nil {
			return nil, fmt.Errorf("invalid hash: %w", err)
		}
		ticketHash = hash
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	choices, _, err := asset.Internal().DCR.AgendaChoices(ctx, ticketHash) // returns saved prefs for current agendas
	if err != nil {
		return nil, err
	}

	// Check for all agendas from the intital stake version to the
	// current stake version, in order to fetch legacy agendas.
	deployments := make([]chaincfg.ConsensusDeployment, 0)
	var i uint32
	for i = 1; i <= voteVersion(asset.chainParams); i++ {
		deployments = append(deployments, asset.chainParams.Deployments[i]...)
	}

	// Fetch high level agenda detail form dcrdata api.
	var dcrdataAgenda []DcrdataAgenda
	host := dcrdataAgendasAPIMainnetUrl
	if asset.chainParams.Net == wire.TestNet3 {
		host = dcrdataAgendasAPITestnetUrl
	}

	req := &utils.ReqConfig{
		Method:  http.MethodGet,
		HttpUrl: host,
	}

	if _, err = utils.HttpRequest(req, &dcrdataAgenda); err != nil {
		return nil, err
	}

	agendas := make([]*Agenda, len(deployments))
	var status string
	for i := range deployments {
		d := &deployments[i]

		votingPreference := "abstain" // assume abstain, if we have the saved pref, it'll be updated below
		for c := range choices {
			if choices[c].AgendaID == d.Vote.Id {
				votingPreference = choices[c].ChoiceID
				break
			}
		}

		for j := range dcrdataAgenda {
			if dcrdataAgenda[j].Name == d.Vote.Id {
				status = AgendaStatusFromStr(dcrdataAgenda[j].Status).String()
			}
		}

		agendas[i] = &Agenda{
			AgendaID:         d.Vote.Id,
			Description:      d.Vote.Description,
			Mask:             uint32(d.Vote.Mask),
			Choices:          d.Vote.Choices,
			VotingPreference: votingPreference,
			StartTime:        int64(d.StartTime),
			ExpireTime:       int64(d.ExpireTime),
			Status:           status,
		}
	}

	if newestFirst {
		sort.Slice(agendas, func(i, j int) bool {
			return agendas[i].StartTime > agendas[j].StartTime
		})
	}
	return agendas, nil
}
