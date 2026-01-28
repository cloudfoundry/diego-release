package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"code.cloudfoundry.org/healthcheck"
)

var network = flag.String(
	"network",
	"tcp",
	"network type to dial with (e.g. unix, tcp)",
)

var uri = flag.String(
	"uri",
	"",
	"uri to healthcheck",
)

var port = flag.String(
	"port",
	"8080",
	"port to healthcheck",
)

var timeout = flag.Duration(
	"timeout",
	1*time.Second,
	"dial timeout",
)

var startupInterval = flag.Duration(
	"startup-interval",
	0,
	"if set, starts the healthcheck in startup mode, i.e. do not exit until the healthcheck passes. runs checks every startup-interval",
)

var startupTimeout = flag.Duration(
	"startup-timeout",
	60*time.Second,
	"Only relevant if healthcheck is running in startup mode. When the timeout is set to a non-zero value, the healthcheck will return non-zero with any errors if this timeout is hit without the healthcheck passing",
)

var livenessInterval = flag.Duration(
	"liveness-interval",
	0,
	"if set, starts the healthcheck in liveness mode, i.e. the app is alive and hasn't crashed, do not exit until the healthcheck fails. runs checks every liveness-interval",
)

var readinessInterval = flag.Duration(
	"readiness-interval",
	0,
	"if set, starts the healthcheck in readiness mode, i.e. the app is ready to serve traffic, do not exit until the healthcheck fails because the target isn't serving traffic or another process doesn't exist. runs checks every readiness-interval",
)

var untilReadyInterval = flag.Duration(
	"until-ready-interval",
	0,
	"if set, starts the healthcheck in until-ready mode, i.e. do not exit until the healthcheck passes and the app is ready to serve traffic. runs checks every until-ready-interval",
)

func main() {
	flag.Parse()

	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get interfaces: %s\n", err)
		os.Exit(1)
		return
	}

	h := newHealthCheck(*network, *uri, *port, *timeout)

	var timeoutTimerCh <-chan time.Time
	if duration := *startupTimeout; duration > 0 {
		timeoutTimerCh = time.NewTimer(duration).C
	}

	if startupInterval != nil && *startupInterval > 0 {
		ticker := time.NewTicker(*startupInterval)
		defer ticker.Stop()
		errCh := make(chan error)

		for attempt := 1; ; attempt++ {
			go func() {
				err = h.CheckInterfaces(interfaces)
				errCh <- err
			}()

			select {
			case err = <-errCh:
				if err == nil {
					os.Exit(0)
				}
			case <-timeoutTimerCh:
				fmt.Fprintf(os.Stderr, "Timed out after %s (%d attempts) waiting for startup check to succeed: ", *startupTimeout, attempt)
				failHealthCheck(err)
			}

			select {
			case <-ticker.C:
			case <-timeoutTimerCh:
				fmt.Fprintf(os.Stderr, "Timed out after %s (%d attempts) waiting for startup check to succeed: ", *startupTimeout, attempt)
				failHealthCheck(err)
			}
		}
	}

	if livenessInterval != nil && *livenessInterval > 0 {
		for {
			err = h.CheckInterfaces(interfaces)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Liveness check unsuccessful: ")
				failHealthCheck(err)
			}
			time.Sleep(*livenessInterval)
		}
	}

	if readinessInterval != nil && *readinessInterval > 0 {
		for {
			err = h.CheckInterfaces(interfaces)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Readiness check unsuccessful: ")
				failHealthCheck(err)
			}
			time.Sleep(*readinessInterval)
		}
	}

	if untilReadyInterval != nil && *untilReadyInterval > 0 {
		for {
			err = h.CheckInterfaces(interfaces)
			if err == nil {
				os.Exit(0)
			}
			time.Sleep(*untilReadyInterval)
		}
	}

	err = h.CheckInterfaces(interfaces)
	if err == nil {
		os.Exit(0)
	}

	failHealthCheck(err)
}

func failHealthCheck(err error) {
	if err, ok := err.(healthcheck.HealthCheckError); ok {
		fmt.Fprintf(os.Stderr, "%s\n", err.Message)
		os.Exit(err.Code)
	}

	fmt.Fprintf(os.Stderr, "Unknown error encountered in healthcheck: %s\n", err.Error())
	os.Exit(127)
}
