package cfattestor

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type stubResolver struct {
	typ string
	err error
}

func (r stubResolver) ProcessType(ctx context.Context, handle string) (string, error) {
	return r.typ, r.err
}

var _ = Describe("Attestor.Attest", func() {
	const pid = 1000
	var procRoot string

	BeforeEach(func() {
		procRoot = GinkgoT().TempDir()
	})

	It("assembles a full Attestation with the resolved process type", func() {
		setupContainer(procRoot, pid, "handle-abc", Selectors{OrgID: "org-1", SpaceID: "space-2", AppID: "app-3"})

		a := New(stubResolver{typ: "web"})
		a.procRoot = procRoot

		att, err := a.Attest(context.Background(), pid)
		Expect(err).NotTo(HaveOccurred())
		Expect(att.Selectors).To(Equal(Selectors{OrgID: "org-1", SpaceID: "space-2", AppID: "app-3", ProcessType: "web"}))
		Expect(att.CertPEM).NotTo(BeEmpty())
		Expect(att.InstanceCert).NotTo(BeNil())
		Expect(att.InstanceKey).NotTo(BeNil())
	})

	It("rejects ssh sessions", func() {
		setupContainer(procRoot, pid, "handle-abc", Selectors{OrgID: "org-1", SpaceID: "space-2", AppID: "app-3"})
		writeStatus(procRoot, pid, "web", 999)
		writeStatus(procRoot, 999, "diego-sshd", 1)

		a := New(stubResolver{typ: "web"})
		a.procRoot = procRoot

		_, err := a.Attest(context.Background(), pid)
		Expect(err).To(MatchError(ContainSubstring("ssh sessions are not attestable")))
	})

	It("propagates resolver errors", func() {
		setupContainer(procRoot, pid, "handle-abc", Selectors{OrgID: "org-1", SpaceID: "space-2", AppID: "app-3"})

		a := New(stubResolver{err: fmt.Errorf("boom")})
		a.procRoot = procRoot

		_, err := a.Attest(context.Background(), pid)
		Expect(err).To(MatchError(ContainSubstring("boom")))
	})
})
