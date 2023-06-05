package dcr

import (
	"encoding/hex"
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	"gitlab.com/cryptopower/cryptopower/libwallet/utils"

	"github.com/decred/dcrd/blockchain/stake/v4"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// SetTreasuryPolicy saves the voting policy for treasury spends by a particular
// PI key.
// If a ticket hash is provided, the voting policy is also updated with the VSP
// controlling the ticket. If a ticket hash isn't provided, the vote choice is
// saved to the local wallet database and the VSPs controlling all unspent,
// unexpired tickets are updated to use the specified vote policy.
func (asset *DCRAsset) SetTreasuryPolicy(PiKey, newVotingPolicy, tixHash string, passphrase string) error {
	if !asset.WalletOpened() {
		return utils.ErrDCRNotInitialized
	}

	var ticketHash *chainhash.Hash
	if tixHash != "" {
		tixHash, err := chainhash.NewHashFromStr(tixHash)
		if err != nil {
			return fmt.Errorf("invalid ticket hash: %w", err)
		}
		ticketHash = tixHash
	}

	pikey, err := hex.DecodeString(PiKey)
	if err != nil {
		return fmt.Errorf("invalid pikey: %w", err)
	}
	if len(pikey) != secp256k1.PubKeyBytesLenCompressed {
		return fmt.Errorf("treasury pikey must be %d bytes", secp256k1.PubKeyBytesLenCompressed)
	}

	var policy stake.TreasuryVoteT
	switch newVotingPolicy {
	case "abstain", "invalid", "":
		policy = stake.TreasuryVoteInvalid
	case "yes":
		policy = stake.TreasuryVoteYes
	case "no":
		policy = stake.TreasuryVoteNo
	default:
		return fmt.Errorf("invalid policy: unknown policy %q", newVotingPolicy)
	}

	// The wallet will need to be unlocked to sign the API
	// request(s) for setting this voting policy with the VSP.
	err = asset.UnlockWallet(passphrase)
	if err != nil {
		return utils.TranslateError(err)
	}
	defer asset.LockWallet()

	currentVotingPolicy := asset.Internal().DCR.TreasuryKeyPolicy(pikey, ticketHash)

	ctx, _ := asset.ShutdownContextWithCancel()
	err = asset.Internal().DCR.SetTreasuryKeyPolicy(ctx, pikey, policy, ticketHash)
	if err != nil {
		return err
	}

	var vspPreferenceUpdateSuccess bool
	defer func() {
		if !vspPreferenceUpdateSuccess {
			// Updating the treasury spend voting preference with the vsp failed,
			// revert the locally saved voting preference for the treasury spend.
			revertError := asset.Internal().DCR.SetTreasuryKeyPolicy(ctx, pikey, currentVotingPolicy, ticketHash)
			if revertError != nil {
				log.Errorf("unable to revert locally saved voting preference: %v", revertError)
			}
		}
	}()

	// If a ticket hash is provided, set the specified vote policy with
	// the VSP associated with the provided ticket. Otherwise, set the
	// vote policy with the VSPs associated with all "votable" tickets.
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
	// Update voting preferences on VSPs if required.
	policyMap := map[string]string{
		PiKey: newVotingPolicy,
	}
	for _, tHash := range ticketHashes {
		vspTicketInfo, err := asset.Internal().DCR.VSPTicketInfo(ctx, tHash)
		if err != nil {
			// Ignore NotExist error, just means the ticket is not
			// registered with a VSP, nothing more to do here.
			if firstErr == nil && !errors.Is(err, errors.NotExist) {
				firstErr = err
			}
			continue // try next tHash
		}

		// Update the vote policy for the ticket with the associated VSP.
		vspClient, err := asset.VSPClient(vspTicketInfo.Host, vspTicketInfo.PubKey)
		if err != nil && firstErr == nil {
			firstErr = err
			continue // try next tHash
		}
		err = vspClient.SetVoteChoice(ctx, tHash, nil, nil, policyMap)
		if err != nil && firstErr == nil {
			firstErr = err
			continue // try next tHash
		}
	}

	vspPreferenceUpdateSuccess = firstErr == nil
	return firstErr
}

// TreasuryPolicies returns saved voting policies for treasury spends
// per pi key. If a pi key is specified, the policy for that pi key
// is returned; otherwise the policies for all pi keys are returned.
// If a ticket hash is provided, the policy(ies) for that ticket
// is/are returned.
func (asset *DCRAsset) TreasuryPolicies(PiKey, tixHash string) ([]*TreasuryKeyPolicy, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrDCRNotInitialized
	}

	var ticketHash *chainhash.Hash
	if tixHash != "" {
		tixHash, err := chainhash.NewHashFromStr(tixHash)
		if err != nil {
			return nil, fmt.Errorf("inavlid hash: %w", err)
		}
		ticketHash = tixHash
	}

	if PiKey != "" {
		pikey, err := hex.DecodeString(PiKey)
		if err != nil {
			return nil, fmt.Errorf("invalid pikey: %w", err)
		}
		var policy string
		switch asset.Internal().DCR.TreasuryKeyPolicy(pikey, ticketHash) {
		case stake.TreasuryVoteYes:
			policy = "yes"
		case stake.TreasuryVoteNo:
			policy = "no"
		default:
			policy = "abstain"
		}
		res := []*TreasuryKeyPolicy{
			{
				TicketHash: tixHash,
				PiKey:      PiKey,
				Policy:     policy,
			},
		}
		return res, nil
	}

	policies := asset.Internal().DCR.TreasuryKeyPolicies()
	res := make([]*TreasuryKeyPolicy, len(policies))
	for i := range policies {
		var policy string
		switch policies[i].Policy {
		case stake.TreasuryVoteYes:
			policy = "yes"
		case stake.TreasuryVoteNo:
			policy = "no"
		}
		r := &TreasuryKeyPolicy{
			PiKey:  hex.EncodeToString(policies[i].PiKey),
			Policy: policy,
		}
		if policies[i].Ticket != nil {
			r.TicketHash = policies[i].Ticket.String()
		}
		res[i] = r
	}
	return res, nil
}
