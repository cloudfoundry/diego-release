package workloadapi

import (
	"bytes"
	"context"
	"net"
	"os"
	"time"

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

// BundleSource provides the current JWT trust bundles (JWKS documents) keyed by
// trust domain SPIFFE ID, for the FetchJWTBundles RPC.
type BundleSource interface {
	Bundles(ctx context.Context) (map[string][]byte, error)
}

// Server implements the SPIFFE Workload API, issuing JWT-SVIDs to the local
// process authenticated via SO_PEERCRED.
type Server struct {
	workload.UnimplementedSpiffeWorkloadAPIServer
	attestor        Attestor
	signer          Signer
	bundles         BundleSource
	refreshInterval time.Duration
}

// NewServer wires an attestor, signer, and JWT bundle source into a Workload API
// server. refreshInterval controls how often FetchJWTBundles polls the bundle
// source for rotations.
func NewServer(attestor Attestor, signer Signer, bundles BundleSource, refreshInterval time.Duration) *Server {
	return &Server{
		attestor:        attestor,
		signer:          signer,
		bundles:         bundles,
		refreshInterval: refreshInterval,
	}
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

// FetchJWTBundles streams the JWT trust bundles (JWKS documents), keyed by trust
// domain SPIFFE ID. It sends the current bundles immediately, then re-sends
// whenever they change, polling the bundle source at the configured refresh
// interval to pick up signing-key rotation. The stream stays open until the
// client disconnects. Bundles are public, so no peer attestation is performed;
// only the SPIFFE security header is required.
func (s *Server) FetchJWTBundles(_ *workload.JWTBundlesRequest, stream workload.SpiffeWorkloadAPI_FetchJWTBundlesServer) error {
	ctx := stream.Context()
	if err := checkSecurityHeader(ctx); err != nil {
		return err
	}

	last, err := s.bundles.Bundles(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "fetch bundles: %v", err)
	}
	if err := stream.Send(&workload.JWTBundlesResponse{Bundles: last}); err != nil {
		return err
	}

	ticker := time.NewTicker(s.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, err := s.bundles.Bundles(ctx)
			if err != nil {
				// Transient fetch failure: keep streaming the last good bundle
				// rather than tearing down the client's stream.
				continue
			}
			if sameBundles(current, last) {
				continue
			}
			last = current
			if err := stream.Send(&workload.JWTBundlesResponse{Bundles: current}); err != nil {
				return err
			}
		}
	}
}

// sameBundles reports whether two bundle maps have identical keys and bytes.
func sameBundles(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !bytes.Equal(av, bv) {
			return false
		}
	}
	return true
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
