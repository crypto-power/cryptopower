// Copyright (c) 2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dexdcr

import (
	"fmt"

	"decred.org/dcrwallet/v2/p2p"
)

// SpvSyncer defines methods we expect to find in an spv wallet backend.
type SpvSyncer interface {
	Synced() bool
	EstimateMainChainTip() int32
	GetRemotePeers() map[string]*p2p.RemotePeer
}

// spvSyncer returns the spv syncer connected to the wallet or returns an error
// if the wallet isn't connected to an spv syncer backend.
func (spvw *SpvWallet) spvSyncer() (SpvSyncer, error) {
	n, err := spvw.wallet.NetworkBackend()
	if err != nil {
		return nil, fmt.Errorf("wallet network backend error: %w", err)
	}
	if spvSyncer, ok := n.(SpvSyncer); ok {
		return spvSyncer, nil
	}
	return nil, fmt.Errorf("wallet is not connected to a supported spv backend")
}
