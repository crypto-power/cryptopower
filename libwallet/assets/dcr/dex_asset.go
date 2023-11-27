// This code is available on the terms of the project LICENSE.md file, and as
// terms of the BlueOak License. See: https://blueoakcouncil.org/license/1.0.0.

package dcr

// Note: Most of the code here is a copy-pasta from:
// https://github.com/decred/dcrdex/blob/master/client/asset/dcr/spv.go

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	dexasset "decred.org/dcrdex/client/asset"
	"decred.org/dcrdex/client/asset/dcr"
	"decred.org/dcrdex/dex"
	walleterrors "decred.org/dcrwallet/v3/errors"
	walletjson "decred.org/dcrwallet/v3/rpc/jsonrpc/types"
	dcrwallet "decred.org/dcrwallet/v3/wallet"
	"github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/decred/dcrd/blockchain/stake/v5"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	chainjson "github.com/decred/dcrd/rpc/jsonrpc/types/v4"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
)

const (
	walletTypeSPV = "SPV"
)

// DEXAsset wraps *Asset and implements dexasset.Wallet.
type DEXAsset struct {
	asset *Asset

	dexAccountNumber uint32
	dexAccountName   string
}

// NewDEXAsset returns a new *DEXAsset.
func NewDEXAsset(asset *Asset, accountNumber uint32, accountName string) *DEXAsset {
	return &DEXAsset{
		asset:            asset,
		dexAccountNumber: accountNumber,
		dexAccountName:   accountName,
	}
}

// Connect establishes a connection to the wallet.
// Part of the Wallet interface.
func (da *DEXAsset) Connect(ctx context.Context) error {
	return nil
}

// Disconnect shuts down access to the wallet.
// Part of the Wallet interface.
func (da *DEXAsset) Disconnect() {}

// SpvMode returns true if the wallet is connected to the Decred
// network via SPV peers.
// Part of the Wallet interface.
func (da *DEXAsset) SpvMode() bool {
	return true
}

// NotifyOnTipChange registers a callback function that the should be
// invoked when the wallet sees new mainchain blocks. The return value
// indicates if this notification can be provided. Where this tip change
// notification is unimplemented, monitorBlocks should be used to track
// tip changes. TODO: Use tipFeed if it's exported.
// Part of the Wallet interface.
func (da *DEXAsset) NotifyOnTipChange(ctx context.Context, cb dcr.TipChangeCallback) bool {
	const dexAssetOnTipCallbackIdentifier = "dex-asset-on-tip-change"
	listener := &wallet.TxAndBlockNotificationListener{
		OnBlockAttached: func(walletID int, blockHeight int32) {
			cb(ctx, nil, int64(blockHeight), nil)
		},
	}

	da.asset.RemoveTxAndBlockNotificationListener(dexAssetOnTipCallbackIdentifier) // Clear previous listener if any.
	err := da.asset.AddTxAndBlockNotificationListener(listener, dexAssetOnTipCallbackIdentifier)
	return err != nil // TODO
}

// AddressInfo returns information for the provided address. It is an error
// if the address is not owned by the wallet.
// Part of the Wallet interface.
func (da *DEXAsset) AddressInfo(ctx context.Context, address string) (*dcr.AddressInfo, error) {
	addr, err := stdaddr.DecodeAddress(address, da.asset.chainParams)
	if err != nil {
		return nil, err
	}
	ka, err := da.asset.Internal().DCR.KnownAddress(ctx, addr)
	if err != nil {
		return nil, err
	}

	if ka, ok := ka.(dcrwallet.BIP0044Address); ok {
		_, branch, _ := ka.Path()
		return &dcr.AddressInfo{Account: ka.AccountName(), Branch: branch}, nil
	}
	return nil, fmt.Errorf("unsupported address type %T", ka)
}

