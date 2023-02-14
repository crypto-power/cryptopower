// Copyright (c) 2018-2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package spv

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"decred.org/dcrwallet/v2/errors"
	"decred.org/dcrwallet/v2/lru"
	"decred.org/dcrwallet/v2/p2p"
	"decred.org/dcrwallet/v2/validate"
	"decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/addrmgr/v2"
	"github.com/decred/dcrd/blockchain/stake/v4"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/gcs/v3/blockcf2"
	"github.com/decred/dcrd/wire"
	"golang.org/x/sync/errgroup"
)

// reqSvcs defines the services that must be supported by outbounded peers.
// After fetching more addresses (if needed), peers are disconnected from if
// they do not provide each of these services.
const reqSvcs = wire.SFNodeNetwork

// Syncer implements wallet synchronization services by over the Decred wire
// protocol using Simplified Payment Verification (SPV) with compact filters.
type Syncer struct {
	// atomics
	atomicCatchUpTryLock uint32          // CAS (entered=1) to perform discovery/rescan
	atomicWalletsSynced  map[int]*uint32 // CAS (synced=1) when wallet syncing complete

	wallets map[int]*wallet.Wallet
	lp      *p2p.LocalPeer

	// Protected by atomicCatchUpTryLock
	loadedFilters map[int]bool

	persistentPeers []string

	connectingRemotes map[string]struct{}
	remotes           map[string]*p2p.RemotePeer
	remotesMu         sync.Mutex

	// Data filters
	//
	// TODO: Replace precise rescan filter with wallet db accesses to avoid
	// needing to keep all relevant data in memory.
	rescanFilter map[int]*wallet.RescanFilter
	filterData   map[int]*blockcf2.Entries
	filterMu     sync.Mutex

	// seenTxs records hashes of received inventoried transactions.  Once a
	// transaction is fetched and processed from one peer, the hash is added to
	// this cache to avoid fetching it again from other peers that announce the
	// transaction.
	seenTxs lru.Cache

	// Sidechain management
	sidechains  map[int]*wallet.SidechainForest
	sidechainMu sync.Mutex

	currentLocators   []*chainhash.Hash
	locatorGeneration uint
	locatorMu         sync.Mutex

	// Holds all potential callbacks used to notify clients
	notifications *Notifications

	// Mempool for non-wallet-relevant transactions.
	mempool     sync.Map // k=chainhash.Hash v=*wire.MsgTx
	mempoolAdds chan *chainhash.Hash
}

// Notifications struct to contain all of the upcoming callbacks that will
// be used to update the rpc streams for syncing.
type Notifications struct {
	Synced                       func(walletID int, sync bool)
	PeerConnected                func(peerCount int32, addr string)
	PeerDisconnected             func(peerCount int32, addr string)
	FetchMissingCFiltersStarted  func(walletID int)
	FetchMissingCFiltersProgress func(walletID int, startCFiltersHeight, endCFiltersHeight int32)
	FetchMissingCFiltersFinished func(walletID int)
	FetchHeadersStarted          func(peerInitialHeight int32)
	FetchHeadersProgress         func(lastHeaderHeight int32, lastHeaderTime int64)
	FetchHeadersFinished         func()
	DiscoverAddressesStarted     func(walletID int)
	DiscoverAddressesFinished    func(walletID int)
	RescanStarted                func(walletID int)
	RescanProgress               func(walletID int, rescannedThrough int32)
	RescanFinished               func(walletID int)

	// MempoolTxs is called whenever new relevant unmined transactions are
	// observed and saved.
	MempoolTxs func(walletID int, txs []*wire.MsgTx)

	// TipChanged is called when the main chain tip block changes.
	// When reorgDepth is zero, the new block is a direct child of the previous tip.
	// If non-zero, one or more blocks described by the parameter were removed from
	// the previous main chain.
	// txs contains all relevant transactions mined in each attached block in
	// unspecified order.
	// reorgDepth is guaranteed to be non-negative.
	TipChanged func(tip *wire.BlockHeader, reorgDepth int32, txs []*wire.MsgTx)
}

// NewSyncer creates a Syncer that will sync the wallet using SPV.
func NewSyncer(wallets map[int]*wallet.Wallet, lp *p2p.LocalPeer) *Syncer {
	atomicWalletsSynced := make(map[int]*uint32, len(wallets))
	rescanFilter := make(map[int]*wallet.RescanFilter, len(wallets))
	filterData := make(map[int]*blockcf2.Entries, len(wallets))
	sidechains := make(map[int]*wallet.SidechainForest, len(wallets))

	for walletID := range wallets {
		atomicWalletsSynced[walletID] = new(uint32)
		rescanFilter[walletID] = wallet.NewRescanFilter(nil, nil)
		filterData[walletID] = &blockcf2.Entries{}
		sidechains[walletID] = &wallet.SidechainForest{}
	}

	return &Syncer{
		atomicWalletsSynced: atomicWalletsSynced,
		wallets:             wallets,
		loadedFilters:       make(map[int]bool, len(wallets)),
		connectingRemotes:   make(map[string]struct{}),
		remotes:             make(map[string]*p2p.RemotePeer),
		rescanFilter:        rescanFilter,
		filterData:          filterData,
		seenTxs:             lru.NewCache(2000),
		sidechains:          sidechains,
		lp:                  lp,
		mempoolAdds:         make(chan *chainhash.Hash),
	}
}

// SetPersistentPeers sets each peer as a persistent peer and disables DNS
// seeding and peer discovery.
func (s *Syncer) SetPersistentPeers(peers []string) {
	s.persistentPeers = peers
}

// SetNotifications sets the possible various callbacks that are used
// to notify interested parties to the syncing progress.
func (s *Syncer) SetNotifications(ntfns *Notifications) {
	s.notifications = ntfns
}

// syncedWallet checks the atomic that controls wallet syncness and if previously
// unsynced, updates to synced and notifies the callback, if set.
func (s *Syncer) syncedWallet(walletID int) {
	if atomic.CompareAndSwapUint32(s.atomicWalletsSynced[walletID], 0, 1) &&
		s.notifications != nil &&
		s.notifications.Synced != nil {
		s.notifications.Synced(walletID, true)
	}
}

