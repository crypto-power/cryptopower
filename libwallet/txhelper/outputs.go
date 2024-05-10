package txhelper

import (
	btcchaincfg "github.com/btcsuite/btcd/chaincfg"
	btcWire "github.com/btcsuite/btcd/wire"
	"github.com/crypto-power/cryptopower/libwallet/addresshelper"
	dcrutil "github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
	ltcchaincfg "github.com/ltcsuite/ltcd/chaincfg"
	ltcWire "github.com/ltcsuite/ltcd/wire"
	bchchaincfg "github.com/gcash/bchd/chaincfg"
	bchWire "github.com/gcash/bchd/wire"
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

func MakeBTCTxOutput(address string, amountInSatoshi int64, net *btcchaincfg.Params) (output *btcWire.TxOut, err error) {
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

func MakeLTCTxOutput(address string, amountInLitoshi int64, net *ltcchaincfg.Params) (output *ltcWire.TxOut, err error) {
	pkScript, err := addresshelper.LTCPkScript(address, net)
	if err != nil {
		return
	}

	output = &ltcWire.TxOut{
		Value:    amountInLitoshi,
		PkScript: pkScript,
	}
	return
}

func MakeBCHTxOutput(address string, amountInSatoshi int64, net *bchchaincfg.Params) (output *bchWire.TxOut, err error) {
	pkScript, err := addresshelper.BCHPkScript(address, net)
	if err != nil {
		return
	}

	output = &bchWire.TxOut{
		Value:    amountInSatoshi,
		PkScript: pkScript,
	}
	return
}