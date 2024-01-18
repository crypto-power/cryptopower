// This code is available on the terms of the project LICENSE.md file, and as
// terms of the BlueOak License. See: https://blueoakcouncil.org/license/1.0.0.

package btc

// Note: Most of the code here is a copy-paste from:
// https://github.com/decred/dcrdex/blob/master/client/asset/btc/spv.go

import (
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
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/lightninglabs/neutrino"
	"github.com/lightninglabs/neutrino/headerfs"
)

// DEXWallet wraps *wallet.Wallet and implements dexbtc.BTCWallet.
type DEXWallet struct {
	w          *wallet.Wallet
	acctNum    int32
	spvService *btcChainService
}

var _ dexbtc.BTCWallet = (*DEXWallet)(nil)

// NewDEXWallet returns a new *DEXWallet.
func NewDEXWallet(w *wallet.Wallet, acctNum int32, nc *chain.NeutrinoClient) *DEXWallet {
	return &DEXWallet{
		w:       w,
		acctNum: acctNum,
		spvService: &btcChainService{
			NeutrinoClient: nc,
		},
	}
}

func (dw *DEXWallet) PublishTransaction(tx *wire.MsgTx, label string) error {
	return dw.w.PublishTransaction(tx, label)
}

func (dw *DEXWallet) CalculateAccountBalances(account uint32, confirms int32) (wallet.Balances, error) {
	return dw.w.CalculateAccountBalances(account, confirms)
}

func (dw *DEXWallet) ListUnspent(minconf, maxconf int32, acctName string) ([]*btcjson.ListUnspentResult, error) {
	return dw.w.ListUnspent(minconf, maxconf, acctName)
}

func (dw *DEXWallet) FetchInputInfo(prevOut *wire.OutPoint) (*wire.MsgTx, *wire.TxOut, *psbt.Bip32Derivation, int64, error) {
	return dw.w.FetchInputInfo(prevOut)
}

func (dw *DEXWallet) ResetLockedOutpoints() {
	dw.w.ResetLockedOutpoints()
}

func (dw *DEXWallet) LockOutpoint(op wire.OutPoint) {
	dw.w.LockOutpoint(op)
}

func (dw *DEXWallet) UnlockOutpoint(op wire.OutPoint) {
	dw.w.UnlockOutpoint(op)
}

func (dw *DEXWallet) LockedOutpoints() []btcjson.TransactionInput {
	return dw.w.LockedOutpoints()
}

func (dw *DEXWallet) NewChangeAddress(account uint32, scope waddrmgr.KeyScope) (btcutil.Address, error) {
	return dw.w.NewChangeAddress(account, scope)
}

func (dw *DEXWallet) NewAddress(account uint32, scope waddrmgr.KeyScope) (btcutil.Address, error) {
	return dw.w.NewAddress(account, scope)
}

