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
	"net"
	"net/http"

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
	// UpgradeAwareHandler used to communicate to upstream SPDY.
	upgradeHandler http.Handler
}

func NewTunnelingHandler(upgradeHandler http.Handler) *TunnelingHandler {
	return &TunnelingHandler{upgradeHandler: upgradeHandler}
}

func (h *TunnelingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	klog.Infoln("TunnelingHandler ServeHTTP")
	clone := utilnet.CloneRequest(req)

	// First, terminate the websocket connection, creating the net.Conn
	// that will be used to tunnel SPDY.
	klog.Infoln("Upgrading, terminating websocket connection...")
	var upgrader = gwebsocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Accepting all requests
		},
		Subprotocols: []string{
			constants.PortForwardV2Name,
		},
	}
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		klog.Errorf("error upgrading websocket connection: %v", err)
		return
	}
	defer conn.Close() //nolint:errcheck
	klog.Infof("websocket connection created: %s", conn.Subprotocol())
	tunnelingConn := portforward.NewTunnelingConnection("server", conn)

	// Create ResponseWriter which will be hijacked to use the tunnel.
	writer := createTunnelingResponseWriter(w, tunnelingConn)

	// Force the method to POST
	clone.Method = "POST"
	// Remove the request body which is unused (success case hijacks, error case doesn't read the body)
	clone.Body = io.NopCloser(bytes.NewBuffer(nil))
	// Remove all the websocket headers
	clone.Header.Del(wsstream.WebSocketProtocolHeader)
	clone.Header.Del("Sec-Websocket-Key")
	clone.Header.Del("Sec-Websocket-Version")
	clone.Header.Del(httpstream.HeaderUpgrade)
	// Add all the SPDY headers
	clone.Header.Add(httpstream.HeaderUpgrade, spdy.HeaderSpdy31)
	clone.Header.Add(httpstream.HeaderProtocolVersion, "portforward.k8s.io")

	// UpgradeHandler will first create the upstream SPDY connection,
	// copying SPDY data between the upstream connection and
	// the tunneling websocket connection (hijacked from writer).
	klog.Infoln("Calling upgrade aware proxy...")
	h.upgradeHandler.ServeHTTP(writer, clone)
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

func (w *tunnelingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	klog.Infof("hijacked--returning websocket tunneling net.Conn")
	return w.conn, nil, nil
}

func (w *tunnelingWriter) Header() http.Header {
	return http.Header{}
}

func (w *tunnelingWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

func (w *tunnelingWriter) WriteHeader(statusCode int) {}
