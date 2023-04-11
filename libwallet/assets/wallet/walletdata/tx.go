package walletdata

import "go.etcd.io/bbolt"

type transaction struct {
	LTC    *LTCTX
	BTC    *BTCTX
	boltTx *bbolt.Tx
}
