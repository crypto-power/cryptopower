package libwallet

import "gitlab.com/raedah/cryptopower/libwallet/wallets/dcr"

func (mw *MultiWallet) AllDCRWallets() (wallets []*dcr.Wallet) {
	for _, wallet := range mw.Assets.DCR.Wallets {
		wallets = append(wallets, wallet)
	}
	return wallets
}

func (mw *MultiWallet) WalletsIterator() *dcr.WalletsIterator {
	return &dcr.WalletsIterator{
		CurrentIndex: 0,
		Wallets:      mw.AllDCRWallets(),
	}
}