package uaa

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"errors"

	"golang.org/x/oauth2"
)

func (a *API) doJSON(method string, url *url.URL, body io.Reader, response interface{}, needsAuthentication bool) error {
	if strings.Contains(url.Path, "/Users/") || strings.Contains(url.Path, "/Groups/") && method == "PUT" {
		return a.doJSONWithHeaders(method, url, map[string]string{"If-Match": "*"}, body, response, needsAuthentication)
	}
	return a.doJSONWithHeaders(method, url, nil, body, response, needsAuthentication)
}

func (a *API) doJSONWithHeaders(method string, url *url.URL, headers map[string]string, body io.Reader, response interface{}, needsAuthentication bool) error {
	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	bytes, err := a.doAndRead(req, needsAuthentication)
	if err != nil {
		return err
	}

	if response != nil {
		if err := json.Unmarshal(bytes, response); err != nil {
			return parseError(err, url.String(), bytes)
		}
	}

	return nil
}

func (a *API) doAndRead(req *http.Request, needsAuthentication bool) ([]byte, error) {
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Identity-Zone-Id", a.zoneID)
	userAgent := a.userAgent
	if userAgent == "" {
		userAgent = "go-uaa"
	}
	req.Header.Set("User-Agent", userAgent)
	switch req.Method {
	case http.MethodPut, http.MethodPost, http.MethodPatch:
		req.Header.Add("Content-Type", "application/json")
	}
	a.ensureTimeout()
	var (
		resp *http.Response
		err  error
	)
	if !needsAuthentication && a.baseClient != nil {
		a.ensureTransport(a.baseClient.Transport)
		resp, err = a.baseClient.Do(req)
	} else {
		if a.Client == nil {
			return nil, errors.New("doAndRead: the Client cannot be nil")
		}
		a.ensureTransport(a.Client.Transport)
		resp, err = a.Client.Do(req)
	}

	if err != nil {
		if a.verbose {
			fmt.Printf("%v\n\n", err)
		}

		return nil, requestError(req.URL.String())
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if a.verbose {
			fmt.Printf("%v\n\n", err)
		}
		return nil, requestError(req.URL.String())
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(bytes) > 0 {
			return nil, requestErrorWithBody(req.URL.String(), bytes)
		}
		return nil, requestError(req.URL.String())
	}
	return bytes, nil
}

func (a *API) ensureTimeout() {
	if a.Client != nil && a.Client.Timeout == 0 {
		a.Client.Timeout = time.Second * 120
	}

	if a.baseClient != nil && a.baseClient.Timeout == 0 {
		a.baseClient.Timeout = time.Second * 120
	}
}

func (a *API) ensureTransport(c http.RoundTripper) {
	if c == nil {
		return
	}
	switch t := c.(type) {
	case *oauth2.Transport:
		b, ok := t.Base.(*http.Transport)
		if !ok {
			return
		}
		if b.TLSClientConfig == nil && !a.skipSSLValidation {
			return
		}
		if b.TLSClientConfig == nil {
			b.TLSClientConfig = &tls.Config{}
		}
		b.TLSClientConfig.InsecureSkipVerify = a.skipSSLValidation
	case *tokenTransport:
		a.ensureTransport(t.underlyingTransport)
	case *http.Transport:
		if t.TLSClientConfig == nil && !a.skipSSLValidation {
			return
		}
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}
		t.TLSClientConfig.InsecureSkipVerify = a.skipSSLValidation
	}
}
