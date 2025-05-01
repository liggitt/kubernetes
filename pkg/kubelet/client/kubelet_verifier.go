/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"k8s.io/apimachinery/pkg/types"
)

func makeCombinedAddress(nodeName types.NodeName, nodeAddress string) string {
	return string(nodeName) + "+" + nodeAddress
}

// newNodeVerifyingTransport returns a copy of baseTransport with the custom dialer replaced
// with one that additional verifies the CN of the server's certificate matches the expected node name
// pulled from the request context.
//
// This must be paired with a newNodeInfoInjectingTransport to make requests to a particular node.
func newNodeVerifyingTransport(baseTransport *http.Transport) (http.RoundTripper, error) {
	// assert a custom dialer
	if baseTransport.DialContext == nil {
		return nil, fmt.Errorf("error creating node verifying transport, no DialContext specified")
	}
	// assert a tls client config
	if baseTransport.TLSClientConfig == nil {
		return nil, fmt.Errorf("error creating node verifying transport, no TLSClientConfig specified")
	}
	// assert there are no custom TLS dialers
	if baseTransport.DialTLS != nil || baseTransport.DialTLSContext != nil {
		return nil, fmt.Errorf("error creating node verifying transport, opaque TLS dialers configured")
	}

	// copy the base transport
	tlsVerifyingTransport := baseTransport.Clone()

	// clear custom dialers
	tlsVerifyingTransport.Dial = nil
	tlsVerifyingTransport.DialContext = nil

	// TODO: what about Proxy? That skips DialTLSContext

	// set a custom TLS dialer that uses the original dialer to get a non-TLS connection, and TLS config to do standard TLS validation
	tlsVerifyingTransport.DialTLSContext = (&nodeVerifyingTLSDialer{
		dialContext:     baseTransport.DialContext,
		tlsClientConfig: baseTransport.TLSClientConfig,
	}).DialTLSContext

	return tlsVerifyingTransport, nil
}

type nodeVerifyingTLSDialer struct {
	dialContext     func(ctx context.Context, network, addr string) (net.Conn, error)
	tlsClientConfig *tls.Config
}

func (n *nodeVerifyingTLSDialer) DialTLSContext(ctx context.Context, network, combinedAddress string) (returnedConn net.Conn, returnedErr error) {
	nodeInfo, ok := NodeInfoFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("context missing node info")
	}

	expectedCN := "system:node:" + string(nodeInfo.nodeName)
	expectedCombinedAddress := makeCombinedAddress(nodeInfo.nodeName, nodeInfo.nodeHostPort)
	if combinedAddress != expectedCombinedAddress {
		return nil, fmt.Errorf("unexpected dial address")
	}

	// get a connection to the real address from the base dialer
	conn, err := n.dialContext(ctx, network, nodeInfo.nodeHostPort)
	if err != nil {
		return nil, err
	}

	// get a normal TLS connection
	tlsConfig := n.tlsClientConfig.Clone()
	tlsConfig.ServerName, _, _ = net.SplitHostPort(nodeInfo.nodeHostPort)
	tlsConn := tls.Client(conn, tlsConfig)
	defer func() {
		// If we return an error, make sure we close the tlsConn and don't return it
		if returnedErr != nil {
			tlsConn.Close()
			returnedConn = nil
		}
	}()

	// do normal TLS verification
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return nil, err
	}
	state := tlsConn.ConnectionState()
	if !state.HandshakeComplete {
		return nil, fmt.Errorf("handshake did not complete")
	}

	// do additional verification on the subject CN
	if len(state.VerifiedChains) == 0 || len(state.VerifiedChains[0]) == 0 || state.VerifiedChains[0][0] == nil {
		return nil, fmt.Errorf("VerifiedChains missing verified peer certificate")
	}
	if cn := state.VerifiedChains[0][0].Subject.CommonName; cn != expectedCN {
		return nil, fmt.Errorf("serving certificate for node %q was %q, expected %q", nodeInfo.nodeName, cn, expectedCN)
	}

	return tlsConn, nil
}

func newNodeInfoInjectingTransport(nodeName types.NodeName, verifyingTransport http.RoundTripper) http.RoundTripper {
	return &nodeInfoInjectingTransport{
		nodeName:           nodeName,
		verifyingTransport: verifyingTransport,
	}
}

type nodeInfoInjectingTransport struct {
	nodeName           types.NodeName
	verifyingTransport http.RoundTripper
}

func (n *nodeInfoInjectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	nodeHost, nodePort, err := net.SplitHostPort(req.URL.Host)
	if err != nil || nodeHost == "" || nodePort == "" {
		return nil, fmt.Errorf("could not extract node host and port from %q", req.URL.Host)
	}
	nodeHostPort := net.JoinHostPort(nodeHost, nodePort)
	combinedHost := makeCombinedAddress(n.nodeName, nodeHostPort)

	ctx := WithNodeInfo(req.Context(), nodeInfo{nodeName: n.nodeName, nodeHostPort: nodeHostPort})
	req = req.Clone(ctx)
	req.URL.Host = combinedHost

	return n.verifyingTransport.RoundTrip(req)
}

type contextKeyType int

const (
	nodeInfoKey contextKeyType = iota
)

type nodeInfo struct {
	nodeName     types.NodeName
	nodeHostPort string
}

func NodeInfoFromContext(ctx context.Context) (nodeInfo, bool) {
	value, ok := ctx.Value(nodeInfoKey).(nodeInfo)
	return value, ok
}
func WithNodeInfo(ctx context.Context, value nodeInfo) context.Context {
	return context.WithValue(ctx, nodeInfoKey, value)
}
