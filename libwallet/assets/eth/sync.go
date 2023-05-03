package eth

import "code.cryptopower.dev/group/cryptopower/libwallet/utils"

func (asset *Asset) IsSynced() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsSynced"))
	return false
}

func (asset *Asset) IsSyncing() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsSyncing"))
	return false
}

func (asset *Asset) SpvSync() error {
	return utils.ErrETHMethodNotImplemented("SpvSync")
}

func (asset *Asset) CancelRescan() {
	log.Error(utils.ErrETHMethodNotImplemented("CancelRescan"))
}

func (asset *Asset) CancelSync() {
	log.Error(utils.ErrETHMethodNotImplemented("CancelSync"))
}

func (asset *Asset) IsRescanning() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsRescanning"))
	return false
}

func (asset *Asset) RescanBlocks() error {
	return utils.ErrETHMethodNotImplemented("RescanBlocks")
}

func (asset *Asset) ConnectedPeers() int32 {
	log.Error(utils.ErrETHMethodNotImplemented("ConnectedPeers"))
	return -1
}

func (asset *Asset) RemovePeers() {
	log.Error(utils.ErrETHMethodNotImplemented("RemovePeers"))
}

func (asset *Asset) SetSpecificPeer(address string) {
	log.Error(utils.ErrETHMethodNotImplemented("SetSpecificPeer"))
}

func (asset *Asset) GetExtendedPubKey(account int32) (string, error) {
	return "", utils.ErrETHMethodNotImplemented("GetExtendedPubKey")
}

func (asset *Asset) IsSyncShuttingDown() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsSyncShuttingDown"))
	return false
}
