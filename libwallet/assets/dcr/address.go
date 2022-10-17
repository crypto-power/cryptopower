package dcr

import (
	"fmt"

	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

// AddressInfo holds information about an address
// If the address belongs to the querying wallet, IsMine will be true and the AccountNumber and AccountName values will be populated
type AddressInfo struct {
	Address       string
	IsMine        bool
	AccountNumber uint32
	AccountName   string
}

func (asset *DCRAsset) IsAddressValid(address string) bool {
	_, err := stdaddr.DecodeAddress(address, asset.chainParams)
	return err == nil
}

func (asset *DCRAsset) HaveAddress(address string) bool {
	addr, err := stdaddr.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return false
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	have, err := asset.Internal().DCR.HaveAddress(ctx, addr)
	if err != nil {
		return false
	}

	return have
}

func (asset *DCRAsset) AccountOfAddress(address string) (string, error) {
	addr, err := stdaddr.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return "", utils.TranslateError(err)
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	a, err := asset.Internal().DCR.KnownAddress(ctx, addr)
	if err != nil {
		return "", utils.TranslateError(err)
	}

	return a.AccountName(), nil
}

func (asset *DCRAsset) AddressInfo(address string) (*AddressInfo, error) {
	addr, err := stdaddr.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return nil, err
	}

	addressInfo := &AddressInfo{
		Address: address,
	}
	ctx, _ := asset.ShutdownContextWithCancel()
	known, _ := asset.Internal().DCR.KnownAddress(ctx, addr)
	if known != nil {
		addressInfo.IsMine = true
		addressInfo.AccountName = known.AccountName()

		accountNumber, err := asset.AccountNumber(known.AccountName())
		if err != nil {
			return nil, err
		}
		addressInfo.AccountNumber = uint32(accountNumber)
	}

	return addressInfo, nil
}

// CurrentAddress gets the most recently requested payment address from the
// asset. If that address has already been used to receive funds, the next
// chained address is returned.
func (asset *DCRAsset) CurrentAddress(account int32) (string, error) {
	if asset.IsRestored && !asset.HasDiscoveredAccounts {
		return "", errors.E(utils.ErrAddressDiscoveryNotDone)
	}

	addr, err := asset.Internal().DCR.CurrentAddress(uint32(account))
	if err != nil {
		log.Errorf("CurrentAddress error: %w", err)
		return "", err
	}
	return addr.String(), nil
}

// NextAddress returns the address immediately following the last requested
// payment address. If that address has already been used to receive funds,
// the next chained address is returned.
func (asset *DCRAsset) NextAddress(account int32) (string, error) {
	if asset.IsRestored && !asset.HasDiscoveredAccounts {
		return "", errors.E(utils.ErrAddressDiscoveryNotDone)
	}

	// NewExternalAddress increments the lastReturnedAddressIndex but does
	// not return the address at the new index. The actual new address (at
	// the newly incremented index) is returned below by CurrentAddress.
	// NOTE: This workaround will be unnecessary once this anomaly is corrected
	// upstream.
	ctx, _ := asset.ShutdownContextWithCancel()
	_, err := asset.Internal().DCR.NewExternalAddress(ctx, uint32(account), w.WithGapPolicyWrap())
	if err != nil {
		log.Errorf("NewExternalAddress error: %w", err)
		return "", err
	}

	return asset.CurrentAddress(account)
}

func (asset *DCRAsset) AddressPubKey(address string) (string, error) {
	addr, err := stdaddr.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return "", err
	}

	ctx, _ := asset.ShutdownContextWithCancel()
	known, err := asset.Internal().DCR.KnownAddress(ctx, addr)
	if err != nil {
		return "", err
	}

	switch known := known.(type) {
	case w.PubKeyHashAddress:
		pubKeyAddr, err := stdaddr.NewAddressPubKeyEcdsaSecp256k1V0Raw(known.PubKey(), asset.chainParams)
		if err != nil {
			return "", err
		}
		return pubKeyAddr.String(), nil

	default:
		return "", fmt.Errorf("address is not a managed pub key address")
	}
}
