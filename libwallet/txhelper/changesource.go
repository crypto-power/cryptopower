package txhelper

import (
	btccfg "github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/libwallet/addresshelper"
)

const scriptVersion = 0

// implements Script() and ScriptSize() functions of txauthor.ChangeSource
type txChangeSource struct {
	// Shared fields.
	script []byte

	// DCR fields.
	version uint16
}

func (src *txChangeSource) Script() ([]byte, uint16, error) {
	return src.script, src.version, nil
}

func (src *txChangeSource) ScriptSize() int {
	return len(src.script)
}

func MakeTxChangeSource(destAddr string, net dcrutil.AddressParams) (*txChangeSource, error) {
	pkScript, err := addresshelper.PkScript(destAddr, net)
	if err != nil {
		return nil, err
	}
	changeSource := &txChangeSource{
		script:  pkScript,
		version: scriptVersion,
	}
	return changeSource, nil
}

func MakeBTCTxChangeSource(destAddr string, net *btccfg.Params) (*txauthor.ChangeSource, error) {
	// pkScript, err := addresshelper.BTCPkScript(destAddr, net)
	// if err != nil {
	// 	return nil, err
	// }
	changeSource := &txauthor.ChangeSource{
		NewScript: func() ([]byte, error) {
			return addresshelper.BTCPkScript(destAddr, net)
		},
		// ScriptSize: 0,
	}
	return changeSource, nil
}
