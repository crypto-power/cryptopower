// This code is available on the terms of the project LICENSE.md file, and as
// terms of the BlueOak License. See: https://blueoakcouncil.org/license/1.0.0.

package btc

// Note: Most of the code here is a copy-paste from:
// https://github.com/decred/dcrdex/blob/master/client/asset/btc/spv.go

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"decred.org/dcrdex/client/asset"
	dexbtc "decred.org/dcrdex/client/asset/btc"
	dexbtchelper "decred.org/dcrdex/dex/networks/btc"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/gcs"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/lightninglabs/neutrino"
)

// DEXWallet wraps *wallet.Wallet and implements dexbtc.Wallet.
type DEXWallet struct {
	w         *wallet.Wallet
	acctNum   int32
	cl        *btcChainService
	isSyncing func() bool
}

var _ dexbtc.CustomWallet = (*DEXWallet)(nil)

// NewDEXWallet returns a new *DEXWallet.
func NewDEXWallet(w *wallet.Wallet, acctNum int32, nc *chain.NeutrinoClient, isSyncing func() bool) *DEXWallet {
	return &DEXWallet{
		w:       w,
		acctNum: acctNum,
		cl: &btcChainService{
			NeutrinoClient: nc,
		},
		isSyncing: isSyncing,
	}
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) Reconfigure(*asset.WalletConfig, string) (bool, error) {
	return false, errors.New("Reconfigure not supported for Cyptopower btc wallet")
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) RawRequest(_ context.Context, _ string, _ []json.RawMessage) (json.RawMessage, error) {
	// Not needed for spv wallet.
	return nil, errors.New("RawRequest not available on spv")
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) OwnsAddress(addr btcutil.Address) (bool, error) {
	return dw.w.HaveAddress(addr)
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) SendRawTransaction(tx *wire.MsgTx) (*chainhash.Hash, error) {
	/*
		Note:

		The upstream *spvWallet implementation of this method expects that this
		w.PublishTransaction method may take quite some time to return, so it calls the
		method in a goroutine and assumes the method call was successful if the method
		does not complete after waiting for some seconds.

		The upstream implementation also unlocks the tx inputs to ensure that the locked
		balance computation isn't affected by coins that were previously locked but are
		now spent.
	*/

	err := dw.w.PublishTransaction(tx, "")
	if err != nil {
		return nil, err
	}

	txHash := tx.TxHash()
	return &txHash, nil
}

// Part of dexbtc.TipRedemptionWallet interface.
func (dw *DEXWallet) GetBlock(blockHash chainhash.Hash) (*wire.MsgBlock, error) {
	block, err := dw.cl.CS.GetBlock(blockHash)
	if err != nil {
		return nil, fmt.Errorf("neutrino GetBlock error: %v", err)
	}

	return block.MsgBlock(), nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetBlockHash(blockHeight int64) (*chainhash.Hash, error) {
	return dw.cl.CS.GetBlockHash(blockHeight)
}

// Part of dexbtc.TipRedemptionWallet interface.
func (dw *DEXWallet) GetBlockHeight(h *chainhash.Hash) (int32, error) {
	return dw.cl.CS.GetBlockHeight(h)
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetBestBlockHash() (*chainhash.Hash, error) {
	blk := dw.syncedTo()
	return &blk.Hash, nil
}

// GetBestBlockHeight returns the height of the best block processed by the
// wallet, which indicates the height at which the compact filters have been
// retrieved and scanned for wallet addresses.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetBestBlockHeight() (int32, error) {
	return dw.syncedTo().Height, nil
}

// getChainStamp satisfies dexbtc.chainStamper for manual median time
// calculations.
func (dw *DEXWallet) getChainStamp(blockHash *chainhash.Hash) (stamp time.Time, prevHash *chainhash.Hash, err error) {
	hdr, err := dw.cl.CS.GetBlockHeader(blockHash)
	if err != nil {
		return
	}
	return hdr.Timestamp, &hdr.PrevBlock, nil
}

// MedianTime is the median time for the current best block.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) MedianTime() (time.Time, error) {
	blk := dw.syncedTo()
	return dexbtc.CalcMedianTime(dw.getChainStamp, &blk.Hash)
}

// getChainHeight is only for confirmations since it does not reflect the wallet
// manager's sync height, just the chain service.
func (dw *DEXWallet) getChainHeight() (int32, error) {
	blk, err := dw.cl.CS.BestBlock()
	if err != nil {
		return -1, err
	}
	return blk.Height, err
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) PeerCount() (uint32, error) {
	return uint32(len(dw.cl.Peers())), nil
}

// syncHeight is the best known sync height among peers.
func (dw *DEXWallet) syncHeight() int32 {
	var maxHeight int32
	for _, p := range dw.cl.Peers() {
		tipHeight := p.StartingHeight()
		lastBlockHeight := p.LastBlock()
		if lastBlockHeight > tipHeight {
			tipHeight = lastBlockHeight
		}
		if tipHeight > maxHeight {
			maxHeight = tipHeight
		}
	}
	return maxHeight
}

