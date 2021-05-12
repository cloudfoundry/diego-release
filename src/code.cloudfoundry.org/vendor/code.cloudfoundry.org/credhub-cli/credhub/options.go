package credhub

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/url"
	"runtime"
	"time"

	"code.cloudfoundry.org/credhub-cli/credhub/auth"
)

// Option can be provided to New() to specify additional parameters for
// connecting to the CredHub server
type Option func(*CredHub) error

// Auth specifies the authentication Strategy. See the auth package
// for a full list of supported strategies.
func Auth(method auth.Builder) Option {
	return func(c *CredHub) error {
		c.authBuilder = method
		return nil
	}
}

// AuthURL specifies the authentication server for the OAuth strategy.
// If AuthURL provided, the AuthURL will be fetched from /info.
func AuthURL(authURL string) Option {
	return func(c *CredHub) error {
		if authURL != "" {
			var err error
			c.authURL, err = url.Parse(authURL)
			return err
		}
		return nil
	}
}

// CaCerts specifies the root certificates for HTTPS connections with the CredHub server.
//
// If the OAuthStrategy is used for Auth, the root certificates will also be used for HTTPS
// connections with the OAuth server.
func CaCerts(certs ...string) Option {
	return func(c *CredHub) error {
		// TODO: remove else block once x509.SystemCertPool is supported on Windows
		// see: https://github.com/golang/go/issues/16736
		var pool *x509.CertPool
		if runtime.GOOS != "windows" {
			var err error
			pool, err = x509.SystemCertPool()
			if err != nil {
				return err
			}
		} else {
			pool = x509.NewCertPool()
		}
		c.caCerts = pool

		for _, cert := range certs {
			ok := c.caCerts.AppendCertsFromPEM([]byte(cert))
			if !ok {
				return errors.New("provided ca certs are invalid")
			}
		}

		return nil
	}
}

// SkipTLSValidation will skip root certificate verification for HTTPS. Not recommended!
func SkipTLSValidation(skipTLSvalidation bool) Option {
	return func(c *CredHub) error {
		c.insecureSkipVerify = skipTLSvalidation
		return nil
	}
}

// ClientCert will use a certificate for authentication
func ClientCert(certificate, key string) Option {
	return func(c *CredHub) error {
		cert, err := tls.LoadX509KeyPair(certificate, key)
		if err != nil {
			return err
		}
		c.clientCertificate = &cert

		return nil
	}
}

//SetHttpTimeout will set the timeout for the CredHub client
func SetHttpTimeout(timeout *time.Duration) Option {
	return func(c *CredHub) error {
		c.httpTimeout = timeout
		return nil
	}
}

func ServerVersion(version string) Option {
	return func(c *CredHub) error {
		c.cachedServerVersion = version
		return nil
	}
}