func (dw *DEXWallet) PrivKeyForAddress(a btcutil.Address) (*btcec.PrivateKey, error) {
	return dw.w.PrivKeyForAddress(a)
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

func (dw *DEXWallet) SendOutputs(outputs []*wire.TxOut, keyScope *waddrmgr.KeyScope, account uint32, minconf int32,
	satPerKb btcutil.Amount, coinSelectionStrategy wallet.CoinSelectionStrategy, label string) (*wire.MsgTx, error) {
	return dw.w.SendOutputs(outputs, keyScope, account, minconf, satPerKb, coinSelectionStrategy, label)
}

func (dw *DEXWallet) HaveAddress(a btcutil.Address) (bool, error) {
	return dw.w.HaveAddress(a)
}

func (dw *DEXWallet) WaitForShutdown() {}

// currently unused
func (dw *DEXWallet) ChainSynced() bool {
	return dw.w.ChainSynced()
}

func (dw *DEXWallet) AccountProperties(scope waddrmgr.KeyScope, acct uint32) (*waddrmgr.AccountProperties, error) {
	return dw.w.AccountProperties(scope, acct)
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

func (dw *DEXWallet) WalletTransaction(txHash *chainhash.Hash) (*wtxmgr.TxDetails, error) {
	details, err := wallet.UnstableAPI(dw.w).TxDetails(txHash)
	if err != nil {
		return nil, err
	}
	if details == nil {
		return nil, dexbtc.WalletTransactionNotFound
	}

	return details, nil
}

func (dw *DEXWallet) SyncedTo() waddrmgr.BlockStamp {
	return dw.w.Manager.SyncedTo()
}

func (dw *DEXWallet) SignTx(tx *wire.MsgTx) error {
	var prevPkScripts [][]byte
	var inputValues []btcutil.Amount
	for _, txIn := range tx.TxIn {
		_, txOut, _, _, err := dw.w.FetchInputInfo(&txIn.PreviousOutPoint)
		if err != nil {
			return err
		}
		inputValues = append(inputValues, btcutil.Amount(txOut.Value))
		prevPkScripts = append(prevPkScripts, txOut.PkScript)
		// Zero the previous witness and signature script or else
		// AddAllInputScripts does some weird stuff.
		txIn.SignatureScript = nil
		txIn.Witness = nil
	}
	return txauthor.AddAllInputScripts(tx, prevPkScripts, inputValues, &secretSource{dw, dw.w.ChainParams()})
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
						Hash:   *lastBlock.Hash,
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

func (dw *DEXWallet) RescanAsync() error {
	return errors.New("RescanAsync not implemented for Cyptopower btc wallet")
}

func (dw *DEXWallet) ForceRescan() {}

func (dw *DEXWallet) Start() (dexbtc.SPVService, error) {
	return dw.spvService, nil
}

func (dw *DEXWallet) Stop() {}

func (dw *DEXWallet) Reconfigure(*asset.WalletConfig, string) (bool, error) {
	return false, errors.New("Reconfigure not supported for Cyptopower btc wallet")
}

func (dw *DEXWallet) Birthday() time.Time {
	return dw.w.Manager.Birthday()
}

func (dw *DEXWallet) Peers() ([]*asset.WalletPeer, error) {
	peerChan := make(chan []*asset.WalletPeer)
	go func() {
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
		peerChan <- walletPeers
	}()

	select {
	case <-time.After(1 * time.Second):
		return nil, nil // okay.
	case peers := <-peerChan:
		return peers, nil
	}
}

func (dw *DEXWallet) AddPeer(_ string) error {
	return errors.New("AddPeer not implemented by DEX wallet")
}

func (dw *DEXWallet) RemovePeer(_ string) error {
	return errors.New("RemovePeer not implemented by DEX wallet")
}

func (dw *DEXWallet) ListSinceBlock(start, end, syncHeight int32) ([]btcjson.ListTransactionsResult, error) {
	return dw.w.ListSinceBlock(start, end, syncHeight)
}

// btcChainService wraps *chain.NeutrinoClient in order to translate the
// neutrino.ServerPeer to the SPVPeer interface type as required by the dex btc
// pkg.
type btcChainService struct {
	*chain.NeutrinoClient
}

var _ dexbtc.SPVService = (*btcChainService)(nil)

func (s *btcChainService) Peers() []dexbtc.SPVPeer {
	rawPeers := s.CS.Peers()
	peers := make([]dexbtc.SPVPeer, 0, len(rawPeers))
	for _, p := range rawPeers {
		peers = append(peers, p)
	}
	return peers
}

func (s *btcChainService) AddPeer(addr string) error {
	return s.CS.ConnectNode(addr, true)
}

func (s *btcChainService) RemovePeer(addr string) error {
	return s.CS.RemoveNodeByAddr(addr)
}

func (s *btcChainService) BestBlock() (*headerfs.BlockStamp, error) {
	return s.CS.BestBlock()
}

func (s *btcChainService) GetBlock(blockHash chainhash.Hash, options ...neutrino.QueryOption) (*btcutil.Block, error) {
	return s.CS.GetBlock(blockHash, options...)
}

func (s *btcChainService) GetCFilter(blockHash chainhash.Hash, filterType wire.FilterType, options ...neutrino.QueryOption) (*gcs.Filter, error) {
	return s.NeutrinoClient.CS.GetCFilter(blockHash, filterType, options...)
}

func (s *btcChainService) Stop() error {
	return s.CS.Stop()
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
