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
	"sync"

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

	writer := &tunnelingResponseWriter{
		w: w,
		conn: &headerInterceptingConnection{
			Conn:         tunnelingConn,
			headerBuffer: bytes.NewBuffer(nil),
		},
	}

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

var _ http.ResponseWriter = &tunnelingResponseWriter{}
var _ http.Hijacker = &tunnelingResponseWriter{}

// tunnelingResponseWriter implements the http.ResponseWriter and http.Hijacker interfaces.
// Only non-upgrade responses can be written using WriteHeader() and Write().
// Once Write or WriteHeader is called, Hijack returns an error.
// Once Hijack is called, Write, WriteHeader, and Hijack return errors.
type tunnelingResponseWriter struct {
	// w is used to delegate Header(), WriteHeader(), and Write() calls
	w http.ResponseWriter
	// conn is returned from Hijack()
	conn net.Conn
	// mu guards writes
	mu sync.Mutex
	// wrote tracks whether WriteHeader or Write has been called
	written bool
	// hijacked tracks whether Hijack has been called
	hijacked bool
}

// Hijack returns a delegate "net.Conn".
// An error is returned if Write(), WriteHeader(), or Hijack() was previously called.
// The returned bufio.ReadWriter is always nil.
func (w *tunnelingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.written {
		klog.Errorf("Hijack called after write")
		return nil, nil, errors.New("connection has already been written to")
	}
	if w.hijacked {
		klog.Errorf("Hijack called after hijack")
		return nil, nil, errors.New("connection has already been hijacked")
	}
	w.hijacked = true
	klog.V(6).Infof("Hijack returning websocket tunneling net.Conn")
	return w.conn, nil, nil
}

// Header is delegated to the stored "http.ResponseWriter".
func (w *tunnelingResponseWriter) Header() http.Header {
	return w.w.Header()
}

// Write is delegated to the stored "http.ResponseWriter".
func (w *tunnelingResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.hijacked {
		klog.Errorf("Write called after hijack")
		return 0, http.ErrHijacked
	}
	w.written = true
	return w.w.Write(p)
}

// WriteHeader is delegated to the stored "http.ResponseWriter".
func (w *tunnelingResponseWriter) WriteHeader(statusCode int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.written {
		klog.Errorf("WriteHeader called after write")
		return
	}
	if w.hijacked {
		klog.Errorf("WriteHeader called after hijack")
		return
	}
	w.written = true

	if statusCode == http.StatusSwitchingProtocols {
		// 101 upgrade responses must come via the hijacked connection, not WriteHeader
		klog.Errorf("WriteHeader called with 101 upgrade")
		http.Error(w.w, "unexpected upgrade", http.StatusInternalServerError)
		return
	}

	// pass through non-upgrade responses we don't need to translate
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
