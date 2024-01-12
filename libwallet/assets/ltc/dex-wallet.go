// This code is available on the terms of the project LICENSE.md file, and as
// terms of the BlueOak License. See: https://blueoakcouncil.org/license/1.0.0.

package ltc

// Note: Most of the code here is a copy-paste from:
// https://github.com/decred/dcrdex/blob/master/client/asset/ltc/spv.go

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"decred.org/dcrdex/client/asset"
	dexbtc "decred.org/dcrdex/client/asset/btc"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/gcs"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/waddrmgr"
	btcwallet "github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/wtxmgr"
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
	w          *wallet.Wallet
	acctNum    int32
	spvService *ltcChainService
	btcParams  *chaincfg.Params
}

var _ dexbtc.BTCWallet = (*DEXWallet)(nil)

// NewDEXWallet returns a new *DEXWallet.
func NewDEXWallet(w *wallet.Wallet, acctNum int32, nc *chain.NeutrinoClient, btcParams *chaincfg.Params) *DEXWallet {
	return &DEXWallet{
		w:       w,
		acctNum: acctNum,
		spvService: &ltcChainService{
			NeutrinoClient: nc,
		},
		btcParams: btcParams,
	}
}

// AccountInfo returns the account information of the wallet for use by the
// exchange wallet.
func (dw *DEXWallet) AccountInfo() dexbtc.XCWalletAccount {
	acct := dexbtc.XCWalletAccount{
		AccountNumber: uint32(dw.acctNum),
	}

	accountName, err := dw.w.AccountName(GetScope(), acct.AccountNumber)
	if err == nil {
		acct.AccountName = accountName
	} else {
		log.Errorf("error checking selected DEX account name: %v", err)
	}

	return acct
}

func (dw *DEXWallet) Start() (dexbtc.SPVService, error) {
	return dw.spvService, nil
}

func (dw *DEXWallet) Birthday() time.Time {
	return dw.w.Manager.Birthday()
}