// Synced returns whether this wallet is completely synced to the network.
func (s *Syncer) Synced() bool {
	synced := true
	for walletID := range s.wallets {
		synced = synced && atomic.LoadUint32(s.atomicWalletsSynced[walletID]) == 1
	}
	return synced
}

// EstimateMainChainTip returns an estimated height for the current tip of the
// blockchain. The estimate is made by comparing the initial height reported by
// all connected peers and the wallet's current tip. The highest of these values
// is estimated to be the mainchain's tip height.
func (s *Syncer) EstimateMainChainTip() int32 {
	_, chainTip, _ := s.highestChainTip(context.Background())
	s.forRemotes(func(rp *p2p.RemotePeer) error {
		if rp.InitialHeight() > chainTip {
			chainTip = rp.InitialHeight()
		}
		return nil
	})
	return chainTip
}

// GetRemotePeers returns a map of connected remote peers.
func (s *Syncer) GetRemotePeers() map[string]*p2p.RemotePeer {
	s.remotesMu.Lock()
	defer s.remotesMu.Unlock()

	remotes := make(map[string]*p2p.RemotePeer, len(s.remotes))
	for k, rp := range s.remotes {
		remotes[k] = rp
	}
	return remotes
}

// unsynced checks the atomic that controls wallet syncness and if previously
// synced, updates to unsynced and notifies the callback, if set.
func (s *Syncer) unsynced(walletID int) {
	if atomic.CompareAndSwapUint32(s.atomicWalletsSynced[walletID], 1, 0) &&
		s.notifications != nil &&
		s.notifications.Synced != nil {
		s.notifications.Synced(walletID, false)
	}
}

// peerConnected updates the notification for peer count, if set.
func (s *Syncer) peerConnected(remotesCount int, addr string) {
	if s.notifications != nil && s.notifications.PeerConnected != nil {
		s.notifications.PeerConnected(int32(remotesCount), addr)
	}
}

// peerDisconnected updates the notification for peer count, if set.
func (s *Syncer) peerDisconnected(remotesCount int, addr string) {
	if s.notifications != nil && s.notifications.PeerDisconnected != nil {
		s.notifications.PeerDisconnected(int32(remotesCount), addr)
	}
}

func (s *Syncer) fetchMissingCfiltersStart(walletID int) {
	if s.notifications != nil && s.notifications.FetchMissingCFiltersStarted != nil {
		s.notifications.FetchMissingCFiltersStarted(walletID)
	}
}

func (s *Syncer) fetchMissingCfiltersProgress(walletID int, startMissingCFilterHeight, endMissinCFilterHeight int32) {
	if s.notifications != nil && s.notifications.FetchMissingCFiltersProgress != nil {
		s.notifications.FetchMissingCFiltersProgress(walletID, startMissingCFilterHeight, endMissinCFilterHeight)
	}
}

func (s *Syncer) fetchMissingCfiltersFinished(walletID int) {
	if s.notifications != nil && s.notifications.FetchMissingCFiltersFinished != nil {
		s.notifications.FetchMissingCFiltersFinished(walletID)
	}
}

func (s *Syncer) fetchHeadersStart(peerInitialHeight int32) {
	if s.notifications != nil && s.notifications.FetchHeadersStarted != nil {
		s.notifications.FetchHeadersStarted(peerInitialHeight)
	}
}

func (s *Syncer) fetchHeadersProgress(lastHeader *wire.BlockHeader) {
	if s.notifications != nil && s.notifications.FetchHeadersProgress != nil {
		s.notifications.FetchHeadersProgress(int32(lastHeader.Height), lastHeader.Timestamp.Unix())
	}
}

func (s *Syncer) fetchHeadersFinished() {
	if s.notifications != nil && s.notifications.FetchHeadersFinished != nil {
		s.notifications.FetchHeadersFinished()
	}
}

func (s *Syncer) discoverAddressesStart(walletID int) {
	if s.notifications != nil && s.notifications.DiscoverAddressesStarted != nil {
		s.notifications.DiscoverAddressesStarted(walletID)
	}
}

func (s *Syncer) discoverAddressesFinished(walletID int) {
	if s.notifications != nil && s.notifications.DiscoverAddressesFinished != nil {
		s.notifications.DiscoverAddressesFinished(walletID)
	}
}

func (s *Syncer) rescanStart(walletID int) {
	if s.notifications != nil && s.notifications.RescanStarted != nil {
		s.notifications.RescanStarted(walletID)
	}
}

func (s *Syncer) rescanProgress(walletID int, rescannedThrough int32) {
	if s.notifications != nil && s.notifications.RescanProgress != nil {
		s.notifications.RescanProgress(walletID, rescannedThrough)
	}
}

func (s *Syncer) rescanFinished(walletID int) {
	if s.notifications != nil && s.notifications.RescanFinished != nil {
		s.notifications.RescanFinished(walletID)
	}
}

func (s *Syncer) mempoolTxs(walletID int, txs []*wire.MsgTx) {
	if s.notifications != nil && s.notifications.MempoolTxs != nil {
		s.notifications.MempoolTxs(walletID, txs)
	}
}

func (s *Syncer) tipChanged(tip *wire.BlockHeader, reorgDepth int32, matchingTxs map[chainhash.Hash][]*wire.MsgTx) {
	if s.notifications != nil && s.notifications.TipChanged != nil {
		var txs []*wire.MsgTx
		for _, matching := range matchingTxs {
			txs = append(txs, matching...)
		}
		s.notifications.TipChanged(tip, reorgDepth, txs)
	}
}

// setRequiredHeight sets the required height a peer must advertise as their
// last height.  Initial height 6 blocks below the current chain tip height
// result in a handshake error.
func (s *Syncer) setRequiredHeight(tipHeight int32) {
	requireHeight := tipHeight
	if requireHeight > 6 {
		requireHeight -= 6
	}
	s.lp.RequirePeerHeight(requireHeight)
}

