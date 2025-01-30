package utils

import (
	"encoding/base64"
	"encoding/hex"
	"io/fs"
	"math"
	"net"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"gioui.org/app"
)

type ProposalStatus int
type AgendaSyncStatus int

const (
	AgendaStatusSynced AgendaSyncStatus = iota
	AgendaStatusSyncing
)

const (
	ProposalStatusSynced ProposalStatus = iota
	ProposalStatusNewProposal
	ProposalStatusVoteStarted
	ProposalStatusVoteFinished
)

type (
	// AssetType is the capitalized version of an asset's symbol (e.g BTC, DCR,
	// LTC) that serves as the asset's unique identity.
	AssetType string
	SyncStage int8
)

const (
	LogFileName = "libwallet.log"

	NilAsset       AssetType = ""
	BTCWalletAsset AssetType = "BTC"
	DCRWalletAsset AssetType = "DCR"
	LTCWalletAsset AssetType = "LTC"

	fullDateformat  = "2006-01-02 15:04:05"
	dateOnlyFormat  = "2006-01-02"
	timeOnlyformat  = "15:04:05"
	shortTimeformat = "2006-01-02 15:04"

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

	TypeFilter          = "Type"
	DirectionFilter     = "Direction"
	HeightFilter        = "BlockHeight"
	TicketSpenderFilter = "TicketSpender"

	LogLevelOff      = "off"
	LogLevelTrace    = "trace"
	LogLevelDebug    = "debug"
	LogLevelInfo     = "info"
	LogLevelWarn     = "warn"
	LogLevelError    = "error"
	LogLevelCritical = "critical"
	DefaultLogLevel  = LogLevelInfo

	// UserFilePerm contains permissions for the user only. Attempting to modify
	// more permissions require a super user permission that isn't readily available.
	UserFilePerm = fs.FileMode(0o700)
)

// Stringer used in generating the directory path where the lowercase of the
// asset type is required. The uppercase defined by default is required to
// asset previously created using the uppercase.
func (str AssetType) ToStringLower() string {
	return strings.ToLower(string(str))
}

// ToFull returns the full network name of the provided asset.
func (str AssetType) ToFull() string {
	switch str {
	case BTCWalletAsset:
		return "Bitcoin"
	case DCRWalletAsset:
		return "Decred"
	case LTCWalletAsset:
		return "Litecoin"
	default:
		return "Unknown"
	}
}

func (str AssetType) String() string {
	return string(str)
}

// ExtractDateOrTime returns the date represented by the timestamp as a date string
// if the timestamp is over 24 hours ago. Otherwise, the time alone is returned as a string.
func ExtractDateOrTime(timestamp int64) string {
	utcTime := time.Unix(timestamp, 0).UTC()
	if time.Now().UTC().Sub(utcTime).Hours() > 24 {
		return utcTime.Format(dateOnlyFormat)
	}
	return utcTime.Format(timeOnlyformat)
}

func FormatUTCTime(timestamp int64) string {
	return time.Unix(timestamp, 0).UTC().Format(fullDateformat)
}

func FormatUTCShortTime(timestamp int64) string {
	return time.Unix(timestamp, 0).UTC().Format(shortTimeformat)
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

func NormalizeAddress(addr string, defaultPort string) (string, error) {
	// If the first SplitHostPort errors because of a missing port and not
	// for an invalid host, add the port.  If the second SplitHostPort
	// fails, then a port is not missing and the original error should be
	// returned.
	host, port, origErr := net.SplitHostPort(addr)
	if origErr == nil {
		return net.JoinHostPort(host, port), nil
	}
	addr = net.JoinHostPort(addr, defaultPort)
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", origErr
	}
	return addr, nil
}

// TrimNonAphanumeric removes all the characters that don't include the following:
// `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-`
func TrimNonAphaNumeric(text string) string {
	reg, _ := regexp.Compile("[^a-zA-Z0-9-]+")
	return reg.ReplaceAllString(text, "")
}

// GetFreeDiskSpace returns the available disk space (in MB).
func GetFreeDiskSpace() (uint64, error) {
	var path string
	var err error
	switch runtime.GOOS {
	case "android", "ios", "linux", "darwin", "windows":
		if runtime.GOOS == "android" || runtime.GOOS == "ios" {
			path, err = app.DataDir()
			if err != nil {
				return 0, err
			}
		} else {
			path, err = os.UserHomeDir()
			if err != nil {
				return 0, err
			}
		}

		freeSpace, err := DiskSpace(path)
		if err != nil {
			return 0, err
		}

		return freeSpace, nil

	default:
		return 0, nil
	}

}
