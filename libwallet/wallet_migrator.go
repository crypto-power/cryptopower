package libwallet

import (
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
)

type WalletMigrator struct {
	wallet            sharedW.Asset
	privatePassphrase string
	seed              string
	isMigrate         bool
}

func NewWalletMigrator(wallet sharedW.Asset) *WalletMigrator {
	return &WalletMigrator{
		wallet: wallet,
	}
}

func (wm *WalletMigrator) GetIsMigrate() bool {
	return wm.isMigrate
}

func (wm *WalletMigrator) SetIsMigrate(isMigrate bool) {
	wm.isMigrate = isMigrate
}

func (wm *WalletMigrator) GetAssetType() libutils.AssetType {
	return wm.wallet.GetAssetType()
}

func (wm *WalletMigrator) GetWalletName() string {
	return wm.wallet.GetWalletName()
}

func (wm *WalletMigrator) IsWatchingOnlyWallet() bool {
	return wm.wallet.IsWatchingOnlyWallet()
}

func (wm *WalletMigrator) SetPrivatePassphrase(privatePassphrase string) error {
	seed, err := wm.wallet.DecryptSeed(privatePassphrase)
	if err != nil {
		return err
	}

	wm.privatePassphrase = privatePassphrase
	wm.seed = seed
	wm.isMigrate = true
	return nil
}

func (wm *WalletMigrator) SetSeed(seed string) error {
	wm.seed = seed
	wm.isMigrate = true
	return nil
}

func (wm *WalletMigrator) Migrate(mgr *AssetsManager) error {
	if !wm.isMigrate {
		return nil
	}

	if wm.wallet.IsWatchingOnlyWallet() {
		return wm.migrateWatchingOnlyWallet(mgr)
	}
	var err error
	switch wm.wallet.GetAssetType() {
	case libutils.DCRWalletAsset:
		_, err = mgr.RestoreDCRWallet(wm.wallet.GetWalletName(), wm.seed, wm.privatePassphrase, sharedW.WordSeedType(wm.wallet.GetPrivatePassphraseType()), wm.wallet.GetPrivatePassphraseType())
		if err != nil {
			return err
		}

	case libutils.BTCWalletAsset:
		_, err = mgr.RestoreBTCWallet(wm.wallet.GetWalletName(), wm.seed, wm.privatePassphrase, sharedW.WordSeedType(wm.wallet.GetPrivatePassphraseType()), wm.wallet.GetPrivatePassphraseType())
		if err != nil {
			return err
		}

	case libutils.LTCWalletAsset:
		_, err = mgr.RestoreLTCWallet(wm.wallet.GetWalletName(), wm.seed, wm.privatePassphrase, sharedW.WordSeedType(wm.wallet.GetPrivatePassphraseType()), wm.wallet.GetPrivatePassphraseType())
		if err != nil {
			return err
		}
	}
	return nil
}

func (wm *WalletMigrator) migrateWatchingOnlyWallet(mgr *AssetsManager) error {
	if !wm.isMigrate {
		return nil
	}
	switch wm.wallet.GetAssetType() {
	case libutils.DCRWalletAsset:
		_, err := mgr.CreateNewDCRWatchOnlyWallet(wm.wallet.GetWalletName(), wm.seed)
		if err != nil {
			return err
		}

	case libutils.BTCWalletAsset:
		_, err := mgr.CreateNewBTCWatchOnlyWallet(wm.wallet.GetWalletName(), wm.seed)
		if err != nil {
			return err
		}

	case libutils.LTCWalletAsset:
		_, err := mgr.CreateNewLTCWatchOnlyWallet(wm.wallet.GetWalletName(), wm.seed)
		if err != nil {
			return err
		}

	}
	return nil
}
