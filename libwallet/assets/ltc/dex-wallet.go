// This code is available on the terms of the project LICENSE.md file, and as
// terms of the BlueOak License. See: https://blueoakcouncil.org/license/1.0.0.

package ltc

// Note: Most of the code here is a copy-paste from:
// https://github.com/decred/dcrdex/blob/master/client/asset/ltc/spv.go

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"decred.org/dcrdex/client/asset"
	dexbtc "decred.org/dcrdex/client/asset/btc"
	dexbtchelper "decred.org/dcrdex/dex/networks/btc"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/gcs"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/waddrmgr"
	btcwallet "github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/wtxmgr"
	neutrino "github.com/dcrlabs/neutrino-ltc"
	"github.com/dcrlabs/neutrino-ltc/chain"
	btcneutrino "github.com/lightninglabs/neutrino"
	"github.com/lightninglabs/neutrino/headerfs"
	ltcchaincfg "github.com/ltcsuite/ltcd/chaincfg"
	ltcchainhash "github.com/ltcsuite/ltcd/chaincfg/chainhash"
	"github.com/ltcsuite/ltcd/ltcutil"
	ltctxscript "github.com/ltcsuite/ltcd/txscript"
	ltcwire "github.com/ltcsuite/ltcd/wire"
	ltcwaddrmgr "github.com/ltcsuite/ltcwallet/waddrmgr"
	"github.com/ltcsuite/ltcwallet/wallet"
	"github.com/ltcsuite/ltcwallet/wallet/txauthor"
	ltcwtxmgr "github.com/ltcsuite/ltcwallet/wtxmgr"
)

const (
	DefaultM uint64 = 784931 // From ltcutil. Used for gcs filters.
)

// DEXWallet wraps *wallet.Wallet and implements dexbtc.BTCWallet.
type DEXWallet struct {
	w         *wallet.Wallet
	acctNum   int32
	cl        *ltcChainService
	btcParams *chaincfg.Params
	syncData  *SyncData
}

var _ dexbtc.CustomWallet = (*DEXWallet)(nil)

