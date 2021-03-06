// Copyright 2020 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package grpc

import (
	"fmt"
	"sync/atomic"

	"google.golang.org/grpc"
)

// ConnPool is a pool of grpc.ClientConns.
type ConnPool interface {
	// Conn returns a ClientConn from the pool.
	//
	// Conns aren't returned to the pool.
	Conn() *grpc.ClientConn

	// Close closes every ClientConn in the group.
	//
	// The error returned by Close may be a single error or multiple errors.
	Close() error
}

var _ ConnPool = &roundRobinConnPool{}
var _ ConnPool = &singleConnPool{}

// singleConnPool is a special case for a single connection.
type singleConnPool struct {
	conn *grpc.ClientConn
}

func (p *singleConnPool) Conn() *grpc.ClientConn {
	return p.conn
}

func (p *singleConnPool) Close() error {
	return p.conn.Close()
}

type roundRobinConnPool struct {
	conns []*grpc.ClientConn

	idx uint32 // access via sync/atomic
}

func (p *roundRobinConnPool) Conn() *grpc.ClientConn {
	i := atomic.AddUint32(&p.idx, 1)
	return p.conns[i%uint32(len(p.conns))]
}

func (p *roundRobinConnPool) Close() error {
	var errs multiError
	for _, conn := range p.conns {
		if err := conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// multiError represents errors from mulitple conns in the group.
//
// TODO: figure out how and whether this is useful to export. End users should
// not be depending on the transport/grpc package directly, so there might need
// to be some service-specific multi-error type.
type multiError []error

func (m multiError) Error() string {
	s, n := "", 0
	for _, e := range m {
		if e != nil {
			if n == 0 {
				s = e.Error()
			}
			n++
		}
	}
	switch n {
	case 0:
		return "(0 errors)"
	case 1:
		return s
	case 2:
		return s + " (and 1 other error)"
	}
	return fmt.Sprintf("%s (and %d other errors)", s, n-1)
}
