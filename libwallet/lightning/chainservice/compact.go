package chainservice

import (
	"os"

	bbolt "go.etcd.io/bbolt"
)

const (
	txMaxSize = 65536
)

func purgeOversizeFilters(neutrinoFile string) error {
	f, err := os.Stat(neutrinoFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	logger.Infof("neutrino file size = %v", f.Size())
	if f.Size() > 150000000 {
		logger.Infof("compacting neutrino file size = %v", f.Size())
		if err := deleteCompactFilters(neutrinoFile); err != nil {
			logger.Errorf("Error in deleting compact filters %v", err)
			return err
		}
		f, err := os.Stat(neutrinoFile)
		if err == nil {
			logger.Infof("after compacting neutrino new size = %v", f.Size())
		}
	}
	return nil
}

func deleteCompactFilters(neutrinoFile string) error {
	filterBucket := "filter-store"
	targetFilePath := neutrinoFile + ".tmp"
	if _, err := os.Stat(targetFilePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	err := BoltCopy(neutrinoFile, targetFilePath, func(keyPath [][]byte, k []byte, v []byte) bool {
		return len(keyPath) == 0 && v == nil && string(k) == filterBucket
	})
	if err != nil {
		return err
	}
	return os.Rename(targetFilePath, neutrinoFile)
}

func BoltCopy(srcfile, destfile string, skip skipFunc) error {
	// Open source database.
	src, err := bbolt.Open(srcfile, 0444, nil)
	if err != nil {
		return err
	}
	defer src.Close()

	// Open destination database.
	dst, err := bbolt.Open(destfile, 0600, nil)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Run compaction.
	err = compact(dst, src, skip)
	return err
}

func compact(dst, src *bbolt.DB, skip skipFunc) error {
	// commit regularly, or we'll run out of memory for large datasets if using one transaction.
	var size int64
	tx, err := dst.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := walk(src, func(keys [][]byte, k, v []byte, seq uint64) error {
		// On each key/value, check if we have exceeded tx size.
		sz := int64(len(k) + len(v))
		if size+sz > txMaxSize {
			// Commit previous transaction.
			if err := tx.Commit(); err != nil {
				return err
			}

			// Start new transaction.
			tx, err = dst.Begin(true)
			if err != nil {
				return err
			}
			size = 0
		}
		size += sz

		// Create bucket on the root transaction if this is the first level.
		nk := len(keys)
		if nk == 0 {
			bkt, err := tx.CreateBucket(k)
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
			bkt, err := b.CreateBucket(k)
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

	return tx.Commit()
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
