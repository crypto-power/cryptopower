package db

import (
	"encoding/json"
)

type MismatchedChannels struct {
	LSPPubkey  string
	ChanPoints []MismatchedChannel
}

type MismatchedChannel struct {
	ChanPoint   string
	ShortChanID uint64
}

// FetchLastSyncedHeaderTimestamp the last known header timestamp that the node is synced to.
func (db *DB) FetchLastSyncedHeaderTimestamp() (int64, error) {
	tsBytes, err := db.fetchItem([]byte(syncstatus), []byte("last_header_timestamp"))
	if err != nil || tsBytes == nil {
		return 0, err
	}
	return int64(btoi(tsBytes)), nil
}

// SetLastSyncedHeaderTimestamp saves the last known header timestamp that the node is synced to.
func (db *DB) SetLastSyncedHeaderTimestamp(ts int64) error {
	return db.saveItem([]byte(syncstatus), []byte("last_header_timestamp"), itob(uint64(ts)))
}

func (db *DB) SetMismatchedChannels(mismatched *MismatchedChannels) error {
	serialized, err := json.Marshal(mismatched)
	if err != nil {
		return err
	}
	return db.saveItem([]byte(syncstatus), []byte("mismatched_channels"), serialized)
}

func (db *DB) FetchMismatchedChannels() (*MismatchedChannels, error) {
	raw, err := db.fetchItem([]byte(syncstatus), []byte("mismatched_channels"))
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var mismatched MismatchedChannels
	if err := json.Unmarshal(raw, &mismatched); err != nil {
		return nil, err
	}
	return &mismatched, err
}

func (db *DB) RemoveChannelMismatch() error {
	return db.deleteItem([]byte(syncstatus), []byte("mismatched_channels"))
}
