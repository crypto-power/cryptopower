package db

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"code.cryptopower.dev/group/cryptopower/libwallet/lightning/data"
	bolt "go.etcd.io/bbolt"
)

func (db *DB) SaveLNUrlPayInfo(info *data.LNUrlPayInfo) error {

	db.log.Info("SaveLNUrlPayInfo")
	err := db.Update(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(lnurlPayBucket))
		if data := b.Get([]byte(info.PaymentHash)); data == nil {

			buf, err := json.Marshal(info)
			if err != nil {
				return err
			}

			b.Put([]byte(info.PaymentHash), buf)
			return nil

		} else {

			return fmt.Errorf("successAction for paymentHash %x already exists.", info.PaymentHash)

		}
	})

	return err
}

func (db *DB) FetchLNUrlPayInfo(paymentHash string) (*data.LNUrlPayInfo, error) {

	var info *data.LNUrlPayInfo
	err := db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(lnurlPayBucket))
		if v := b.Get([]byte(paymentHash)); v != nil {
			if info, err = deserializeLNUrlPayInfo(v); err != nil {
				return err
			}
		}

		return nil
	})

	return info, err
}

func (db *DB) FetchAllLNUrlPayInfos() ([]*data.LNUrlPayInfo, error) {

	var infos []*data.LNUrlPayInfo
	err := db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(lnurlPayBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v == nil {
				continue
			}
			info, err := deserializeLNUrlPayInfo(v)
			if err != nil {
				return err
			}
			infos = append(infos, info)
		}

		return nil
	})

	// db.log.Infof("FetchAllLNUrlPayInfos infos = %v", infos)
	return infos, err
}

func deserializeLNUrlPayInfo(bytes []byte) (*data.LNUrlPayInfo, error) {
	var info data.LNUrlPayInfo
	err := json.Unmarshal(bytes, &info)
	return &info, err
}

func (db *DB) MigrateAllLNUrlPayMetadata() error {
	err := db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(lnurlPayMetadataMigrationBucket))
		if v := b.Get([]byte("migration_1")); v == nil {

			infos, err := db.FetchAllLNUrlPayInfos()
			if err != nil {
				return err
			}

			b := tx.Bucket([]byte(lnurlPayBucket))
			for _, info := range infos {
				if info != nil && info.Metadata != nil {
					var description string
					var longDescription string
					var image *data.LNUrlPayImage
					for _, m := range info.Metadata {
						if m.Entry != nil && len(m.Entry) > 0 {
							var key = m.Entry[0]
							var value = m.Entry[1]
							if key == "text/plain" {
								description = value
							}
							if key == "text/long-desc" {
								longDescription = value
							}
							if key == "image/png;base64" || key == "image/jpeg;base64" {
								split := strings.Split(value, "base64,")
								if len(split) > 1 {
									converted, err := base64.StdEncoding.DecodeString(split[1])
									if err == nil {
										image = &data.LNUrlPayImage{
											DataUri: value,
											Ext:     strings.Split(strings.Split(key, "/")[1], ";")[0],
											Bytes:   converted,
										}
									}
								}
							}
						} else if m.Image != nil && image == nil {
							image = m.Image
						}
					}
					info.Metadata = []*data.LNUrlPayMetadata{
						{
							Description:     description,
							LongDescription: longDescription,
							Image:           image,
						},
					}
					buf, err := json.Marshal(info)
					if err != nil {
						return err
					}

					err = b.Put([]byte(info.PaymentHash), buf)
					if err != nil {
						return err
					}
				}
			}
		}
		err = b.Put([]byte("migration_1"), []byte("true"))
		if err != nil {
			return err
		}

		return nil
	})
	return err
}