// AccountOwnsAddress checks if the provided address belongs to the
// specified account.
// Part of the Wallet interface.
func (da *DEXAsset) AccountOwnsAddress(ctx context.Context, addr stdaddr.Address, acctName string) (bool, error) {
	a, err := da.asset.Internal().DCR.KnownAddress(ctx, addr)
	if err != nil {
		return false, utils.TranslateError(err)
	}

	if a.AccountName() != acctName {
		return false, nil
	}

	if kind := a.AccountKind(); kind != dcrwallet.AccountKindBIP0044 && kind != dcrwallet.AccountKindImported {
		return false, nil
	}

	return true, nil
}

// AccountBalance returns the balance breakdown for the specified account.
// Part of the Wallet interface.
func (da *DEXAsset) AccountBalance(ctx context.Context, confirms int32, _ string) (*walletjson.GetAccountBalanceResult, error) {
	bal, err := da.asset.GetAccountBalance(int32(da.dexAccountNumber))
	if err != nil {
		return nil, err
	}

	return &walletjson.GetAccountBalanceResult{
		AccountName:             da.dexAccountName,
		ImmatureCoinbaseRewards: bal.ImmatureReward.ToCoin(),
		ImmatureStakeGeneration: bal.ImmatureStakeGeneration.ToCoin(),
		LockedByTickets:         bal.LockedByTickets.ToCoin(),
		Spendable:               bal.Spendable.ToCoin(),
		Total:                   bal.Total.ToCoin(),
		Unconfirmed:             bal.UnConfirmed.ToCoin(),
		VotingAuthority:         bal.VotingAuthority.ToCoin(),
	}, nil
}

// LockedOutputs fetches locked outputs for the Wallet.
// Part of the Wallet interface.
func (da *DEXAsset) LockedOutputs(ctx context.Context, acctName string) ([]chainjson.TransactionInput, error) {
	return da.asset.Internal().DCR.LockedOutpoints(ctx, acctName)
}

// Unspents fetches unspent outputs for the Wallet.
// Part of the Wallet interface.
func (da *DEXAsset) Unspents(ctx context.Context, acctName string) ([]*walletjson.ListUnspentResult, error) {
	return da.asset.Internal().DCR.ListUnspent(ctx, 0, math.MaxInt32, nil, da.dexAccountName)
}

// LockUnspent locks or unlocks the specified outpoint.
// Part of the Wallet interface.
func (da *DEXAsset) LockUnspent(ctx context.Context, unlock bool, ops []*wire.OutPoint) error {
	fun := da.asset.Internal().DCR.LockOutpoint
	if unlock {
		fun = da.asset.Internal().DCR.UnlockOutpoint
	}
	for _, op := range ops {
		fun(&op.Hash, op.Index)
	}

	return nil
}

// UnspentOutput returns information about an unspent tx output, if found
// and unspent. Use wire.TxTreeUnknown if the output tree is unknown, the
// correct tree will be returned if the unspent output is found.
// This method is only guaranteed to return results for outputs that pay to
// the wallet, although wallets connected to a full node may return results
// for non-wallet outputs. Returns da.asset.CoinNotFoundError if the unspent
// output cannot be located.
// Part of the Wallet interface.
func (da *DEXAsset) UnspentOutput(ctx context.Context, txHash *chainhash.Hash, index uint32, _ int8) (*dcr.TxOutput, error) {
	w := da.asset.Internal().DCR
	txd, err := dcrwallet.UnstableAPI(w).TxDetails(ctx, txHash)
	if errors.Is(err, walleterrors.NotExist) {
		return nil, dexasset.CoinNotFoundError
	} else if err != nil {
		return nil, err
	}

	details, err := w.ListTransactionDetails(ctx, txHash)
	if err != nil {
		return nil, err
	}

	var addrStr string
	for _, detail := range details {
		if detail.Vout == index {
			addrStr = detail.Address
		}
	}
	if addrStr == "" {
		return nil, fmt.Errorf("error locating address for output")
	}

	tree := wire.TxTreeRegular
	if txd.TxType != stake.TxTypeRegular {
		tree = wire.TxTreeStake
	}

	if len(txd.MsgTx.TxOut) <= int(index) {
		return nil, fmt.Errorf("not enough outputs")
	}

	_, tipHeight := w.MainChainTip(ctx)

	var ours bool
	for _, credit := range txd.Credits {
		if credit.Index == index {
			if credit.Spent {
				return nil, dexasset.CoinNotFoundError
			}
			ours = true
			break
		}
	}

	if !ours {
		return nil, dexasset.CoinNotFoundError
	}

	return &dcr.TxOutput{
		TxOut:         txd.MsgTx.TxOut[index],
		Tree:          tree,
		Addresses:     []string{addrStr},
		Confirmations: uint32(txd.Block.Height - tipHeight + 1),
	}, nil
}