func (dw *DEXWallet) Reconfigure(*asset.WalletConfig, string) (bool, error) {
	return false, errors.New("Reconfigure not supported for Cyptopower ltc wallet")
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

func (dw *DEXWallet) PublishTransaction(btcTx *wire.MsgTx, label string) error {
	ltcTx, err := convertMsgTxToLTC(btcTx)
	if err != nil {
		return err
	}

	return dw.w.PublishTransaction(ltcTx, label)
}

func (dw *DEXWallet) CalculateAccountBalances(account uint32, confirms int32) (btcwallet.Balances, error) {
	bals, err := dw.w.CalculateAccountBalances(account, confirms)
	if err != nil {
		return btcwallet.Balances{}, err
	}
	return btcwallet.Balances{
		Total:          btcutil.Amount(bals.Total),
		Spendable:      btcutil.Amount(bals.Spendable),
		ImmatureReward: btcutil.Amount(bals.ImmatureReward),
	}, nil
}

func (dw *DEXWallet) ListSinceBlock(start, end, syncHeight int32) ([]btcjson.ListTransactionsResult, error) {
	res, err := dw.w.ListSinceBlock(start, end, syncHeight)
	if err != nil {
		return nil, err
	}

	btcRes := make([]btcjson.ListTransactionsResult, len(res))
	for i, r := range res {
		btcRes[i] = btcjson.ListTransactionsResult{
			Abandoned:         r.Abandoned,
			Account:           r.Account,
			Address:           r.Address,
			Amount:            r.Amount,
			BIP125Replaceable: r.BIP125Replaceable,
			BlockHash:         r.BlockHash,
			BlockHeight:       r.BlockHeight,
			BlockIndex:        r.BlockIndex,
			BlockTime:         r.BlockTime,
			Category:          r.Category,
			Confirmations:     r.Confirmations,
			Fee:               r.Fee,
			Generated:         r.Generated,
			InvolvesWatchOnly: r.InvolvesWatchOnly,
			Label:             r.Label,
			Time:              r.Time,
			TimeReceived:      r.TimeReceived,
			Trusted:           r.Trusted,
			TxID:              r.TxID,
			Vout:              r.Vout,
			WalletConflicts:   r.WalletConflicts,
			Comment:           r.Comment,
			OtherAccount:      r.OtherAccount,
		}
	}

	return btcRes, nil
}

func (dw *DEXWallet) ListUnspent(minconf, maxconf int32, acctName string) ([]*btcjson.ListUnspentResult, error) {
	// ltcwallet's ListUnspent takes either a list of addresses, or else returns
	// all non-locked unspent outputs for all accounts. We need to iterate the
	// results anyway to convert type.
	uns, err := dw.w.ListUnspent(minconf, maxconf, acctName)
	if err != nil {
		return nil, err
	}

	outs := make([]*btcjson.ListUnspentResult, len(uns))
	for i, u := range uns {
		if u.Account != acctName {
			continue
		}
		outs[i] = &btcjson.ListUnspentResult{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Address:       u.Address,
			Account:       u.Account,
			ScriptPubKey:  u.ScriptPubKey,
			RedeemScript:  u.RedeemScript,
			Amount:        u.Amount,
			Confirmations: u.Confirmations,
			Spendable:     u.Spendable,
		}
	}

	return outs, nil
}

// FetchInputInfo is not actually implemented in ltcwallet. This is based on the
// btcwallet implementation. As this is used by btc.spvWallet, we really only
// need the TxOut, and to show ownership.
func (dw *DEXWallet) FetchInputInfo(prevOut *wire.OutPoint) (*wire.MsgTx, *wire.TxOut, *psbt.Bip32Derivation, int64, error) {

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

	return nil, btcTxOut, nil, 0, nil
}

func (dw *DEXWallet) LockOutpoint(op wire.OutPoint) {
	dw.w.LockOutpoint(ltcwire.OutPoint{
		Hash:  ltcchainhash.Hash(op.Hash),
		Index: op.Index,
	})
}

func (dw *DEXWallet) UnlockOutpoint(op wire.OutPoint) {
	dw.w.UnlockOutpoint(ltcwire.OutPoint{
		Hash:  ltcchainhash.Hash(op.Hash),
		Index: op.Index,
	})
}

func (dw *DEXWallet) LockedOutpoints() []btcjson.TransactionInput {
	locks := dw.w.LockedOutpoints()
	locked := make([]btcjson.TransactionInput, len(locks))
	for i, lock := range locks {
		locked[i] = btcjson.TransactionInput{
			Txid: lock.Txid,
			Vout: lock.Vout,
		}
	}
	return locked
}

func (dw *DEXWallet) ResetLockedOutpoints() {
	dw.w.ResetLockedOutpoints()
}

func (dw *DEXWallet) NewChangeAddress(account uint32, _ waddrmgr.KeyScope) (btcutil.Address, error) {
	ltcAddr, err := dw.w.NewChangeAddress(account, ltcwaddrmgr.KeyScopeBIP0084)
	if err != nil {
		return nil, err
	}
	return dw.addrLTC2BTC(ltcAddr)
}

func (dw *DEXWallet) NewAddress(account uint32, _ waddrmgr.KeyScope) (btcutil.Address, error) {
	ltcAddr, err := dw.w.NewAddress(account, ltcwaddrmgr.KeyScopeBIP0084)
	if err != nil {
		return nil, err
	}
	return dw.addrLTC2BTC(ltcAddr)
}

func (dw *DEXWallet) PrivKeyForAddress(a btcutil.Address) (*btcec.PrivateKey, error) {
	ltcAddr, err := dw.addrBTC2LTC(a)
	if err != nil {
		return nil, err
	}

	ltcKey, err := dw.w.PrivKeyForAddress(ltcAddr)
	if err != nil {
		return nil, err
	}

	priv, _ /* pub */ := btcec.PrivKeyFromBytes(ltcKey.Serialize())
	return priv, nil
}

func (dw *DEXWallet) Unlock(passphrase []byte, lock <-chan time.Time) error {
	return dw.w.Unlock(passphrase, lock)
}

func (dw *DEXWallet) Lock() {
	dw.w.Lock()
}

func (dw *DEXWallet) Locked() bool {
	return dw.w.Locked()
}

func (dw *DEXWallet) SendOutputs(outputs []*wire.TxOut, _ *waddrmgr.KeyScope, account uint32, minconf int32,
	satPerKb btcutil.Amount, coinSelectionStrategy btcwallet.CoinSelectionStrategy, label string) (*wire.MsgTx, error) {
	ltcOuts := make([]*ltcwire.TxOut, len(outputs))
	for i, op := range outputs {
		ltcOuts[i] = &ltcwire.TxOut{
			Value:    op.Value,
			PkScript: op.PkScript,
		}
	}

	ltcTx, err := dw.w.SendOutputs(ltcOuts, &ltcwaddrmgr.KeyScopeBIP0084, account,
		minconf, ltcutil.Amount(satPerKb), wallet.CoinSelectionStrategy(coinSelectionStrategy), label)
	if err != nil {
		return nil, err
	}

	btcTx, err := convertMsgTxToBTC(ltcTx)
	if err != nil {
		return nil, err
	}

	return btcTx, nil
}

func (dw *DEXWallet) HaveAddress(a btcutil.Address) (bool, error) {
	ltcAddr, err := dw.addrBTC2LTC(a)
	if err != nil {
		return false, err
	}

	return dw.w.HaveAddress(ltcAddr)
}

func (dw *DEXWallet) Stop() {}

func (dw *DEXWallet) AccountProperties(_ waddrmgr.KeyScope, acct uint32) (*waddrmgr.AccountProperties, error) {
	props, err := dw.w.AccountProperties(ltcwaddrmgr.KeyScopeBIP0084, acct)
	if err != nil {
		return nil, err
	}
	return &waddrmgr.AccountProperties{
		AccountNumber:        props.AccountNumber,
		AccountName:          props.AccountName,
		ExternalKeyCount:     props.ExternalKeyCount,
		InternalKeyCount:     props.InternalKeyCount,
		ImportedKeyCount:     props.ImportedKeyCount,
		MasterKeyFingerprint: props.MasterKeyFingerprint,
		KeyScope:             waddrmgr.KeyScopeBIP0084,
		IsWatchOnly:          props.IsWatchOnly,
		// The last two would need conversion but aren't currently used.
		// AccountPubKey:        props.AccountPubKey,
		// AddrSchema:           props.AddrSchema,
	}, nil
}

func (dw *DEXWallet) RescanAsync() error {
	return errors.New("RescanAsync not implemented for Cyptopower ltc wallet")
}

func (dw *DEXWallet) ForceRescan() {}

func (dw *DEXWallet) WalletTransaction(txHash *chainhash.Hash) (*wtxmgr.TxDetails, error) {
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

func (dw *DEXWallet) SyncedTo() waddrmgr.BlockStamp {
	bs := dw.w.Manager.SyncedTo()
	return waddrmgr.BlockStamp{
		Height:    bs.Height,
		Hash:      chainhash.Hash(bs.Hash),
		Timestamp: bs.Timestamp,
	}
}

func (dw *DEXWallet) SignTx(btcTx *wire.MsgTx) error {
	ltcTx, err := convertMsgTxToLTC(btcTx)
	if err != nil {
		return err
	}

	var prevPkScripts [][]byte
	var inputValues []ltcutil.Amount
	for _, txIn := range btcTx.TxIn {
		_, txOut, _, _, err := dw.FetchInputInfo(&txIn.PreviousOutPoint)
		if err != nil {
			return err
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
		return err
	}
	if len(ltcTx.TxIn) != len(btcTx.TxIn) {
		return fmt.Errorf("txin count mismatch")
	}
	for i, txIn := range btcTx.TxIn {
		ltcIn := ltcTx.TxIn[i]
		txIn.SignatureScript = ltcIn.SignatureScript
		txIn.Witness = make(wire.TxWitness, len(ltcIn.Witness))
		copy(txIn.Witness, ltcIn.Witness)
	}
	return nil
}

func (dw *DEXWallet) BlockNotifications(ctx context.Context) <-chan *dexbtc.BlockNotification {
	cl := dw.w.NtfnServer.TransactionNotifications()
	ch := make(chan *dexbtc.BlockNotification, 1)
	go func() {
		defer cl.Done()
		for {
			select {
			case note := <-cl.C:
				if len(note.AttachedBlocks) > 0 {
					lastBlock := note.AttachedBlocks[len(note.AttachedBlocks)-1]
					select {
					case ch <- &dexbtc.BlockNotification{
						Hash:   chainhash.Hash(*lastBlock.Hash),
						Height: lastBlock.Height,
					}:
					default:
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func (dw *DEXWallet) WaitForShutdown() {}

// currently unused
func (dw *DEXWallet) ChainSynced() bool {
	return dw.w.ChainSynced()
}

func (dw *DEXWallet) Peers() ([]*asset.WalletPeer, error) {
	peers := dw.spvService.CS.Peers()
	var walletPeers []*asset.WalletPeer
	for i := range peers {
		p := peers[i]
		walletPeers = append(walletPeers, &asset.WalletPeer{
			Addr:      p.Addr(),
			Connected: p.Connected(),
			Source:    asset.WalletDefault,
		})
	}
	return walletPeers, nil
}

func (dw *DEXWallet) AddPeer(_ string) error {
	return errors.New("AddPeer not implemented by DEX wallet")
}

func (dw *DEXWallet) RemovePeer(_ string) error {
	return errors.New("RemovePeer not implemented by DEX wallet")
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
	rawPeers := s.CS.Peers()
	peers := make([]dexbtc.SPVPeer, len(rawPeers))
	for i, p := range rawPeers {
		peers[i] = p
	}
	return peers
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
