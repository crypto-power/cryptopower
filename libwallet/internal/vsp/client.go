package vsp

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
)

type client struct {
	http.Client
	url  string
	pub  []byte
	sign func(context.Context, string, stdaddr.Address) ([]byte, error)
}

type signer interface {
	SignMessage(ctx context.Context, message string, address stdaddr.Address) ([]byte, error)
}

func newClient(url string, pub []byte, s signer) *client {
	return &client{url: url, pub: pub, sign: s.SignMessage}
}

type BadRequestError struct {
	HTTPStatus int    `json:"-"`
	Code       int    `json:"code"`
	Message    string `json:"message"`
}

func (e *BadRequestError) Error() string { return e.Message }

func (c *client) post(ctx context.Context, path string, addr stdaddr.Address, response interface{}, body []byte) error {
	return c.do(ctx, http.MethodPost, path, addr, response, body)
}

func (c *client) get(ctx context.Context, path string, resp interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, resp, nil)
}

func (c *client) do(ctx context.Context, method, path string, addr stdaddr.Address, response interface{}, body []byte) error {
	var err error
	var sig []byte
	reqConf := &utils.ReqConfig{
		Method:    method,
		HTTPURL:   c.url + path,
		IsRetByte: true,
		Headers:   make(http.Header),
	}

	if method == http.MethodPost {
		sig, err = c.sign(ctx, string(body), addr)
		if err != nil {
			return fmt.Errorf("sign request: %w", err)
		}
		reqConf.Payload = body
	}

	// Add cookies.
	if sig != nil {
		reqConf.Headers.Add("VSP-Client-Signature", base64.StdEncoding.EncodeToString(sig))
	}

	respBytes := []byte{}
	reply, err := utils.HTTPRequest(reqConf, &respBytes)
	if err != nil && reply == nil {
		// Status code errors are handled below.
		return err
	}

	status := reply.StatusCode
	is200 := status == 200
	is4xx := status >= 400 && status <= 499
	if !(is200 || is4xx) {
		return err
	}

	if err = json.Unmarshal(respBytes, response); err != nil {
		return fmt.Errorf("could not pack response data: %w", err)
	}

	sigBase64 := reply.Header.Get("VSP-Server-Signature")
	sig, err = base64.StdEncoding.DecodeString(sigBase64)
	if err != nil {
		return fmt.Errorf("cannot authenticate server: %w", err)
	}

	if !ed25519.Verify(c.pub, respBytes, sig) {
		return fmt.Errorf("cannot authenticate server: invalid signature")
	}

	var apiError *BadRequestError
	if is4xx {
		apiError = new(BadRequestError)
		apiError.HTTPStatus = status
		return apiError
	}

	return nil
}
