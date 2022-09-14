package libwallet

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
)

// AddressInfo holds information about an address
// If the address belongs to the querying wallet, IsMine will be true and the AccountNumber and AccountName values will be populated
type AddressInfo struct {
	Address       string
	IsMine        bool
	AccountNumber uint32
	AccountName   string
}

func (mw *MultiWallet) IsAddressValid(address string) bool {
	_, err := stdaddr.DecodeAddress(address, mw.chainParams)
	return err == nil
}

func (wallet *Wallet) HaveAddress(address string) bool {
	addr, err := stdaddr.DecodeAddress(address, wallet.chainParams)
	if err != nil {
		return false
	}

	have, err := wallet.Internal().HaveAddress(wallet.shutdownContext(), addr)
	if err != nil {
		return false
	}

	return have
}

func (wallet *Wallet) AccountOfAddress(address string) (string, error) {
	addr, err := stdaddr.DecodeAddress(address, wallet.chainParams)
	if err != nil {
		return "", translateError(err)
	}

	a, err := wallet.Internal().KnownAddress(wallet.shutdownContext(), addr)
	if err != nil {
		return "", translateError(err)
	}

	return a.AccountName(), nil
}

func (wallet *Wallet) AddressInfo(address string) (*AddressInfo, error) {
	addr, err := stdaddr.DecodeAddress(address, wallet.chainParams)
	if err != nil {
		return nil, err
	}

	addressInfo := &AddressInfo{
		Address: address,
	}

	known, _ := wallet.Internal().KnownAddress(wallet.shutdownContext(), addr)
	if known != nil {
		addressInfo.IsMine = true
		addressInfo.AccountName = known.AccountName()

		accountNumber, err := wallet.AccountNumber(known.AccountName())
		if err != nil {
			return nil, err
		}
		addressInfo.AccountNumber = uint32(accountNumber)
	}

	return addressInfo, nil
}

// CurrentAddress gets the most recently requested payment address from the
// wallet. If that address has already been used to receive funds, the next
// chained address is returned.
func (wallet *Wallet) CurrentAddress(account int32) (string, error) {
	if wallet.IsRestored && !wallet.HasDiscoveredAccounts {
		return "", errors.E(ErrAddressDiscoveryNotDone)
	}

	addr, err := wallet.Internal().CurrentAddress(uint32(account))
	if err != nil {
		log.Errorf("CurrentAddress error: %w", err)
		return "", err
	}
	return addr.String(), nil
}

// NextAddress returns the address immediately following the last requested
// payment address. If that address has already been used to receive funds,
// the next chained address is returned.
func (wallet *Wallet) NextAddress(account int32) (string, error) {
	if wallet.IsRestored && !wallet.HasDiscoveredAccounts {
		return "", errors.E(ErrAddressDiscoveryNotDone)
	}

	// NewExternalAddress increments the lastReturnedAddressIndex but does
	// not return the address at the new index. The actual new address (at
	// the newly incremented index) is returned below by CurrentAddress.
	// NOTE: This workaround will be unnecessary once this anomaly is corrected
	// upstream.
	_, err := wallet.Internal().NewExternalAddress(wallet.shutdownContext(), uint32(account), w.WithGapPolicyWrap())
	if err != nil {
		log.Errorf("NewExternalAddress error: %w", err)
		return "", err
	}

	return wallet.CurrentAddress(account)
}

func (wallet *Wallet) AddressPubKey(address string) (string, error) {
	addr, err := stdaddr.DecodeAddress(address, wallet.chainParams)
	if err != nil {
		return "", err
	}

	known, err := wallet.Internal().KnownAddress(wallet.shutdownContext(), addr)
	if err != nil {
		return "", err
	}

	switch known := known.(type) {
	case w.PubKeyHashAddress:
		pubKeyAddr, err := stdaddr.NewAddressPubKeyEcdsaSecp256k1V0Raw(known.PubKey(), wallet.chainParams)
		if err != nil {
			return "", err
		}
		return pubKeyAddr.String(), nil

	default:
		return "", fmt.Errorf("address is not a managed pub key address")
	}
}
