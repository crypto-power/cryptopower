package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"sync/atomic"
	"time"
)

type HttpAPIType uint8

const (
	// Default http client timeout in secs.
	defaultHttpClientTimeout = 30 * time.Second
	// Address to look up during DNS connectivity check.
	addressToLookUp = "www.google.com"

	// Below lists the Http APIs that have a privacy control implemented on them.
	GovernanceHttpAPI HttpAPIType = iota
	FeeRateHttpAPI
	ExchangeHttpAPI
	VspAPI
)

type (
	// Client is the base for http/https calls
	Client struct {
		httpClient *http.Client
		cancelFunc context.CancelFunc
		context    context.Context
	}

	// ReqConfig models the configuration options for requests.
	ReqConfig struct {
		Payload interface{}
		Cookies []*http.Cookie
		Method  string
		HttpUrl string
		// If IsRetByte is set to true, client.Do will delegate
		// response processing to caller.
		IsRetByte bool
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

// newClient configures and returns a new client
func newClient() (c *Client) {
	// Initialize context use to cancel all pending requests when shutdown request is made.
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		context:    ctx,
		cancelFunc: cancel,
		httpClient: &http.Client{
			Timeout:   defaultHttpClientTimeout,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
	}
}

// ShutdownHttpClients shutdowns any active connection by cancelling the context.
func ShutdownHttpClients() {
	for _, c := range activeAPIs {
		c.cancelFunc()
	}
	activeAPIs = nil
}

func (c *Client) getRequestBody(method string, body interface{}) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	if method == http.MethodPost {
		if requestBody, ok := body.([]byte); ok {
			return requestBody, nil
		}
	} else if method == http.MethodGet {
		if requestBody, ok := body.(map[string]string); ok {
			params := url.Values{}
			for key, val := range requestBody {
				params.Add(key, val)
			}
			return []byte(params.Encode()), nil
		}
	}

	return nil, errors.New("invalid request body")
}

// query prepares and process HTTP request to backend resources.
func (c *Client) query(reqConfig *ReqConfig) (rawData []byte, resp *http.Response, err error) {
	// package the request body for POST and PUT requests
	var requestBody []byte
	if reqConfig.Payload != nil {
		requestBody, err = c.getRequestBody(reqConfig.Method, reqConfig.Payload)
		if err != nil {
			return nil, nil, err
		}
	}

	// package request URL for GET requests.
	if reqConfig.Method == http.MethodGet && requestBody != nil {
		reqConfig.HttpUrl += string(requestBody)
	}

	// Create http request
	req, err := http.NewRequestWithContext(c.context, reqConfig.Method, reqConfig.HttpUrl, bytes.NewReader(requestBody))
	if err != nil {
		return nil, nil, fmt.Errorf("error creating http request: %v", err)
	}

	if req == nil {
		return nil, nil, errors.New("error: nil request")
	}

	if reqConfig.Method == http.MethodPost || reqConfig.Method == http.MethodPut {
		req.Header.Add("Content-Type", "application/json;charset=utf-8")
	} else {
		req.Header.Add("Accept", "application/json")
	}

	for _, cookie := range reqConfig.Cookies {
		req.AddCookie(cookie)
	}

	// Send request
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp, fmt.Errorf("error: status: %v resp: %s", resp.Status, body)
	}

	return body, resp, nil
}

// HttpRequest queries the API provided in the ReqConfig object and converts
// the returned json(Byte data) into an respObj interface.
func HttpRequest(reqConfig *ReqConfig, respObj interface{}) (*http.Response, error) {
	// validate the API Url address
	urlPath, err := url.ParseRequestURI(reqConfig.HttpUrl)
	if err != nil {
		return nil, fmt.Errorf("error: url not properly constituted: %v", err)
	}

	// Reuse the same client for requests that share a host.
	client, ok := activeAPIs[urlPath.Host]
	if !ok {
		client = newClient()
	}

	body, httpResp, err := client.query(reqConfig)
	if err != nil {
		return nil, err
	}

	// cache a new client connection since it was successful
	activeAPIs[urlPath.Host] = client

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
// established. If established, IsOnline should return true otherwise IsOnline returns false.
func IsOnline() bool {
	// If the wallet was online, and the wallet's online status was updated in
	// the last 2 minutes return true.
	if time.Since(netC.lastUpdate) < time.Minute*2 && netC.isConnected {
		return true
	}

	// If the last poll made is in progress, return the last cached status.
	if !atomic.CompareAndSwapUint32(&netC.networkCheck, 0, 1) {
		return netC.isConnected
	}

	// DNS lookup failed if err != nil.
	_, err := net.LookupHost(addressToLookUp)

	// if err == nil, the internet link is up.
	netC.isConnected = err == nil
	netC.lastUpdate = time.Now()

	atomic.StoreUint32(&netC.networkCheck, 0)

	return netC.isConnected
}
