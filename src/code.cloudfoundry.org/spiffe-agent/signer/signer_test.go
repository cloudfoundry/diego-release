package signer

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/spiffe-agent/cfattestor"
	"code.cloudfoundry.org/spiffe-agent/internal/testsupport"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Signer", func() {
	const (
		trustDomain  = "example.org"
		audience     = "https://uaa.example.org/oauth/token"
		clientID     = "spiffe-agent"
		clientSecret = "s3cr3t"
		// GOLDEN VECTOR: must byte-match Plan A (UAA Java) for these selectors.
		goldenSpiffeID = "spiffe://example.org/cf/org/org-1/space/space-2/app/app-3/process/web"
	)

	var (
		att cfattestor.Attestation
		pub *rsa.PublicKey
	)

	BeforeEach(func() {
		sel := cfattestor.Selectors{OrgID: "org-1", SpaceID: "space-2", AppID: "app-3", ProcessType: "web"}
		certPEM, _, key := testsupport.MintInstanceCert("handle-1", sel)
		rsaKey, ok := key.(*rsa.PrivateKey)
		Expect(ok).To(BeTrue(), "MintInstanceCert must return an RSA signer")
		pub = &rsaKey.PublicKey
		att = cfattestor.Attestation{Selectors: sel, CertPEM: certPEM, InstanceKey: key}
	})

	It("signs a JWT-SVID with a proof-of-possession that matches the golden canonical message", func() {
		var body map[string]string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/jwt-svid/sign"))
			u, p, ok := r.BasicAuth()
			Expect(ok).To(BeTrue())
			Expect(u).To(Equal(clientID))
			Expect(p).To(Equal(clientSecret))

			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			Expect(body).To(HaveKey("instance_certificate"))
			Expect(body).To(HaveKey("process_type"))
			Expect(body).To(HaveKey("audience"))
			Expect(body).To(HaveKey("timestamp"))
			Expect(body).To(HaveKey("pop_signature"))
			Expect(body["instance_certificate"]).To(Equal(att.CertPEM))
			Expect(body["process_type"]).To(Equal("web"))
			Expect(body["audience"]).To(Equal(audience))

			// GOLDEN: canonical PoP message = spiffeID\naudience\ntimestamp.
			message := goldenSpiffeID + "\n" + audience + "\n" + body["timestamp"]
			sig, err := base64.StdEncoding.DecodeString(body["pop_signature"])
			Expect(err).NotTo(HaveOccurred())
			sum := sha256.Sum256([]byte(message))
			Expect(rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig)).To(Succeed())

			Expect(json.NewEncoder(w).Encode(signResponse{
				Svid:      "jwt",
				SpiffeID:  goldenSpiffeID,
				ExpiresAt: "2026-06-29T00:00:00Z",
			})).To(Succeed())
		}))
		defer server.Close()

		s := New(server.Client(), server.URL, clientID, clientSecret, trustDomain)
		svid, spiffeID, err := s.Sign(context.Background(), att, audience)

		Expect(err).NotTo(HaveOccurred())
		Expect(svid).To(Equal("jwt"))
		Expect(spiffeID).To(Equal(goldenSpiffeID))
		// The signed message used the same timestamp sent in the request body.
		Expect(body["timestamp"]).NotTo(BeEmpty())
	})

	It("returns an error on a non-2xx response", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		s := New(server.Client(), server.URL, clientID, clientSecret, trustDomain)
		_, _, err := s.Sign(context.Background(), att, audience)

		Expect(err).To(HaveOccurred())
	})
})
