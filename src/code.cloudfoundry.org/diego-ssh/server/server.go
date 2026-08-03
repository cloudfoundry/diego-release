package server

import (
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter -o fakes/fake_connection_handler.go . ConnectionHandler
type ConnectionHandler interface {
	HandleConnection(net.Conn)
}

type Server struct {
	logger            lager.Logger
	listenAddress     string
	connectionHandler ConnectionHandler
	listener          net.Listener
	mutex             *sync.Mutex
	state             serverState
	idleConnTimeout   time.Duration
	store             connHandler
}

func NewServer(
	logger lager.Logger,
	listenAddress string,
	connectionHandler ConnectionHandler,
	idleConnTimeout time.Duration,
) *Server {
	return &Server{
		logger:            logger.Session("server"),
		listenAddress:     listenAddress,
		connectionHandler: connectionHandler,
		mutex:             &sync.Mutex{},
		idleConnTimeout:   idleConnTimeout,
	}
}

func (s *Server) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	listener, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return err
	}

	s.SetListener(listener)
	go s.Serve()

	close(ready)

	<-signals
	s.Shutdown()

	return nil
}

func (s *Server) Shutdown() {
	if s.state.StopOnce() {
		s.logger.Info("stopping-server")
		err := s.listener.Close()
		if err != nil {
			s.logger.Error("listener-failed-to-close", err)
		}
		s.store.Shutdown()
	}
}

func (s *Server) IsStopping() bool { return s.state.Stopped() }

func (s *Server) SetListener(listener net.Listener) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.listener != nil {
		err := errors.New("Listener has already been set")
		s.logger.Error("listener-already-set", err)
	}

	s.listener = listener
}

func (s *Server) ListenAddr() (net.Addr, error) {
	if s.listener == nil {
		return nil, errors.New("No listener")
	}

	return s.listener.Addr(), nil
}

type idleTimeoutConn struct {
	Timeout time.Duration
	net.Conn
}

func (c *idleTimeoutConn) Read(b []byte) (n int, err error) {
	if err = c.Conn.SetDeadline(time.Now().Add(c.Timeout)); err != nil {
		return
	}
	return c.Conn.Read(b)
}

func (c *idleTimeoutConn) Write(b []byte) (n int, err error) {
	if err = c.Conn.SetDeadline(time.Now().Add(c.Timeout)); err != nil {
		return
	}
	return c.Conn.Write(b)
}

func (s *Server) Serve() {
	logger := s.logger.Session("serve")
	defer s.listener.Close()

	for {
		netConn, err := s.listener.Accept()
		if s.idleConnTimeout > 0 {
			netConn = &idleTimeoutConn{s.idleConnTimeout, netConn}
		}
		if err != nil {
			//lint:ignore SA1019 - http.Server still uses this logic, and they dont want to update it because its scary. Following their lead.
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				logger.Error("accept-temporary-error", netErr)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if s.IsStopping() {
				break
			}

			logger.Error("accept-failed", err)
			return
		}
		s.store.Handle(s.connectionHandler, netConn)
	}
}
