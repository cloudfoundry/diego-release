package http_server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/tedsuo/ifrit"
)

const (
	TCP  = "tcp"
	UNIX = "unix"
)

type httpServer struct {
	protocol string
	address  string
	handler  http.Handler

	tlsConfig *tls.Config
}

func newServerWithListener(protocol, address string, handler http.Handler, tlsConfig *tls.Config) ifrit.Runner {
	return &httpServer{
		address:   address,
		handler:   handler,
		tlsConfig: tlsConfig,
		protocol:  protocol,
	}
}

func NewUnixServer(address string, handler http.Handler) ifrit.Runner {
	return newServerWithListener(UNIX, address, handler, nil)
}

func New(address string, handler http.Handler) ifrit.Runner {
	return newServerWithListener(TCP, address, handler, nil)
}

func NewUnixTLSServer(address string, handler http.Handler, tlsConfig *tls.Config) ifrit.Runner {
	return newServerWithListener(UNIX, address, handler, tlsConfig)
}

func NewTLSServer(address string, handler http.Handler, tlsConfig *tls.Config) ifrit.Runner {
	return newServerWithListener(TCP, address, handler, tlsConfig)
}

func (s *httpServer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	server := http.Server{
		Handler:   s.handler,
		TLSConfig: s.tlsConfig,
	}

	listener, err := s.getListener(server.TLSConfig)
	if err != nil {
		return err
	}

	serverErrChan := make(chan error, 1)
	go func() {
		serverErrChan <- server.Serve(listener)
	}()

	close(ready)

	for {
		select {
		case err = <-serverErrChan:
			return err

		case <-signals:
			listener.Close()

			ctx, _ := context.WithTimeout(context.Background(), 1*time.Minute)
			server.Shutdown(ctx)

			return nil
		}
	}
}

func (s *httpServer) getListener(tlsConfig *tls.Config) (net.Listener, error) {
	listener, err := net.Listen(s.protocol, s.address)
	if err != nil {
		return nil, err
	}
	if tlsConfig == nil {
		return listener, nil
	}
	switch s.protocol {
	case TCP:
		listener = tls.NewListener(tcpKeepAliveListener{listener.(*net.TCPListener)}, tlsConfig)
	default:
		listener = tls.NewListener(listener, tlsConfig)
	}

	return listener, nil
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
