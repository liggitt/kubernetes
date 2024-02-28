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
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	constants "k8s.io/apimachinery/pkg/util/portforward"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/registry/rest"
	restconfig "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
)

func TestTunnelingHandler_UpgradeStreamingAndTunneling(t *testing.T) {
	// Create fake upstream SPDY server, with channel receiving SPDY streams.
	streamChan := make(chan httpstream.Stream)
	defer close(streamChan)
	stopServerChan := make(chan struct{})
	defer close(stopServerChan)
	// Create fake upstream SPDY server.
	spdyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, err := httpstream.Handshake(req, w, []string{constants.PortForwardV1Name})
		require.NoError(t, err)
		upgrader := spdy.NewResponseUpgrader()
		conn := upgrader.UpgradeResponse(w, req, justQueueStream(streamChan))
		require.NotNil(t, conn)
		defer conn.Close() //nolint:errcheck
		<-stopServerChan
	}))
	defer spdyServer.Close()
	// Create UpgradeAwareProxy handler, with url/transport pointing to upstream SPDY. Then
	// create TunnelingHandler by injecting upgrade handler. Create TunnelingServer.
	url, err := url.Parse(spdyServer.URL)
	require.NoError(t, err)
	transport, err := fakeTransport()
	require.NoError(t, err)
	upgradeHandler := proxy.NewUpgradeAwareHandler(url, transport, false, true, proxy.NewErrorResponder(&fakeResponder{}))
	tunnelingHandler := NewTunnelingHandler(upgradeHandler)
	tunnelingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tunnelingHandler.ServeHTTP(w, req)
	}))
	defer tunnelingServer.Close()
	// Create SPDY client connection containing a TunnelingConnection by upgrading
	// a request to TunnelingHandler using new portforward version 2.
	tunnelingURL, err := url.Parse(tunnelingServer.URL)
	require.NoError(t, err)
	dialer, err := portforward.NewSPDYOverWebsocketDialer(tunnelingURL, &restconfig.Config{Host: tunnelingURL.Host})
	require.NoError(t, err)
	spdyClient, protocol, err := dialer.Dial("unused")
	require.NoError(t, err)
	assert.Equal(t, constants.PortForwardV2Name, protocol)
	defer spdyClient.Close() //nolint:errcheck
	// Create a SPDY client stream, which will queue a SPDY server stream
	// on the stream creation channel. Send random data on the client stream
	// reading off the SPDY server stream, and validating it was tunneled.
	randomSize := 1024 * 1024
	randomData := make([]byte, randomSize)
	_, err = rand.Read(randomData)
	require.NoError(t, err)
	var actual []byte
	go func() {
		clientStream, err := spdyClient.CreateStream(http.Header{})
		require.NoError(t, err)
		_, err = io.Copy(clientStream, bytes.NewReader(randomData))
		require.NoError(t, err)
		clientStream.Close() //nolint:errcheck
	}()
	select {
	case serverStream := <-streamChan:
		actual, err = io.ReadAll(serverStream)
		require.NoError(t, err)
		defer serverStream.Close() //nolint:errcheck
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatalf("timeout waiting for spdy stream to arrive on channel.")
	}
	assert.Equal(t, randomData, actual, "error validating tunneled random data")
}

const responseStr = `HTTP/1.1 101 Switching Protocols
Date: Sun, 25 Feb 2024 08:09:25 GMT
X-App-Protocol: portforward.k8s.io

`

const responseWithExtraStr = `HTTP/1.1 101 Switching Protocols
Date: Sun, 25 Feb 2024 08:09:25 GMT
X-App-Protocol: portforward.k8s.io

This is extra data.
`

const invalidResponseStr = `INVALID/1.1 101 Switching Protocols
Date: Sun, 25 Feb 2024 08:09:25 GMT
X-App-Protocol: portforward.k8s.io

`

func TestTunnelingHandler_HeaderInterceptingConnection(t *testing.T) {
	// Basic http response is intercepted correctly; no extra data sent to net.Conn.
	testConn := &mockConn{}
	hic := &headerInterceptingConnection{Conn: testConn, headerBuffer: bytes.NewBuffer(nil)}
	_, err := hic.Write([]byte(responseStr))
	require.NoError(t, err)
	assert.True(t, hic.completedHeaders, "successfully parsed http response headers")
	assert.Equal(t, "101 Switching Protocols", hic.parsedStatus)
	assert.Equal(t, "portforward.k8s.io", hic.parsedHeaders.Get("X-App-Protocol"))
	assert.Equal(t, 0, len(testConn.written), "no extra data written to net.Conn")
	// Extra data after response headers should be sent to net.Conn.
	hic = &headerInterceptingConnection{Conn: testConn, headerBuffer: bytes.NewBuffer(nil)}
	_, err = hic.Write([]byte(responseWithExtraStr))
	require.NoError(t, err)
	assert.True(t, hic.completedHeaders)
	assert.Equal(t, "101 Switching Protocols", hic.parsedStatus)
	assert.Equal(t, "This is extra data.\n", string(testConn.written), "extra data written to net.Conn")
	// Invalid response returns error.
	hic = &headerInterceptingConnection{Conn: &mockConn{}, headerBuffer: bytes.NewBuffer(nil)}
	_, err = hic.Write([]byte(invalidResponseStr))
	assert.Error(t, err, "expected error from invalid http response")
}

// mockConn implements "net.Conn" interface.
var _ net.Conn = &mockConn{}

type mockConn struct {
	written []byte
}

func (mc *mockConn) Write(p []byte) (int, error) {
	mc.written = make([]byte, len(p))
	copy(mc.written, p)
	return len(mc.written), nil
}

func (mc *mockConn) Read(p []byte) (int, error)         { return 0, nil }
func (mc *mockConn) Close() error                       { return nil }
func (mc *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (mc *mockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (mc *mockConn) SetDeadline(t time.Time) error      { return nil }
func (mc *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (mc *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// fakeResponder implements "rest.Responder" interface.
var _ rest.Responder = &fakeResponder{}

type fakeResponder struct{}

func (fr *fakeResponder) Object(statusCode int, obj runtime.Object) {}
func (fr *fakeResponder) Error(err error)                           {}

// justQueueStream skips the usual stream validation before
// queueing the stream on the stream channel.
func justQueueStream(streams chan httpstream.Stream) func(httpstream.Stream, <-chan struct{}) error {
	return func(stream httpstream.Stream, replySent <-chan struct{}) error {
		streams <- stream
		return nil
	}
}