// ExternalAddress returns an external address using GapPolicyIgnore.
// Part of the Wallet interface.
// Using GapPolicyWrap here, introducing a relatively small risk of address
// reuse, but improving wallet recoverability.
func (da *DEXAsset) ExternalAddress(ctx context.Context, _ string) (stdaddr.Address, error) {
	return da.asset.Internal().DCR.NewExternalAddress(ctx, da.dexAccountNumber, dcrwallet.WithGapPolicyWrap())
}

// InternalAddress returns an internal address using GapPolicyIgnore.
// Part of the Wallet interface.
func (da *DEXAsset) InternalAddress(ctx context.Context, _ string) (stdaddr.Address, error) {
	return da.asset.Internal().DCR.NewInternalAddress(ctx, da.dexAccountNumber, dcrwallet.WithGapPolicyWrap())
}

// SignRawTransaction signs the provided transaction. SignRawTransaction is not
// used for redemptions, so previous outpoints and scripts should be known by
// the wallet. SignRawTransaction should not mutate the input transaction.
// Part of the Wallet interface.
func (da *DEXAsset) SignRawTransaction(ctx context.Context, baseTx *wire.MsgTx) (*wire.MsgTx, error) {
	tx := baseTx.Copy()
	sigErrs, err := da.asset.Internal().DCR.SignTransaction(ctx, tx, txscript.SigHashAll, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(sigErrs) > 0 {
		for _, sigErr := range sigErrs {
			log.Errorf("signature error for index %d: %v", sigErr.InputIndex, sigErr.Error)
		}
		return nil, fmt.Errorf("%d signature errors", len(sigErrs))
	}
	return tx, nil
}

// SendRawTransaction broadcasts the provided transaction to the Decred
// network.
// Part of the Wallet interface.
func (da *DEXAsset) SendRawTransaction(ctx context.Context, tx *wire.MsgTx, _ bool) (*chainhash.Hash, error) {
	return da.asset.Internal().DCR.PublishTransaction(ctx, tx, da.asset.syncData.syncer)
}

// GetBestBlock returns the hash and height of the wallet's best block.
// Part of the Wallet interface.
func (da *DEXAsset) GetBestBlock(ctx context.Context) (*chainhash.Hash, int64, error) {
	blockHash, blockHeight := da.asset.Internal().DCR.MainChainTip(ctx)
	return &blockHash, int64(blockHeight), nil
}

// GetBlockHeader generates a *BlockHeader for the specified block hash. The
// returned block header is a wire.BlockHeader with the addition of the
// block's median time.
// Part of the Wallet interface.
func (da *DEXAsset) GetBlockHeader(ctx context.Context, blockHash *chainhash.Hash) (*dcr.BlockHeader, error) {
	w := da.asset.Internal().DCR
	hdr, err := w.BlockHeader(ctx, blockHash)
	if err != nil {
		return nil, err
	}

	medianTime, err := da.medianTime(ctx, hdr)
	if err != nil {
		return nil, err
	}

	// Get next block hash unless there are none.
	var nextHash *chainhash.Hash
	confirmations := int64(-1)
	mainChainHasBlock, _, err := w.BlockInMainChain(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("error checking if block is in mainchain: %w", err)
	}
	if mainChainHasBlock {
		_, tipHeight := w.MainChainTip(ctx)
		if int32(hdr.Height) < tipHeight {
			nextHash, err = da.GetBlockHash(ctx, int64(hdr.Height)+1)
			if err != nil {
				return nil, fmt.Errorf("error getting next hash for block %q: %w", blockHash, err)
			}
		}
		if int32(hdr.Height) <= tipHeight {
			confirmations = int64(tipHeight) - int64(hdr.Height) + 1
		} else { // if tip is less, may be rolling back, so just mock dcrd/dcrwallet
			confirmations = 0
		}
	}

	return &dcr.BlockHeader{
		BlockHeader:   hdr,
		MedianTime:    medianTime,
		Confirmations: confirmations,
		NextHash:      nextHash,
	}, nil
}

// medianTime calculates a blocks median time, which is the median of the
// timestamps of the previous 11 blocks.
func (da *DEXAsset) medianTime(ctx context.Context, iBlkHeader *wire.BlockHeader) (int64, error) {
	// Calculate past median time. Look at the last 11 blocks, starting
	// with the requested block, which is consistent with dcrd.
	const numStamp = 11
	timestamps := make([]int64, 0, numStamp)
	for {
		timestamps = append(timestamps, iBlkHeader.Timestamp.Unix())
		if iBlkHeader.Height == 0 || len(timestamps) == numStamp {
			break
		}
		var err error
		iBlkHeader, err = da.asset.Internal().DCR.BlockHeader(ctx, &iBlkHeader.PrevBlock)
		if err != nil {
			return 0, fmt.Errorf("info not found for previous block: %v", err)
		}
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})
	return timestamps[len(timestamps)/2], nil
}

