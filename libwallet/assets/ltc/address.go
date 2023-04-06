package ltc

import "code.cryptopower.dev/group/cryptopower/libwallet/utils"

// IsAddressValid checks if the provided address is valid.
func (asset *Asset) IsAddressValid(address string) bool {
	return false
}

// HaveAddress checks if the provided address belongs to the wallet.
func (asset *Asset) HaveAddress(address string) bool {
	return false
}

// CurrentAddress gets the most recently requested payment address from the
// asset. If that address has already been used to receive funds, the next
// chained address is returned.
func (asset *Asset) CurrentAddress(account int32) (string, error) {
	return "", utils.ErrLTCMethodNotImplemented("CurrentAddress")
}

// NextAddress returns the address immediately following the last requested
// payment address. If that address has already been used to receive funds,
// the next chained address is returned.
func (asset *Asset) NextAddress(account int32) (string, error) {
	return "", utils.ErrLTCMethodNotImplemented("NextAddress")
}
