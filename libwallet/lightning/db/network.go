package db

import (
	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/data"
	"github.com/golang/protobuf/proto"
)

const (
	peersKey      = "peers"
	txSpentURLKey = "txspenturl"
)

func (db *DB) SetPeers(peers []string) error {
	if len(peers) == 0 {
		err := db.deleteItem([]byte(networkBucket), []byte(peersKey))
		return err
	}

	p := &data.Peers{Peer: peers}
	b, err := proto.Marshal(p)
	if err != nil {
		return err
	}

	err = db.saveItem([]byte(networkBucket), []byte(peersKey), b)
	return err
}

func (db *DB) GetPeers(defaults []string) (peers []string, isDefault bool, err error) {
	peers = defaults
	var b []byte
	b, err = db.fetchItem([]byte(networkBucket), []byte(peersKey))
	if err != nil {
		return
	}
	if len(b) == 0 {
		isDefault = true
		return
	}
	var p data.Peers
	err = proto.Unmarshal(b, &p)
	if err != nil {
		return
	}
	if len(p.Peer) > 0 {
		peers = p.Peer
	} else {
		isDefault = true
	}
	return
}

func (db *DB) SetTxSpentURL(txSpentURL string) error {
	err := db.saveItem([]byte(networkBucket), []byte(txSpentURLKey), []byte(txSpentURL))
	return err
}

func (db *DB) GetTxSpentURL(defaultURL string) (txSpentURL string, isDefault bool, err error) {
	txSpentURL = defaultURL
	var b []byte
	b, err = db.fetchItem([]byte(networkBucket), []byte(txSpentURLKey))
	if err != nil {
		return
	}
	if len(b) == 0 {
		isDefault = true
		return
	}
	txSpentURL = string(b)
	return
}