// SyncStatus is information about the wallet's sync status.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) SyncStatus() (*dexbtc.SyncStatus, error) {
	walletBlock := dw.syncedTo()
	return &dexbtc.SyncStatus{
		Target:  dw.syncHeight(),
		Height:  walletBlock.Height,
		Syncing: dw.isSyncing(),
	}, nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) Balances() (*dexbtc.GetBalancesResult, error) {
	// Determine trusted vs untrusted coins with listunspent.
	unspents, err := dw.w.ListUnspent(0, math.MaxInt32, dw.accountName())
	if err != nil {
		return nil, fmt.Errorf("error listing unspent outputs: %w", err)
	}
	var trusted, untrusted btcutil.Amount
	for _, txout := range unspents {
		if txout.Confirmations > 0 || dw.ownsInputs(txout.TxID) {
			trusted += btcutil.Amount(AmountSatoshi(txout.Amount))
			continue
		}
		untrusted += btcutil.Amount(AmountSatoshi(txout.Amount))
	}

	// listunspent does not include immature coinbase outputs or locked outputs.
	bals, err := dw.w.CalculateAccountBalances(uint32(dw.acctNum), 0 /* confs */)
	if err != nil {
		return nil, err
	}
	log.Tracef("Bals: spendable = %v (%v trusted, %v untrusted, %v assumed locked), immature = %v",
		bals.Spendable, trusted, untrusted, bals.Spendable-trusted-untrusted, bals.ImmatureReward)
	// Locked outputs would be in wallet.Balances.Spendable. Assume they would
	// be considered trusted and add them back in.
	if all := trusted + untrusted; bals.Spendable > all {
		trusted += bals.Spendable - all
	}

	return &dexbtc.GetBalancesResult{
		Mine: dexbtc.Balances{
			Trusted:   trusted.ToBTC(),
			Untrusted: untrusted.ToBTC(),
			Immature:  bals.ImmatureReward.ToBTC(),
		},
	}, nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) ListUnspent() ([]*dexbtc.ListUnspentResult, error) {
	unspents, err := dw.w.ListUnspent(0, math.MaxInt32, dw.accountName())
	if err != nil {
		return nil, err
	}
	res := make([]*dexbtc.ListUnspentResult, 0, len(unspents))
	for _, utxo := range unspents {
		// If the utxo is unconfirmed, we should determine whether it's "safe"
		// by seeing if we control the inputs of its transaction.
		safe := utxo.Confirmations > 0 || dw.ownsInputs(utxo.TxID)

		// These hex decodings are unlikely to fail because they come directly
		// from the listunspent result. Regardless, they should not result in an
		// error for the caller as we can return the valid utxos.
		pkScript, err := hex.DecodeString(utxo.ScriptPubKey)
		if err != nil {
			log.Warnf("ScriptPubKey decode failure: %v", err)
			continue
		}

		redeemScript, err := hex.DecodeString(utxo.RedeemScript)
		if err != nil {
			log.Warnf("ScriptPubKey decode failure: %v", err)
			continue
		}

		res = append(res, &dexbtc.ListUnspentResult{
			TxID:    utxo.TxID,
			Vout:    utxo.Vout,
			Address: utxo.Address,
			// Label: ,
			ScriptPubKey:  pkScript,
			Amount:        utxo.Amount,
			Confirmations: uint32(utxo.Confirmations),
			RedeemScript:  redeemScript,
			Spendable:     utxo.Spendable,
			// Solvable: ,
			SafePtr: &safe,
		})
	}
	return res, nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) LockUnspent(unlock bool, ops []*dexbtc.Output) error {
	switch {
	case unlock && len(ops) == 0:
		dw.w.ResetLockedOutpoints()
	default:
		for _, op := range ops {
			op := wire.OutPoint{Hash: op.Pt.TxHash, Index: op.Pt.Vout}
			if unlock {
				dw.w.UnlockOutpoint(op)
			} else {
				dw.w.LockOutpoint(op)
			}
		}
	}
	return nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) ListLockUnspent() ([]*dexbtc.RPCOutpoint, error) {
	outpoints := dw.w.LockedOutpoints()
	pts := make([]*dexbtc.RPCOutpoint, 0, len(outpoints))
	for _, pt := range outpoints {
		pts = append(pts, &dexbtc.RPCOutpoint{
			TxID: pt.Txid,
			Vout: pt.Vout,
		})
	}
	return pts, nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) ChangeAddress() (btcutil.Address, error) {
	return dw.w.NewChangeAddress(uint32(dw.acctNum), waddrmgr.KeyScopeBIP0084)
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) ExternalAddress() (btcutil.Address, error) {
	return dw.w.NewAddress(uint32(dw.acctNum), waddrmgr.KeyScopeBIP0084)
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) SignTx(tx *wire.MsgTx) (*wire.MsgTx, error) {
	var prevPkScripts [][]byte
	var inputValues []btcutil.Amount
	for _, txIn := range tx.TxIn {
		_, txOut, _, _, err := dw.w.FetchInputInfo(&txIn.PreviousOutPoint)
		if err != nil {
			return nil, err
		}
		inputValues = append(inputValues, btcutil.Amount(txOut.Value))
		prevPkScripts = append(prevPkScripts, txOut.PkScript)
		// Zero the previous witness and signature script or else
		// AddAllInputScripts does some weird stuff.
		txIn.SignatureScript = nil
		txIn.Witness = nil
	}
	return tx, txauthor.AddAllInputScripts(tx, prevPkScripts, inputValues, &secretSource{dw, dw.w.ChainParams()})
}

