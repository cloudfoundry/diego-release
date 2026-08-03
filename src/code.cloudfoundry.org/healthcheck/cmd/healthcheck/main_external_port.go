//go:build external
// +build external

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/healthcheck"
)

type PortMapping struct {
	Internal int `json:"internal"`
	External int `json:"external"`
}

func newHealthCheck(
	network, uri, port string,
	timeout time.Duration,
) healthcheck.HealthCheck {
	jsonPortMappings := os.Getenv("CF_INSTANCE_PORTS")
	var portMappings []PortMapping
	json.Unmarshal([]byte(jsonPortMappings), &portMappings)
	for _, mapping := range portMappings {
		if strconv.Itoa(mapping.Internal) == port {
			port = strconv.Itoa(mapping.External)
		}
	}
	return healthcheck.NewHealthCheck(network, uri, port, timeout)
}