// NewDEXWallet returns a new *DEXWallet.
func NewDEXWallet(w *wallet.Wallet, acctNum int32, nc *chain.NeutrinoClient, btcParams *chaincfg.Params, syncData *SyncData) *DEXWallet {
	return &DEXWallet{
		w:       w,
		acctNum: acctNum,
		cl: &ltcChainService{
			NeutrinoClient: nc,
		},
		btcParams: btcParams,
		syncData:  syncData,
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
	ltcAddr, err := dw.addrBTC2LTC(addr)
	if err != nil {
		return false, err
	}

	return dw.w.HaveAddress(ltcAddr)
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) SendRawTransaction(tx *wire.MsgTx) (*chainhash.Hash, error) {
	ltcTx, err := convertMsgTxToLTC(tx)
	if err != nil {
		return nil, err
	}

	err = dw.w.PublishTransaction(ltcTx, "")
	if err != nil {
		return nil, err
	}

	txHash := tx.TxHash()
	return &txHash, nil
}

// Part of dexbtc.TipRedemptionWallet interface.
func (dw *DEXWallet) GetBlock(blockHash chainhash.Hash) (*wire.MsgBlock, error) {
	block, err := dw.cl.GetBlock(blockHash)
	if err != nil {
		return nil, fmt.Errorf("neutrino GetBlock error: %v", err)
	}

	return block.MsgBlock(), nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetBlockHash(blockHeight int64) (*chainhash.Hash, error) {
	return dw.cl.GetBlockHash(blockHeight)
}

// Part of dexbtc.TipRedemptionWallet interface.
func (dw *DEXWallet) GetBlockHeight(h *chainhash.Hash) (int32, error) {
	return dw.cl.GetBlockHeight(h)
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetBestBlockHash() (*chainhash.Hash, error) {
	blk := dw.syncedTo()
	return &blk.Hash, nil
}

// GetBestBlockHeight returns the height of the best block processed by the
// wallet, which indicates the height at which the compact filters have been
// retrieved and scanned for wallet addresses. This is may be less than
// getChainHeight, which indicates the height that the chain service has reached
// in its retrieval of block headers and compact filter headers.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetBestBlockHeight() (int32, error) {
	return dw.syncedTo().Height, nil
}

// getChainStamp satisfies dexbtc.chainStamper for manual median time
// calculations.
func (dw *DEXWallet) getChainStamp(blockHash *chainhash.Hash) (stamp time.Time, prevHash *chainhash.Hash, err error) {
	hdr, err := dw.cl.GetBlockHeader(blockHash)
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

// GetChainHeight is only for confirmations since it does not reflect the wallet
// manager's sync height, just the chain service.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) GetChainHeight() (int32, error) {
	blk, err := dw.cl.BestBlock()
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
//
// The neutrino wallet has a two stage sync:
//  1. chain service fetching block headers and filter headers
//  2. wallet address manager retrieving and scanning filters
//
// We only report a single sync height, so we are going to show some progress in
// the chain service sync stage that comes before the wallet has performed any
// address recovery/rescan, and switch to the wallet's sync height when it
// reports non-zero height.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) SyncStatus() (*dexbtc.SyncStatus, error) {
	walletBlock := dw.syncedTo()
	return &dexbtc.SyncStatus{
		Target:  dw.syncHeight(),
		Height:  walletBlock.Height,
		Syncing: dw.syncData.isSyncing(),
	}, nil
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) Balances() (*dexbtc.GetBalancesResult, error) {
	// Determine trusted vs untrusted coins with listunspent.
	unspents, err := dw.w.ListUnspent(0, math.MaxInt32, dw.accountName())
	if err != nil {
		return nil, fmt.Errorf("error listing unspent outputs: %w", err)
	}
	var trusted, untrusted ltcutil.Amount
	for _, txout := range unspents {
		if txout.Confirmations > 0 || dw.ownsInputs(txout.TxID) {
			trusted += ltcutil.Amount(AmountLitoshi(txout.Amount))
			continue
		}
		untrusted += ltcutil.Amount(AmountLitoshi(txout.Amount))
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
	acctName := dw.accountName()
	unspents, err := dw.w.ListUnspent(0, math.MaxInt32, acctName)
	if err != nil {
		return nil, err
	}
	res := make([]*dexbtc.ListUnspentResult, 0, len(unspents))
	for _, utxo := range unspents {
		// ltcwallet's ListUnspent takes either a list of addresses, or else
		// returns all non-locked unspent outputs for all accounts. We need to
		// iterate the results anyway to convert type.
		if utxo.Account != acctName {
			continue
		}

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
			op := ltcwire.OutPoint{Hash: ltcchainhash.Hash(op.Pt.TxHash), Index: op.Pt.Vout}
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
	ltcAddr, err := dw.w.NewChangeAddress(uint32(dw.acctNum), ltcwaddrmgr.KeyScopeBIP0084)
	if err != nil {
		return nil, err
	}
	return dw.addrLTC2BTC(ltcAddr)
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) ExternalAddress() (btcutil.Address, error) {
	ltcAddr, err := dw.w.NewAddress(uint32(dw.acctNum), ltcwaddrmgr.KeyScopeBIP0084)
	if err != nil {
		return nil, err
	}
	return dw.addrLTC2BTC(ltcAddr)
}

// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) SignTx(btcTx *wire.MsgTx) (*wire.MsgTx, error) {
	ltcTx, err := convertMsgTxToLTC(btcTx)
	if err != nil {
		return nil, err
	}

	var prevPkScripts [][]byte
	var inputValues []ltcutil.Amount
	for _, txIn := range btcTx.TxIn {
		_, txOut, _, _, err := dw.fetchInputInfo(&txIn.PreviousOutPoint)
		if err != nil {
			return nil, err
		}
		inputValues = append(inputValues, ltcutil.Amount(txOut.Value))
		prevPkScripts = append(prevPkScripts, txOut.PkScript)
		// Zero the previous witness and signature script or else
		// AddAllInputScripts does some weird stuff.
		txIn.SignatureScript = nil
		txIn.Witness = nil
	}

	err = txauthor.AddAllInputScripts(ltcTx, prevPkScripts, inputValues, &secretSource{dw, dw.w.ChainParams()})
	if err != nil {
		return nil, err
	}
	if len(ltcTx.TxIn) != len(btcTx.TxIn) {
		return nil, fmt.Errorf("txin count mismatch")
	}
	for i, txIn := range btcTx.TxIn {
		ltcIn := ltcTx.TxIn[i]
		txIn.SignatureScript = ltcIn.SignatureScript
		txIn.Witness = make(wire.TxWitness, len(ltcIn.Witness))
		copy(txIn.Witness, ltcIn.Witness)
	}
	return btcTx, nil
}

// PrivKeyForAddress retrieves the private key associated with the specified
// address.
// Part of dexbtc.Wallet interface.
func (dw *DEXWallet) PrivKeyForAddress(addr string) (*btcec.PrivateKey, error) {
	a, err := decodeAddress(addr, dw.w.ChainParams())
	if err != nil {
		return nil, err
	}

	ltcKey, err := dw.w.PrivKeyForAddress(a)
	if err != nil {
		return nil, err
	}

	priv, _ /* pub */ := btcec.PrivKeyFromBytes(ltcKey.Serialize())
	return priv, nil
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

	utxos, _, _, err := dexbtc.ConvertUnspent(0, unspents, dw.btcParams)
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
func (dw *DEXWallet) SwapConfirmations(txHash *chainhash.Hash, vout uint32, _ []byte,
	_ time.Time) (confs uint32, spent bool, err error) {
	// First, check if it's a wallet transaction. We probably won't be able
	// to see the spend status, since the wallet doesn't track the swap contract
	// output, but we can get the block if it's been mined.
	blockHash, confs, spent, err := dw.confirmations(txHash, vout)
	if err == nil {
		return confs, spent, nil
	}

	switch err {
	case dexbtc.WalletTransactionNotFound:
		log.Tracef("swapConfirmations - WalletTransactionNotFound: %v:%d", txHash, vout)
	case dexbtc.SpentStatusUnknown:
		log.Tracef("swapConfirmations - SpentStatusUnknown: %v:%d (block %v, confs %d)",
			txHash, vout, blockHash, confs)
	default:
		return 0, false, err
	}

	// unspent
	return confs, false, nil

	// // Our last option is neutrino.
	// log.Tracef("swapConfirmations - scanFilters: %v:%d (block %v, start time %v)",
	// 	txHash, vout, blockHash, startTime)
	// utxo, err := w.scanFilters(txHash, vout, pkScript, startTime, blockHash)
	// if err != nil {
	// 	return 0, false, err
	// }

	// if utxo.spend == nil && utxo.blockHash == nil {
	// 	if assumedMempool {
	// 		log.Tracef("swapConfirmations - scanFilters did not find %v:%d, assuming in mempool.",
	// 			txHash, vout)
	// 		// NOT asset.CoinNotFoundError since this is normal for mempool
	// 		// transactions with an SPV wallet.
	// 		return 0, false, nil
	// 	}
	// 	return 0, false, fmt.Errorf("output %s:%v not found with search parameters startTime = %s, pkScript = %x",
	// 		txHash, vout, startTime, pkScript)
	// }

	// if utxo.blockHash != nil {
	// 	bestHeight, err := dw.GetChainHeight()
	// 	if err != nil {
	// 		return 0, false, fmt.Errorf("getBestBlockHeight error: %v", err)
	// 	}
	// 	confs = uint32(bestHeight) - utxo.blockHeight + 1
	// }

	// if utxo.spend != nil {
	// 	// In the off-chance that a spend was found but not the output itself,
	// 	// confs will be incorrect here.
	// 	// In situations where we're looking for the counter-party's swap, we
	// 	// revoke if it's found to be spent, without inspecting the confs, so
	// 	// accuracy of confs is not significant. When it's our output, we'll
	// 	// know the block and won't end up here. (even if we did, we just end up
	// 	// sending out some inaccurate Data-severity notifications to the UI
	// 	// until the match progresses)
	// 	return confs, true, nil
	// }
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
		height, err := dw.GetChainHeight()
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
	hdr, err := dw.cl.GetBlockHeader(blockHash)
	if err != nil {
		return nil, false, err
	}

	tip, err := dw.cl.BestBlock()
	if err != nil {
		return nil, false, fmt.Errorf("BestBlock error: %v", err)
	}

	blockHeight, err := dw.cl.GetBlockHeight(blockHash)
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
func (dw *DEXWallet) GetTxOut(txHash *chainhash.Hash, vout uint32, _ []byte, _ time.Time) (*wire.TxOut, uint32, error) {
	// Check for a wallet transaction first
	txDetails, err := dw.walletTransaction(txHash)
	if err != nil && !errors.Is(err, dexbtc.WalletTransactionNotFound) {
		return nil, 0, fmt.Errorf("walletTransaction error: %w", err)
	}

	if txDetails == nil {
		return nil, 0, nil // wallet has not seen tx
	}

	spent, found := outputSpendStatus(txDetails, vout)
	if !found || spent {
		return nil, 0, nil
	}

	if len(txDetails.MsgTx.TxOut) <= int(vout) {
		return nil, 0, fmt.Errorf("wallet transaction %s doesn't have enough outputs for vout %d", txHash, vout)
	}

	var confs uint32
	if txDetails.Block.Height > 0 {
		tip, err := dw.cl.BestBlock()
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

	// We don't really know if it's spent, so we'll need to scan.
	// utxo, err := dw.scanFilters(txHash, vout, pkScript, startTime, blockHash)
	// if err != nil {
	// 	return nil, 0, err
	// }

	// if utxo == nil || utxo.spend != nil || utxo.blockHash == nil {
	// 	return nil, 0, nil
	// }

	// tip, err := dw.cl.BestBlock()
	// if err != nil {
	// 	return nil, 0, fmt.Errorf("BestBlock error: %v", err)
	// }

	// confs := uint32(confirms(int32(utxo.blockHeight), tip.Height))

	// return utxo.txOut, confs, nil
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
	block, err := dw.cl.GetBlock(blockHash)
	if err != nil {
		log.Errorf("neutrino GetBlock error: %v", err)
		return
	}

	for _, msgTx := range block.MsgBlock().Transactions {
		newlyDiscovered := dexbtc.FindRedemptionsInTxWithHasher(ctx, true, reqs, msgTx, dw.btcParams, hashTx)
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
	filter, err := dw.cl.GetCFilter(*blockHash, wire.GCSFilterRegular)
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
		blockHeight, err = dw.cl.GetBlockHeight(blockHash)
		if err != nil {
			log.Errorf("Error getting block height for hash %s", blockHash)
			return false
		}
	}
	checkHash, err := dw.cl.GetBlockHash(int64(blockHeight))
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
		ltcOp := &ltcwire.OutPoint{
			Hash:  ltcchainhash.Hash(txIn.PreviousOutPoint.Hash),
			Index: txIn.PreviousOutPoint.Index,
		}
		_, _, _, _, err = dw.w.FetchInputInfo(ltcOp)
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

// accountInfo returns the account name of the wallet.
func (dw *DEXWallet) accountName() string {
	accountName, err := dw.w.AccountName(GetScope(), uint32(dw.acctNum))
	if err == nil {
		return accountName
	}

	log.Errorf("error checking selected DEX account name: %v", err)
	return "" // return "default"?
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

func (dw *DEXWallet) txDetails(txHash *ltcchainhash.Hash) (*ltcwtxmgr.TxDetails, error) {
	details, err := wallet.UnstableAPI(dw.w).TxDetails(txHash)
	if err != nil {
		return nil, err
	}
	if details == nil {
		return nil, dexbtc.WalletTransactionNotFound
	}

	return details, nil
}

func (dw *DEXWallet) addrLTC2BTC(addr ltcutil.Address) (btcutil.Address, error) {
	return btcutil.DecodeAddress(addr.String(), dw.btcParams)
}

func (dw *DEXWallet) addrBTC2LTC(addr btcutil.Address) (ltcutil.Address, error) {
	return ltcutil.DecodeAddress(addr.String(), dw.w.ChainParams())
}

// fetchInputInfo is not actually implemented in ltcwallet. This is based on the
// btcwallet implementation. We really only need the TxOut, and to show
// ownership.
//
//nolint:all
func (dw *DEXWallet) fetchInputInfo(prevOut *wire.OutPoint) (*wire.MsgTx, *wire.TxOut, *psbt.Bip32Derivation, int64, error) {
	td, err := dw.txDetails((*ltcchainhash.Hash)(&prevOut.Hash))
	if err != nil {
		return nil, nil, nil, 0, err
	}

	if prevOut.Index >= uint32(len(td.TxRecord.MsgTx.TxOut)) {
		return nil, nil, nil, 0, fmt.Errorf("not enough outputs")
	}

	ltcTxOut := td.TxRecord.MsgTx.TxOut[prevOut.Index]

	// Verify we own at least one parsed address.
	_, addrs, _, err := ltctxscript.ExtractPkScriptAddrs(ltcTxOut.PkScript, dw.w.ChainParams())
	if err != nil {
		return nil, nil, nil, 0, err
	}
	notOurs := true
	for i := 0; notOurs && i < len(addrs); i++ {
		_, err := dw.w.AddressInfo(addrs[i])
		notOurs = err != nil
	}
	if notOurs {
		return nil, nil, nil, 0, btcwallet.ErrNotMine
	}

	btcTxOut := &wire.TxOut{
		Value:    ltcTxOut.Value,
		PkScript: ltcTxOut.PkScript,
	}

	return nil, btcTxOut, nil, 0, err
}

func (dw *DEXWallet) walletTransaction(txHash *chainhash.Hash) (*wtxmgr.TxDetails, error) {
	txDetails, err := dw.txDetails((*ltcchainhash.Hash)(txHash))
	if err != nil {
		return nil, err
	}

	btcTx, err := convertMsgTxToBTC(&txDetails.MsgTx)
	if err != nil {
		return nil, err
	}

	credits := make([]wtxmgr.CreditRecord, len(txDetails.Credits))
	for i, c := range txDetails.Credits {
		credits[i] = wtxmgr.CreditRecord{
			Amount: btcutil.Amount(c.Amount),
			Index:  c.Index,
			Spent:  c.Spent,
			Change: c.Change,
		}
	}

	debits := make([]wtxmgr.DebitRecord, len(txDetails.Debits))
	for i, d := range txDetails.Debits {
		debits[i] = wtxmgr.DebitRecord{
			Amount: btcutil.Amount(d.Amount),
			Index:  d.Index,
		}
	}

	return &wtxmgr.TxDetails{
		TxRecord: wtxmgr.TxRecord{
			MsgTx:        *btcTx,
			Hash:         chainhash.Hash(txDetails.TxRecord.Hash),
			Received:     txDetails.TxRecord.Received,
			SerializedTx: txDetails.TxRecord.SerializedTx,
		},
		Block: wtxmgr.BlockMeta{
			Block: wtxmgr.Block{
				Hash:   chainhash.Hash(txDetails.Block.Hash),
				Height: txDetails.Block.Height,
			},
			Time: txDetails.Block.Time,
		},
		Credits: credits,
		Debits:  debits,
	}, nil
}

func (dw *DEXWallet) syncedTo() waddrmgr.BlockStamp {
	bs := dw.w.Manager.SyncedTo()
	return waddrmgr.BlockStamp{
		Height:    bs.Height,
		Hash:      chainhash.Hash(bs.Hash),
		Timestamp: bs.Timestamp,
	}
}

// ltcChainService wraps ltcsuite *neutrino.ChainService in order to translate the
// neutrino.ServerPeer to the SPVPeer interface type as required by the dex btc
// pkg.
type ltcChainService struct {
	*chain.NeutrinoClient
}

var _ dexbtc.SPVService = (*ltcChainService)(nil)

func (s *ltcChainService) GetBlockHash(height int64) (*chainhash.Hash, error) {
	ltcHash, err := s.CS.GetBlockHash(height)
	if err != nil {
		return nil, err
	}
	return (*chainhash.Hash)(ltcHash), nil
}

func (s *ltcChainService) BestBlock() (*headerfs.BlockStamp, error) {
	bs, err := s.CS.BestBlock()
	if err != nil {
		return nil, err
	}
	return &headerfs.BlockStamp{
		Height:    bs.Height,
		Hash:      chainhash.Hash(bs.Hash),
		Timestamp: bs.Timestamp,
	}, nil
}

func (s *ltcChainService) Peers() []dexbtc.SPVPeer {
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

func (s *ltcChainService) AddPeer(addr string) error {
	return s.CS.ConnectNode(addr, true)
}

func (s *ltcChainService) RemovePeer(addr string) error {
	return s.CS.RemoveNodeByAddr(addr)
}

func (s *ltcChainService) GetBlockHeight(h *chainhash.Hash) (int32, error) {
	return s.CS.GetBlockHeight((*ltcchainhash.Hash)(h))
}

func (s *ltcChainService) GetBlockHeader(h *chainhash.Hash) (*wire.BlockHeader, error) {
	hdr, err := s.CS.GetBlockHeader((*ltcchainhash.Hash)(h))
	if err != nil {
		return nil, err
	}
	return &wire.BlockHeader{
		Version:    hdr.Version,
		PrevBlock:  chainhash.Hash(hdr.PrevBlock),
		MerkleRoot: chainhash.Hash(hdr.MerkleRoot),
		Timestamp:  hdr.Timestamp,
		Bits:       hdr.Bits,
		Nonce:      hdr.Nonce,
	}, nil
}

func (s *ltcChainService) GetCFilter(blockHash chainhash.Hash, _ wire.FilterType, _ ...btcneutrino.QueryOption) (*gcs.Filter, error) {
	f, err := s.CS.GetCFilter(ltcchainhash.Hash(blockHash), ltcwire.GCSFilterRegular)
	if err != nil {
		return nil, err
	}

	b, err := f.Bytes()
	if err != nil {
		return nil, err
	}

	return gcs.FromBytes(f.N(), f.P(), DefaultM, b)
}

func (s *ltcChainService) GetBlock(blockHash chainhash.Hash, _ ...btcneutrino.QueryOption) (*btcutil.Block, error) {
	blk, err := s.CS.GetBlock(ltcchainhash.Hash(blockHash))
	if err != nil {
		return nil, err
	}

	b, err := blk.Bytes()
	if err != nil {
		return nil, err
	}

	return btcutil.NewBlockFromBytes(b)
}

func (s *ltcChainService) Stop() error {
	return s.CS.Stop()
}

// secretSource is used to locate keys and redemption scripts while signing a
// transaction. secretSource satisfies the txauthor.SecretsSource interface.
type secretSource struct {
	dexW        *DEXWallet
	chainParams *ltcchaincfg.Params
}

// ChainParams returns the chain parameters.
func (s *secretSource) ChainParams() *ltcchaincfg.Params {
	return s.chainParams
}

// GetKey fetches a private key for the specified address.
func (s *secretSource) GetKey(addr ltcutil.Address) (*btcec.PrivateKey, bool, error) {
	ma, err := s.dexW.w.AddressInfo(addr)
	if err != nil {
		return nil, false, err
	}

	mpka, ok := ma.(ltcwaddrmgr.ManagedPubKeyAddress)
	if !ok {
		e := fmt.Errorf("managed address type for %v is `%T` but "+
			"want waddrmgr.ManagedPubKeyAddress", addr, ma)
		return nil, false, e
	}

	privKey, err := mpka.PrivKey()
	if err != nil {
		return nil, false, err
	}

	k, _ /* pub */ := btcec.PrivKeyFromBytes(privKey.Serialize())

	return k, ma.Compressed(), nil
}

// GetScript fetches the redemption script for the specified p2sh/p2wsh address.
func (s *secretSource) GetScript(addr ltcutil.Address) ([]byte, error) {
	ma, err := s.dexW.w.AddressInfo(addr)
	if err != nil {
		return nil, err
	}

	msa, ok := ma.(ltcwaddrmgr.ManagedScriptAddress)
	if !ok {
		e := fmt.Errorf("managed address type for %v is `%T` but "+
			"want waddrmgr.ManagedScriptAddress", addr, ma)
		return nil, e
	}
	return msa.Script()
}

func convertMsgTxToBTC(tx *ltcwire.MsgTx) (*wire.MsgTx, error) {
	buf := new(bytes.Buffer)
	if err := tx.Serialize(buf); err != nil {
		return nil, err
	}

	btcTx := new(wire.MsgTx)
	if err := btcTx.Deserialize(buf); err != nil {
		return nil, err
	}
	return btcTx, nil
}

func convertMsgTxToLTC(tx *wire.MsgTx) (*ltcwire.MsgTx, error) {
	buf := new(bytes.Buffer)
	if err := tx.Serialize(buf); err != nil {
		return nil, err
	}
	ltcTx := new(ltcwire.MsgTx)
	if err := ltcTx.Deserialize(buf); err != nil {
		return nil, err
	}

	return ltcTx, nil
}
