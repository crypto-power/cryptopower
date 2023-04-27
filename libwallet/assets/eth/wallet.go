package eth

import (
	"context"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/loader"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
)

// Asset confirm that LTC implements that shared assets interface.
var _ sharedW.Asset = (*Asset)(nil)

// Asset is a wrapper around the LTCwallet.Wallet struct.
// It implements the sharedW.Asset interface.
// It also implements the sharedW.AssetsManagerDB interface.
// This is done to allow the Asset to be used as a db interface
// for the AssetsManager.
type Asset struct{}

func (asset *Asset) LockWallet() {
	log.Error(utils.ErrETHMethodNotImplemented("LockWallet"))
}

func (asset *Asset) IsLocked() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsLocked"))
	return false
}

func (asset *Asset) IsWaiting() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsWaiting"))
	return false
}

func (asset *Asset) WalletOpened() bool {
	log.Error(utils.ErrETHMethodNotImplemented("WalletOpened"))
	return false
}

func (asset *Asset) OpenWallet() error {
	return utils.ErrETHMethodNotImplemented("OpenWallet")
}

func (asset *Asset) GetWalletID() int {
	log.Error(utils.ErrETHMethodNotImplemented("GetWalletID"))
	return -1
}

func (asset *Asset) GetWalletName() string {
	log.Error(utils.ErrETHMethodNotImplemented("GetWalletName"))
	return ""
}

func (asset *Asset) IsWatchingOnlyWallet() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsWatchingOnlyWallet"))
	return false
}

func (asset *Asset) UnlockWallet(string) error {
	return utils.ErrETHMethodNotImplemented("UnlockWallet")
}

func (asset *Asset) DeleteWallet(privPass string) error {
	return utils.ErrETHMethodNotImplemented("DeleteWallet")
}

func (asset *Asset) RenameWallet(newName string) error {
	return utils.ErrETHMethodNotImplemented("RenameWallet")
}

func (asset *Asset) DecryptSeed(privatePassphrase string) (string, error) {
	return "", utils.ErrETHMethodNotImplemented("DecryptSeed")
}

func (asset *Asset) VerifySeedForWallet(seedMnemonic, privpass string) (bool, error) {
	return false, utils.ErrETHMethodNotImplemented("VerifySeedForWallet")
}

func (asset *Asset) ChangePrivatePassphraseForWallet(oldPrivatePassphrase, newPrivatePassphrase string, privatePassphraseType int32) error {
	return utils.ErrETHMethodNotImplemented("ChangePrivatePassphraseForWallet")
}

func (asset *Asset) RootDir() string {
	log.Error(utils.ErrETHMethodNotImplemented("RootDir"))
	return ""
}

func (asset *Asset) DataDir() string {
	log.Error(utils.ErrETHMethodNotImplemented("DataDir"))
	return ""
}

func (asset *Asset) GetEncryptedSeed() string {
	log.Error(utils.ErrETHMethodNotImplemented("GetEncryptedSeed"))
	return ""
}

func (asset *Asset) IsConnectedToNetwork() bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsConnectedToNetwork"))
	return false
}

func (asset *Asset) NetType() utils.NetworkType {
	log.Error(utils.ErrETHMethodNotImplemented("NetType"))
	return ""
}

func (asset *Asset) ToAmount(v int64) sharedW.AssetAmount {
	log.Error(utils.ErrETHMethodNotImplemented("ToAmount"))
	return nil
}

func (asset *Asset) GetAssetType() utils.AssetType {
	return utils.ETHWalletAsset
}

func (asset *Asset) Internal() *loader.LoaderWallets {
	log.Error(utils.ErrETHMethodNotImplemented("Internal"))
	return nil
}

func (asset *Asset) TargetTimePerBlockMinutes() float64 {
	log.Error(utils.ErrETHMethodNotImplemented("TargetTimePerBlockMinutes"))
	return -1.0
}

func (asset *Asset) RequiredConfirmations() int32 {
	log.Error(utils.ErrETHMethodNotImplemented("RequiredConfirmations"))
	return -1
}

func (asset *Asset) ShutdownContextWithCancel() (context.Context, context.CancelFunc) {
	log.Error(utils.ErrETHMethodNotImplemented("ShutdownContextWithCancel"))
	return nil, nil
}

func (asset *Asset) Shutdown() {
	log.Error(utils.ErrETHMethodNotImplemented("Shutdown"))
}

func (asset *Asset) LogFile() string {
	log.Error(utils.ErrETHMethodNotImplemented("LogFile"))
	return ""
}

func (asset *Asset) GetBestBlock() *sharedW.BlockInfo {
	log.Error(utils.ErrETHMethodNotImplemented("GetBestBlock"))
	return nil
}

func (asset *Asset) GetBestBlockHeight() int32 {
	log.Error(utils.ErrETHMethodNotImplemented("GetBestBlockHeight"))
	return -1
}

func (asset *Asset) GetBestBlockTimeStamp() int64 {
	log.Error(utils.ErrETHMethodNotImplemented("GetBestBlockTimeStamp"))
	return -1
}

func (asset *Asset) SignMessage(passphrase, address, message string) ([]byte, error) {
	return nil, utils.ErrETHMethodNotImplemented("SignMessage")
}

func (asset *Asset) VerifyMessage(address, message, signatureBase64 string) (bool, error) {
	return false, utils.ErrETHMethodNotImplemented("VerifyMessage")
}
