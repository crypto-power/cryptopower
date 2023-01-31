package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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
		Response *http.Response
		Payload  []byte
		Method   string
		HttpUrl  string
		// If IsRetByte is set to true, client.Do will delegate
		// response processing to caller.
		IsRetByte bool
		// IsActive should always be true, signifying that the user has authorised
		// the specific API call to access the internet.
		IsActive bool
	}
)

// NewClient configures and return a new client
func NewClient() (c *Client) {
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

// Do prepare and process HTTP request to backend resources.
func (c *Client) Do(reqConfig *ReqConfig, response interface{}) (err error) {
	if !reqConfig.IsActive {
		return fmt.Errorf("error: API call not allowed: %v", reqConfig.HttpUrl)
	}

	if _, err := url.ParseRequestURI(reqConfig.HttpUrl); err != nil {
		return fmt.Errorf("error: url not properly constituted: %v", err)
	}

	var req *http.Request
	req, err = c.requestFilter(reqConfig)
	if err != nil {
		return err
	}

	if req == nil {
		return errors.New("error: nil request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error: status: %v resp: %s", resp.Status, body)
	}

	// if IsRetByte is option is true. Response from the resource queried
	// is not in json format, don't unmarshal return response byte slice to
	// the caller for further processing.
	if reqConfig.IsRetByte {
		// r := reflect.Indirect(reflect.ValueOf(response))
		// r.Set(reflect.AppendSlice(r.Slice(0, 0), reflect.ValueOf(body)))
		reqConfig.Response = resp
		reqConfig.Payload = make([]byte, len(body))
		reqConfig.Payload = append(reqConfig.Payload, body...)
		return nil
	}

	err = json.Unmarshal(body, response)
	return err
}
