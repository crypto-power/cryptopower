package utils

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"time"
)

type AssetType string
type SyncStage int8

const (
	LogFileName = "libwallet.log"

	// ETHTokenAsset AssetType = "ETH"
	BTCWalletAsset AssetType = "BTC"
	DCRWalletAsset AssetType = "DCR"

	fullDateformat = "2006-01-02 15:04:05"
	dateOnlyFormat = "2006-01-02"
	timeOnlyformat = "15:04:05"

	InvalidSyncStage          SyncStage = -1
	CFiltersFetchSyncStage    SyncStage = 0
	HeadersFetchSyncStage     SyncStage = 1
	AddressDiscoverySyncStage SyncStage = 2
	HeadersRescanSyncStage    SyncStage = 3

	TxFilterAll         int32 = 0
	TxFilterSent        int32 = 1
	TxFilterReceived    int32 = 2
	TxFilterTransferred int32 = 3
	TxFilterStaking     int32 = 4
	TxFilterCoinBase    int32 = 5
	TxFilterRegular     int32 = 6
	TxFilterMixed       int32 = 7
	TxFilterVoted       int32 = 8
	TxFilterRevoked     int32 = 9
	TxFilterImmature    int32 = 10
	TxFilterLive        int32 = 11
	TxFilterUnmined     int32 = 12
	TxFilterExpired     int32 = 13
	TxFilterTickets     int32 = 14
)

// Stringer used in generating the directory path where the lowercase of the
// asset type is required. The uppercase defined by default is required to
// asset previously created using the uppercase.
func (str AssetType) ToString() string {
	return strings.ToLower(string(str))
}

// ToFull returns the full network name of the provided asset.
func (str AssetType) ToFull() string {
	switch str {
	case BTCWalletAsset:
		return "Bitcoin"
	case DCRWalletAsset:
		return "Decred"
	default:
		return "Unknown"
	}
}

// ExtractDateOrTime returns the date represented by the timestamp as a date string
// if the timestamp is over 24 hours ago. Otherwise, the time alone is returned as a string.
func ExtractDateOrTime(timestamp int64) string {
	utcTime := time.Unix(timestamp, 0).UTC()
	if time.Now().UTC().Sub(utcTime).Hours() > 24 {
		return utcTime.Format(dateOnlyFormat)
	} else {
		return utcTime.Format(timeOnlyformat)
	}
}

func FormatUTCTime(timestamp int64) string {
	return time.Unix(timestamp, 0).UTC().Format(fullDateformat)
}

func EncodeHex(hexBytes []byte) string {
	return hex.EncodeToString(hexBytes)
}

func EncodeBase64(text []byte) string {
	return base64.StdEncoding.EncodeToString(text)
}

func DecodeBase64(base64Text string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(base64Text)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func ShannonEntropy(text string) (entropy float64) {
	if text == "" {
		return 0
	}
	for i := 0; i < 256; i++ {
		px := float64(strings.Count(text, string(byte(i)))) / float64(len(text))
		if px > 0 {
			entropy += -px * math.Log2(px)
		}
	}
	return entropy
}

// HttpGet helps to convert json(Byte data) into a struct object.
func HttpGet(url string, respObj interface{}) (*http.Response, []byte, error) {
	rq := new(http.Client)
	resp, err := rq.Get((url))
	if err != nil {
		return nil, nil, err
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return resp, respBytes, fmt.Errorf("%d response from server: %v", resp.StatusCode, string(respBytes))
	}

	err = json.Unmarshal(respBytes, respObj)
	return resp, respBytes, err
}
