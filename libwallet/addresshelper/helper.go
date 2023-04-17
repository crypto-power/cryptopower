package addresshelper

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	btccfg "github.com/btcsuite/btcd/chaincfg"
	btctxscript "github.com/btcsuite/btcd/txscript"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/txscript/v4/stdscript"
	ltccfg "github.com/ltcsuite/ltcd/chaincfg"
	"github.com/ltcsuite/ltcd/ltcutil"
	ltctxscript "github.com/ltcsuite/ltcd/txscript"
)

const scriptVersion = 0

// PkScript returns the public key payment script for the given address.
func PkScript(address string, net dcrutil.AddressParams) ([]byte, error) {
	addr, err := stdaddr.DecodeAddress(address, net)
	if err != nil {
		return nil, fmt.Errorf("error decoding address '%s': %s", address, err.Error())
	}

	_, pkScript := addr.PaymentScript()
	return pkScript, nil
}

// BTCPkScript returns the public key payment script for the given address.
func BTCPkScript(address string, net *btccfg.Params) ([]byte, error) {
	// Parse the address to send the coins to into a btcutil.Address
	// which is useful to ensure the accuracy of the address and determine
	// the address type. It is also required for the upcoming call to
	// PayToAddrScript.
	addr, err := btcutil.DecodeAddress(address, net)
	if err != nil {
		return nil, fmt.Errorf("error decoding address '%s': %s", address, err.Error())
	}

	// Create a public key script that pays to the address.
	pkScript, err := btctxscript.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}

	return pkScript, nil
}

// LTCPkScript returns the public key payment script for the given address.
func LTCPkScript(address string, net *ltccfg.Params) ([]byte, error) {
	// Parse the address to send the coins to into a ltcutil.Address
	// which is useful to ensure the accuracy of the address and determine
	// the address type. It is also required for the upcoming call to
	// PayToAddrScript.
	addr, err := ltcutil.DecodeAddress(address, net)
	if err != nil {
		return nil, fmt.Errorf("error decoding address '%s': %s", address, err.Error())
	}

	// Create a public key script that pays to the address.
	pkScript, err := ltctxscript.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}

	return pkScript, nil
}

// PkScriptAddresses returns the addresses for the given public key script.
func PkScriptAddresses(params *chaincfg.Params, pkScript []byte) []string {
	_, addresses := stdscript.ExtractAddrs(scriptVersion, pkScript, params)
	encodedAddresses := make([]string, len(addresses))
	for i, address := range addresses {
		encodedAddresses[i] = address.String()
	}
	return encodedAddresses
}
