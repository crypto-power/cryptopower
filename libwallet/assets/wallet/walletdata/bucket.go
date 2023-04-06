package walletdata

import (
	// "io"

	"go.etcd.io/bbolt"
)

type bucket struct {
	BTCBucket *BTCBucket
	upstream  *bbolt.Bucket
}
