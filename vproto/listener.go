/*
   Copyright The containerd Authors.

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

package main

import (
	"io"
	"net"
	"os"
	"strconv"

	"github.com/pkg/errors"
)

// socketListener implements net.Listener for an already connected socket.
type socketListener struct {
	next chan net.Conn
	conn net.Conn
}

// newSocketListener returns a listener for the given socket file descriptor.
func newSocketListener(fd int) (*socketListener, error) {
	f := os.NewFile(uintptr(fd), "socket-fd#" + strconv.Itoa(fd))

	conn, err := net.FileConn(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to conn-wrap socket fd")
	}

	next := make(chan net.Conn, 1)
	next <- conn

	return &socketListener{
		next: next,
		conn: conn,
	}, nil
}

// newConnListener returns a listener for the given socket connection.
func newConnListener(conn net.Conn) *socketListener {
	next := make(chan net.Conn, 1)
	next <- conn

	return &socketListener{
		next: next,
		conn: conn,
	}
}

// Accept implements net.Listener.Accept() for a socketListener.
func (sl *socketListener) Accept() (net.Conn, error) {
	conn := <- sl.next
	if conn == nil {
		return nil, io.EOF
	}

	return conn, nil
}

// Close implements net.Listener.Close() for a socketListener.
func (sl *socketListener) Close() error {
	sl.conn.Close()
	close(sl.next)
	return nil
}

// Addr implements net.Listener.Addr() for a socketListener.
func (sl *socketListener) Addr() net.Addr {
	return sl.conn.LocalAddr()
}

