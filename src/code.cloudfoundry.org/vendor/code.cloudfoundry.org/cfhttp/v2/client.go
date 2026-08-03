// Package cfhttp provides defaults and helpers for building http clients.
// It serves to help maintain the same HTTP configuration across multiple
// CloudFoundry components.
package cfhttp

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

type config struct {
	requestTimeout      time.Duration
	dialTimeout         time.Duration
	tcpKeepAliveTimeout time.Duration
	idleConnTimeout     time.Duration
	disableKeepAlives   bool
	maxIdleConnsPerHost int
	tlsConfig           *tls.Config
}

// Option can be used to configure different parts of the HTTP client, including
// its internal transport or the connection dialer.
type Option func(*config)

// WithStreamingDefaults modifies the HTTP client with defaults that are more
// suitable for consuming server-sent events on persistent connections.
func WithStreamingDefaults() Option {
	return func(c *config) {
		c.tcpKeepAliveTimeout = 30 * time.Second
		c.disableKeepAlives = false
		c.requestTimeout = 0
	}
}

// WithRequestTimeout sets the total time limit for requests made by this Client.
//
// A setting of 0 means no timeout.
func WithRequestTimeout(t time.Duration) Option {
	return func(c *config) {
		c.requestTimeout = t
	}
}

// WithDialTimeout sets the time limit for connecting to the remote address. This
// includes DNS resolution and retries on multiple IP addresses.
//
// A setting of 0 means no timeout.
func WithDialTimeout(t time.Duration) Option {
	return func(c *config) {
		c.dialTimeout = t
	}
}

// WithTCPKeepAliveTimeout sets the keep-alive period for an active TCP
// connection.
//
// A setting of 0 disables TCP keep-alives.
func WithTCPKeepAliveTimeout(t time.Duration) Option {
	return func(c *config) {
		c.tcpKeepAliveTimeout = t
	}
}

// WithIdleConnTimeout sets the maximum amount of time a keep-alive
// connection can be idle before it closes itself.
//
// A setting of 0 means no timeout.
func WithIdleConnTimeout(t time.Duration) Option {
	return func(c *config) {
		c.idleConnTimeout = t
	}
}

// WithDisableKeepAlives disables keep-alive on every HTTP connection so that
// every connection is closed as soon as its request is done.
func WithDisableKeepAlives() Option {
	return func(c *config) {
		c.disableKeepAlives = true
	}
}

// WithMaxIdleConnsPerHost sets the maximum number of keep-alive connections that
// can be active at a time per remote host.
//
// A setting of 0 sets means the MaxIdleConnsPerHost is
// http.DefaultMaxIdleConnsPerHost (2 at the time of writing).
func WithMaxIdleConnsPerHost(max int) Option {
	return func(c *config) {
		c.maxIdleConnsPerHost = max
	}
}

// WithTLSConfig sets the TLS configuration on the HTTP client.
func WithTLSConfig(t *tls.Config) Option {
	return func(c *config) {
		c.tlsConfig = t
	}
}

// NewClient builds a HTTP client with suitable defaults.
// The Options can optionally set configuration options on the
// HTTP client, transport, or net dialer. Options are applied
// in the order that they are passed in, so it is possible for
// later Options previous ones.
func NewClient(options ...Option) *http.Client {
	cfg := config{
		dialTimeout:         5 * time.Second,
		tcpKeepAliveTimeout: 0,
		idleConnTimeout:     90 * time.Second,
	}
	for _, v := range options {
		v(&cfg)
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   cfg.dialTimeout,
				KeepAlive: cfg.tcpKeepAliveTimeout,
			}).DialContext,
			IdleConnTimeout:     cfg.idleConnTimeout,
			DisableKeepAlives:   cfg.disableKeepAlives,
			MaxIdleConnsPerHost: cfg.maxIdleConnsPerHost,
			TLSClientConfig:     cfg.tlsConfig,
		},
		Timeout: cfg.requestTimeout,
	}
}
