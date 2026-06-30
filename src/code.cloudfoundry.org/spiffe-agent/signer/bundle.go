package signer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BundleSource fetches the JWT trust bundle (a JWKS document) from UAA's
// token_keys endpoint and publishes it keyed by the trust domain's SPIFFE ID,
// as required by the SPIFFE Workload API FetchJWTBundles RPC. The JWKS bytes are
// passed through verbatim so go-spiffe clients parse exactly what UAA publishes.
type BundleSource struct {
	httpClient  *http.Client
	url         string
	trustDomain string
}

// NewBundleSource returns a BundleSource that GETs url (UAA's token_keys
// endpoint) and publishes the JWKS under spiffe://<trustDomain>.
func NewBundleSource(c *http.Client, url, trustDomain string) *BundleSource {
	return &BundleSource{httpClient: c, url: url, trustDomain: trustDomain}
}

// Bundles fetches the current JWKS and returns it keyed by the trust domain
// SPIFFE ID. It fails fast when the response is not a non-empty JWKS so a
// misconfigured endpoint surfaces as an error rather than an unparsable bundle.
func (b *BundleSource) Bundles(ctx context.Context) (map[string][]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token_keys: status %d: %s", resp.StatusCode, body)
	}

	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("token_keys: invalid JWKS: %w", err)
	}
	if len(jwks.Keys) == 0 {
		return nil, fmt.Errorf("token_keys: JWKS contains no keys")
	}

	return map[string][]byte{"spiffe://" + b.trustDomain: body}, nil
}
