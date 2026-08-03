//go:build !external
// +build !external

package main

import (
	"time"

	"code.cloudfoundry.org/healthcheck"
)

func newHealthCheck(
	network, uri, port string,
	timeout time.Duration,
) healthcheck.HealthCheck {
	return healthcheck.NewHealthCheck(network, uri, port, timeout)
}
