package uaa

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
)

type uaaTransport struct {
	base           http.RoundTripper
	LoggingEnabled bool
}

func (t *uaaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.logRequest(req)

	authHeader := req.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "basic") {
		req.Header.Add("X-CF-ENCODED-CREDENTIALS", "true")
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	t.logResponse(resp)

	return resp, err
}

func (t *uaaTransport) logRequest(req *http.Request) {
	if t.LoggingEnabled {
		bytes, _ := httputil.DumpRequest(req, false)
		fmt.Printf("%s", string(bytes))
	}
}

func (t *uaaTransport) logResponse(resp *http.Response) {
	if t.LoggingEnabled {
		bytes, _ := httputil.DumpResponse(resp, true)
		fmt.Printf("%s", string(bytes))
	}
}