func (s *Syncer) lowestChainTip(ctx context.Context) (chainhash.Hash, int32, *wallet.Wallet) {
	var lowestTip int32 = -1
	var lowestTipHash chainhash.Hash
	var lowestTipWallet *wallet.Wallet
	for _, w := range s.wallets {
		if hash, height := w.MainChainTip(ctx); height < lowestTip || lowestTip == -1 {
			lowestTip = height
			lowestTipHash = hash
			lowestTipWallet = w
		}
	}

	return lowestTipHash, lowestTip, lowestTipWallet
}

func (s *Syncer) highestChainTip(ctx context.Context) (chainhash.Hash, int32, *wallet.Wallet) {
	var highestTip int32 = -1
	var highestTipHash chainhash.Hash
	var highestTipWallet *wallet.Wallet
	for _, w := range s.wallets {
		if hash, height := w.MainChainTip(ctx); height > highestTip || highestTip == -1 {
			highestTip = height
			highestTipHash = hash
			highestTipWallet = w
		}
	}

	return highestTipHash, highestTip, highestTipWallet
}

// Run synchronizes the wallet, returning when synchronization fails or the
// context is cancelled.
func (s *Syncer) Run(ctx context.Context) error {
	log.Infof("Syncing %d wallets", len(s.wallets))

	var highestTipHeight int32
	for id, w := range s.wallets {
		tipHash, tipHeight := w.MainChainTip(ctx)
		log.Infof("[%d] Headers synced through block %v height %d", id, &tipHash, tipHeight)

		if tipHeight > highestTipHeight {
			highestTipHeight = tipHeight
		}

		rescanPoint, err := w.RescanPoint(ctx)
		if err != nil {
			return err
		}
		if rescanPoint != nil {
			h, err := w.BlockHeader(ctx, rescanPoint)
			if err != nil {
				return err
			}
			// The rescan point is the first block that does not have synced
			// transactions, so we are synced with the parent.
			log.Infof("[%d] Transactions synced through block %v height %d", id, &h.PrevBlock, h.Height-1)
		} else {
			log.Infof("[%d] Transactions synced through block %v height %d", id, &tipHash, tipHeight)
		}
	}
	s.setRequiredHeight(highestTipHeight)

	_, _, lowestChainWallet := s.lowestChainTip(ctx)
	locators, err := lowestChainWallet.BlockLocators(ctx, nil)
	if err != nil {
		return err
	}
	s.currentLocators = locators

	s.lp.AddrManager().Start()
	defer func() {
		err := s.lp.AddrManager().Stop()
		if err != nil {
			log.Errorf("Failed to cleanly stop address manager: %v", err)
		}
	}()

	// Seed peers over DNS when not disabled by persistent peers.
	if len(s.persistentPeers) == 0 {
		s.lp.SeedPeers(ctx, wire.SFNodeNetwork)
	}

	// Start background handlers to read received messages from remote peers
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.receiveGetData(ctx) })
	g.Go(func() error { return s.receiveInv(ctx) })
	g.Go(func() error { return s.receiveHeadersAnnouncements(ctx) })
	s.lp.AddHandledMessages(p2p.MaskGetData | p2p.MaskInv)

	if len(s.persistentPeers) != 0 {
		for i := range s.persistentPeers {
			raddr := s.persistentPeers[i]
			g.Go(func() error { return s.connectToPersistent(ctx, raddr) })
		}
	} else {
		g.Go(func() error { return s.connectToCandidates(ctx) })
	}

	g.Go(func() error { return s.handleMempool(ctx) })

	for walletID, w := range s.wallets {
		walletBackend := &WalletBackend{
			Syncer:   s,
			WalletID: walletID,
		}

		w.SetNetworkBackend(walletBackend)
		defer w.SetNetworkBackend(nil)
	}

	// Wait until cancellation or a handler errors.
	return g.Wait()
}

// peerCandidate returns a peer address that we shall attempt to connect to.
// Only peers not already remotes or in the process of connecting are returned.
// Any address returned is marked in s.connectingRemotes before returning.
func (s *Syncer) peerCandidate(svcs wire.ServiceFlag) (*addrmgr.NetAddress, error) {
	// Try to obtain peer candidates at random, decreasing the requirements
	// as more tries are performed.
	for tries := 0; tries < 100; tries++ {
		kaddr := s.lp.AddrManager().GetAddress()
		if kaddr == nil {
			break
		}
		na := kaddr.NetAddress()

		k := na.Key()
		s.remotesMu.Lock()
		_, isConnecting := s.connectingRemotes[k]
		_, isRemote := s.remotes[k]

		switch {
		// Skip peer if already connected, or in process of connecting
		// TODO: this should work with network blocks, not exact addresses.
		case isConnecting || isRemote:
			fallthrough
		// Only allow recent nodes (10mins) after we failed 30 times
		case tries < 30 && time.Since(kaddr.LastAttempt()) < 10*time.Minute:
			fallthrough
		// Skip peers without matching service flags for the first 50 tries.
		case tries < 50 && kaddr.NetAddress().Services&svcs != svcs:
			s.remotesMu.Unlock()
			continue
		}

		s.connectingRemotes[k] = struct{}{}
		s.remotesMu.Unlock()

		return na, nil
	}
	return nil, errors.New("no addresses")
}

func (s *Syncer) connectToPersistent(ctx context.Context, raddr string) error {
	for {
		func() {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			rp, err := s.lp.ConnectOutbound(ctx, raddr, reqSvcs)
			if err != nil {
				if ctx.Err() == nil {
					log.Errorf("Peering attempt failed: %v", err)
				}
				return
			}
			log.Infof("New peer %v %v %v", raddr, rp.UA(), rp.Services())

			k := rp.NA().Key()
			s.remotesMu.Lock()
			s.remotes[k] = rp
			n := len(s.remotes)
			s.remotesMu.Unlock()
			s.peerConnected(n, k)

			wait := make(chan struct{})
			go func() {
				err := s.startupSync(ctx, rp)
				if err != nil {
					rp.Disconnect(err)
				}
				wait <- struct{}{}
			}()

			err = rp.Err()
			s.remotesMu.Lock()
			delete(s.remotes, k)
			n = len(s.remotes)
			s.remotesMu.Unlock()
			s.peerDisconnected(n, k)
			<-wait
			if ctx.Err() != nil {
				return
			}
			log.Warnf("Lost peer %v: %v", raddr, err)
		}()

		if err := ctx.Err(); err != nil {
			return err
		}

		time.Sleep(5 * time.Second)
	}
}

