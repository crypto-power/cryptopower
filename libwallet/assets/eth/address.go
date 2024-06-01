package eth

import "github.com/crypto-power/cryptopower/libwallet/utils"

func (asset *Asset) CurrentAddress(account int32) (string, error) {
	return "", utils.ErrETHMethodNotImplemented("CurrentAddress")
}

func (asset *Asset) NextAddress(account int32) (string, error) {
	return "", utils.ErrETHMethodNotImplemented("NextAddress")
}

func (asset *Asset) IsAddressValid(address string) bool {
	log.Error(utils.ErrETHMethodNotImplemented("IsAddressValid"))
	return false
}

func (asset *Asset) HaveAddress(address string) bool {
	log.Error(utils.ErrETHMethodNotImplemented("HaveAddress"))
	return false
}
