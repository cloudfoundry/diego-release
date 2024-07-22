package keychain

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/authn"
)

func NewHTTPClient(keychain authn.Keychain) *http.Client {
	return &http.Client{
		Transport: &roundTripper{
			keychain: keychain,
			inner:    http.DefaultTransport,
		},
	}
}

type roundTripper struct {
	keychain authn.Keychain
	inner    http.RoundTripper
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.keychain == nil {
		return rt.inner.RoundTrip(req)
	}

	authenticator, err := rt.keychain.Resolve(&urlResource{url: req.URL})
	if err != nil {
		return nil, err
	}

	if authenticator == authn.Anonymous {
		return rt.inner.RoundTrip(req)
	}

	conf, err := authenticator.Authorization()
	if err != nil {
		return nil, err
	}

	if conf.RegistryToken != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", conf.RegistryToken))
	} else {
		req.SetBasicAuth(conf.Username, conf.Password)
	}

	return rt.inner.RoundTrip(req)
}

type urlResource struct {
	url *url.URL
}

func (r *urlResource) RegistryStr() string {
	return r.url.Hostname()
}

func (r *urlResource) String() string {
	return r.RegistryStr()
}