func (s *Syncer) connectToCandidates(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	sem := make(chan struct{}, 8)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		na, err := s.peerCandidate(reqSvcs)
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				<-sem
				continue
			}
		}

		wg.Add(1)
		go func() {
			ctx, cancel := context.WithCancel(ctx)
			defer func() {
				cancel()
				wg.Done()
				<-sem
			}()

			// Make outbound connections to remote peers.
			raddr := na.String()
			rp, err := s.lp.ConnectOutbound(ctx, raddr, reqSvcs)
			if err != nil {
				s.remotesMu.Lock()
				delete(s.connectingRemotes, raddr)
				s.remotesMu.Unlock()
				if ctx.Err() == nil {
					log.Warnf("Peering attempt failed: %v", err)
				}
				return
			}
			log.Infof("New peer %v %v %v", raddr, rp.UA(), rp.Services())

			s.remotesMu.Lock()
			delete(s.connectingRemotes, raddr)
			s.remotes[raddr] = rp
			n := len(s.remotes)
			s.remotesMu.Unlock()
			s.peerConnected(n, raddr)

			wait := make(chan struct{})
			go func() {
				err := s.startupSync(ctx, rp)
				if err != nil {
					rp.Disconnect(err)
				}
				wait <- struct{}{}
			}()

			err = rp.Err()
			if ctx.Err() != context.Canceled {
				log.Warnf("Lost peer %v: %v", raddr, err)
			}

			<-wait
			s.remotesMu.Lock()
			delete(s.remotes, raddr)
			n = len(s.remotes)
			s.remotesMu.Unlock()
			s.peerDisconnected(n, raddr)
		}()
	}
}

func (s *Syncer) forRemotes(f func(rp *p2p.RemotePeer) error) error {
	defer s.remotesMu.Unlock()
	s.remotesMu.Lock()
	if len(s.remotes) == 0 {
		return errors.E(errors.NoPeers)
	}
	for _, rp := range s.remotes {
		err := f(rp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncer) pickRemote(pick func(*p2p.RemotePeer) bool) (*p2p.RemotePeer, error) {
	defer s.remotesMu.Unlock()
	s.remotesMu.Lock()

	for _, rp := range s.remotes {
		if pick(rp) {
			return rp, nil
		}
	}
	return nil, errors.E(errors.NoPeers)
}

func (s *Syncer) getTransactionsByHashes(ctx context.Context, txHashes []*chainhash.Hash) ([]*wire.MsgTx, []*wire.InvVect, error) {
	if len(txHashes) == 0 {
		return nil, nil, nil
	}

	var notFound []*wire.InvVect
	var foundTxs []*wire.MsgTx

	for walletID, w := range s.wallets {
		walletFoundTxs, _, err := w.GetTransactionsByHashes(ctx, txHashes)
		if err != nil && !errors.Is(err, errors.NotExist) {
			return nil, nil, errors.Errorf("[%d] Failed to look up transactions for getdata reply to peer: %v", walletID, err)
		}

		if len(walletFoundTxs) != 0 {
			foundTxs = append(foundTxs, walletFoundTxs...)
		}

		// remove hashes for found txs from the `txHashes` slice
		// so that the next wallet does not attempt to find them.
		for _, tx := range walletFoundTxs {
			for index, hash := range txHashes {
				if tx.TxHash() == *hash {
					txHashes = append(txHashes[:index], txHashes[index+1:]...)
				}
			}
		}

		if len(txHashes) == 0 {
			break
		}
	}

	for _, hash := range txHashes {
		notFound = append(notFound, wire.NewInvVect(wire.InvTypeTx, hash))
	}

	return foundTxs, notFound, nil
}

// receiveGetData handles all received getdata requests from peers.  An inv
// message declaring knowledge of the data must have been previously sent to the
// peer, or a notfound message reports the data as missing.  Only transactions
// may be queried by a peer.
func (s *Syncer) receiveGetData(ctx context.Context) error {
	var wg sync.WaitGroup
	for {
		rp, msg, err := s.lp.ReceiveGetData(ctx)
		if err != nil {
			wg.Wait()
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Ensure that the data was (recently) announced using an inv.
			var txHashes []*chainhash.Hash
			var notFound []*wire.InvVect
			for _, inv := range msg.InvList {
				if !rp.InvsSent().Contains(inv.Hash) {
					notFound = append(notFound, inv)
					continue
				}
				switch inv.Type {
				case wire.InvTypeTx:
					txHashes = append(txHashes, &inv.Hash)
				default:
					notFound = append(notFound, inv)
				}
			}

			// Search for requested transactions
			var foundTxs []*wire.MsgTx
			if len(txHashes) != 0 {
				var missing []*wire.InvVect
				var err error
				foundTxs, missing, err = s.getTransactionsByHashes(ctx, txHashes)
				if err != nil && !errors.Is(err, errors.NotExist) {
					log.Warnf("Failed to look up transactions for getdata reply to peer %v: %v",
						rp.RemoteAddr(), err)
					return
				}

				// For the missing ones, attempt to search in
				// the non-wallet-relevant syncer mempool.
				for _, miss := range missing {
					if v, ok := s.mempool.Load(miss.Hash); ok {
						tx := v.(*wire.MsgTx)
						foundTxs = append(foundTxs, tx)
						continue
					}
					notFound = append(notFound, miss)
				}
			}

			// Send all found transactions
			for _, tx := range foundTxs {
				err := rp.SendMessage(ctx, tx)
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					log.Warnf("Failed to send getdata reply to peer %v: %v",
						rp.RemoteAddr(), err)
				}
			}

			// Send notfound message for all missing or unannounced data.
			if len(notFound) != 0 {
				err := rp.SendMessage(ctx, &wire.MsgNotFound{InvList: notFound})
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					log.Warnf("Failed to send notfound reply to peer %v: %v",
						rp.RemoteAddr(), err)
				}
			}
		}()
	}
}

