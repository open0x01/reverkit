package reverkit

import (
	"bytes"
	"github.com/iami317/hubur"
	"io"
	"net"
	"sync"
	"time"
)

// A peekedConn subverts the net.Conn.Read implementation, primarily so that
// sniffed bytes can be transparently prepended.
type peekedConn struct {
	net.Conn
	r io.Reader
}

// Read allows control over the embedded net.Conn's read data. By using an
// io.MultiReader one can read from a conn, and then replace what they read, to
// be read again.
func (c *peekedConn) Read(buf []byte) (int, error) { return c.r.Read(buf) }

type rmiHTTPListener struct {
	net.Listener
	token                 string
	db                    *DB
	internalGroupEventMap *sync.Map
}

func (s *rmiHTTPListener) Accept() (net.Conn, error) {
	conn, err := s.Listener.Accept()
	if err != nil {
		return conn, err
	}
	b := make([]byte, 14)
	n, err := conn.Read(b)
	if err != nil || n < 4 {
		_ = conn.Close()
		return &stuckConn{}, nil
	}
	newConn := &peekedConn{
		Conn: conn,
		r:    io.MultiReader(bytes.NewReader(b[:n]), conn),
	}

	var ldapheader = []byte{48, 12, 2, 1, 1, 96, 7, 2, 1, 3, 4, 0, 128, 0}
	if string(b[:4]) == "JRMI" {
		go handleRMIConn(s.token, s.db, s.internalGroupEventMap, hubur.NewTimeoutConn(newConn, 5*time.Second).(*hubur.TimeoutConn))
		return &stuckConn{}, nil
	} else if string(b[:14]) == string(ldapheader) {
		go handleLdapConn(s.token, s.db, s.internalGroupEventMap, hubur.NewTimeoutConn(newConn, 5*time.Second).(*hubur.TimeoutConn))
		return &stuckConn{}, nil
	}
	return newConn, nil
}

// 一个什么也不做的，io 操作都 EOF 的 conn
type stuckConn struct {
}

func (s stuckConn) Read(b []byte) (n int, err error) {
	return 0, io.EOF
}

func (s stuckConn) Write(b []byte) (n int, err error) {
	return 0, io.ErrClosedPipe
}

func (s stuckConn) Close() error {
	return nil
}

func (s stuckConn) LocalAddr() net.Addr {
	noUseLocalTCPAddr, _ := net.ResolveTCPAddr("tcp", ":0")
	return noUseLocalTCPAddr
}

func (s stuckConn) RemoteAddr() net.Addr {
	noUseRemoteAddr, _ := net.ResolveTCPAddr("tcp", ":0")
	return noUseRemoteAddr
}

func (s stuckConn) SetDeadline(t time.Time) error {
	return nil
}

func (s stuckConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (s stuckConn) SetWriteDeadline(t time.Time) error {
	return nil
}
