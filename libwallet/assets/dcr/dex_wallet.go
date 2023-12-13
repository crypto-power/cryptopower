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
	dexdcr "decred.org/dcrdex/client/asset/dcr"
	"decred.org/dcrdex/dex"
	walleterrors "decred.org/dcrwallet/v3/errors"
	walletjson "decred.org/dcrwallet/v3/rpc/jsonrpc/types"
	dcrwallet "decred.org/dcrwallet/v3/wallet"
	"github.com/decred/dcrd/blockchain/stake/v5"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	chainjson "github.com/decred/dcrd/rpc/jsonrpc/types/v4"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
)

// DEXWallet wraps *Asset and implements dexdcr.Wallet.
type DEXWallet struct {
	wallet   *dcrwallet.Wallet
	syncData *SyncData
}

var _ dexdcr.Wallet = (*DEXWallet)(nil)

// NewDEXWallet returns a new *DEXWallet.
func NewDEXWallet(w *dcrwallet.Wallet, syncData *SyncData) *DEXWallet {
	return &DEXWallet{
		wallet:   w,
		syncData: syncData,
	}
}

// Connect establishes a connection to the wallet.
// Part of the Wallet interface.
func (dw *DEXWallet) Connect(_ context.Context) error {
	return nil
}

// Disconnect shuts down access to the wallet.
// Part of the Wallet interface.
func (dw *DEXWallet) Disconnect() {}

// SpvMode returns true if the wallet is connected to the Decred
// network via SPV peers.
// Part of the Wallet interface.
func (dw *DEXWallet) SpvMode() bool {
	return true
}

// NotifyOnTipChange is not used, in favor of the tipNotifier pattern. See:
// https://github.com/decred/dcrdex/blob/master/client/asset/dcr/spv.go#513.
// Part of the Wallet interface.
func (dw *DEXWallet) NotifyOnTipChange(_ context.Context, _ dexdcr.TipChangeCallback) bool {
	return false
}

// AddressInfo returns information for the provided address. It is an error
// if the address is not owned by the wallet.
// Part of the Wallet interface.
func (dw *DEXWallet) AddressInfo(ctx context.Context, address string) (*dexdcr.AddressInfo, error) {
	addr, err := stdaddr.DecodeAddress(address, dw.wallet.ChainParams())
	if err != nil {
		return nil, err
	}
	ka, err := dw.wallet.KnownAddress(ctx, addr)
	if err != nil {
		return nil, err
	}

	if ka, ok := ka.(dcrwallet.BIP0044Address); ok {
		_, branch, _ := ka.Path()
		return &dexdcr.AddressInfo{Account: ka.AccountName(), Branch: branch}, nil
	}
	return nil, fmt.Errorf("unsupported address type %T", ka)
}

// AccountOwnsAddress checks if the provided address belongs to the
// specified account.
// Part of the Wallet interface.
func (dw *DEXWallet) AccountOwnsAddress(ctx context.Context, addr stdaddr.Address, acctName string) (bool, error) {
	ka, err := dw.wallet.KnownAddress(ctx, addr)
	if err != nil {
		if errors.Is(err, walleterrors.NotExist) {
			return false, nil
		}
		return false, fmt.Errorf("KnownAddress error: %w", err)
	}
	if ka.AccountName() != acctName {
		return false, nil
	}
	if kind := ka.AccountKind(); kind != dcrwallet.AccountKindBIP0044 && kind != dcrwallet.AccountKindImported {
		return false, nil
	}
	return true, nil
}

// AccountBalance returns the balance breakdown for the specified account.
// Part of the Wallet interface.
func (dw *DEXWallet) AccountBalance(ctx context.Context, confirms int32, accountName string) (*walletjson.GetAccountBalanceResult, error) {
	accountNumber, err := dw.wallet.AccountNumber(ctx, accountName)
	if err != nil {
		return nil, err
	}
	bal, err := dw.wallet.AccountBalance(ctx, accountNumber, confirms)
	if err != nil {
		return nil, err
	}

	return &walletjson.GetAccountBalanceResult{
		AccountName:             accountName,
		ImmatureCoinbaseRewards: bal.ImmatureCoinbaseRewards.ToCoin(),
		ImmatureStakeGeneration: bal.ImmatureStakeGeneration.ToCoin(),
		LockedByTickets:         bal.LockedByTickets.ToCoin(),
		Spendable:               bal.Spendable.ToCoin(),
		Total:                   bal.Total.ToCoin(),
		Unconfirmed:             bal.Unconfirmed.ToCoin(),
		VotingAuthority:         bal.VotingAuthority.ToCoin(),
	}, nil
}

// LockedOutputs fetches locked outputs for the Wallet.
// Part of the Wallet interface.
func (dw *DEXWallet) LockedOutputs(ctx context.Context, accountName string) ([]chainjson.TransactionInput, error) {
	return dw.wallet.LockedOutpoints(ctx, accountName)
}

// Unspents fetches unspent outputs for the Wallet.
// Part of the Wallet interface.
func (dw *DEXWallet) Unspents(ctx context.Context, accountName string) ([]*walletjson.ListUnspentResult, error) {
	return dw.wallet.ListUnspent(ctx, 0, math.MaxInt32, nil, accountName)
}

