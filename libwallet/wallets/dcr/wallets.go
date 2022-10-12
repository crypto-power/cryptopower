package dcr

type WalletsIterator struct {
	CurrentIndex int
	Wallets      []*Wallet
}

func (walletsIterator *WalletsIterator) Next() *Wallet {
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
