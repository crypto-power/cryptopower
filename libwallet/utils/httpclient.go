package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"sync/atomic"
	"time"
)

const (
	// Default http client timeout in secs.
	defaultHttpClientTimeout = 10 * time.Second
)

type (
	// Client is the base for http/https calls
	Client struct {
		httpClient *http.Client
	}

	// ReqConfig models the configuration options for requests.
	ReqConfig struct {
		Payload []byte
		Method  string
		HttpUrl string
		// If IsRetByte is set to true, client.Do will delegate
		// response processing to caller.
		IsRetByte bool
		// IsActive should always be true, signifying that the user has authorised
		// the specific API call to access the internet.
		IsActive bool
	}

	monitorNetwork struct {
		networkCheck uint32
		isConnected  bool
		lastUpdate   time.Time
	}
)

var (
	netC       monitorNetwork
	activeAPIs map[string]*Client
)

func init() {
	netC = monitorNetwork{}

	// activeAPIs allows a previous successful client connection to be reused
	// shortening the time it takes to get a response.
	activeAPIs = make(map[string]*Client)
}

// newClient configures and return a new client
func newClient() (c *Client) {
	return &Client{
		httpClient: &http.Client{
			Timeout:   defaultHttpClientTimeout,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
	}
}

func (c *Client) requestFilter(reqConfig *ReqConfig) (req *http.Request, err error) {
	req, err = http.NewRequest(reqConfig.Method, reqConfig.HttpUrl, bytes.NewBuffer(reqConfig.Payload))
	if err != nil {
		return
	}
	if reqConfig.Method == http.MethodPost || reqConfig.Method == http.MethodPut {
		req.Header.Add("Content-Type", "application/json;charset=utf-8")
	}
	req.Header.Add("Accept", "application/json")
	return
}

// query prepares and process HTTP request to backend resources.
func (c *Client) query(reqConfig *ReqConfig) (rawData []byte, resp *http.Response, err error) {
	if !reqConfig.IsActive {
		return nil, nil, fmt.Errorf("error: API call not allowed: %v", reqConfig.HttpUrl)
	}

	if _, err := url.ParseRequestURI(reqConfig.HttpUrl); err != nil {
		return nil, nil, fmt.Errorf("error: url not properly constituted: %v", err)
	}

	var req *http.Request
	req, err = c.requestFilter(reqConfig)
	if err != nil {
		return nil, nil, err
	}

	if req == nil {
		return nil, nil, errors.New("error: nil request")
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("error: status: %v resp: %s", resp.Status, body)
	}

	return body, resp, nil
}

// HttpRequest helps to convert json(Byte data) into a struct object.
func HttpRequest(reqConfig *ReqConfig, respObj interface{}) (*http.Response, error) {
	var client, ok = activeAPIs[reqConfig.HttpUrl]
	if !ok {
		client = newClient()
	}

	body, httpResp, err := client.query(reqConfig)
	if err != nil {
		return nil, err
	}

	// cache a new client connection since it was successful
	if !ok {
		activeAPIs[reqConfig.HttpUrl] = client
	}

	// if IsRetByte is option is true. Response from the resource queried
	// is not in json format, don't unmarshal return response byte slice to
	// the caller for further processing.
	if reqConfig.IsRetByte {
		r := reflect.Indirect(reflect.ValueOf(respObj))
		r.Set(reflect.AppendSlice(r.Slice(0, 0), reflect.ValueOf(body)))
		return httpResp, nil
	}

	err = json.Unmarshal(body, respObj)
	return httpResp, err
}

// IsOnline is a function to check whether an internet connection can be
// established. If established bool true should be returned otherwise false.
// Default url to check connection is http://google.com.
func IsOnline() bool {
	// If the was wallet online, and the wallet's online status was updated in
	// the last 2 minutes return true.
	if time.Since(netC.lastUpdate) < time.Minute*2 && netC.isConnected {
		return true
	}

	// If the last poll made is in progress, return the last cached status.
	if !atomic.CompareAndSwapUint32(&netC.networkCheck, 0, 1) {
		return netC.isConnected
	}

	_, err := new(http.Client).Get("https://google.com")
	// When err != nil, internet connection test failed.
	netC.isConnected = err == nil
	netC.lastUpdate = time.Now()

	atomic.StoreUint32(&netC.networkCheck, 0)

	return netC.isConnected
}
