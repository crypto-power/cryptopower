package ext

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"reflect"
	"strings"
	"time"
)

const (
	// Default http client timeout in secs.
	defaultHttpClientTimeout = 10 * time.Second
)

type (
	// Client is the base for http/https calls
	Client struct {
		httpClient    *http.Client
		RequestFilter func(reqConfig *ReqConfig) (req *http.Request, err error)
	}

	// ReqConfig models the configuration options for requests.
	ReqConfig struct {
		payload []byte
		method  string
		url     string
		retByte bool // if set to true, client.Do will delegate response processing to caller.
	}
)

// NewClient configures and return a new client
func NewClient() (c *Client) {
	t := http.DefaultTransport.(*http.Transport).Clone()
	client := &http.Client{
		Timeout:   defaultHttpClientTimeout,
		Transport: t,
	}

	return &Client{
		httpClient:    client,
		RequestFilter: nil,
	}
}

// Do prepare and process HTTP request to backend resources.
func (c *Client) Do(backend, net string, reqConfig *ReqConfig, response interface{}) (err error) {
	c.setBackend(backend, net, reqConfig)
	if c.RequestFilter == nil {
		return errors.New("Request Filter was not set")
	}

	var req *http.Request
	req, err = c.RequestFilter(reqConfig)
	if err != nil {
		return err
	}

	if req == nil {
		return errors.New("error: nil request")
	}

	c.dumpRequest(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	c.dumpResponse(resp)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error: status: %v resp: %s", resp.Status, body)
	}

	// if retByte is option is true. Response from the resource queried
	// is not in json format, don't unmarshal return response byte slice to the caller for further processing.
	if reqConfig.retByte {
		r := reflect.Indirect(reflect.ValueOf(response))
		r.Set(reflect.AppendSlice(r.Slice(0, 0), reflect.ValueOf(body)))
		return nil
	}

	err = json.Unmarshal(body, response)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) dumpRequest(r *http.Request) {
	if r == nil {
		log.Debug("dumpReq ok: <nil>")
		return
	}
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Debug("dumpReq err: %v", err)
	} else {
		log.Debug("dumpReq ok: %v", string(dump))
	}
}

func (c *Client) dumpResponse(r *http.Response) {
	if r == nil {
		log.Debug("dumpResponse ok: <nil>")
		return
	}
	dump, err := httputil.DumpResponse(r, true)
	if err != nil {
		log.Debug("dumpResponse err: %v", err)
	} else {
		log.Debug("dumpResponse ok: %v", string(dump))
	}
}

// Setbackend sets the appropriate URL scheme and authority for the backend resource.
func (c *Client) setBackend(backend string, net string, reqConfig *ReqConfig) {
	// Check if URL scheme and authority is already set.
	if strings.HasPrefix(reqConfig.url, "http") {
		return
	}

	// Prepend URL sheme and authority to the URL.
	if authority, ok := backendUrl[net][backend]; ok {
		reqConfig.url = fmt.Sprintf("%s%s", authority, reqConfig.url)
	}
}
