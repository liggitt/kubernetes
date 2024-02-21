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
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	gwebsocket "github.com/gorilla/websocket"

	"k8s.io/klog/v2"
)

const writeDeadline = 2 * time.Second

var _ net.Conn = &TunnelingConnection{}

// TunnelingConnection implements the "httpstream.Connection" interface, wrapping
// a websocket connection that tunnels SPDY.
type TunnelingConnection struct {
	conn              *gwebsocket.Conn
	closeChan         chan bool
	inProgressMessage io.Reader
}

// NewTunnelingConnection wraps the passed gorilla/websockets connection
// with the TunnelingConnection struct (implementing net.Conn).
func NewTunnelingConnection(conn *gwebsocket.Conn) *TunnelingConnection {
	closeChan := make(chan bool)
	tConn := &TunnelingConnection{
		conn:      conn,
		closeChan: closeChan,
	}
	// Close channel when detecting close connection.
	closeHandler := conn.CloseHandler()
	conn.SetCloseHandler(func(code int, text string) error {
		klog.V(3).Infof("websocket conn close: %d--%s", code, text)
		close(closeChan)
		err := closeHandler(code, text)
		return err
	})
	return tConn
}

func (c *TunnelingConnection) Read(p []byte) (int, error) {
	klog.Infoln("tunneling connection read...")
	defer klog.Infoln("tunneling connection read...complete")
	for {
		if c.inProgressMessage == nil {
			klog.Infoln("tunneling connection read before NextReader()...")
			messageType, nextReader, err := c.conn.NextReader()
			if err != nil {
				closeError := &gwebsocket.CloseError{}
				if errors.As(err, &closeError) && closeError.Code == gwebsocket.CloseNormalClosure {
					return 0, io.EOF
				}
				klog.Errorf("tunneling connection NextReader() error: %v", err)
				return 0, err
			}
			if messageType != gwebsocket.BinaryMessage {
				return 0, fmt.Errorf("invalid message type received")
			}
			c.inProgressMessage = nextReader
		}

		klog.Infoln("tunneling connection read in progress message...")
		i, err := c.inProgressMessage.Read(p)
		klog.Infof("tunneling connection read in progress message...%d bytes, (error: %v)", i, err)
		switch {
		case err == nil:
			return i, nil
		case err == io.EOF:
			c.inProgressMessage = nil
		case err != nil:
			return i, err
		}
	}
}

func (c *TunnelingConnection) Write(p []byte) (int, error) {
	klog.Infof("tunneling connection write: %d bytes", len(p))
	defer klog.Infoln("tunneling connection write...complete")
	if c.conn == nil {
		return 0, fmt.Errorf("write on closed tunneling connection")
	}
	// err := c.SetWriteDeadline(time.Now().Add(writeDeadline))
	// if err != nil {
	// 	klog.V(4).Infof("Tunneling conn setting write deadline failed %v", err)
	// 	return 0, err
	// }
	w, err := c.conn.NextWriter(gwebsocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	defer func() {
		if w != nil {
			w.Close() //nolint:errcheck
		}
	}()
	// Next, write the passed data in "p".
	n, err := w.Write(p)
	if err != nil {
		return n, err
	}
	err = w.Close()
	w = nil
	return n, err
}

func (c *TunnelingConnection) Close() error {
	klog.Infoln("tunneling connection Close()...")
	// Signal other endpoint that websocket connection is closing.
	c.conn.WriteControl(gwebsocket.CloseMessage, []byte{}, time.Now().Add(writeDeadline)) //nolint:errcheck
	return c.conn.Close()
}

func (c *TunnelingConnection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *TunnelingConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *TunnelingConnection) SetDeadline(t time.Time) error {
	err := c.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *TunnelingConnection) SetReadDeadline(t time.Time) error {
	klog.Infof("tunneling connection set read deadline: %v", t)
	return c.conn.SetReadDeadline(t)
}

func (c *TunnelingConnection) SetWriteDeadline(t time.Time) error {
	klog.Infof("tunneling connection set write deadline: %v", t)
	return c.conn.SetWriteDeadline(t)
}
