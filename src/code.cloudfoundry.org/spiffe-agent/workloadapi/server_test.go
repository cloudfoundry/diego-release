package workloadapi_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/spiffe-agent/cfattestor"
	"code.cloudfoundry.org/spiffe-agent/workloadapi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon_v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type fakeAttestor struct {
	mu     sync.Mutex
	gotPID int
	att    cfattestor.Attestation
	err    error
}

func (f *fakeAttestor) Attest(_ context.Context, pid int) (cfattestor.Attestation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotPID = pid
	return f.att, f.err
}

type fakeSigner struct {
	mu          sync.Mutex
	gotAudience string
	svid        string
	spiffeID    string
	err         error
}

func (f *fakeSigner) Sign(_ context.Context, _ cfattestor.Attestation, audience string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotAudience = audience
	return f.svid, f.spiffeID, f.err
}

type fakeBundleSource struct {
	mu      sync.Mutex
	bundles map[string][]byte
	err     error
}

func (f *fakeBundleSource) set(bundles map[string][]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles = bundles
}

func (f *fakeBundleSource) Bundles(_ context.Context) (map[string][]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bundles, f.err
}

var _ = Describe("WorkloadAPI Server", func() {
	var (
		attestor     *fakeAttestor
		signer       *fakeSigner
		bundleSource *fakeBundleSource
		sock         string
		process      ifrit.Process
		client       workload.SpiffeWorkloadAPIClient
		conn         *grpc.ClientConn
	)

	BeforeEach(func() {
		attestor = &fakeAttestor{att: cfattestor.Attestation{CertPEM: "canned"}}
		signer = &fakeSigner{svid: "jwt", spiffeID: "spiffe://example.org/workload/foo"}
		bundleSource = &fakeBundleSource{bundles: map[string][]byte{"spiffe://example.org": []byte("jwks-1")}}

		sock = filepath.Join(GinkgoT().TempDir(), "agent.sock")
		srv := workloadapi.NewServer(attestor, signer, bundleSource, 20*time.Millisecond)
		runner := workloadapi.NewRunner(sock, srv, lagertest.NewTestLogger("workloadapi"))
		process = ginkgomon_v2.Invoke(runner)

		var err error
		conn, err = grpc.NewClient("unix://"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
		Expect(err).NotTo(HaveOccurred())
		client = workload.NewSpiffeWorkloadAPIClient(conn)
	})

	AfterEach(func() {
		if conn != nil {
			conn.Close()
		}
		ginkgomon_v2.Kill(process)
	})

	withHeader := func() context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "workload.spiffe.io", "true")
	}

	It("issues a JWT-SVID from the attested PID", func() {
		resp, err := client.FetchJWTSVID(withHeader(), &workload.JWTSVIDRequest{Audience: []string{"foo"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Svids).To(HaveLen(1))
		Expect(resp.Svids[0].Svid).To(Equal("jwt"))
		Expect(resp.Svids[0].SpiffeId).To(Equal("spiffe://example.org/workload/foo"))
		Expect(signer.gotAudience).To(Equal("foo"))
		Expect(attestor.gotPID).To(Equal(os.Getpid()))
	})

	It("rejects requests missing the security header", func() {
		_, err := client.FetchJWTSVID(context.Background(), &workload.JWTSVIDRequest{Audience: []string{"foo"}})
		Expect(err).To(HaveOccurred())
	})

	It("rejects requests with no audience", func() {
		_, err := client.FetchJWTSVID(withHeader(), &workload.JWTSVIDRequest{Audience: nil})
		Expect(err).To(HaveOccurred())
	})

	It("streams the current JWT bundles to an authenticated client", func() {
		stream, err := client.FetchJWTBundles(withHeader(), &workload.JWTBundlesRequest{})
		Expect(err).NotTo(HaveOccurred())

		resp, err := stream.Recv()
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Bundles).To(HaveKeyWithValue("spiffe://example.org", []byte("jwks-1")))
	})

	It("rejects bundle requests missing the security header", func() {
		stream, err := client.FetchJWTBundles(context.Background(), &workload.JWTBundlesRequest{})
		Expect(err).NotTo(HaveOccurred())

		_, err = stream.Recv()
		Expect(err).To(HaveOccurred())
	})

	It("re-sends the JWT bundles when they rotate", func() {
		stream, err := client.FetchJWTBundles(withHeader(), &workload.JWTBundlesRequest{})
		Expect(err).NotTo(HaveOccurred())

		first, err := stream.Recv()
		Expect(err).NotTo(HaveOccurred())
		Expect(first.Bundles).To(HaveKeyWithValue("spiffe://example.org", []byte("jwks-1")))

		bundleSource.set(map[string][]byte{"spiffe://example.org": []byte("jwks-2")})

		second, err := stream.Recv()
		Expect(err).NotTo(HaveOccurred())
		Expect(second.Bundles).To(HaveKeyWithValue("spiffe://example.org", []byte("jwks-2")))
	})
})
