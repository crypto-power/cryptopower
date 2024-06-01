package walletdata

import "go.etcd.io/bbolt"

type transaction struct {
	LTC    *LTCTX
	BTC    *BTCTX
	BCH    *BCHTX
	boltTx *bbolt.Tx
}
