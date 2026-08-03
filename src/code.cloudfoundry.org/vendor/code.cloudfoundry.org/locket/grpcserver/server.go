package grpcserver

import (
	"crypto/tls"
	"net"
	"os"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type grpcServerRunner struct {
	listenAddress string
	handler       models.LocketServer
	logger        lager.Logger
	tlsConfig     *tls.Config
}

func NewGRPCServer(logger lager.Logger, listenAddress string, tlsConfig *tls.Config, handler models.LocketServer) grpcServerRunner {
	return grpcServerRunner{
		listenAddress: listenAddress,
		handler:       handler,
		logger:        logger,
		tlsConfig:     tlsConfig,
	}
}

func (s grpcServerRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := s.logger.Session("grpc-server")

	logger.Info("started")
	defer logger.Info("complete")

	lis, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		logger.Error("failed-to-listen", err)
		return err
	}

	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(s.tlsConfig)))
	models.RegisterLocketServer(server, s.handler)

	errCh := make(chan error)
	go func() {
		errCh <- server.Serve(lis)
	}()

	close(ready)

	select {
	case sig := <-signals:
		logger.Info("signalled", lager.Data{"signal": sig})
		break
	case err = <-errCh:
		logger.Error("failed-to-serve", err)
		break
	}

	server.GracefulStop()
	return err
}
