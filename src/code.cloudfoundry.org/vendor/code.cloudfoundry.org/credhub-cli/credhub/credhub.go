/*
Package credhub is a client library for interacting with a CredHub server.

More information on CredHub can be found at https://code.cloudfoundry.org/credhub

Server HTTP API specification can be found at https://docs.cloudfoundry.org/api/credhub/
*/
package credhub

import (
	"net/http"
	"net/url"
	"time"

	"crypto/tls"
	"crypto/x509"

	"code.cloudfoundry.org/credhub-cli/credhub/auth"
)

// CredHub client to access CredHub APIs.
//
// Use New() to construct a new CredHub object, which can then interact with the CredHub API.
type CredHub struct {
	// ApiURL is the host and port of the CredHub server to target
	// Example: https://credhub.example.com:8844
	ApiURL string

	// Auth provides an authentication Strategy for authenticated requests to the CredHub server
	// Can be type asserted to a specific Strategy type to get additional functionality and information.
	// eg. auth.OAuthStrategy provides Logout(), Refresh(), AccessToken() and RefreshToken()
	Auth auth.Strategy

	baseURL       *url.URL
	defaultClient *http.Client

	// Trusted CA certificates in PEM format for making TLS connections to CredHub and auth servers
	caCerts *x509.CertPool

	// client certificates
	clientCertificate *tls.Certificate

	// Skip certificate verification of TLS connections to CredHub and auth servers. Not recommended!
	insecureSkipVerify bool

	authBuilder auth.Builder
	authURL     *url.URL

	// Version of the server to make API requests against. Some methods will hit alternate endpoints based on this value
	cachedServerVersion string

	// Timeout for http client
	httpTimeout *time.Duration
}
