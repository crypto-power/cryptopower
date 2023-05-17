package lnnode

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/btcsuite/btclog"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnrpc"
)

type txInfo struct {
	Spent  bool `json:"spent"`
	Status struct {
		Confirmed bool `json:"confirmed"`
	} `json:"status"`
}

func TestTxSpentURL(URL string) error {
	spent, confirmed, err := isSpent(
		URL,
		"f4184fc596403b9d638783cf57adfe4c75c605f6356fbc91338530e9831e9e16",
		1,
	)
	if err != nil {
		return err
	}
	if spent && confirmed {
		return nil
	}
	return errors.New("Bad URL")
}

func isSpent(txSpentURL string, txid string, vout uint32) (spent, confirmed bool, err error) {
	txSpentURL = strings.ReplaceAll(txSpentURL, ":txid", txid)
	txSpentURL = strings.ReplaceAll(txSpentURL, ":vout", strconv.FormatUint(uint64(vout), 10))
	var resp *http.Response
	if resp, err = http.Get(txSpentURL); err != nil {
		return
	}
	defer resp.Body.Close()
	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}
	var txi txInfo
	if err = json.Unmarshal(body, &txi); err != nil {
		return
	}
	return txi.Spent, txi.Status.Confirmed, nil
}

func closedChannels(log btclog.Logger, db *channeldb.DB, txSpentURL string) (int, error) {
	if !(strings.Contains(txSpentURL, ":txid") && strings.Contains(txSpentURL, ":vout")) {
		return 0, errors.New("Bad or disabled URL")
	}
	channels, err := db.ChannelStateDB().FetchAllOpenChannels()
	if err != nil {
		log.Errorf("Error in fetching channels:%v", err)
		return 0, err
	}
	closed := 0
	for _, c := range channels {
		txid := c.FundingOutpoint.Hash.String()
		vout := c.FundingOutpoint.Index
		spent, _, err := isSpent(txSpentURL, txid, vout)
		if err != nil {
			log.Errorf("Error in checking closed channel URL:%v error:%v", txSpentURL, err)
			return closed, err
		}
		if spent {
			closed++
		}
	}
	return closed, nil
}

func (d *Daemon) ClosedChannels() (int, error) {
	txSpentURL, _, err := d.breezDB.GetTxSpentURL(d.cfg.TxSpentURL)
	if err != nil {
		d.log.Errorf("Error in d.breezDB.GetTxSpentURL: %v", err)
		return 0, err
	}
	lnclient := d.APIClient()
	if lnclient == nil {
		return 0, errors.New("lnclient not ready")
	}
	channels, err := lnclient.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{})
	count := 0
	for _, c := range channels.Channels {
		cp := c.ChannelPoint
		cDetails := strings.Split(cp, ":")
		if len(cDetails) < 2 {
			return count, errors.New("Bad ChannelPoint")
		}
		vout, err := strconv.ParseUint(cDetails[1], 10, 64)
		if err != nil {
			return count, err
		}
		spent, _, err := isSpent(txSpentURL, cDetails[0], uint32(vout))
		if err != nil {
			return count, err
		}
		if spent {
			count++
		}
	}
	return count, nil
}
