package workloadapi_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"

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

var _ = Describe("WorkloadAPI Server", func() {
	var (
		attestor *fakeAttestor
		signer   *fakeSigner
		sock     string
		process  ifrit.Process
		client   workload.SpiffeWorkloadAPIClient
		conn     *grpc.ClientConn
	)

	BeforeEach(func() {
		attestor = &fakeAttestor{att: cfattestor.Attestation{CertPEM: "canned"}}
		signer = &fakeSigner{svid: "jwt", spiffeID: "spiffe://example.org/workload/foo"}

		sock = filepath.Join(GinkgoT().TempDir(), "agent.sock")
		srv := workloadapi.NewServer(attestor, signer)
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
})
