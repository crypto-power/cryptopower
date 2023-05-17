package bootstrap

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/breez/breez/channeldbservice"
	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcwallet/walletdb/bdb"
	"github.com/lightningnetwork/lnd/channeldb"
	"go.etcd.io/bbolt"
)

var (
	logger                btclog.Logger
	ErrMissingPolicyError = errors.New("missing channel policy")
)

// SyncGraphDB syncs the channeldb from another db.
func SyncGraphDB(workingDir, sourceDBPath string) error {

	dir, fileName := filepath.Split(sourceDBPath)
	if fileName != "channel.db" {
		return errors.New("file name must be channel.db")
	}

	// open the source database.
	channelDBSource, err := channeldb.Open(dir)
	if err != nil {
		return err
	}
	defer channelDBSource.Close()

	// open the destination database.
	channelDBDest, cleanup, err := channeldbservice.Get(workingDir)
	if err != nil {
		return fmt.Errorf("failed to open channeldb %v", err)
	}
	defer cleanup()
	sourceDB, err := bdb.UnderlineDB(channelDBSource.Backend)
	if err != nil {
		return err
	}

	// buckets we want to copy from source to destination.
	bucketsToCopy := map[string]struct{}{
		"graph-edge": {},
		"graph-meta": {},
		"graph-node": {},
	}

	// utility function to convert bolts key to a string path.
	extractPathElements := func(bytesPath [][]byte, key []byte) []string {
		var path []string
		for _, b := range bytesPath {
			path = append(path, string(b))
		}
		return append(path, string(key))
	}

	ourNode, err := ourNode(channelDBDest)
	if err != nil {
		return err
	}

	kvdbTx, err := channelDBDest.BeginReadWriteTx()
	if err != nil {
		return err
	}
	tx, err := bdb.UnderlineTX(kvdbTx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if ourNode == nil && hasSourceNode(tx) {
		return errors.New("source node was set before sync transaction, rolling back")
	}

	if ourNode != nil {
		channelNodes, channels, policies, err := ourData(kvdbTx, ourNode)
		if err != nil {
			return err
		}

		// add our data to the source db.
		if err := putOurData(channelDBSource, ourNode, channelNodes, channels, policies); err != nil {
			return err
		}
	}

	// clear graph data from the destination db
	for b := range bucketsToCopy {
		if err := tx.DeleteBucket([]byte(b)); err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}
	}

	err = merge(tx, sourceDB,
		func(keyPath [][]byte, k []byte, v []byte) bool {
			pathElements := extractPathElements(keyPath, k)
			_, shouldCopy := bucketsToCopy[pathElements[0]]
			return !shouldCopy
		})
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Merge copies from source to dest and ignoring items using the skip function.
// It is different from Compact in that it tries to create a bucket only if not exists.
func merge(tx *bbolt.Tx, src *bbolt.DB, skip skipFunc) error {

	if err := walk(src, func(keys [][]byte, k, v []byte, seq uint64) error {

		// Create bucket on the root transaction if this is the first level.
		nk := len(keys)
		if nk == 0 {
			bkt, err := tx.CreateBucketIfNotExists(k)
			if err != nil {
				return err
			}
			if err := bkt.SetSequence(seq); err != nil {
				return err
			}
			return nil
		}

		// Create buckets on subsequent levels, if necessary.
		b := tx.Bucket(keys[0])
		if nk > 1 {
			for _, k := range keys[1:] {
				b = b.Bucket(k)
			}
		}

		// Fill the entire page for best compaction.
		b.FillPercent = 1.0

		// If there is no value then this is a bucket call.
		if v == nil {
			bkt, err := b.CreateBucketIfNotExists(k)
			if err != nil {
				return err
			}
			if err := bkt.SetSequence(seq); err != nil {
				return err
			}
			return nil
		}

		// Otherwise treat it as a key/value pair.
		return b.Put(k, v)
	}, skip); err != nil {
		return err
	}

	return nil
}

// walkFunc is the type of the function called for keys (buckets and "normal"
// values) discovered by Walk. keys is the list of keys to descend to the bucket
// owning the discovered key/value pair k/v.
type walkFunc func(keys [][]byte, k, v []byte, seq uint64) error

type skipFunc func(keys [][]byte, k, v []byte) bool

// walk walks recursively the bolt database db, calling walkFn for each key it finds.
func walk(db *bbolt.DB, walkFn walkFunc, skipFn skipFunc) error {
	return db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			return walkBucket(b, nil, name, nil, b.Sequence(), walkFn, skipFn)
		})
	})
}

func walkBucket(b *bbolt.Bucket, keypath [][]byte, k, v []byte, seq uint64, fn walkFunc, skip skipFunc) error {

	if skip != nil && skip(keypath, k, v) {
		return nil
	}

	// Execute callback.
	if err := fn(keypath, k, v, seq); err != nil {
		return err
	}

	// If this is not a bucket then stop.
	if v != nil {
		return nil
	}

	// Iterate over each child key/value.
	keypath = append(keypath, k)
	return b.ForEach(func(k, v []byte) error {
		if v == nil {
			bkt := b.Bucket(k)
			return walkBucket(bkt, keypath, k, nil, bkt.Sequence(), fn, skip)
		}
		return walkBucket(b, keypath, k, v, b.Sequence(), fn, skip)
	})
}
