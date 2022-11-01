package wallet

import "code.cryptopower.dev/group/cryptopower/libwallet/assets/dcr"

// TODO: responses.go file to be deprecated with future code clean up

type UnspentOutput struct {
	UTXO     dcr.UnspentOutput
	Amount   string
	DateTime string
}

// UnspentOutputs wraps the libwallet UTXO type and adds processed data
type UnspentOutputs struct {
	List []*UnspentOutput
}
