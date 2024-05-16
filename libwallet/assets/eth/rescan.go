package eth

import (
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	// "github.com/crypto-power/cryptopower/libwallet/utils"
)

// SetBlocksRescanProgressListener sets the blocks rescan progress listener.
func (asset *Asset) SetBlocksRescanProgressListener(blocksRescanProgressListener *sharedW.BlocksRescanProgressListener) {
	asset.blocksRescanProgressListener = blocksRescanProgressListener
}
