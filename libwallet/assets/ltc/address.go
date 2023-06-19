package ltc

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/ltcsuite/ltcd/ltcutil"
)

// AddressInfo holds information about an address.
// If the address belongs to the querying wallet,
// IsMine will be true and the AccountNumber and
// AccountName values will be populated
type AddressInfo struct {
	Address       string
	IsMine        bool
	AccountNumber uint32
	AccountName   string
}

// IsAddressValid checks if the provided address is valid.
func (asset *Asset) IsAddressValid(address string) bool {
	_, err := ltcutil.DecodeAddress(address, asset.chainParams)
	return err == nil
}

// HaveAddress checks if the provided address belongs to the wallet.
func (asset *Asset) HaveAddress(address string) bool {
	if !asset.WalletOpened() {
		return false
	}

	addr, err := ltcutil.DecodeAddress(address, asset.chainParams)
	if err != nil {
		log.Debugf("DecodeAddress failed: ", err)
		return false
	}

	have, err := asset.Internal().LTC.HaveAddress(addr)
	if err != nil {
		log.Debugf("HaveAddress failed: ", err)
		return false
	}

	return have
}

// AddressInfo returns information about an address.
func (asset *Asset) AddressInfo(address string) (*AddressInfo, error) {
	const op errors.Op = "ltc.AddressInfo"

	if !asset.WalletOpened() {
		return nil, utils.ErrLTCNotInitialized
	}

	addr, err := ltcutil.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return nil, err
	}

	addressInfo := &AddressInfo{
		Address: address,
	}
	isMine, err := asset.Internal().LTC.HaveAddress(addr)
	if err != nil {
		log.Error(op, err)
	}
	if isMine {
		addressInfo.IsMine = isMine

		accountNumber, err := asset.Internal().LTC.AccountOfAddress(addr)
		if err != nil {
			return nil, err
		}
		addressInfo.AccountNumber = accountNumber

		accountName, err := asset.AccountName(int32(accountNumber))
		if err != nil {
			return nil, err
		}
		addressInfo.AccountName = accountName
	}

	return addressInfo, nil
}

// CurrentAddress gets the most recently requested payment address from the
// asset. If that address has already been used to receive funds, the next
// chained address is returned.
func (asset *Asset) CurrentAddress(account int32) (string, error) {
	if asset.IsRestored && !asset.ContainsDiscoveredAccounts() {
		return "", errors.E(utils.ErrAddressDiscoveryNotDone)
	}

	if !asset.WalletOpened() {
		return "", utils.ErrLTCNotInitialized
	}

	addr, err := asset.Internal().LTC.CurrentAddress(uint32(account), GetScope())
	if err != nil {
		log.Errorf("CurrentAddress error: %v", err)
		return "", err
	}
	return addr.String(), nil
}

// NextAddress returns the address immediately following the last requested
// payment address. If that address has already been used to receive funds,
// the next chained address is returned.
func (asset *Asset) NextAddress(account int32) (string, error) {
	if asset.IsRestored && !asset.ContainsDiscoveredAccounts() {
		return "", errors.E(utils.ErrAddressDiscoveryNotDone)
	}

	if !asset.WalletOpened() {
		return "", utils.ErrLTCNotInitialized
	}

	// NewAddress returns the next external chained address for a wallet.
	address, err := asset.Internal().LTC.NewAddress(uint32(account), GetScope())
	if err != nil {
		log.Errorf("NewExternalAddress error: %w", err)
		return "", err
	}

	return address.String(), nil
}

// AccountOfAddress returns the account name of the provided address.
func (asset *Asset) AccountOfAddress(address string) (string, error) {
	addr, err := ltcutil.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return "", utils.TranslateError(err)
	}

	if !asset.WalletOpened() {
		return "", utils.ErrLTCNotInitialized
	}

	accountNumber, err := asset.Internal().LTC.AccountOfAddress(addr)
	if err != nil {
		return "", utils.TranslateError(err)
	}

	accountName, err := asset.AccountName(int32(accountNumber))
	if err != nil {
		return "", err
	}

	return accountName, nil
}

// AddressPubKey returns the public key of the provided address.
func (asset *Asset) AddressPubKey(address string) (string, error) {
	addr, err := ltcutil.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return "", err
	}

	if !asset.WalletOpened() {
		return "", utils.ErrLTCNotInitialized
	}

	isMine, _ := asset.Internal().LTC.HaveAddress(addr)
	if !isMine {
		return "", fmt.Errorf("address does not belong to the wallet")
	}

	pubKey, err := asset.Internal().LTC.PubKeyForAddress(addr)
	if err != nil {
		return "", err
	}

	pubKeyAddr, err := ltcutil.NewAddressPubKey(pubKey.SerializeCompressed(), asset.chainParams)
	if err != nil {
		return "", err
	}
	return pubKeyAddr.String(), nil
}
