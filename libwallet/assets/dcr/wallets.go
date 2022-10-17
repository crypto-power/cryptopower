package dcr

func (walletsIterator *WalletsIterator) Next() *DCRAsset {
	if walletsIterator.CurrentIndex < len(walletsIterator.Wallets) {
		wallet := walletsIterator.Wallets[walletsIterator.CurrentIndex]
		walletsIterator.CurrentIndex++
		return wallet
	}

	return nil
}

func (walletsIterator *WalletsIterator) Reset() {
	walletsIterator.CurrentIndex = 0
}
