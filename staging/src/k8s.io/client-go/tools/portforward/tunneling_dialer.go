/*
Copyright 2023 The Kubernetes Authors.

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

package portforward

import (
	"fmt"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/transport/websocket"
	"k8s.io/klog/v2"
)

// tunnelingDialer implements "httpstream.Dial" interface
type tunnelingDialer struct {
	url       *url.URL
	transport http.RoundTripper
	holder    websocket.ConnectionHolder
}

// NewTunnelingDialer creates and returns the tunnelingDialer structure which implemements the "httpstream.Dialer"
// interface. The dialer can upgrade a websocket request, creating a websocket connection. This function
// returns an error if one occurs.
func NewTunnelingDialer(url *url.URL, config *restclient.Config) (httpstream.Dialer, error) {
	transport, holder, err := websocket.RoundTripperFor(config)
	if err != nil {
		return nil, err
	}
	return &tunnelingDialer{
		url:       url,
		transport: transport,
		holder:    holder,
	}, nil
}

// Dial upgrades a websocket request, returning a websocket connection (wrapped
// by an "httpstream.Connection"), the negotiated protocol, or an error if one occurred.
func (d *tunnelingDialer) Dial(protocols ...string) (httpstream.Connection, string, error) {
	// There is no passed context, so skip the context when creating request for now.
	// Websockets requires "GET" method: RFC 6455 Sec. 4.1 (page 17).
	req, err := http.NewRequest("GET", d.url.String(), nil)
	if err != nil {
		return nil, "", err
	}
	// Hard-code the v2 portforward protocol for the websocket dialer for now.
	websocketProtocols := []string{"v2.portforward.k8s.io"}
	klog.V(4).Infoln("Before WebSocket Upgrade Connection...")
	conn, err := websocket.Negotiate(d.transport, d.holder, req, websocketProtocols...)
	if err != nil {
		return nil, "", err
	}
	if conn == nil {
		return nil, "", fmt.Errorf("negotiated websocket connection is nil")
	}
	protocol := conn.Subprotocol()
	klog.V(4).Infof("negotiated protocol: %s", protocol)

	// Create tunneling websocket connection implementing net.Conn
	tConn := NewTunnelingConnection("client", conn)
	// Create SPDY connection with the previously created tConn.
	spdyConn, err := spdy.NewClientConnectionWithPings(tConn, 0)

	return spdyConn, protocol, err
}
