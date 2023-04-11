package walletdata

import (
	"go.etcd.io/bbolt"
)

type bucket struct {
	BTCBucket *BTCBucket
	LTCBucket *LTCBucket
	upstream  *bbolt.Bucket
}
