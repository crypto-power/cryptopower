package wallet

import "gitlab.com/raedah/cryptopower/libwallet"

// TODO command.go file to be deprecated in subsiquent code clean up

// TODO move method to libwallet
// HaveAddress checks if the given address is valid for the wallet
func (wal *Wallet) HaveAddress(address string) (bool, string) {
	for _, wallet := range wal.multi.AllWallets() {
		result := wallet.HaveAddress(address)
		if result {
			return true, wallet.Name
		}
	}
	return false, ""
}

func (wal *Wallet) GetMultiWallet() *libwallet.MultiWallet {
	return wal.multi
}