// PrivKeyForAddress retrieves the private key associated with the specified
// address.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) PrivKeyForAddress(addr string) (*btcec.PrivateKey, error) {
	a, err := decodeAddress(addr, dw.w.ChainParams())
	if err != nil {
		return nil, err
	}
	return dw.w.PrivKeyForAddress(a)
}

// Part of dexbtc.TxFeeEstimator interface.
func (dw *DEXWallet) EstimateSendTxFee(tx *wire.MsgTx, feeRate uint64, subtract bool) (fee uint64, err error) {
	minTxSize := uint64(tx.SerializeSize())
	var sendAmount uint64
	for _, txOut := range tx.TxOut {
		sendAmount += uint64(txOut.Value)
	}

	unspents, err := dw.ListUnspent()
	if err != nil {
		return 0, fmt.Errorf("error listing unspent outputs: %w", err)
	}

	utxos, _, _, err := dexbtc.ConvertUnspent(0, unspents, dw.w.ChainParams())
	if err != nil {
		return 0, fmt.Errorf("error converting unspent outputs: %w", err)
	}

	enough := dexbtc.SendEnough(sendAmount, feeRate, subtract, minTxSize, true, false)
	sum, _, inputsSize, _, _, _, _, err := dexbtc.TryFund(utxos, enough)
	if err != nil {
		return 0, err
	}

	txSize := minTxSize + inputsSize
	estFee := feeRate * txSize
	remaining := sum - sendAmount

	// Check if there will be a change output if there is enough remaining.
	estFeeWithChange := (txSize + dexbtchelper.P2WPKHOutputSize) * feeRate
	var changeValue uint64
	if remaining > estFeeWithChange {
		changeValue = remaining - estFeeWithChange
	}

	if subtract {
		// fees are already included in sendAmount, anything else is change.
		changeValue = remaining
	}

	var finalFee uint64
	if dexbtchelper.IsDustVal(dexbtchelper.P2WPKHOutputSize, changeValue, feeRate, true) {
		// remaining cannot cover a non-dust change and the fee for the change.
		finalFee = estFee + remaining
	} else {
		// additional fee will be paid for non-dust change
		finalFee = estFeeWithChange
	}

	if subtract {
		sendAmount -= finalFee
	}
	if dexbtchelper.IsDustVal(minTxSize, sendAmount, feeRate, true) {
		return 0, errors.New("output value is dust")
	}

	return finalFee, nil
}