// LockUnspent locks or unlocks the specified outpoint.
// Part of the Wallet interface.
func (dw *DEXWallet) LockUnspent(_ context.Context, unlock bool, ops []*wire.OutPoint) error {
	fun := dw.wallet.LockOutpoint
	if unlock {
		fun = dw.wallet.UnlockOutpoint
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
// for non-wallet outputs. Returns dw.wallet.CoinNotFoundError if the unspent
// output cannot be located.
// Part of the Wallet interface.
func (dw *DEXWallet) UnspentOutput(ctx context.Context, txHash *chainhash.Hash, index uint32, _ int8) (*dexdcr.TxOutput, error) {
	txd, err := dcrwallet.UnstableAPI(dw.wallet).TxDetails(ctx, txHash)
	if errors.Is(err, walleterrors.NotExist) {
		return nil, dexasset.CoinNotFoundError
	} else if err != nil {
		return nil, err
	}

	details, err := dw.wallet.ListTransactionDetails(ctx, txHash)
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

	_, tipHeight := dw.wallet.MainChainTip(ctx)

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

	return &dexdcr.TxOutput{
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
func (dw *DEXWallet) ExternalAddress(ctx context.Context, accountName string) (stdaddr.Address, error) {
	acctNum, err := dw.wallet.AccountNumber(ctx, accountName)
	if err != nil {
		return nil, err
	}
	return dw.wallet.NewExternalAddress(ctx, acctNum, dcrwallet.WithGapPolicyWrap())
}

// InternalAddress returns an internal address using GapPolicyIgnore.
// Part of the Wallet interface.
func (dw *DEXWallet) InternalAddress(ctx context.Context, accountName string) (stdaddr.Address, error) {
	acctNum, err := dw.wallet.AccountNumber(ctx, accountName)
	if err != nil {
		return nil, err
	}
	return dw.wallet.NewInternalAddress(ctx, acctNum, dcrwallet.WithGapPolicyWrap())
}

// SignRawTransaction signs the provided transaction. SignRawTransaction is not
// used for redemptions, so previous outpoints and scripts should be known by
// the wallet. SignRawTransaction should not mutate the input transaction.
// Part of the Wallet interface.
func (dw *DEXWallet) SignRawTransaction(ctx context.Context, baseTx *wire.MsgTx) (*wire.MsgTx, error) {
	tx := baseTx.Copy()
	sigErrs, err := dw.wallet.SignTransaction(ctx, tx, txscript.SigHashAll, nil, nil, nil)
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
func (dw *DEXWallet) SendRawTransaction(ctx context.Context, tx *wire.MsgTx, _ bool) (*chainhash.Hash, error) {
	// TODO: Conditional high fee check?
	return dw.wallet.PublishTransaction(ctx, tx, dw.syncData.syncer)
}

// GetBestBlock returns the hash and height of the wallet's best block.
// Part of the Wallet interface.
func (dw *DEXWallet) GetBestBlock(ctx context.Context) (*chainhash.Hash, int64, error) {
	blockHash, blockHeight := dw.wallet.MainChainTip(ctx)
	return &blockHash, int64(blockHeight), nil
}

// GetBlockHeader generates a *BlockHeader for the specified block hash. The
// returned block header is a wire.BlockHeader with the addition of the
// block's median time.
// Part of the Wallet interface.
func (dw *DEXWallet) GetBlockHeader(ctx context.Context, blockHash *chainhash.Hash) (*dexdcr.BlockHeader, error) {
	hdr, err := dw.wallet.BlockHeader(ctx, blockHash)
	if err != nil {
		return nil, err
	}

	medianTime, err := dw.medianTime(ctx, hdr)
	if err != nil {
		return nil, err
	}

	// Get next block hash unless there are none.
	var nextHash *chainhash.Hash
	confirmations := int64(-1)
	mainChainHasBlock, _, err := dw.wallet.BlockInMainChain(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("error checking if block is in mainchain: %w", err)
	}
	if mainChainHasBlock {
		_, tipHeight := dw.wallet.MainChainTip(ctx)
		if int32(hdr.Height) < tipHeight {
			nextHash, err = dw.GetBlockHash(ctx, int64(hdr.Height)+1)
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

	return &dexdcr.BlockHeader{
		BlockHeader:   hdr,
		MedianTime:    medianTime,
		Confirmations: confirmations,
		NextHash:      nextHash,
	}, nil
}

// medianTime calculates a blocks median time, which is the median of the
// timestamps of the previous 11 blocks.
func (dw *DEXWallet) medianTime(ctx context.Context, iBlkHeader *wire.BlockHeader) (int64, error) {
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
		iBlkHeader, err = dw.wallet.BlockHeader(ctx, &iBlkHeader.PrevBlock)
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
func (dw *DEXWallet) GetBlock(ctx context.Context, blockHash *chainhash.Hash) (*wire.MsgBlock, error) {
	// TODO: Use a block cache.
	blocks, err := dw.syncData.syncer.Blocks(ctx, []*chainhash.Hash{blockHash})
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 { // Shouldn't actually be possible.
		return nil, fmt.Errorf("network returned 0 blocks")
	}
	return blocks[0], nil
}

// GetTransaction returns the details of a wallet tx, if the wallet contains a
// tx with the provided hash. Returns dw.wallet. CoinNotFoundError if the tx is not
// found in the wallet.
func (dw *DEXWallet) GetTransaction(ctx context.Context, txHash *chainhash.Hash) (*dexdcr.WalletTransaction, error) {
	// copy-pasted from dcrwallet/internal/rpc/jsonrpc/methods.go
	txd, err := dcrwallet.UnstableAPI(dw.wallet).TxDetails(ctx, txHash)
	if errors.Is(err, walleterrors.NotExist) {
		return nil, dexasset.CoinNotFoundError
	} else if err != nil {
		return nil, err
	}

	_, tipHeight := dw.wallet.MainChainTip(ctx)

	var b strings.Builder
	b.Grow(2 * txd.MsgTx.SerializeSize())
	err = txd.MsgTx.Serialize(hex.NewEncoder(&b))
	if err != nil {
		return nil, err
	}

	ret := dexdcr.WalletTransaction{
		Hex: b.String(),
	}

	if txd.Block.Height != -1 {
		ret.BlockHash = txd.Block.Hash.String()
		ret.Confirmations = int64(tipHeight - txd.Block.Height + 1)
	}

	details, err := dw.wallet.ListTransactionDetails(ctx, txHash)
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
// Returns dw.wallet. CoinNotFoundError if the tx is not found.
// Part of the Wallet interface.
func (dw *DEXWallet) GetRawTransaction(ctx context.Context, txHash *chainhash.Hash) (*wire.MsgTx, error) {
	txs, _, err := dw.wallet.GetTransactionsByHashes(ctx, []*chainhash.Hash{txHash})
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
func (dw *DEXWallet) GetBlockHash(ctx context.Context, blockHeight int64) (*chainhash.Hash, error) {
	info, err := dw.wallet.BlockInfo(ctx, dcrwallet.NewBlockIdentifierFromHeight(int32(blockHeight)))
	if err != nil {
		return nil, err
	}
	return &info.Hash, nil
}

// MatchAnyScript looks for any of the provided scripts in the block specified.
// Part of the Wallet interface.
func (dw *DEXWallet) MatchAnyScript(ctx context.Context, blockHash *chainhash.Hash, scripts [][]byte) (bool, error) {
	key, filter, err := dw.wallet.CFilterV2(ctx, blockHash)
	if err != nil {
		return false, err
	}
	return filter.MatchAny(key, scripts), nil
}

// AccountUnlocked returns true if the account is unlocked.
// Part of the Wallet interface.
func (dw *DEXWallet) AccountUnlocked(ctx context.Context, accountName string) (bool, error) {
	acctNum, err := dw.wallet.AccountNumber(ctx, accountName)
	if err != nil {
		return false, err
	}
	return dw.wallet.AccountUnlocked(ctx, acctNum)
}

// LockAccount locks the specified account.
// Part of the Wallet interface.
func (dw *DEXWallet) LockAccount(ctx context.Context, accountName string) error {
	acctNum, err := dw.wallet.AccountNumber(ctx, accountName)
	if err != nil {
		return err
	}
	return dw.wallet.LockAccount(ctx, acctNum)
}

// UnlockAccount unlocks the specified account or the wallet if account is not
// encrypted.
// Part of the Wallet interface.
func (dw *DEXWallet) UnlockAccount(ctx context.Context, passphrase []byte, accountName string) error {
	acctNum, err := dw.wallet.AccountNumber(ctx, accountName)
	if err != nil {
		return err
	}
	return dw.wallet.UnlockAccount(ctx, acctNum, passphrase)
}

// SyncStatus returns the wallet's sync status.
// Part of the Wallet interface.
func (dw *DEXWallet) SyncStatus(_ context.Context) (synced bool, progress float32, err error) {
	syncProgress := dw.syncData.generalSyncProgress()
	if syncProgress != nil {
		progress = float32(syncProgress.TotalSyncProgress)
	}
	return dw.syncData.isSynced(), progress, nil
}

// PeerCount returns the number of network peers to which the wallet or its
// backing node are connected.
// Part of the Wallet interface.
func (dw *DEXWallet) PeerCount(_ context.Context) (uint32, error) {
	return uint32(dw.syncData.connectedPeers()), nil
}

// AddressPrivKey fetches the privkey for the specified address.
// Part of the Wallet interface.
func (dw *DEXWallet) AddressPrivKey(ctx context.Context, addr stdaddr.Address) (*secp256k1.PrivateKey, error) {
	privKey, _, err := dw.wallet.LoadPrivateKey(ctx, addr)
	return privKey, err
}

// Part of the Wallet interface.
func (dw *DEXWallet) Reconfigure(_ context.Context, _ *dexasset.WalletConfig, _ dex.Network, _, _ string) (restart bool, err error) {
	return false, nil
}
