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
	"sync"
	"sync/atomic"
	"time"
)

type HTTPAPIType uint8

const (
	// Default http client timeout in secs.
	defaultHTTPClientTimeout = 30 * time.Second
	// Address to look up during DNS connectivity check.
	addressToLookUp = "www.google.com"

	// Below lists the Http APIs that have a privacy control implemented on them.
	GovernanceHTTPAPI HTTPAPIType = iota
	FeeRateHTTPAPI
	ExchangeHTTPAPI
	VspAPI
)

type (
	// Client is the base for http/https calls
	Client struct {
		HTTPClient *http.Client
		cancelFunc context.CancelFunc
		context    context.Context
	}

	// ReqConfig models the configuration options for requests.
	ReqConfig struct {
		Payload interface{}
		Cookies []*http.Cookie
		Headers http.Header
		Method  string
		HTTPURL string
		// If IsRetByte is set to true, client.Do will delegate
		// response processing to caller.
		IsRetByte bool
	}

	monitorNetwork struct {
		networkCheck uint32
		isConnected  bool
		lastUpdate   time.Time
	}

	Dailer func(addr net.Addr) (net.Conn, error)
)

var (
	netC       monitorNetwork
	apiMtx     sync.Mutex
	activeAPIs map[string]*Client
)

// DialerFunc returns a customized dialer function that is make it easier to
// control node level tcp connections especially after a shutdown. It also
// includes a timeout value preventing a connection waiting forever for a
// response to be returned.
func DialerFunc(ctx context.Context) Dailer {
	d := &net.Dialer{
		Timeout: defaultHTTPClientTimeout,
	}
	return func(addr net.Addr) (net.Conn, error) {
		return d.DialContext(ctx, addr.Network(), addr.String())
	}
}

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
		HTTPClient: &http.Client{
			Timeout:   defaultHTTPClientTimeout,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
	}
}

// ShutdownHTTPClients shutdowns any active connection by cancelling the context.
func ShutdownHTTPClients() {
	apiMtx.Lock()
	defer apiMtx.Unlock()
	for _, c := range activeAPIs {
		c.cancelFunc()
	}
	activeAPIs = make(map[string]*Client)
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
		reqConfig.HTTPURL += string(requestBody)
	}

	// Create http request
	req, err := http.NewRequestWithContext(c.context, reqConfig.Method, reqConfig.HTTPURL, bytes.NewReader(requestBody))
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

	// assign the headers.
	req.Header = reqConfig.Headers

	// Send request
	resp, err = c.HTTPClient.Do(req)
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

// HTTPRequest queries the API provided in the ReqConfig object and converts
// the returned json(Byte data) into an respObj interface.
// Returned http response body is usually empty because the http stream
// cannot be read twice.
func HTTPRequest(reqConfig *ReqConfig, respObj interface{}) (*http.Response, error) {
	// validate the API Url address
	urlPath, err := url.ParseRequestURI(reqConfig.HTTPURL)
	if err != nil {
		return nil, fmt.Errorf("error: url not properly constituted: %v", err)
	}

	// Reuse the same client for requests that share a host.
	apiMtx.Lock()
	client, ok := activeAPIs[urlPath.Host]
	if !ok {
		client = newClient()
	}
	apiMtx.Unlock()

	body, httpResp, err := client.query(reqConfig)
	if err != nil {
		return nil, err
	}

	// cache a new client connection since it was successful
	apiMtx.Lock()
	activeAPIs[urlPath.Host] = client
	apiMtx.Unlock()

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