// SwapConfirmations attempts to get the number of confirmations and the spend
// status for the specified tx output. For swap outputs that were not generated
// by this wallet, startTime must be supplied to limit the search. Use the match
// time assigned by the server.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) SwapConfirmations(txHash *chainhash.Hash, vout uint32, pkScript []byte,
	startTime time.Time) (confs uint32, spent bool, err error) {

	// First, check if it's a wallet transaction. We probably won't be able
	// to see the spend status, since the wallet doesn't track the swap contract
	// output, but we can get the block if it's been mined.
	blockHash, confs, spent, err := dw.confirmations(txHash, vout)
	if err == nil {
		return confs, spent, nil
	}
	var assumedMempool bool
	switch err {
	case dexbtc.WalletTransactionNotFound:
		log.Tracef("swapConfirmations - WalletTransactionNotFound: %v:%d", txHash, vout)
	case dexbtc.SpentStatusUnknown:
		log.Tracef("swapConfirmations - SpentStatusUnknown: %v:%d (block %v, confs %d)",
			txHash, vout, blockHash, confs)
		if blockHash == nil {
			// We generated this swap, but it probably hasn't been mined yet.
			// It's SpentStatusUnknown because the wallet doesn't track the
			// spend status of the swap contract output itself, since it's not
			// recognized as a wallet output. We'll still try to find the
			// confirmations with other means, but if we can't find it, we'll
			// report it as a zero-conf unspent output. This ignores the remote
			// possibility that the output could be both in mempool and spent.
			assumedMempool = true
		}
	default:
		return 0, false, err
	}

	// Upstream checks a block cache but we don't have that here.

	// Our last option is neutrino.
	log.Tracef("swapConfirmations - scanFilters: %v:%d (block %v, start time %v)",
		txHash, vout, blockHash, startTime)
	utxo, err := dw.scanFilters(txHash, vout, pkScript, startTime, blockHash)
	if err != nil {
		return 0, false, err
	}

	if utxo.spend == nil && utxo.blockHash == nil {
		if assumedMempool {
			log.Tracef("swapConfirmations - scanFilters did not find %v:%d, assuming in mempool.",
				txHash, vout)
			// NOT asset.CoinNotFoundError since this is normal for mempool
			// transactions with an SPV wallet.
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("output %s:%v not found with search parameters startTime = %s, pkScript = %x",
			txHash, vout, startTime, pkScript)
	}

	if utxo.blockHash != nil {
		bestHeight, err := dw.getChainHeight()
		if err != nil {
			return 0, false, fmt.Errorf("getBestBlockHeight error: %v", err)
		}
		confs = uint32(bestHeight) - utxo.blockHeight + 1
	}

	if utxo.spend != nil {
		// In the off-chance that a spend was found but not the output itself,
		// confs will be incorrect here.
		// In situations where we're looking for the counter-party's swap, we
		// revoke if it's found to be spent, without inspecting the confs, so
		// accuracy of confs is not significant. When it's our output, we'll
		// know the block and won't end up here. (even if we did, we just end up
		// sending out some inaccurate Data-severity notifications to the UI
		// until the match progresses)
		return confs, true, nil
	}

	// unspent
	return confs, false, nil
}

// scanFilters enables searching for an output and its spending input by
// scanning BIP158 compact filters. Caller should supply either blockHash or
// startTime. blockHash takes precedence. If blockHash is supplied, the scan
// will start at that block and continue to the current blockchain tip, or until
// both the output and a spending transaction is found. if startTime is
// supplied, and the blockHash for the output is not known to the wallet, a
// candidate block will be selected with findBlockTime.
func (dw *DEXWallet) scanFilters(txHash *chainhash.Hash, vout uint32, pkScript []byte, startTime time.Time, blockHash *chainhash.Hash) (*filterScanResult, error) {
	// TODO: Check that any blockHash supplied is not orphaned?

	// Check if we know the block hash for the tx.
	var limitHeight int32
	if blockHash == nil {
		// No checkpoint and no block hash. Gotta guess based on time.
		var err error
		_, limitHeight, err = dw.findBlockForTime(startTime)
		if err != nil {
			return nil, err
		}
	} else {
		// No checkpoint, but user supplied a block hash.
		var err error
		limitHeight, err = dw.GetBlockHeight(blockHash)
		if err != nil {
			return nil, fmt.Errorf("error getting height for supplied block hash %s", blockHash)
		}
	}

	log.Debugf("Performing cfilters scan for %v:%d from height %d", txHash, vout, limitHeight)

	// Do a filter scan.
	utxo, err := dw.filterScanFromHeight(*txHash, vout, pkScript, limitHeight, nil)
	if err != nil {
		return nil, fmt.Errorf("filterScanFromHeight error: %w", err)
	}
	if utxo == nil {
		return nil, asset.CoinNotFoundError
	}

	// If we found a block, let's store a reference in our local database so we
	// can maybe bypass a long search next time.
	if utxo.blockHash != nil {
		log.Debugf("cfilters scan SUCCEEDED for %v:%d. block hash: %v, spent: %v",
			txHash, vout, utxo.blockHash, utxo.spend != nil)
	}

	return utxo, nil
}

const medianTimeBlocks = 11

var maxFutureBlockTime = 2 * time.Hour // see MaxTimeOffsetSeconds in btcd/blockchain/validate.go

// findBlockForTime locates a good start block so that a search beginning at the
// returned block has a very low likelihood of missing any blocks that have time
// > matchTime. This is done by performing a binary search (sort.Search) to find
// a block with a block time maxFutureBlockTime before matchTime. To ensure
// we also accommodate the median-block time rule and aren't missing anything
// due to out of sequence block times we use an unsophisticated algorithm of
// choosing the first block in an 11 block window with no times >= matchTime.
func (dw *DEXWallet) findBlockForTime(matchTime time.Time) (*chainhash.Hash, int32, error) {
	offsetTime := matchTime.Add(-maxFutureBlockTime)

	bestHeight, err := dw.getChainHeight()
	if err != nil {
		return nil, 0, fmt.Errorf("getChainHeight error: %v", err)
	}

	getBlockTimeForHeight := func(height int32) (*chainhash.Hash, time.Time, error) {
		hash, err := dw.cl.GetBlockHash(int64(height))
		if err != nil {
			return nil, time.Time{}, err
		}
		header, err := dw.cl.GetBlockHeader(hash)
		if err != nil {
			return nil, time.Time{}, err
		}
		return hash, header.Timestamp, nil
	}

	iHeight := sort.Search(int(bestHeight), func(h int) bool {
		var iTime time.Time
		_, iTime, err = getBlockTimeForHeight(int32(h))
		if err != nil {
			return true
		}
		return iTime.After(offsetTime)
	})
	if err != nil {
		return nil, 0, fmt.Errorf("binary search error finding best block for time %q: %w", matchTime, err)
	}

	// We're actually breaking an assumption of sort.Search here because block
	// times aren't always monotonically increasing. This won't matter though as
	// long as there are not > medianTimeBlocks blocks with inverted time order.
	var count int
	var iHash *chainhash.Hash
	var iTime time.Time
	for iHeight > 0 {
		iHash, iTime, err = getBlockTimeForHeight(int32(iHeight))
		if err != nil {
			return nil, 0, fmt.Errorf("getBlockTimeForHeight error: %w", err)
		}
		if iTime.Before(offsetTime) {
			count++
			if count == medianTimeBlocks {
				return iHash, int32(iHeight), nil
			}
		} else {
			count = 0
		}
		iHeight--
	}
	return dw.w.ChainParams().GenesisHash, 0, nil
}

// spendingInput is added to a filterScanResult if a spending input is found.
type spendingInput struct {
	txHash      chainhash.Hash
	vin         uint32
	blockHash   chainhash.Hash
	blockHeight uint32
}

// filterScanResult is the result from a filter scan.
type filterScanResult struct {
	// blockHash is the block that the output was found in.
	blockHash *chainhash.Hash
	// blockHeight is the height of the block that the output was found in.
	blockHeight uint32
	// txOut is the output itself.
	txOut *wire.TxOut
	// spend will be set if a spending input is found.
	spend *spendingInput
	// checkpoint is used to track the last block scanned so that future scans
	// can skip scanned blocks.
	checkpoint chainhash.Hash
}

// / filterScanFromHeight scans BIP158 filters beginning at the specified block
// height until the tip, or until a spending transaction is found.
func (dw *DEXWallet) filterScanFromHeight(txHash chainhash.Hash, vout uint32, pkScript []byte, startBlockHeight int32, checkPt *filterScanResult) (*filterScanResult, error) {
	walletBlock := dw.syncedTo() // where cfilters are received and processed
	tip := walletBlock.Height

	res := checkPt
	if res == nil {
		res = new(filterScanResult)
	}

search:
	for height := startBlockHeight; height <= tip; height++ {
		if res.spend != nil && res.blockHash == nil {
			log.Warnf("A spending input (%s) was found during the scan but the output (%s) "+
				"itself wasn't found. Was the startBlockHeight early enough?",
				dexbtc.NewOutPoint(&res.spend.txHash, res.spend.vin),
				dexbtc.NewOutPoint(&txHash, vout),
			)
			return res, nil
		}
		blockHash, err := dw.cl.GetBlockHash(int64(height))
		if err != nil {
			return nil, fmt.Errorf("error getting block hash for height %d: %w", height, err)
		}
		matched, err := dw.matchPkScript(blockHash, [][]byte{pkScript})
		if err != nil {
			return nil, fmt.Errorf("matchPkScript error: %w", err)
		}

		res.checkpoint = *blockHash
		if !matched {
			continue search
		}
		// Pull the block.
		log.Tracef("Block %v matched pkScript for output %v:%d. Pulling the block...",
			blockHash, txHash, vout)
		block, err := dw.cl.GetBlock(blockHash)
		if err != nil {
			return nil, fmt.Errorf("GetBlock error: %v", err)
		}
		msgBlock := btcutil.NewBlock(block).MsgBlock()

		// Scan every transaction.
	nextTx:
		for _, tx := range msgBlock.Transactions {
			// Look for a spending input.
			if res.spend == nil {
				for vin, txIn := range tx.TxIn {
					prevOut := &txIn.PreviousOutPoint
					if prevOut.Hash == txHash && prevOut.Index == vout {
						res.spend = &spendingInput{
							txHash:      tx.TxHash(),
							vin:         uint32(vin),
							blockHash:   *blockHash,
							blockHeight: uint32(height),
						}
						log.Tracef("Found txn %v spending %v in block %v (%d)", res.spend.txHash,
							txHash, res.spend.blockHash, res.spend.blockHeight)
						if res.blockHash != nil {
							break search
						}
						// The output could still be in this block, just not
						// in this transaction.
						continue nextTx
					}
				}
			}
			// Only check for the output if this is the right transaction.
			if res.blockHash != nil || tx.TxHash() != txHash {
				continue nextTx
			}
			for _, txOut := range tx.TxOut {
				if bytes.Equal(txOut.PkScript, pkScript) {
					res.blockHash = blockHash
					res.blockHeight = uint32(height)
					res.txOut = txOut
					log.Tracef("Found txn %v in block %v (%d)", txHash, res.blockHash, height)
					if res.spend != nil {
						break search
					}
					// Keep looking for the spending transaction.
					continue nextTx
				}
			}
		}
	}
	return res, nil
}

// confirmations looks for the confirmation count and spend status on a
// transaction output that pays to this wallet.
func (dw *DEXWallet) confirmations(txHash *chainhash.Hash, vout uint32) (blockHash *chainhash.Hash, confs uint32, spent bool, err error) {
	details, err := dw.walletTransaction(txHash)
	if err != nil {
		return nil, 0, false, err
	}

	if details.Block.Hash != (chainhash.Hash{}) {
		blockHash = &details.Block.Hash
		height, err := dw.getChainHeight()
		if err != nil {
			return nil, 0, false, err
		}
		confs = uint32(confirms(details.Block.Height, height))
	}

	spent, found := outputSpendStatus(details, vout)
	if found {
		return blockHash, confs, spent, nil
	}

	return blockHash, confs, false, dexbtc.SpentStatusUnknown
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) Locked() bool {
	return dw.w.Locked()
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) WalletLock() error {
	dw.w.Lock()
	return nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) WalletUnlock(pw []byte) error {
	return dw.w.Unlock(pw, nil)
}

// GetBlockHeader gets the *blockHeader for the specified block hash. It also
// returns a bool value to indicate whether this block is a part of main chain.
// For orphaned blocks header.Confirmations is negative.
// Part of dexbtc.TipRedemptionWallet interface.
func (dw *DEXWallet) GetBlockHeader(blockHash *chainhash.Hash) (header *dexbtc.BlockHeader, mainchain bool, err error) {
	hdr, err := dw.cl.CS.GetBlockHeader(blockHash)
	if err != nil {
		return nil, false, err
	}

	tip, err := dw.cl.CS.BestBlock()
	if err != nil {
		return nil, false, fmt.Errorf("BestBlock error: %v", err)
	}

	blockHeight, err := dw.cl.CS.GetBlockHeight(blockHash)
	if err != nil {
		return nil, false, err
	}

	confirmations := int64(-1)
	mainchain = dw.blockIsMainchain(blockHash, blockHeight)
	if mainchain {
		confirmations = int64(confirms(blockHeight, tip.Height))
	}

	return &dexbtc.BlockHeader{
		Hash:              hdr.BlockHash().String(),
		Confirmations:     confirmations,
		Height:            int64(blockHeight),
		Time:              hdr.Timestamp.Unix(),
		PreviousBlockHash: hdr.PrevBlock.String(),
	}, mainchain, nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetBestBlockHeader() (*dexbtc.BlockHeader, error) {
	hash, err := dw.GetBestBlockHash()
	if err != nil {
		return nil, err
	}
	hdr, _, err := dw.GetBlockHeader(hash)
	return hdr, err
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) Connect(_ context.Context, _ *sync.WaitGroup) (err error) {
	return nil
}

// GetTxOut finds an unspent transaction output and its number of confirmations.
// To match the behavior of the RPC method, even if an output is found, if it's
// known to be spent, no *wire.TxOut and no error will be returned.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetTxOut(txHash *chainhash.Hash, vout uint32, pkScript []byte, startTime time.Time) (*wire.TxOut, uint32, error) {
	// Check for a wallet transaction first
	txDetails, err := dw.walletTransaction(txHash)
	var blockHash *chainhash.Hash
	if err != nil && !errors.Is(err, dexbtc.WalletTransactionNotFound) {
		return nil, 0, fmt.Errorf("walletTransaction error: %w", err)
	}

	if txDetails != nil {
		spent, found := outputSpendStatus(txDetails, vout)
		if found {
			if spent {
				return nil, 0, nil
			}
			if len(txDetails.MsgTx.TxOut) <= int(vout) {
				return nil, 0, fmt.Errorf("wallet transaction %s doesn't have enough outputs for vout %d", txHash, vout)
			}

			var confs uint32
			if txDetails.Block.Height > 0 {
				tip, err := dw.cl.CS.BestBlock()
				if err != nil {
					return nil, 0, fmt.Errorf("BestBlock error: %v", err)
				}
				confs = uint32(confirms(txDetails.Block.Height, tip.Height))
			}

			msgTx := &txDetails.MsgTx
			if len(msgTx.TxOut) <= int(vout) {
				return nil, 0, fmt.Errorf("wallet transaction %s found, but not enough outputs for vout %d", txHash, vout)
			}
			return msgTx.TxOut[vout], confs, nil
		}
		if txDetails.Block.Hash != (chainhash.Hash{}) {
			blockHash = &txDetails.Block.Hash
		}
	}

	// We don't really know if it's spent, so we'll need to scan.
	utxo, err := dw.scanFilters(txHash, vout, pkScript, startTime, blockHash)
	if err != nil {
		return nil, 0, err
	}

	if utxo == nil || utxo.spend != nil || utxo.blockHash == nil {
		return nil, 0, nil
	}

	tip, err := dw.cl.CS.BestBlock()
	if err != nil {
		return nil, 0, fmt.Errorf("BestBlock error: %v", err)
	}

	confs := uint32(confirms(int32(utxo.blockHeight), tip.Height))

	return utxo.txOut, confs, nil
}

// SearchBlockForRedemptions attempts to find spending info for the specified
// contracts by searching every input of all txs in the provided block range.
// Part of dexbtc.TipRedemptionWallet interface.
func (dw *DEXWallet) SearchBlockForRedemptions(ctx context.Context, reqs map[dexbtc.OutPoint]*dexbtc.FindRedemptionReq,
	blockHash chainhash.Hash) (discovered map[dexbtc.OutPoint]*dexbtc.FindRedemptionResult) {

	// Just match all the scripts together.
	scripts := make([][]byte, 0, len(reqs))
	for _, req := range reqs {
		scripts = append(scripts, req.PkScript())
	}

	discovered = make(map[dexbtc.OutPoint]*dexbtc.FindRedemptionResult, len(reqs))

	matchFound, err := dw.matchPkScript(&blockHash, scripts)
	if err != nil {
		log.Errorf("matchPkScript error: %v", err)
		return
	}

	if !matchFound {
		return
	}

	// There is at least one match. Pull the block.
	block, err := dw.cl.CS.GetBlock(blockHash)
	if err != nil {
		log.Errorf("neutrino GetBlock error: %v", err)
		return
	}

	for _, msgTx := range block.MsgBlock().Transactions {
		newlyDiscovered := dexbtc.FindRedemptionsInTxWithHasher(ctx, true, reqs, msgTx, dw.w.ChainParams(), hashTx)
		for outPt, res := range newlyDiscovered {
			discovered[outPt] = res
		}
	}
	return
}

// FindRedemptionsInMempool is unsupported for SPV.
func (dw *DEXWallet) FindRedemptionsInMempool(_ context.Context, _ map[dexbtc.OutPoint]*dexbtc.FindRedemptionReq) (discovered map[dexbtc.OutPoint]*dexbtc.FindRedemptionResult) {
	return
}

// GetWalletTransaction checks the wallet database for the specified
// transaction. Only transactions with output scripts that pay to the wallet or
// transactions that spend wallet outputs are stored in the wallet database.
// This is pretty much copy-paste from btcwallet 'gettransaction' JSON-RPC
// handler.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetWalletTransaction(txHash *chainhash.Hash) (*dexbtc.GetTransactionResult, error) {
	details, err := dw.walletTransaction(txHash)
	if err != nil {
		if errors.Is(err, dexbtc.WalletTransactionNotFound) {
			return nil, asset.CoinNotFoundError // for the asset.Wallet interface
		}
		return nil, err
	}

	syncBlock := dw.syncedTo()

	// TODO: The serialized transaction is already in the DB, so reserializing
	// might be avoided here. According to btcwallet, details.SerializedTx is
	// "optional" (?), but we might check for it.
	txRaw, err := serializeMsgTx(&details.MsgTx)
	if err != nil {
		return nil, err
	}

	ret := &dexbtc.GetTransactionResult{
		TxID:         txHash.String(),
		Bytes:        txRaw, // 'Hex' field name is a lie, kinda
		Time:         uint64(details.Received.Unix()),
		TimeReceived: uint64(details.Received.Unix()),
	}

	if details.Block.Height >= 0 {
		ret.BlockHash = details.Block.Hash.String()
		ret.BlockTime = uint64(details.Block.Time.Unix())
		// ret.BlockHeight = uint64(details.Block.Height)
		ret.Confirmations = uint64(confirms(details.Block.Height, syncBlock.Height))
	}

	return ret, nil
}