// GetBlock returns the *wire.MsgBlock.
// Part of the Wallet interface.
func (da *DEXAsset) GetBlock(ctx context.Context, blockHash *chainhash.Hash) (*wire.MsgBlock, error) {
	// TODO: Use a block cache.
	blocks, err := da.asset.syncData.syncer.Blocks(ctx, []*chainhash.Hash{blockHash})
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 { // Shouldn't actually be possible.
		return nil, fmt.Errorf("network returned 0 blocks")
	}
	return blocks[0], nil
}

// GetTransaction returns the details of a wallet tx, if the wallet contains a
// tx with the provided hash. Returns da.asset.CoinNotFoundError if the tx is not
// found in the wallet.
func (da *DEXAsset) GetTransaction(ctx context.Context, txHash *chainhash.Hash) (*dcr.WalletTransaction, error) {
	// copy-pasted from dcrwallet/internal/rpc/jsonrpc/methods.go
	w := da.asset.Internal().DCR
	txd, err := dcrwallet.UnstableAPI(w).TxDetails(ctx, txHash)
	if errors.Is(err, walleterrors.NotExist) {
		return nil, dexasset.CoinNotFoundError
	} else if err != nil {
		return nil, err
	}

	_, tipHeight := w.MainChainTip(ctx)

	var b strings.Builder
	b.Grow(2 * txd.MsgTx.SerializeSize())
	err = txd.MsgTx.Serialize(hex.NewEncoder(&b))
	if err != nil {
		return nil, err
	}

	ret := dcr.WalletTransaction{
		Hex: b.String(),
	}

	if txd.Block.Height != -1 {
		ret.BlockHash = txd.Block.Hash.String()
		ret.Confirmations = int64(tipHeight - txd.Block.Height + 1)
	}

	details, err := w.ListTransactionDetails(ctx, txHash)
	if err != nil {
		return nil, err
	}
	ret.Details = make([]walletjson.GetTransactionDetailsResult, len(details))
	for i, d := range details {
		ret.Details[i] = walletjson.GetTransactionDetailsResult{
			Account:           d.Account,
			Address:           d.Address,
			Amount:            d.Amount,
			Category:          d.Category,
			InvolvesWatchOnly: d.InvolvesWatchOnly,
			Fee:               d.Fee,
			Vout:              d.Vout,
		}
	}

	return &ret, nil
}

