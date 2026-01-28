package healthcheck

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

type HealthCheckError struct {
	Code    int
	Message string
}

func (e HealthCheckError) Error() string {
	return e.Message
}

type HealthCheck struct {
	network string
	uri     string
	port    string
	timeout time.Duration
}

func NewHealthCheck(network, uri, port string, timeout time.Duration) HealthCheck {
	return HealthCheck{network, uri, port, timeout}
}

func (h *HealthCheck) CheckInterfaces(interfaces []net.Interface) error {
	healthcheck := h.HTTPHealthCheck
	if len(h.uri) == 0 {
		healthcheck = h.PortHealthCheck
	}

	for _, intf := range interfaces {
		addrs, err := intf.Addrs()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed getting addresses for interface %v\n", intf)
			continue
		}

		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				err := healthcheck(ipnet.IP.String())
				return err
			}
		}
	}

	return HealthCheckError{Code: 3, Message: "failure to find suitable interface"}
}

func (h *HealthCheck) PortHealthCheck(ip string) error {
	addr := ip + ":" + h.port
	conn, err := net.DialTimeout(h.network, addr, h.timeout)
	if err == nil {
		// #nosec G104-  don't check this error because we want to return OK if we were able to connect, closing is not an issue
		conn.Close()
		return nil
	}

	if err, ok := err.(net.Error); ok && err.Timeout() {
		msg := fmt.Sprintf("failed to make TCP connection to %s: timed out after %.2f seconds", addr, h.timeout.Seconds())
		return HealthCheckError{Code: 64, Message: msg}
	}

	return HealthCheckError{Code: 4, Message: fmt.Sprintf("failed to make TCP connection to %s: %s", addr, err.Error())}
}

func (h *HealthCheck) HTTPHealthCheck(ip string) error {
	addr := fmt.Sprintf("http://%s:%s%s", ip, h.port, h.uri)
	client := http.Client{
		Timeout: h.timeout,
	}
	now := time.Now()
	req, err := http.NewRequest("GET", addr, nil)
	if err != nil {
		errMsg := fmt.Sprintf(
			"failed to create an HTTP request to '%s' on port %s",
			h.uri,
			h.port,
		)
		return HealthCheckError{Code: 6, Message: errMsg}
	}

	req.Header.Set("User-Agent", "diego-healthcheck")
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := client.Do(req)
	dur := time.Since(now)
	if err == nil {
		defer resp.Body.Close()

		// We need to read the request body to prevent extraneous errors in the server.
		// We could make a HEAD request but there are concerns about servers that may
		// not implement the RFC correctly.
		// #nosec G104 - as such, ignore errors because we don't care about the body as long as its not there anymore
		io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		errMsg := fmt.Sprintf(
			"failed to make HTTP request to '%s' on port %s: received status code %d in %dms",
			h.uri,
			h.port,
			resp.StatusCode,
			dur.Nanoseconds()/time.Millisecond.Nanoseconds(),
		)
		return HealthCheckError{Code: 6, Message: errMsg}
	}

	if err, ok := err.(net.Error); ok && err.Timeout() {
		errMsg := fmt.Sprintf(
			"failed to make HTTP request to '%s' on port %s: timed out after %.2f seconds",
			h.uri,
			h.port,
			h.timeout.Seconds(),
		)
		return HealthCheckError{Code: 65, Message: errMsg}
	}

	errMsg := fmt.Sprintf(
		"failed to make HTTP request to '%s' on port %s: connection refused",
		h.uri,
		h.port,
	)
	return HealthCheckError{Code: 5, Message: errMsg}
}