func hashTx(tx *wire.MsgTx) *chainhash.Hash {
	h := tx.TxHash()
	return &h
}

// matchPkScript pulls the filter for the block and attempts to match the
// supplied scripts.
func (dw *DEXWallet) matchPkScript(blockHash *chainhash.Hash, scripts [][]byte) (bool, error) {
	filter, err := dw.cl.CS.GetCFilter(*blockHash, wire.GCSFilterRegular)
	if err != nil {
		return false, fmt.Errorf("GetCFilter error: %w", err)
	}

	if filter.N() == 0 {
		return false, fmt.Errorf("unexpected empty filter for %s", blockHash)
	}

	var filterKey [gcs.KeySize]byte
	copy(filterKey[:], blockHash[:gcs.KeySize])

	matchFound, err := filter.MatchAny(filterKey, scripts)
	if err != nil {
		return false, fmt.Errorf("MatchAny error: %w", err)
	}
	return matchFound, nil
}

// blockIsMainchain will be true if the blockHash is that of a mainchain block.
func (dw *DEXWallet) blockIsMainchain(blockHash *chainhash.Hash, blockHeight int32) bool {
	if blockHeight < 0 {
		var err error
		blockHeight, err = dw.cl.CS.GetBlockHeight(blockHash)
		if err != nil {
			log.Errorf("Error getting block height for hash %s", blockHash)
			return false
		}
	}
	checkHash, err := dw.cl.CS.GetBlockHash(int64(blockHeight))
	if err != nil {
		log.Errorf("Error retrieving block hash for height %d", blockHeight)
		return false
	}

	return *checkHash == *blockHash
}

