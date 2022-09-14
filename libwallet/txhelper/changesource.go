package txhelper

import (
	"github.com/decred/dcrd/dcrutil/v4"
	"gitlab.com/raedah/cryptopower/libwallet/addresshelper"
)

const scriptVersion = 0

// implements Script() and ScriptSize() functions of txauthor.ChangeSource
type txChangeSource struct {
	version uint16
	script  []byte
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
