// Copyright (c) 2019 The Decred developers
// Use of this source code is governed by a 3-clause BSD
// license that can be found in the LICENSE file.

package socks

import (
	"context"
	"errors"
	"net"
	"sync"
)

var (
	ErrPoolMaxConnections = errors.New("maximum number of connections reached")
)

type Pool struct {
	mtx     sync.Mutex
	numOpen uint32
	maxOpen uint32

	proxy Proxy
}

func NewPool(proxy Proxy, maxOpen uint32) *Pool {
	pool := Pool{
		maxOpen: maxOpen,
		proxy:   proxy,
	}

	return &pool
}

func (p *Pool) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mtx.Lock()
	maxOpen := p.maxOpen
	if p.numOpen+1 > maxOpen {
		p.mtx.Unlock()
		return nil, ErrPoolMaxConnections
	}

	p.numOpen++
	p.mtx.Unlock()

	var d net.Dialer
	conn, err := p.proxy.dial(ctx, &d, network, address)
	if err != nil {
		p.mtx.Lock()
		p.numOpen--
		p.mtx.Unlock()
		return nil, err
	}
	conn.pool = p

	return conn, nil
}