// ownsInputs determines if we own the inputs of the tx.
func (dw *DEXWallet) ownsInputs(txid string) bool {
	txHash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		log.Warnf("Error decoding txid %q: %v", txid, err)
		return false
	}
	txDetails, err := dw.walletTransaction(txHash)
	if err != nil {
		log.Warnf("walletTransaction(%v) error: %v", txid, err)
		return false
	}

	for _, txIn := range txDetails.MsgTx.TxIn {
		_, _, _, _, err = dw.w.FetchInputInfo(&txIn.PreviousOutPoint)
		if err != nil {
			if !errors.Is(err, wallet.ErrNotMine) {
				log.Warnf("FetchInputInfo error: %v", err)
			}
			return false
		}
	}
	return true
}

// outputSpendStatus will return the spend status of the output if it's found
// in the TxDetails.Credits.
func outputSpendStatus(details *wtxmgr.TxDetails, vout uint32) (spend, found bool) {
	for _, credit := range details.Credits {
		if credit.Index == vout {
			return credit.Spent, true
		}
	}
	return false, false
}

func confirms(txHeight, curHeight int32) int32 {
	switch {
	case txHeight == -1, txHeight > curHeight:
		return 0
	default:
		return curHeight - txHeight + 1
	}
}

func (dw *DEXWallet) syncedTo() waddrmgr.BlockStamp {
	bs := dw.w.Manager.SyncedTo()
	return waddrmgr.BlockStamp{
		Height:    bs.Height,
		Hash:      bs.Hash,
		Timestamp: bs.Timestamp,
	}
}

