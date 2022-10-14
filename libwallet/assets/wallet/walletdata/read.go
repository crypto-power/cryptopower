package walletdata

import (
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
)

const MaxReOrgBlocks = 6

// ReadIndexingStartBlock checks if the end block height was saved from last indexing operation.
// If so, the end block height - MaxReOrgBlocks is returned.
// Otherwise, 0 is returned to begin indexing from height 0.
func (db *DB) ReadIndexingStartBlock() (int32, error) {
	var startBlockHeight int32
	err := db.walletDataDB.Get(TxBucketName, KeyEndBlock, &startBlockHeight)
	if err != nil && err != storm.ErrNotFound {
		return 0, err
	}

	startBlockHeight -= MaxReOrgBlocks
	if startBlockHeight < 0 {
		startBlockHeight = 0
	}
	return startBlockHeight, nil
}

// Read queries the db for `limit` count transactions that match the specified `txFilter`
// starting from the specified `offset`; and saves the transactions found to the received `transactions` object.
// `transactions` should be a pointer to a slice of Transaction objects.
func (db *DB) Read(offset, limit, txFilter int32, newestFirst bool, requiredConfirmations, bestBlock int32, transactions interface{}) error {
	query := db.prepareTxQuery(txFilter, requiredConfirmations, bestBlock)
	if offset > 0 {
		query = query.Skip(int(offset))
	}
	if limit > 0 {
		query = query.Limit(int(limit))
	}
	if newestFirst {
		query = query.OrderBy("Timestamp").Reverse()
	} else {
		query = query.OrderBy("Timestamp")
	}

	err := query.Find(transactions)
	if err != nil && err != storm.ErrNotFound {
		return err
	}
	return nil
}

// Count queries the db for transactions of the `txObj` type
// to return the number of records matching the specified `txFilter`.
func (db *DB) Count(txFilter int32, requiredConfirmations, bestBlock int32, txObj interface{}) (int, error) {
	query := db.prepareTxQuery(txFilter, requiredConfirmations, bestBlock)

	count, err := query.Count(txObj)
	if err != nil {
		return -1, err
	}

	return count, nil
}

func (db *DB) Find(matcher q.Matcher, transactions interface{}) error {
	query := db.walletDataDB.Select(matcher)

	err := query.Find(transactions)
	if err != nil && err != storm.ErrNotFound {
		return err
	}
	return nil
}

func (db *DB) FindOne(fieldName string, value interface{}, obj interface{}) error {
	return db.walletDataDB.One(fieldName, value, obj)
}

func (db *DB) FindLast(fieldName string, value interface{}, txObj interface{}) error {
	query := db.walletDataDB.Select(q.Eq(fieldName, value)).OrderBy("Timestamp").Reverse()
	return query.First(txObj)
}

func (db *DB) FindAll(fieldName string, value interface{}, txObj interface{}) error {
	return db.walletDataDB.Find(fieldName, value, txObj)
}
