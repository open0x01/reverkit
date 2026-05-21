package ldapserver

import (
	"bufio"
	"net"
	"sync"
	"time"
)

// Server is an LDAP server.
type Server struct {
	Listener     net.Listener
	ReadTimeout  time.Duration  // optional read timeout
	WriteTimeout time.Duration  // optional write timeout
	Wg           sync.WaitGroup // group of goroutines (1 by client)
	chDone       chan bool      // Channel Done, value => shutdown

	// OnNewConnection, if non-nil, is called on new connections.
	// If it returns non-nil, the connection is closed.
	OnNewConnection func(c net.Conn) error

	// Handler handles ldap message received from client
	// it SHOULD "implement" RequestHandler interface
	Handler Handler
}

// NewServer return a LDAP Server
func NewServer() *Server {
	return &Server{
		chDone: make(chan bool),
	}
}

// Handle registers the handler for the server.
// If a handler already exists for pattern, Handle panics
func (s *Server) Handle(h Handler) {
	if s.Handler != nil {
		panic("LDAP: multiple Handler registrations")
	}
	s.Handler = h
}

// ListenAndServe listens on the TCP network address s.Addr and then
// calls Serve to handle requests on incoming connections.  If
// s.Addr is blank, ":389" is used.
func (s *Server) ListenAndServe(addr string, options ...func(*Server)) error {

	var e error
	s.Listener, e = net.Listen("tcp", addr)
	if e != nil {
		return e
	}
	Logger.Debug("Listening on %s\n", addr)

	for _, option := range options {
		option(s)
	}

	return s.Serve()
}

func (s *Server) UseConnServer(rw net.Conn) error {
	if s.ReadTimeout != 0 {
		rw.SetReadDeadline(time.Now().Add(s.ReadTimeout))
	}
	if s.WriteTimeout != 0 {
		rw.SetWriteDeadline(time.Now().Add(s.WriteTimeout))
	}
	cli, err := s.NewClient(rw)

	if err != nil {
		return err
	}
	s.Wg.Add(1)
	go cli.Serve()
	return nil
}

// Handle requests messages on the ln listener
func (s *Server) Serve() error {
	defer s.Listener.Close()

	if s.Handler == nil {
		Logger.Debug("No LDAP Request Handler defined")
	}
	for {
		select {
		case <-s.chDone:
			Logger.Debug("Stopping server")
			s.Listener.Close()
			return nil
		default:
		}

		rw, err := s.Listener.Accept()
		if nil != err {
			if opErr, ok := err.(*net.OpError); ok || opErr.Timeout() {
				continue
			}
			Logger.Debug(err)
		}
		return s.UseConnServer(rw)

	}
}

// Return a new session with the connection
// client has a writer and reader buffer
func (s *Server) NewClient(rwc net.Conn) (c *client, err error) {
	c = &client{
		srv: s,
		rwc: rwc,
		br:  bufio.NewReader(rwc),
		bw:  bufio.NewWriter(rwc),
	}
	return c, nil
}

func (s *Server) Stop() {
	close(s.chDone)
	Logger.Debug("gracefully closing client connections...")
	s.Wg.Wait()
	Logger.Debug("all clients connection closed")
}