// accountName returns the account name of the wallet.
func (dw *DEXWallet) accountName() string {
	accountName, err := dw.w.AccountName(GetScope(), uint32(dw.acctNum))
	if err == nil {
		return accountName
	}

	log.Errorf("error checking selected DEX account name: %v", err)
	return "" // return "default"?
}

func (dw *DEXWallet) walletTransaction(txHash *chainhash.Hash) (*wtxmgr.TxDetails, error) {
	details, err := wallet.UnstableAPI(dw.w).TxDetails(txHash)
	if err != nil {
		return nil, err
	}
	if details == nil {
		return nil, dexbtc.WalletTransactionNotFound
	}
	return details, nil
}

// serializeMsgTx serializes the wire.MsgTx.
func serializeMsgTx(msgTx *wire.MsgTx) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, msgTx.SerializeSize()))
	err := msgTx.Serialize(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// btcChainService wraps *chain.NeutrinoClient in order to translate the
// neutrino.ServerPeer to the SPVPeer interface.
type btcChainService struct {
	*chain.NeutrinoClient
}

func (s *btcChainService) Peers() []dexbtc.SPVPeer {
	// *neutrino.ChainService.Peers() may stall, especially if the wallet hasn't
	// started sync yet. Call the method in a goroutine and wait below to see if
	// we get a response. Return an empty slice if we don't get a response after
	// waiting briefly.
	rawPeersChan := make(chan []*neutrino.ServerPeer)
	go func() {
		rawPeersChan <- s.CS.Peers()
	}()

	select {
	case rawPeers := <-rawPeersChan:
		peers := make([]dexbtc.SPVPeer, 0, len(rawPeers))
		for _, p := range rawPeers {
			peers = append(peers, p)
		}
		return peers

	case <-time.After(2 * time.Second):
		return nil // CS.Peers() is taking too long to respond
	}
}

// secretSource is used to locate keys and redemption scripts while signing a
// transaction. secretSource satisfies the txauthor.SecretsSource interface.
type secretSource struct {
	dexW        *DEXWallet
	chainParams *chaincfg.Params
}

// ChainParams returns the chain parameters.
func (s *secretSource) ChainParams() *chaincfg.Params {
	return s.chainParams
}

// GetKey fetches a private key for the specified address.
func (s *secretSource) GetKey(addr btcutil.Address) (*btcec.PrivateKey, bool, error) {
	ma, err := s.dexW.w.AddressInfo(addr)
	if err != nil {
		return nil, false, err
	}

	mpka, ok := ma.(waddrmgr.ManagedPubKeyAddress)
	if !ok {
		e := fmt.Errorf("managed address type for %v is `%T` but "+
			"want waddrmgr.ManagedPubKeyAddress", addr, ma)
		return nil, false, e
	}

	privKey, err := mpka.PrivKey()
	if err != nil {
		return nil, false, err
	}
	return privKey, ma.Compressed(), nil
}

// GetScript fetches the redemption script for the specified p2sh/p2wsh address.
func (s *secretSource) GetScript(addr btcutil.Address) ([]byte, error) {
	ma, err := s.dexW.w.AddressInfo(addr)
	if err != nil {
		return nil, err
	}

	msa, ok := ma.(waddrmgr.ManagedScriptAddress)
	if !ok {
		e := fmt.Errorf("managed address type for %v is `%T` but "+
			"want waddrmgr.ManagedScriptAddress", addr, ma)
		return nil, e
	}
	return msa.Script()
}