// receiveInv receives all inv messages from peers and starts goroutines to
// handle block and tx announcements.
func (s *Syncer) receiveInv(ctx context.Context) error {
	var wg sync.WaitGroup
	for {
		rp, msg, err := s.lp.ReceiveInv(ctx)
		if err != nil {
			wg.Wait()
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			var blocks []*chainhash.Hash
			var txs []*chainhash.Hash

			for _, inv := range msg.InvList {
				switch inv.Type {
				case wire.InvTypeBlock:
					blocks = append(blocks, &inv.Hash)
				case wire.InvTypeTx:
					txs = append(txs, &inv.Hash)
				}
			}

			if len(blocks) != 0 {
				wg.Add(1)
				go func() {
					defer wg.Done()

					err := s.handleBlockInvs(ctx, rp, blocks)
					if ctx.Err() != nil {
						return
					}
					if errors.Is(err, errors.Protocol) || errors.Is(err, errors.Consensus) {
						log.Warnf("Disconnecting peer %v: %v", rp, err)
						rp.Disconnect(err)
						return
					}
					if err != nil {
						log.Warnf("Failed to handle blocks inventoried by %v: %v", rp, err)
					}
				}()
			}
			if len(txs) != 0 {
				wg.Add(1)
				go func() {
					s.handleTxInvs(ctx, rp, txs)
					wg.Done()
				}()
			}
		}()
	}
}

func (s *Syncer) handleBlockInvs(ctx context.Context, rp *p2p.RemotePeer, hashes []*chainhash.Hash) error {
	const opf = "spv.handleBlockInvs(%v)"

	// We send a sendheaders msg at the end of our startup stage. Ignore
	// any invs sent before that happens, since we'll still be performing
	// an initial sync with the peer.
	if !rp.SendHeadersSent() {
		log.Debugf("Ignoring block invs from %v before "+
			"sendheaders is sent", rp)
		return nil
	}

	blocks, err := rp.Blocks(ctx, hashes)
	if err != nil {
		op := errors.Opf(opf, rp)
		return errors.E(op, err)
	}
	headers := make([]*wire.BlockHeader, len(blocks))
	bmap := make(map[chainhash.Hash]*wire.MsgBlock)
	for i, block := range blocks {
		bmap[block.BlockHash()] = block
		h := block.Header
		headers[i] = &h
	}

	return s.handleBlockAnnouncements(ctx, rp, headers, bmap)
}

// handleTxInvs responds to the inv message created by rp by fetching
// all unseen transactions announced by the peer.  Any transactions
// that are relevant to the wallet are saved as unconfirmed
// transactions.  Transaction invs are ignored when a rescan is
// necessary or ongoing.
func (s *Syncer) handleTxInvs(ctx context.Context, rp *p2p.RemotePeer, hashes []*chainhash.Hash) {
	for walletID := range s.wallets {
		s.handleTxInvsForWallet(ctx, rp, hashes, walletID)
	}
}

func (s *Syncer) handleTxInvsForWallet(ctx context.Context, rp *p2p.RemotePeer, hashes []*chainhash.Hash, walletID int) {
	wallet := s.wallets[walletID]
	const opf = "spv.handleTxInvs(%v)"

	rpt, err := wallet.RescanPoint(ctx)
	if err != nil {
		op := errors.Opf(opf, rp.RemoteAddr())
		log.Warn(errors.E(op, err))
		return
	}

	if rpt != nil {
		return // don't process new txs for wallets with a pending rescan.
	}

	// Ignore already-processed transactions
	unseen := hashes[:0]
	for _, h := range hashes {
		if !s.seenTxs.Contains(*h) {
			unseen = append(unseen, h)
		}
	}
	if len(unseen) == 0 {
		return
	}

	txs, err := rp.Transactions(ctx, unseen)
	if errors.Is(err, errors.NotExist) {
		err = nil
		// Remove notfound txs.
		prevTxs, prevUnseen := txs, unseen
		txs, unseen = txs[:0], unseen[:0]
		for i, tx := range prevTxs {
			if tx != nil {
				txs = append(txs, tx)
				unseen = append(unseen, prevUnseen[i])
			}
		}
	}
	if err != nil {
		if ctx.Err() == nil {
			op := errors.Opf(opf, rp.RemoteAddr())
			err := errors.E(op, err)
			log.Warn(err)
		}
		return
	}

	// Mark transactions as processed so they are not queried from other nodes
	// who announce them in the future.
	for _, h := range unseen {
		s.seenTxs.Add(*h)
	}

	// Save any relevant transaction.
	relevant := s.filterRelevant(txs, walletID)
	for _, tx := range relevant {
		if wallet.ManualTickets() && stake.IsSStx(tx) {
			continue
		}
		err := wallet.AddTransaction(ctx, tx, nil)
		if err != nil {
			op := errors.Opf(opf, rp.RemoteAddr())
			log.Warn(errors.E(op, err))
		}
	}
	s.mempoolTxs(walletID, relevant)
}

// receiveHeaderAnnouncements receives all block announcements through pushed
// headers messages messages from peers and starts goroutines to handle the
// announced header.
func (s *Syncer) receiveHeadersAnnouncements(ctx context.Context) error {
	for {
		rp, headers, err := s.lp.ReceiveHeadersAnnouncement(ctx)
		if err != nil {
			return err
		}

		go func() {
			err := s.handleBlockAnnouncements(ctx, rp, headers, nil)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				if errors.Is(err, errors.Protocol) || errors.Is(err, errors.Consensus) {
					log.Warnf("Disconnecting peer %v: %v", rp, err)
					rp.Disconnect(err)
					return
				}

				log.Warnf("Failed to handle headers announced by %v: %v", rp, err)
			}
		}()
	}
}

