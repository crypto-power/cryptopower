package txhelper

import (
	btccfg "github.com/btcsuite/btcd/chaincfg"
	btctxauthor "github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/crypto-power/cryptopower/libwallet/addresshelper"
	"github.com/decred/dcrd/dcrutil/v4"
	ltccfg "github.com/ltcsuite/ltcd/chaincfg"
	ltctxauthor "github.com/ltcsuite/ltcwallet/wallet/txauthor"
)

const scriptVersion = 0

// implements Script() and ScriptSize() functions of txauthor.ChangeSource
type TxChangeSource struct {
	// Shared fields.
	script []byte

	// DCR fields.
	version uint16
}

func (src *TxChangeSource) Script() ([]byte, uint16, error) {
	return src.script, src.version, nil
}

func (src *TxChangeSource) ScriptSize() int {
	return len(src.script)
}

func MakeTxChangeSource(destAddr string, net dcrutil.AddressParams) (*TxChangeSource, error) {
	pkScript, err := addresshelper.PkScript(destAddr, net)
	if err != nil {
		return nil, err
	}
	changeSource := &TxChangeSource{
		script:  pkScript,
		version: scriptVersion,
	}
	return changeSource, nil
}

func MakeBTCTxChangeSource(destAddr string, net *btccfg.Params) (*btctxauthor.ChangeSource, error) {
	var pkScript []byte
	changeSource := &btctxauthor.ChangeSource{
		NewScript: func() ([]byte, error) {
			pkScript, err := addresshelper.BTCPkScript(destAddr, net)
			if err != nil {
				return nil, err
			}
			return pkScript, nil
		},
		ScriptSize: len(pkScript),
	}
	return changeSource, nil
}

func MakeLTCTxChangeSource(destAddr string, net *ltccfg.Params) (*ltctxauthor.ChangeSource, error) {
	var pkScript []byte
	changeSource := &ltctxauthor.ChangeSource{
		NewScript: func() ([]byte, error) {
			pkScript, err := addresshelper.LTCPkScript(destAddr, net)
			if err != nil {
				return nil, err
			}
			return pkScript, nil
		},
		ScriptSize: len(pkScript),
	}
	return changeSource, nil
}
