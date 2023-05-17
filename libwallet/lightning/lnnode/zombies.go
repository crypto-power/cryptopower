package lnnode

import (
	"github.com/btcsuite/btcwallet/walletdb/bdb"
	"github.com/lightningnetwork/lnd/channeldb"
	"go.etcd.io/bbolt"
)

const (
	maxZombies = 10000
)

var (
	edgeBucket   = []byte("graph-edge")
	zombieBucket = []byte("zombie-index")
)

func deleteZombies(chanDB *channeldb.DB) error {
	boltDB, err := bdb.UnderlineDB(chanDB.Backend)
	if err != nil {
		return err
	}
	err = boltDB.Update(func(tx *bbolt.Tx) error {
		edges := tx.Bucket(edgeBucket)
		if edges == nil {
			return channeldb.ErrGraphNoEdgesFound
		}
		zombies := edges.Bucket(zombieBucket)
		if zombies == nil {
			return nil
		}
		if zombies.Stats().KeyN > maxZombies {
			return edges.DeleteBucket(zombieBucket)
		}
		return nil
	})
	return err
}
