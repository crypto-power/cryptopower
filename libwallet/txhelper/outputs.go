package txhelper

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/addresshelper"
	"github.com/btcsuite/btcd/chaincfg"
	btcWire "github.com/btcsuite/btcd/wire"
	dcrutil "github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
)

func MakeTxOutput(address string, amountInAtom int64, net dcrutil.AddressParams) (output *wire.TxOut, err error) {
	pkScript, err := addresshelper.PkScript(address, net)
	if err != nil {
		return
	}

	output = &wire.TxOut{
		Value:    amountInAtom,
		Version:  scriptVersion,
		PkScript: pkScript,
	}
	return
}

func MakeBTCTxOutput(address string, amountInSatoshi int64, net *chaincfg.Params) (output *btcWire.TxOut, err error) {
	pkScript, err := addresshelper.BTCPkScript(address, net)
	if err != nil {
		return
	}

	output = &btcWire.TxOut{
		Value:    amountInSatoshi,
		PkScript: pkScript,
	}
	return
}
