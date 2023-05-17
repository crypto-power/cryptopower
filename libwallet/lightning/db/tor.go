package db

const (
	torActiveKey = "torActive"
)

func (db *DB) SetTorActive(active bool) error {
	var _active byte
	if active {
		_active = 1
	}

	return db.saveItem([]byte(torBucket), []byte(torActiveKey), []byte{_active})
}

func (db *DB) GetTorActive() (result bool, err error) {
	if active, err := db.fetchItem([]byte(torBucket), []byte(torActiveKey)); err == nil {
		if active != nil && active[0] == 1 {
			result = true
		}
	}

	return result, err
}
