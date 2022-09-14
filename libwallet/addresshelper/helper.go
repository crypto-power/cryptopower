package addresshelper

import (
	"fmt"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/txscript/v4/stdscript"
)

const scriptVersion = 0

func PkScript(address string, net dcrutil.AddressParams) ([]byte, error) {
	addr, err := stdaddr.DecodeAddress(address, net)
	if err != nil {
		return nil, fmt.Errorf("error decoding address '%s': %s", address, err.Error())
	}

	_, pkScript := addr.PaymentScript()
	return pkScript, nil
}

func PkScriptAddresses(params *chaincfg.Params, pkScript []byte) []string {
	_, addresses := stdscript.ExtractAddrs(scriptVersion, pkScript, params)
	encodedAddresses := make([]string, len(addresses))
	for i, address := range addresses {
		encodedAddresses[i] = address.String()
	}
	return encodedAddresses
}
