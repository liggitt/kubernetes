/*
Copyright 2014 The Kubernetes Authors.

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

package rest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	gwebsocket "github.com/gorilla/websocket"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/httpstream/wsstream"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/proxy"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	translator "k8s.io/apiserver/pkg/util/proxy"
	"k8s.io/klog/v2"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/capabilities"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/registry/core/pod"
)

// ProxyREST implements the proxy subresource for a Pod
type ProxyREST struct {
	Store          *genericregistry.Store
	ProxyTransport http.RoundTripper
}

// Implement Connecter
var _ = rest.Connecter(&ProxyREST{})

var proxyMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

// New returns an empty podProxyOptions object.
func (r *ProxyREST) New() runtime.Object {
	return &api.PodProxyOptions{}
}

// Destroy cleans up resources on shutdown.
func (r *ProxyREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// ConnectMethods returns the list of HTTP methods that can be proxied
func (r *ProxyREST) ConnectMethods() []string {
	return proxyMethods
}

// NewConnectOptions returns versioned resource that represents proxy parameters
func (r *ProxyREST) NewConnectOptions() (runtime.Object, bool, string) {
	return &api.PodProxyOptions{}, true, "path"
}

// Connect returns a handler for the pod proxy
func (r *ProxyREST) Connect(ctx context.Context, id string, opts runtime.Object, responder rest.Responder) (http.Handler, error) {
	proxyOpts, ok := opts.(*api.PodProxyOptions)
	if !ok {
		return nil, fmt.Errorf("Invalid options object: %#v", opts)
	}
	location, transport, err := pod.ResourceLocation(ctx, r.Store, r.ProxyTransport, id)
	if err != nil {
		return nil, err
	}
	location.Path = net.JoinPreservingTrailingSlash(location.Path, proxyOpts.Path)
	// Return a proxy handler that uses the desired transport, wrapped with additional proxy handling (to get URL rewriting, X-Forwarded-* headers, etc)
	return newThrottledUpgradeAwareProxyHandler(location, transport, true, false, responder), nil
}

// Support both GET and POST methods. We must support GET for browsers that want to use WebSockets.
var upgradeableMethods = []string{"GET", "POST"}

// AttachREST implements the attach subresource for a Pod
type AttachREST struct {
	Store       *genericregistry.Store
	KubeletConn client.ConnectionInfoGetter
}

// Implement Connecter
var _ = rest.Connecter(&AttachREST{})

// New creates a new podAttachOptions object.
func (r *AttachREST) New() runtime.Object {
	return &api.PodAttachOptions{}
}

// Destroy cleans up resources on shutdown.
func (r *AttachREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Connect returns a handler for the pod exec proxy
func (r *AttachREST) Connect(ctx context.Context, name string, opts runtime.Object, responder rest.Responder) (http.Handler, error) {
	attachOpts, ok := opts.(*api.PodAttachOptions)
	if !ok {
		return nil, fmt.Errorf("Invalid options object: %#v", opts)
	}
	location, transport, err := pod.AttachLocation(ctx, r.Store, r.KubeletConn, name, attachOpts)
	if err != nil {
		return nil, err
	}
	handler := newThrottledUpgradeAwareProxyHandler(location, transport, false, true, responder)
	if utilfeature.DefaultFeatureGate.Enabled(features.TranslateStreamCloseWebsocketRequests) {
		// Wrap the upgrade aware handler to implement stream translation
		// for WebSocket/V5 upgrade requests.
		streamOptions := translator.Options{
			Stdin:  attachOpts.Stdin,
			Stdout: attachOpts.Stdout,
			Stderr: attachOpts.Stderr,
			Tty:    attachOpts.TTY,
		}
		maxBytesPerSec := capabilities.Get().PerConnectionBandwidthLimitBytesPerSec
		streamtranslator := translator.NewStreamTranslatorHandler(location, transport, maxBytesPerSec, streamOptions)
		handler = translator.NewTranslatingHandler(handler, streamtranslator, wsstream.IsWebSocketRequestWithStreamCloseProtocol)
	}
	return handler, nil
}

// NewConnectOptions returns the versioned object that represents exec parameters
func (r *AttachREST) NewConnectOptions() (runtime.Object, bool, string) {
	return &api.PodAttachOptions{}, false, ""
}

// ConnectMethods returns the methods supported by exec
func (r *AttachREST) ConnectMethods() []string {
	return upgradeableMethods
}

// ExecREST implements the exec subresource for a Pod
type ExecREST struct {
	Store       *genericregistry.Store
	KubeletConn client.ConnectionInfoGetter
}

// Implement Connecter
var _ = rest.Connecter(&ExecREST{})

// New creates a new podExecOptions object.
func (r *ExecREST) New() runtime.Object {
	return &api.PodExecOptions{}
}

// Destroy cleans up resources on shutdown.
func (r *ExecREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Connect returns a handler for the pod exec proxy
func (r *ExecREST) Connect(ctx context.Context, name string, opts runtime.Object, responder rest.Responder) (http.Handler, error) {
	execOpts, ok := opts.(*api.PodExecOptions)
	if !ok {
		return nil, fmt.Errorf("invalid options object: %#v", opts)
	}
	location, transport, err := pod.ExecLocation(ctx, r.Store, r.KubeletConn, name, execOpts)
	if err != nil {
		return nil, err
	}
	handler := newThrottledUpgradeAwareProxyHandler(location, transport, false, true, responder)
	if utilfeature.DefaultFeatureGate.Enabled(features.TranslateStreamCloseWebsocketRequests) {
		// Wrap the upgrade aware handler to implement stream translation
		// for WebSocket/V5 upgrade requests.
		streamOptions := translator.Options{
			Stdin:  execOpts.Stdin,
			Stdout: execOpts.Stdout,
			Stderr: execOpts.Stderr,
			Tty:    execOpts.TTY,
		}
		maxBytesPerSec := capabilities.Get().PerConnectionBandwidthLimitBytesPerSec
		streamtranslator := translator.NewStreamTranslatorHandler(location, transport, maxBytesPerSec, streamOptions)
		handler = translator.NewTranslatingHandler(handler, streamtranslator, wsstream.IsWebSocketRequestWithStreamCloseProtocol)
	}
	return handler, nil
}

// NewConnectOptions returns the versioned object that represents exec parameters
func (r *ExecREST) NewConnectOptions() (runtime.Object, bool, string) {
	return &api.PodExecOptions{}, false, ""
}

// ConnectMethods returns the methods supported by exec
func (r *ExecREST) ConnectMethods() []string {
	return upgradeableMethods
}

// PortForwardREST implements the portforward subresource for a Pod
type PortForwardREST struct {
	Store       *genericregistry.Store
	KubeletConn client.ConnectionInfoGetter
}

// Implement Connecter
var _ = rest.Connecter(&PortForwardREST{})

// New returns an empty podPortForwardOptions object
func (r *PortForwardREST) New() runtime.Object {
	return &api.PodPortForwardOptions{}
}

// Destroy cleans up resources on shutdown.
func (r *PortForwardREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// NewConnectOptions returns the versioned object that represents the
// portforward parameters
func (r *PortForwardREST) NewConnectOptions() (runtime.Object, bool, string) {
	return &api.PodPortForwardOptions{}, false, ""
}

// ConnectMethods returns the methods supported by portforward
func (r *PortForwardREST) ConnectMethods() []string {
	return upgradeableMethods
}

// Connect returns a handler for the pod portforward proxy
func (r *PortForwardREST) Connect(ctx context.Context, name string, opts runtime.Object, responder rest.Responder) (http.Handler, error) {
	portForwardOpts, ok := opts.(*api.PodPortForwardOptions)
	if !ok {
		return nil, fmt.Errorf("invalid options object: %#v", opts)
	}
	location, transport, err := pod.PortForwardLocation(ctx, r.Store, r.KubeletConn, name, portForwardOpts)
	if err != nil {
		return nil, err
	}
	upgradeHandler := newThrottledUpgradeAwareProxyHandler(location, transport, false, true, responder), nil

	if wrap {
		tunnelingHandler(upgradeHandler)
	}
}

func newThrottledUpgradeAwareProxyHandler(location *url.URL, transport http.RoundTripper, wrapTransport, upgradeRequired bool, responder rest.Responder) http.Handler {
	handler := proxy.NewUpgradeAwareHandler(location, transport, wrapTransport, upgradeRequired, proxy.NewErrorResponder(responder))
	handler.MaxBytesPerSec = capabilities.Get().PerConnectionBandwidthLimitBytesPerSec
	return handler
}

type tunnelingHandler struct {
	delegate http.Handler
}

func (t *tunnelingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	tunneled := true //recognize websocket portforwardv2 protocol
	if tunneled {
		// If SPDY upstream connection was successfully established, then
		// upgrade the current request to a websocket server connection.
		var upgrader = gwebsocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Accepting all requests
			},
			Subprotocols: []string{
				"portforwardv2",
			},
		}

		// make synthetic request upstream to make sure upgrade succeeds

		// then, upgrade websocket request from the client
		// to negotiate protocol and get a websocket conn
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			klog.Errorf("error upgrading websocket connection: %v", err)
			return
		}

		tunneledReader := &websocketReader{conn: conn}

		// construct tunneledRequest
		tunneledRequest := http.Request{
			// TODO: make spdy 3.1 portforward upgrade headers
			Body: tunneledReader,
		}

		// construct tunneledWriter which can be hijacked
		var tunneledWriter http.ResponseWriter

		writer, err := conn.NextWriter(gwebsocket.BinaryMessage)
		writer.Write()

		t.delegate.ServeHTTP(tunneledWriter, tunneledRequest)

	}
}

type websocketReader struct {
	conn              *gwebsocket.Conn
	inProgressMessage io.Reader
}

func (r *websocketReader) Close() error {
	// TODO: is this right?
	return w.conn.Close()
}

func (r *websocketReader) Read(buf []byte) (i int, err error) {
	for {
		if r.inProgressMessage == nil {
			messageType, nextReader, err := w.conn.NextReader()
			if err != nil {
				closeError := &gwebsocket.CloseError{}
				if errors.As(err, closeError) && closeError.Code == gwebsocket.CloseNormalClosure {
					return 0, io.EOF
				}
				return 0, err
			}
			if messageType != gwebsocket.BinaryMessage {
				return 0, fmt.Errorf("invalid message type received")
			}
			r.inProgressMessage = nextReader
		}

		i, err = r.inProgressMessage.Read(buf)
		switch {
		case err == nil:
			return i, nil
		case err == io.EOF:
			r.inProgressMessage = nil
		case err != nil:
			return i, err
		}
	}
}


func websocketWriter struct {
	conn              *gwebsocket.Conn
}

// upgrade headers
func (w* websocketWriter) Header() Header
func (w* websocketWriter) WriteHeader(statusCode int)

// error case, write non-upgrade headers and error data to the response
// success case, we hijack before writing
func (w* websocketWriter) Write([]byte) (int, error)
func (w* websocketWriter) Hijack() (net.Conn, *bufio.ReadWriter, error)