// GetRawTransaction returns details of the tx with the provided hash.
// Returns da.asset.CoinNotFoundError if the tx is not found.
// Part of the Wallet interface.
func (da *DEXAsset) GetRawTransaction(ctx context.Context, txHash *chainhash.Hash) (*wire.MsgTx, error) {
	txs, _, err := da.asset.Internal().DCR.GetTransactionsByHashes(ctx, []*chainhash.Hash{txHash})
	if err != nil {
		return nil, err
	}
	if len(txs) != 1 {
		return nil, dexasset.CoinNotFoundError
	}
	return txs[0], nil
}

// GetBlockHash returns the hash of the mainchain block at the specified height.
// Part of the Wallet interface.
func (da *DEXAsset) GetBlockHash(ctx context.Context, blockHeight int64) (*chainhash.Hash, error) {
	info, err := da.asset.Internal().DCR.BlockInfo(ctx, dcrwallet.NewBlockIdentifierFromHeight(int32(blockHeight)))
	if err != nil {
		return nil, err
	}
	return &info.Hash, nil
}

// MatchAnyScript looks for any of the provided scripts in the block specified.
// Part of the Wallet interface.
func (da *DEXAsset) MatchAnyScript(ctx context.Context, blockHash *chainhash.Hash, scripts [][]byte) (bool, error) {
	key, filter, err := da.asset.Internal().DCR.CFilterV2(ctx, blockHash)
	if err != nil {
		return false, err
	}
	return filter.MatchAny(key, scripts), nil
}

// AccountUnlocked returns true if the account is unlocked.
// Part of the Wallet interface.
func (da *DEXAsset) AccountUnlocked(ctx context.Context, _ string) (bool, error) {
	return !da.asset.IsLocked(), nil
}

// LockAccount locks the specified account.
// Part of the Wallet interface.
func (da *DEXAsset) LockAccount(ctx context.Context, _ string) error {
	// Cryptopower does not offer an account by account lock for a single wallet
	// so lock the wallet.
	da.asset.LockWallet()
	return nil
}

// UnlockAccount unlocks the specified account or the wallet if account is not
// encrypted.
// Part of the Wallet interface.
func (da *DEXAsset) UnlockAccount(ctx context.Context, passphrase []byte, _ string) error {
	return da.asset.UnlockWallet(string(passphrase))
}

// SyncStatus returns the wallet's sync status.
// Part of the Wallet interface.
func (da *DEXAsset) SyncStatus(ctx context.Context) (synced bool, progress float32, err error) {
	syncProgress := da.asset.GeneralSyncProgress()
	if syncProgress != nil {
		progress = float32(syncProgress.TotalSyncProgress)
	}
	return da.asset.synced, progress, nil
}

// PeerCount returns the number of network peers to which the wallet or its
// backing node are connected.
// Part of the Wallet interface.
func (da *DEXAsset) PeerCount(ctx context.Context) (uint32, error) {
	return uint32(da.asset.ConnectedPeers()), nil
}

// AddressPrivKey fetches the privkey for the specified address.
// Part of the Wallet interface.
func (da *DEXAsset) AddressPrivKey(ctx context.Context, addr stdaddr.Address) (*secp256k1.PrivateKey, error) {
	privKey, _, err := da.asset.Internal().DCR.LoadPrivateKey(ctx, addr)
	return privKey, err
}

// Part of the Wallet interface.
func (da *DEXAsset) Reconfigure(ctx context.Context, cfg *dexasset.WalletConfig, net dex.Network, currentAddress, depositAccount string) (restart bool, err error) {
	return cfg.Type != walletTypeSPV, nil
}
