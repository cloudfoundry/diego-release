package workloadapi

import (
	"context"
	"net"
	"os"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/spiffe-agent/cfattestor"
	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/tedsuo/ifrit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const securityHeader = "workload.spiffe.io"

// Attestor turns a peer pid into a verified Cloud Foundry workload attestation.
type Attestor interface {
	Attest(ctx context.Context, pid int) (cfattestor.Attestation, error)
}

// Signer exchanges an attestation for a signed JWT-SVID and its SPIFFE ID.
type Signer interface {
	Sign(ctx context.Context, att cfattestor.Attestation, audience string) (svid, spiffeID string, err error)
}

// Server implements the SPIFFE Workload API, issuing JWT-SVIDs to the local
// process authenticated via SO_PEERCRED.
type Server struct {
	workload.UnimplementedSpiffeWorkloadAPIServer
	attestor Attestor
	signer   Signer
}

// NewServer wires an attestor and signer into a Workload API server.
func NewServer(attestor Attestor, signer Signer) *Server {
	return &Server{attestor: attestor, signer: signer}
}

// FetchJWTSVID attests the calling pid and returns a JWT-SVID for the first
// requested audience.
func (s *Server) FetchJWTSVID(ctx context.Context, req *workload.JWTSVIDRequest) (*workload.JWTSVIDResponse, error) {
	if err := checkSecurityHeader(ctx); err != nil {
		return nil, err
	}
	if len(req.Audience) < 1 {
		return nil, status.Error(codes.InvalidArgument, "audience is required")
	}

	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "no peer information in context")
	}
	authInfo, ok := p.AuthInfo.(peerCredAuthInfo)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing peer credentials")
	}

	att, err := s.attestor.Attest(ctx, int(authInfo.PID))
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "attestation failed: %v", err)
	}
	svid, spiffeID, err := s.signer.Sign(ctx, att, req.Audience[0])
	if err != nil {
		return nil, status.Errorf(codes.Internal, "signing failed: %v", err)
	}

	return &workload.JWTSVIDResponse{
		Svids: []*workload.JWTSVID{{SpiffeId: spiffeID, Svid: svid}},
	}, nil
}

func checkSecurityHeader(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.InvalidArgument, "missing metadata")
	}
	for _, v := range md.Get(securityHeader) {
		if v == "true" {
			return nil
		}
	}
	return status.Errorf(codes.InvalidArgument, "missing %q header", securityHeader)
}

// NewRunner returns an ifrit.Runner that serves the Workload API over a Unix
// socket guarded by SO_PEERCRED peer credentials, shutting down gracefully on
// signal.
func NewRunner(socketPath string, srv *Server, logger lager.Logger) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		logger = logger.Session("workloadapi")

		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			return err
		}
		if err := os.Chmod(socketPath, 0666); err != nil {
			listener.Close()
			return err
		}

		grpcServer := grpc.NewServer(grpc.Creds(peerCredentials{}))
		workload.RegisterSpiffeWorkloadAPIServer(grpcServer, srv)

		serveErr := make(chan error, 1)
		go func() {
			serveErr <- grpcServer.Serve(listener)
		}()

		logger.Info("listening", lager.Data{"socket": socketPath})
		close(ready)

		select {
		case err := <-serveErr:
			return err
		case <-signals:
			logger.Info("stopping")
			grpcServer.GracefulStop()
			return nil
		}
	})
}
