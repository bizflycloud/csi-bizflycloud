// This file is part of csi-bizflycloud
//
// Copyright (C) 2020  Bizfly Cloud
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>

package csiclient

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog"
)

var (
	grpcCallCounter uint64

	dialOptions = []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  1.0 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   time.Second,
			},
		}),
		grpc.WithNoProxy(),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			callID := atomic.AddUint64(&grpcCallCounter, 1)

			klog.V(3).Infof("[ID:%d] FWD GRPC call: %s", callID, method)
			klog.V(5).Infof("[ID:%d] FWD GRPC request: %s", callID, protosanitizer.StripSecrets(req))

			err := invoker(ctx, method, req, reply, cc, opts...)
			if err != nil {
				klog.Infof("[ID:%d] FWD GRPC error: %v", callID, err)
			} else {
				klog.V(5).Infof("[ID:%d] FWD GRPC response: %s", callID, protosanitizer.StripSecrets(reply))
			}

			return err
		}),
	}

	_ Builder = &ClientBuilder{}
)

// NewNodeSvcClient creates a new Node service client from a gRPC connection.
func NewNodeSvcClient(conn *grpc.ClientConn) *NodeSvcClient {
	return &NodeSvcClient{cl: csi.NewNodeClient(conn)}
}

// NewIdentitySvcClient creates a new Identity service client from a gRPC connection.
func NewIdentitySvcClient(conn *grpc.ClientConn) *IdentitySvcClient {
	return &IdentitySvcClient{cl: csi.NewIdentityClient(conn)}
}

// NewConnection creates a new gRPC connection to the given endpoint.
func NewConnection(endpoint string) (*grpc.ClientConn, error) {
	var (
		conn *grpc.ClientConn
		err  error
	)

	dialFinished := make(chan bool)
	go func() {
		conn, err = grpc.Dial(endpoint, dialOptions...)
		close(dialFinished)
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			klog.Warningf("still connecting to %s", endpoint)
		case <-dialFinished:
			return conn, err
		}
	}
}

// ClientBuilder implements the Builder interface.
type ClientBuilder struct{}

func (b *ClientBuilder) NewConnection(endpoint string) (*grpc.ClientConn, error) {
	return NewConnection(endpoint)
}

func (b *ClientBuilder) NewConnectionWithContext(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	return grpc.DialContext(ctx, endpoint, dialOptions...) //nolint:staticcheck
}

func (b *ClientBuilder) NewNodeServiceClient(conn *grpc.ClientConn) Node {
	return NewNodeSvcClient(conn)
}

func (b *ClientBuilder) NewIdentityServiceClient(conn *grpc.ClientConn) Identity {
	return NewIdentitySvcClient(conn)
}