// scanChain checks for matching filters of chain and returns a map of
// relevant wallet transactions keyed by block hash.  bmap is queried
// for the block first with fallback to querying rp using getdata.
func (s *Syncer) scanChain(ctx context.Context, rp *p2p.RemotePeer, chain []*wallet.BlockNode,
	bmap map[chainhash.Hash]*wire.MsgBlock, walletID int,
) (map[chainhash.Hash][]*wire.MsgTx, error) {
	found := make(map[chainhash.Hash][]*wire.MsgTx)

	s.filterMu.Lock()
	filterData := *s.filterData[walletID]
	s.filterMu.Unlock()

	fetched := make([]*wire.MsgBlock, len(chain))
	if bmap != nil {
		for i := range chain {
			if b, ok := bmap[*chain[i].Hash]; ok {
				fetched[i] = b
			}
		}
	}

	idx := 0
FilterLoop:
	for idx < len(chain) {
		var fmatches []*chainhash.Hash
		var fmatchidx []int
		var fmatchMu sync.Mutex

		// Scan remaining filters with up to ncpu workers
		c := make(chan int)
		var wg sync.WaitGroup
		worker := func() {
			for i := range c {
				n := chain[i]
				f := n.FilterV2
				k := blockcf2.Key(&n.Header.MerkleRoot)
				if f.N() != 0 && f.MatchAny(k, filterData) {
					fmatchMu.Lock()
					fmatches = append(fmatches, n.Hash)
					fmatchidx = append(fmatchidx, i)
					fmatchMu.Unlock()
				}
			}
			wg.Done()
		}
		nworkers := 0
		for i := idx; i < len(chain); i++ {
			if fetched[i] != nil {
				continue // Already have block
			}
			select {
			case c <- i:
			default:
				if nworkers < runtime.NumCPU() {
					nworkers++
					wg.Add(1)
					go worker()
				}
				c <- i
			}
		}
		close(c)
		wg.Wait()

		if len(fmatches) != 0 {
			blocks, err := rp.Blocks(ctx, fmatches)
			if err != nil {
				return nil, err
			}
			for j, b := range blocks {
				i := fmatchidx[j]

				// Perform context-free validation on the block.
				// Disconnect peer when invalid.
				err := validate.MerkleRoots(b)
				if err != nil {
					err = validate.DCP0005MerkleRoot(b)
				}
				if err != nil {
					rp.Disconnect(err)
					return nil, err
				}

				fetched[i] = b
			}
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		for i := idx; i < len(chain); i++ {
			b := fetched[i]
			if b == nil {
				continue
			}
			matches, fadded := s.rescanBlock(b, walletID)
			found[*chain[i].Hash] = matches
			if len(fadded) != 0 {
				idx = i + 1
				filterData = fadded
				continue FilterLoop
			}
		}
		return found, nil
	}
	return found, nil
}

// handleBlockAnnouncements handles blocks announced through block invs or
// headers messages by rp.  bmap should contain the full blocks of any
// inventoried blocks, but may be nil in case the blocks were announced through
// headers.
func (s *Syncer) handleBlockAnnouncements(ctx context.Context, rp *p2p.RemotePeer, headers []*wire.BlockHeader,
	bmap map[chainhash.Hash]*wire.MsgBlock,
) (err error) {
	const opf = "spv.handleBlockAnnouncements(%v)"
	defer func() {
		if err != nil && ctx.Err() == nil {
			op := errors.Opf(opf, rp.RemoteAddr())
			err = errors.E(op, err)
		}
	}()

	if len(headers) == 0 {
		return nil
	}

	firstHeader := headers[0]

	// Disconnect if the peer announced a header that is significantly
	// behind our main chain height.
	const maxAnnHeaderTipDelta = int32(256)
	_, tipHeight, _ := s.highestChainTip(ctx)
	if int32(firstHeader.Height) < tipHeight && tipHeight-int32(firstHeader.Height) > maxAnnHeaderTipDelta {
		err = errors.E(errors.Protocol, "peer announced old header")
		return err
	}

	for walletID := range s.wallets {
		err = s.handleBlockAnnouncementsForWallet(ctx, walletID, rp, headers, bmap)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Syncer) handleBlockAnnouncementsForWallet(ctx context.Context, walletID int, rp *p2p.RemotePeer, headers []*wire.BlockHeader,
	bmap map[chainhash.Hash]*wire.MsgBlock,
) (err error) {
	firstHeader := headers[0]
	w := s.wallets[walletID]
	newBlocks := make([]*wallet.BlockNode, 0, len(headers))
	var bestChain []*wallet.BlockNode
	var matchingTxs map[chainhash.Hash][]*wire.MsgTx
	cnet := w.ChainParams().Net
	err = func() error {
		defer s.sidechainMu.Unlock()
		s.sidechainMu.Lock()

		// Determine if the peer sent a header that connects to an
		// unknown sidechain (i.e. an orphan chain). In that case,
		// re-request headers to hopefully find the missing ones.
		//
		// The header is an orphan if its parent block is not in the
		// mainchain nor on a previously known side chain.
		prevInMainChain, _, err := w.BlockInMainChain(ctx, &firstHeader.PrevBlock)
		if err != nil {
			return err
		}
		if !prevInMainChain && !s.sidechains[walletID].HasSideChainBlock(&firstHeader.PrevBlock) {
			if err := rp.ReceivedOrphanHeader(); err != nil {
				return err
			}

			locators, err := w.BlockLocators(ctx, nil)
			if err != nil {
				return err
			}
			if err := rp.HeadersAsync(ctx, locators, &hashStop); err != nil {
				return err
			}

			// We requested async headers, so return early and wait
			// for the next headers msg.
			//
			// newBlocks and bestChain are empty at this point, so
			// the rest of this function continues without
			// producing side effects.
			return nil
		}

		for i := range headers {
			hash := headers[i].BlockHash()

			// Skip the first blocks sent if they are already in
			// the mainchain or on a known side chain. We only skip
			// those at the start of the list to ensure every block
			// in newBlocks still connects in sequence.
			if len(newBlocks) == 0 {
				haveBlock, _, err := w.BlockInMainChain(ctx, &hash)
				if err != nil {
					return err
				}
				if haveBlock || s.sidechains[walletID].HasSideChainBlock(&hash) {
					continue
				}
			}

			n := wallet.NewBlockNode(headers[i], &hash, nil)
			newBlocks = append(newBlocks, n)
		}

		if len(newBlocks) == 0 {
			// Peer did not send any headers we didn't already
			// have.
			return nil
		}

		fullsc, err := s.sidechains[walletID].FullSideChain(newBlocks)
		if err != nil {
			return err
		}
		_, err = w.ValidateHeaderChainDifficulties(ctx, fullsc, 0)
		if err != nil {
			return err
		}

		for _, n := range newBlocks {
			s.sidechains[walletID].AddBlockNode(n)
		}

		bestChain, err = w.EvaluateBestChain(ctx, s.sidechains[walletID])
		if err != nil {
			return err
		}

		if len(bestChain) == 0 {
			return nil
		}

		bestChainHashes := make([]*chainhash.Hash, len(bestChain))
		for i, n := range bestChain {
			bestChainHashes[i] = n.Hash
		}

		filters, err := rp.CFiltersV2(ctx, bestChainHashes)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		for i, cf := range filters {
			filter, proofIndex, proof := cf.Filter, cf.ProofIndex, cf.Proof

			err = validate.CFilterV2HeaderCommitment(cnet,
				bestChain[i].Header, filter, proofIndex, proof)
			if err != nil {
				return err
			}

			bestChain[i].FilterV2 = filter
		}

		rpt, err := w.RescanPoint(ctx)
		if err != nil {
			return err
		}
		if rpt == nil {
			matchingTxs, err = s.scanChain(ctx, rp, bestChain, bmap, walletID)
			if err != nil {
				return err
			}
		}

		prevChain, err := w.ChainSwitch(ctx, s.sidechains[walletID], bestChain, matchingTxs)
		if err != nil {
			return err
		}
		if len(prevChain) != 0 {
			log.Infof("[%d] Reorganize from %v to %v (total %d block(s) reorged)",
				walletID, prevChain[len(prevChain)-1].Hash, bestChain[len(bestChain)-1].Hash, len(prevChain))
			for _, n := range prevChain {
				s.sidechains[walletID].AddBlockNode(n)
			}
		}
		tipHeader := bestChain[len(bestChain)-1].Header
		s.setRequiredHeight(int32(tipHeader.Height))
		s.tipChanged(tipHeader, int32(len(prevChain)), matchingTxs)

		return nil
	}()
	if err != nil {
		return err
	}

	if len(bestChain) != 0 {
		s.locatorMu.Lock()
		s.currentLocators = nil
		s.locatorGeneration++
		s.locatorMu.Unlock()
	}

	// Log connected blocks.
	for _, n := range bestChain {
		log.Infof("[%d] Connected block %v, height %d, %d wallet transaction(s)",
			walletID, n.Hash, n.Header.Height, len(matchingTxs[*n.Hash]))
	}
	// Announced blocks not in the main chain are logged as sidechain or orphan
	// blocks.
	for _, n := range newBlocks {
		haveBlock, _, err := w.BlockInMainChain(ctx, n.Hash)
		if err != nil {
			return err
		}
		if haveBlock {
			continue
		}
		log.Infof("[%d] Received sidechain or orphan block %v, height %v", walletID,
			n.Hash, n.Header.Height)
	}
	return nil
}

// hashStop is a zero value stop hash for fetching all possible data using
// locators.
var hashStop chainhash.Hash

// getHeaders iteratively fetches headers from rp using the latest locators.
// Returns when no more headers are available.  A sendheaders message is pushed
// to the peer when there are no more headers to fetch.
func (s *Syncer) getHeaders(ctx context.Context, rp *p2p.RemotePeer) error {
	_, _, lowestChainWallet := s.lowestChainTip(ctx)

	var locators []*chainhash.Hash
	var err error
	s.locatorMu.Lock()
	locators = s.currentLocators
	if len(locators) == 0 {
		locators, err = lowestChainWallet.BlockLocators(ctx, nil)
		if err != nil {
			s.locatorMu.Unlock()
			return err
		}
		s.currentLocators = locators
		s.locatorGeneration++
	}
	s.locatorMu.Unlock()

	var lastHeight int32
	cnet := lowestChainWallet.ChainParams().Net

	for {
		headers, err := rp.Headers(ctx, locators, &hashStop)
		if err != nil {
			return err
		}

		if len(headers) == 0 {
			// Ensure that the peer provided headers through the height
			// advertised during handshake.
			if lastHeight < rp.InitialHeight() {
				// Peer may not have provided any headers if our own locators
				// were up to date.  Compare the best locator hash with the
				// advertised height.
				h, err := lowestChainWallet.BlockHeader(ctx, locators[0])
				if err == nil && int32(h.Height) < rp.InitialHeight() {
					return errors.E(errors.Protocol, "peer did not provide "+
						"headers through advertised height")
				}
			}

			return rp.SendHeaders(ctx)
		}

		lastHeight = int32(headers[len(headers)-1].Height)

		nodes := make([]*wallet.BlockNode, len(headers))
		g, ctx := errgroup.WithContext(ctx)
		for i := range headers {
			i := i
			g.Go(func() error {
				header := headers[i]
				hash := header.BlockHash()
				filter, proofIndex, proof, err := rp.CFilterV2(ctx, &hash)
				if err != nil {
					return err
				}

				err = validate.CFilterV2HeaderCommitment(cnet, header,
					filter, proofIndex, proof)
				if err != nil {
					return err
				}

				nodes[i] = wallet.NewBlockNode(header, &hash, filter)
				if wallet.BadCheckpoint(cnet, &hash, int32(header.Height)) {
					nodes[i].BadCheckpoint()
				}
				return nil
			})
		}
		err = g.Wait()
		if err != nil {
			return err
		}

		for walletID, w := range s.wallets {
			var added int
			s.sidechainMu.Lock()
			for _, n := range nodes {
				haveBlock, _, _ := w.BlockInMainChain(ctx, n.Hash)
				if haveBlock {
					continue
				}
				if s.sidechains[walletID].AddBlockNode(n) {
					added++
				}
			}

			log.Debugf("[%d] Fetched %d new header(s) ending at height %d from %v",
				walletID, added, nodes[len(nodes)-1].Header.Height, rp)

			bestChain, err := w.EvaluateBestChain(ctx, s.sidechains[walletID])
			if err != nil {
				s.sidechainMu.Unlock()
				return err
			}
			if len(bestChain) == 0 {
				s.sidechainMu.Unlock()
				continue
			}

			_, err = w.ValidateHeaderChainDifficulties(ctx, bestChain, 0)
			if err != nil {
				s.sidechainMu.Unlock()
				return err
			}

			prevChain, err := w.ChainSwitch(ctx, s.sidechains[walletID], bestChain, nil)
			if err != nil {
				s.sidechainMu.Unlock()
				return err
			}

			if len(prevChain) != 0 {
				log.Infof("[%d] Reorganize from %v to %v (total %d block(s) reorged)",
					walletID, prevChain[len(prevChain)-1].Hash, bestChain[len(bestChain)-1].Hash, len(prevChain))
				for _, n := range prevChain {
					s.sidechains[walletID].AddBlockNode(n)
				}
			}
			tip := bestChain[len(bestChain)-1]
			if len(bestChain) == 1 {
				log.Infof("[%d] Connected block %v, height %d", walletID, tip.Hash, tip.Header.Height)
			} else {
				s.fetchHeadersProgress(headers[len(headers)-1])
				log.Infof("[%d] Connected %d blocks, new tip %v, height %d, date %v",
					walletID, len(bestChain), tip.Hash, tip.Header.Height, tip.Header.Timestamp)
			}

			s.sidechainMu.Unlock()
		}

		// Generate new locators
		s.locatorMu.Lock()
		locators, err = lowestChainWallet.BlockLocators(ctx, nil)
		if err != nil {
			s.locatorMu.Unlock()
			return err
		}
		s.currentLocators = locators
		s.locatorGeneration++
		s.locatorMu.Unlock()
	}
}

func (s *Syncer) fetchMissingCFilters(ctx context.Context, rp *p2p.RemotePeer) error {
	for walletID, w := range s.wallets {
		s.fetchMissingCfiltersStart(walletID)
		progress := make(chan wallet.MissingCFilterProgress, 1)
		go w.FetchMissingCFiltersWithProgress(ctx, rp, progress)

		for p := range progress {
			if p.Err != nil {
				return p.Err
			}
			s.fetchMissingCfiltersProgress(walletID, p.BlockHeightStart, p.BlockHeightEnd)
		}
		s.fetchMissingCfiltersFinished(walletID)
	}
	return nil
}

func (s *Syncer) startupSync(ctx context.Context, rp *p2p.RemotePeer) error {
	// Disconnect from the peer if their advertised block height is
	// significantly behind the highest block height recorded by all wallets.
	_, tipHeight, _ := s.highestChainTip(ctx)
	if rp.InitialHeight() < tipHeight-6 {
		return errors.E("peer is not synced")
	}

	if err := s.fetchMissingCFilters(ctx, rp); err != nil {
		return err
	}

	// Fetch any unseen headers from the peer.
	s.fetchHeadersStart(rp.InitialHeight())
	log.Debugf("Fetching headers from %v", rp.RemoteAddr())
	err := s.getHeaders(ctx, rp)
	if err != nil {
		return err
	}
	s.fetchHeadersFinished()
	log.Debugf("Finished fetching headers from %v", rp.RemoteAddr())

	if atomic.CompareAndSwapUint32(&s.atomicCatchUpTryLock, 0, 1) {
		for walletID, w := range s.wallets {
			err = func() error {
				rescanPoint, err := w.RescanPoint(ctx)
				if err != nil {
					return err
				}
				walletBackend := &WalletBackend{
					Syncer:   s,
					WalletID: walletID,
				}
				if rescanPoint == nil {
					if !s.loadedFilters[walletID] {
						err = w.LoadActiveDataFilters(ctx, walletBackend, true)
						if err != nil {
							return err
						}
						s.loadedFilters[walletID] = true
					}

					s.syncedWallet(walletID)

					return nil
				}
				// RescanPoint is != nil so we are not synced to the peer and
				// check to see if it was previously synced
				s.unsynced(walletID)

				s.discoverAddressesStart(walletID)
				err = w.DiscoverActiveAddresses(ctx, rp, rescanPoint, !w.Locked(), w.GapLimit())
				if err != nil {
					return err
				}

				s.discoverAddressesFinished(walletID)

				err = w.LoadActiveDataFilters(ctx, walletBackend, true)
				if err != nil {
					return err
				}
				s.loadedFilters[walletID] = true

				s.rescanStart(walletID)

				rescanBlock, err := w.BlockHeader(ctx, rescanPoint)
				if err != nil {
					return err
				}
				progress := make(chan wallet.RescanProgress, 1)
				go w.RescanProgressFromHeight(ctx, walletBackend, int32(rescanBlock.Height), progress)

				for p := range progress {
					if p.Err != nil {
						return p.Err
					}
					s.rescanProgress(walletID, p.ScannedThrough)
				}
				s.rescanFinished(walletID)

				s.syncedWallet(walletID)

				return nil
			}()
		}

		atomic.StoreUint32(&s.atomicCatchUpTryLock, 0)
		if err != nil {
			return err
		}
	}

	for _, w := range s.wallets {
		unminedTxs, err := w.UnminedTransactions(ctx)
		if err != nil {
			log.Errorf("Cannot load unmined transactions for resending: %v", err)
			continue
		}
		if len(unminedTxs) == 0 {
			continue
		}
		err = rp.PublishTransactions(ctx, unminedTxs...)
		if err != nil {
			// TODO: Transactions should be removed if this is a double spend.
			log.Errorf("Failed to resent one or more unmined transactions: %v", err)
		}
	}

	return nil
}

// handleMempool handles eviction from the local mempool of non-wallet-backed
// transactions. It MUST be run as a goroutine.
func (s *Syncer) handleMempool(ctx context.Context) error {
	const mempoolEvictionTimeout = 60 * time.Minute

	for {
		select {
		case txHash := <-s.mempoolAdds:
			go func() {
				select {
				case <-ctx.Done():
				case <-time.After(mempoolEvictionTimeout):
					s.mempool.Delete(*txHash)
				}
			}()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
