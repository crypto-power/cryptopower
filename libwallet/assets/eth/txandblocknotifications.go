package eth

import (
	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

func (asset *Asset) AddSyncProgressListener(syncProgressListener sharedW.SyncProgressListener, uniqueIdentifier string) error {
	return utils.ErrETHMethodNotImplemented("AddSyncProgressListener")
}

func (asset *Asset) RemoveSyncProgressListener(uniqueIdentifier string) {
	log.Error(utils.ErrETHMethodNotImplemented("RemoveSyncProgressListener"))
}

func (asset *Asset) AddTxAndBlockNotificationListener(txAndBlockNotificationListener sharedW.TxAndBlockNotificationListener, async bool, uniqueIdentifier string) error {
	return utils.ErrETHMethodNotImplemented("AddTxAndBlockNotificationListener")
}

func (asset *Asset) RemoveTxAndBlockNotificationListener(uniqueIdentifier string) {
	log.Error(utils.ErrETHMethodNotImplemented("RemoveTxAndBlockNotificationListener"))
}

func (asset *Asset) SetBlocksRescanProgressListener(blocksRescanProgressListener sharedW.BlocksRescanProgressListener) {
	log.Error(utils.ErrETHMethodNotImplemented("SetBlocksRescanProgressListener"))
}
