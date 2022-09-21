package libwallet

import "gitlab.com/raedah/cryptopower/libwallet/wallets/dcr"

func (mw *MultiWallet) AllDCRWallets() (wallets []*dcr.Wallet) {
	for _, wallet := range mw.Assets.DCR.Wallets {
		wallets = append(wallets, wallet)
	}
	return wallets
}

// func (mw *MultiWallet) WalletsIterator() *WalletsIterator {
// 	return &WalletsIterator{
// 		currentIndex: 0,
// 		wallets:      mw.AllWallets(),
// 	}
// }

// func (walletsIterator *WalletsIterator) Next() *Wallet {
// 	if walletsIterator.currentIndex < len(walletsIterator.wallets) {
// 		wallet := walletsIterator.wallets[walletsIterator.currentIndex]
// 		walletsIterator.currentIndex++
// 		return wallet
// 	}

// 	return nil
// }

// func (walletsIterator *WalletsIterator) Reset() {
// 	walletsIterator.currentIndex = 0
// }
