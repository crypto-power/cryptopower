// Copyright (c) 2019-2021, The Decred developers
// Copyright (c) 2023, The Cryptopower developers
// See LICENSE for details.

// This was almost entirely written using
// https://blog.3d-logic.com/2015/03/29/signalr-on-the-wire-an-informal-description-of-the-signalr-protocol/
// and github.com/carterjones/signalr as a reference guide.

package ext

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/url"
)

// defaultClientProtocol is the default protocol version used when connecting to
// a signalR websocket.
const defaultClientProtocol = "1.5"

// signalRClientMsg represents a message sent from or to the signalR server on a
// persistent websocket connection.
type signalRClientMsg struct {
	// invocation identifier – allows to match up responses with requests
	I int
	// the name of the hub
	H string
	// the name of the method
	M string
	// arguments (an array, can be empty if the method does not have any
	// parameters)
	A []interface{}
	// state – a dictionary containing additional custom data (optional)
	S *json.RawMessage `json:",omitempty"`
}

// makeSignalRURL is used to construct a signalR connection URL for the action
// specified.
func makeSignalRURL(action, host, endpoint string, params map[string]string) string {
	var u url.URL
	u.Scheme = "https"
	u.Host = host
	u.Path = endpoint

	param := url.Values{}
	for key, value := range params {
		param.Set(key, value)
	}

	switch action {
	case "negotiate":
		u.Path += "/negotiate"
	case "connect":
		u.Path += "/connect"
		u.Scheme = "wss"
		param.Set("tid", fmt.Sprintf("%.0f", math.Floor(rand.Float64()*11)))
	case "start":
		u.Path += "/start"
	}

	u.RawQuery = param.Encode()
	return u.String()
}
