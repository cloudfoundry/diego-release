// Package gcrhelper provides credential support for Google Container Registry
// (gcr.io) and Artifact Registry (*.pkg.dev). GCR is the legacy service; Google
// now routes gcr.io requests through Artifact Registry. Both URL patterns are
// supported. Authentication uses the GCE instance metadata server to obtain a
// short-lived OAuth2 token from the VM's attached service account.
package gcrhelper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

const (
	GCE_METADATA_TOKEN_URL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
	GCR_USERNAME           = "oauth2accesstoken"
	GCR_REPO_REGEX         = `([a-zA-Z0-9-]+\.)?gcr\.io|[a-zA-Z0-9-]+\.pkg\.dev`

	// metadataTimeout is intentionally short: the GCE metadata server is a
	// link-local address (169.254.169.254) that responds in <1ms on GCE and
	// typically fails immediately off-GCE due to no route. Since this is only
	// attempted once per process lifetime (notOnGCE is set on first failure),
	// 1s is a safe upper bound.
	metadataTimeout = 1 * time.Second
)

var gcrRepoRegex = regexp.MustCompile(GCR_REPO_REGEX)

//go:generate counterfeiter -o fakes/fake_gcrhelper.go . GCRHelper
type GCRHelper interface {
	IsGCRRepo(registryURL string) (bool, error)
	GetGCRCredentials() (string, string, error)
}

// TokenFetcher retrieves an OAuth2 access token for use with GCR/Artifact Registry.
// The default implementation calls the GCE instance metadata server, which
// automatically uses the VM's attached service account without any stored credentials.
type TokenFetcher func() (string, error)

func DefaultTokenFetcher() (string, error) {
	req, err := http.NewRequest("GET", GCE_METADATA_TOKEN_URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")

	client := &http.Client{Timeout: metadataTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to reach GCE metadata server: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GCE metadata server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", fmt.Errorf("failed to parse GCE metadata token response: %s", err.Error())
	}
	if tokenResponse.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in GCE metadata response")
	}

	return tokenResponse.AccessToken, nil
}

type gcrHelper struct {
	tokenFetcher TokenFetcher
	mu           sync.Mutex
	notOnGCE     bool
}

func NewGCRHelper() GCRHelper {
	return &gcrHelper{
		tokenFetcher: DefaultTokenFetcher,
	}
}

func NewGCRHelperWithTokenFetcher(fetcher TokenFetcher) GCRHelper {
	if fetcher == nil {
		fetcher = DefaultTokenFetcher
	}
	return &gcrHelper{
		tokenFetcher: fetcher,
	}
}

func (h *gcrHelper) IsGCRRepo(registryURL string) (bool, error) {
	return gcrRepoRegex.MatchString(registryURL), nil
}

func (h *gcrHelper) GetGCRCredentials() (string, string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.notOnGCE {
		return "", "", nil
	}

	token, err := h.tokenFetcher()
	if err != nil {
		// Not running on GCE or metadata unavailable — fall back to unauthenticated
		// so that truly public GCR/Artifact Registry images still pull successfully.
		// We remember this for the lifetime of the process to avoid a dial attempt
		// on every subsequent container start.
		h.notOnGCE = true
		return "", "", nil
	}
	return GCR_USERNAME, token, nil
}
