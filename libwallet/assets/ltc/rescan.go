package ltc

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

// SetBlocksRescanProgressListener sets the blocks rescan progress listener.
func (asset *Asset) SetBlocksRescanProgressListener(blocksRescanProgressListener sharedW.BlocksRescanProgressListener) {
	log.Error(utils.ErrLTCMethodNotImplemented("SetBlocksRescanProgressListener"))
}

// RescanBlocks rescans the blockchain for all addresses in the wallet.
func (asset *Asset) RescanBlocks() error {
	return utils.ErrLTCMethodNotImplemented("RescanBlocks")
}

// IsRescanning returns true if the wallet is currently rescanning the blockchain.
func (asset *Asset) IsRescanning() bool {
	log.Error(utils.ErrLTCMethodNotImplemented("IsRescanning"))
	return false
}

// CancelRescan cancels the current rescan.
func (asset *Asset) CancelRescan() {
	log.Error(utils.ErrLTCMethodNotImplemented("CancelRescan"))
}
