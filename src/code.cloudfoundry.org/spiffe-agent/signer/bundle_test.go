package signer

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BundleSource", func() {
	const (
		trustDomain = "example.org"
		// A minimal but well-formed JWKS, shaped like UAA's /token_keys body.
		jwks = `{"keys":[{"kty":"RSA","kid":"key-1","alg":"RS256","use":"sig","e":"AQAB","n":"nnnn"}]}`
	)

	It("fetches the UAA JWKS and keys it by the trust domain SPIFFE ID", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.Path).To(Equal("/token_keys"))
			_, err := w.Write([]byte(jwks))
			Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		bs := NewBundleSource(server.Client(), server.URL+"/token_keys", trustDomain)
		bundles, err := bs.Bundles(context.Background())

		Expect(err).NotTo(HaveOccurred())
		Expect(bundles).To(HaveLen(1))
		Expect(bundles).To(HaveKey("spiffe://example.org"))
		// The JWKS bytes are passed through verbatim for go-spiffe clients.
		Expect(string(bundles["spiffe://example.org"])).To(Equal(jwks))
	})

	It("returns an error on a non-2xx response", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		bs := NewBundleSource(server.Client(), server.URL+"/token_keys", trustDomain)
		_, err := bs.Bundles(context.Background())

		Expect(err).To(HaveOccurred())
	})

	It("returns an error when the body is not a JWKS with keys", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			defer GinkgoRecover()
			_, err := w.Write([]byte(`{"not":"a jwks"}`))
			Expect(err).NotTo(HaveOccurred())
		}))
		defer server.Close()

		bs := NewBundleSource(server.Client(), server.URL+"/token_keys", trustDomain)
		_, err := bs.Bundles(context.Background())

		Expect(err).To(HaveOccurred())
	})
})
