// This code is available on the terms of the project LICENSE.md file, and as
// terms of the BlueOak License. See: https://blueoakcouncil.org/license/1.0.0.

package btc

// Note: Most of the code here is a copy-pasta from:
// https://github.com/decred/dcrdex/blob/master/client/asset/btc/spv.go

import (
	"context"
	"errors"
	"fmt"
	"time"

	"decred.org/dcrdex/client/asset"
	dexbtc "decred.org/dcrdex/client/asset/btc"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wtxmgr"
)

// DEXWallet wraps *wallet.Wallet and implements dexbtc.BTCWallet.
type DEXWallet struct {
	*wallet.Wallet // Implements most of dexbtc.BTCWallet
	asset          *Asset
}

var _ dexbtc.BTCWallet = (*DEXWallet)(nil)

// NewDEXWallet returns a new *DEXWallet.
func NewDEXWallet(asset *Asset) *DEXWallet {
	return &DEXWallet{
		Wallet: asset.Internal().BTC,
		asset:  asset,
	}
}

// The below methods are not implemented by *wallet.Wallet, so must be
// implemented by the BTCWallet implementation.

func (dw *DEXWallet) WalletTransaction(txHash *chainhash.Hash) (*wtxmgr.TxDetails, error) {
	details, err := wallet.UnstableAPI(dw.Wallet).TxDetails(txHash)
	if err != nil {
		return nil, err
	}
	if details == nil {
		return nil, dexbtc.WalletTransactionNotFound
	}

	return details, nil
}

func (dw *DEXWallet) SyncedTo() waddrmgr.BlockStamp {
	return dw.Wallet.Manager.SyncedTo()
}

func (dw *DEXWallet) SignTx(tx *wire.MsgTx) error {
	var prevPkScripts [][]byte
	var inputValues []btcutil.Amount
	for _, txIn := range tx.TxIn {
		_, txOut, _, _, err := dw.Wallet.FetchInputInfo(&txIn.PreviousOutPoint)
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
	return txauthor.AddAllInputScripts(tx, prevPkScripts, inputValues, &secretSource{dw, dw.ChainParams()})
}

func (dw *DEXWallet) BlockNotifications(ctx context.Context) <-chan *dexbtc.BlockNotification {
	cl := dw.Wallet.NtfnServer.TransactionNotifications()
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
	return dw.asset.rescanAsync()
}

func (dw *DEXWallet) ForceRescan() {
	dw.asset.forceRescan()
}

func (dw *DEXWallet) Start() (dexbtc.SPVService, error) {
	return dw.asset.chainClient, nil
}

func (dw *DEXWallet) Reconfigure(*asset.WalletConfig, string) (bool, error) {
	return false, errors.New("Reconfigure not supported for Cyptopower btc wallet")
}

func (dw *DEXWallet) Birthday() time.Time {
	return dw.Manager.Birthday()
}

func (dw *DEXWallet) Peers() ([]*asset.WalletPeer, error) {
	peers := dw.asset.chainClient.CS.Peers()
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

func (dw *DEXWallet) AddPeer(address string) error {
	dw.asset.SetSpecificPeer(address)
	return nil
}

func (dw *DEXWallet) RemovePeer(address string) error {
	dw.asset.RemoveSpecificPeer(address)
	return nil
}

// secretSource is used to locate keys and redemption scripts while signing a
// transaction. secretSource satisfies the txauthor.SecretsSource interface.
type secretSource struct {
	w           *DEXWallet
	chainParams *chaincfg.Params
}

// ChainParams returns the chain parameters.
func (s *secretSource) ChainParams() *chaincfg.Params {
	return s.chainParams
}

// GetKey fetches a private key for the specified address.
func (s *secretSource) GetKey(addr btcutil.Address) (*btcec.PrivateKey, bool, error) {
	ma, err := s.w.Wallet.AddressInfo(addr)
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
	ma, err := s.w.Wallet.AddressInfo(addr)
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
