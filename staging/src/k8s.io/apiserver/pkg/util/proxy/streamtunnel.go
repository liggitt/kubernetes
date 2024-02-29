/*
Copyright 2024 The Kubernetes Authors.

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

package proxy

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	gwebsocket "github.com/gorilla/websocket"

	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/apimachinery/pkg/util/httpstream/wsstream"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	constants "k8s.io/apimachinery/pkg/util/portforward"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/klog/v2"
)

// TunnelingHandler is a handler which tunnels SPDY through WebSockets.
type TunnelingHandler struct {
	// Used to communicate between upstream SPDY and downstream tunnel.
	upgradeHandler http.Handler
}

// NewTunnelingHandler is used to create the tunnel between an upstream
// SPDY connection and a downstream tunneling connection through the stored
// UpgradeAwareProxy.
func NewTunnelingHandler(upgradeHandler http.Handler) *TunnelingHandler {
	return &TunnelingHandler{upgradeHandler: upgradeHandler}
}

// ServeHTTP uses the upgradeHandler to tunnel between a downstream tunneling
// connection and an upstream SPDY connection. The tunneling connection is
// a wrapped WebSockets connection which communicates SPDY framed data.
func (h *TunnelingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	klog.V(4).Infoln("TunnelingHandler ServeHTTP")

	spdyProtocols := spdyProtocolsFromWebsocketProtocols(req)
	if len(spdyProtocols) == 0 {
		http.Error(w, "unable to upgrade: no tunneling spdy protocols provided", http.StatusBadRequest)
		return
	}

	spdyRequest := createSPDYRequest(req, spdyProtocols...)

	// First, terminate the websocket connection, creating the net.Conn
	// that will be used to tunnel SPDY.
	var upgrader = gwebsocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Accepting all requests
		},
		Subprotocols: websocketProtocolsFromSPDYProtocols(spdyProtocols),
	}
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		klog.Errorf("error upgrading websocket connection: %v", err)
		return
	}
	defer conn.Close() //nolint:errcheck
	tunnelProtocol := conn.Subprotocol()
	klog.V(4).Infof("websocket connection created: %s", tunnelProtocol)
	tunnelingConn := portforward.NewTunnelingConnection("server", conn)
	headerInterceptingConnection := &headerInterceptingConnection{Conn: tunnelingConn, headerBuffer: bytes.NewBuffer(nil)}
	// Create ResponseWriter which will be hijacked to use the tunnel.
	writer := createTunnelingResponseWriter(w, headerInterceptingConnection)

	klog.V(4).Infoln("Tunnel spdy through websockets using the UpgradeAwareProxy")
	h.upgradeHandler.ServeHTTP(writer, spdyRequest)
}

// createSPDYRequest modifies the passed request to remove
// WebSockets headers and add SPDY upgrade information, including
// spdy protocols acceptable to the client.
func createSPDYRequest(req *http.Request, spdyProtocols ...string) *http.Request {
	clone := utilnet.CloneRequest(req)
	// Clean up the websocket headers from the http request.
	clone.Header.Del(wsstream.WebSocketProtocolHeader)
	clone.Header.Del("Sec-Websocket-Key")
	clone.Header.Del("Sec-Websocket-Version")
	clone.Header.Del(httpstream.HeaderUpgrade)
	// Update the http request for an upstream SPDY upgrade.
	clone.Method = "POST"
	clone.Body = nil // Remove the request body which is unused.
	clone.Header.Set(httpstream.HeaderUpgrade, spdy.HeaderSpdy31)
	clone.Header.Del(httpstream.HeaderProtocolVersion)
	for i := range spdyProtocols {
		clone.Header.Add(httpstream.HeaderProtocolVersion, spdyProtocols[i])
	}
	return clone
}

// spdyProtocolsFromWebsocketProtocols returns a list of spdy protocols by filtering
// to Kubernetes websocket subprotocols prefixed with "SPDY/3.1+", then removing the prefix
func spdyProtocolsFromWebsocketProtocols(req *http.Request) []string {
	var spdyProtocols []string
	for _, protocol := range gwebsocket.Subprotocols(req) {
		if strings.HasPrefix(protocol, constants.WebsocketsSPDYTunnelingPrefix) && strings.HasSuffix(protocol, constants.KubernetesSuffix) {
			spdyProtocols = append(spdyProtocols, strings.TrimPrefix(protocol, constants.WebsocketsSPDYTunnelingPrefix))
		}
	}
	return spdyProtocols
}

// websocketProtocolsFromSPDYProtocols returns a list of "SPDY/3.1+" prefixed websocket protocols
func websocketProtocolsFromSPDYProtocols(spdyProtocols []string) []string {
	var websocketProtocols []string
	for _, spdyProtocol := range spdyProtocols {
		websocketProtocols = append(websocketProtocols, constants.WebsocketsSPDYTunnelingPrefix+spdyProtocol)
	}
	return websocketProtocols
}

var _ http.ResponseWriter = &tunnelingWriter{}
var _ http.Hijacker = &tunnelingWriter{}

// tunnelingWriter implements the http.ResponseWriter and http.Hijcker
// interfaces. After "Hijack", the stored ResponseWriter will not work.
type tunnelingWriter struct {
	conn net.Conn
	w    http.ResponseWriter
}

func createTunnelingResponseWriter(w http.ResponseWriter, tunnel net.Conn) http.ResponseWriter {
	return &tunnelingWriter{conn: tunnel, w: w}
}

// Hijack returns a "net.Conn" which tunnels SPDY through WebSockets.
// The returned bufio.ReadWriter and error are always nil.
func (w *tunnelingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	klog.V(6).Infof("Hijack returning websocket tunneling net.Conn")
	return w.conn, nil, nil
}

// Header is delegated to the stored "http.ResponseWriter".
func (w *tunnelingWriter) Header() http.Header {
	return w.w.Header()
}

// Write is delegated to the stored "http.ResponseWriter".
func (w *tunnelingWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

// WriteHeader is delegated to the stored "http.ResponseWriter".
func (w *tunnelingWriter) WriteHeader(statusCode int) {
	w.w.WriteHeader(statusCode)
}

// headerInterceptingConnection wraps the tunneling "net.Conn" to drain the
// HTTP response from the upstream SPDY connection, since the client has
// already received a "101 Switching Protocols" from the WebSockets
// upgrade.
type headerInterceptingConnection struct {
	net.Conn

	headerBuffer     *bytes.Buffer
	completedHeaders bool
	parsedStatus     string
	parsedHeaders    http.Header
}

// Write intercepts to initially swallow the HTTP response, then
// delegate to the tunneling "net.Conn" once the response has been
// seen and processed.
func (h *headerInterceptingConnection) Write(b []byte) (int, error) {
	if h.completedHeaders {
		return h.Conn.Write(b)
	}

	// Write into the headerBuffer, then attempt to parse the bytes
	// as an http response.
	n, err := h.headerBuffer.Write(b)
	if err != nil {
		return n, err
	}
	bufferedReader := bufio.NewReader(h.headerBuffer)
	resp, err := http.ReadResponse(bufferedReader, nil)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		// don't yet have a complete set of headers
		// TODO: reset headerBuffer to append to end and read from start
		return n, nil
	}
	if err != nil {
		klog.Errorf("invalid headers: %v", err)
		return n, err
	}

	h.completedHeaders = true
	h.parsedStatus = resp.Status
	h.parsedHeaders = resp.Header
	h.headerBuffer = nil

	// Copy any remaining buffered data to the underlying conn
	remainingBuffer, _ := io.ReadAll(bufferedReader)
	if len(remainingBuffer) > 0 {
		_, err = h.Conn.Write(remainingBuffer)
	}

	return n, err
}
