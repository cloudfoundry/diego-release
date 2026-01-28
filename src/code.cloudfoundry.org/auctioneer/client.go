package auctioneer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/tedsuo/rata"
)

//go:generate counterfeiter -o auctioneerfakes/fake_client.go . Client
type Client interface {
	RequestLRPAuctions(logger lager.Logger, traceID string, lrpStart []*LRPStartRequest) error
	RequestTaskAuctions(logger lager.Logger, traceID string, tasks []*TaskStartRequest) error
}

type auctioneerClient struct {
	httpClient         *http.Client
	insecureHTTPClient *http.Client
	url                string
	requireTLS         bool
	reqGen             *rata.RequestGenerator
}

func NewClient(auctioneerURL string, requestTimeout time.Duration) Client {
	return &auctioneerClient{
		httpClient: cfhttp.NewClient(
			cfhttp.WithRequestTimeout(requestTimeout),
		),
		url:    auctioneerURL,
		reqGen: rata.NewRequestGenerator(auctioneerURL, Routes),
	}
}

func NewSecureClient(auctioneerURL, caFile, certFile, keyFile string, requireTLS bool, requestTimeout time.Duration) (Client, error) {
	insecureHTTPClient := cfhttp.NewClient(
		cfhttp.WithRequestTimeout(requestTimeout),
	)

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certFile, keyFile),
	).Client(tlsconfig.WithAuthorityFromFile(caFile))
	if err != nil {
		return nil, err
	}

	httpClient := cfhttp.NewClient(
		cfhttp.WithRequestTimeout(requestTimeout),
		cfhttp.WithTLSConfig(tlsConfig),
	)

	return &auctioneerClient{
		httpClient:         httpClient,
		insecureHTTPClient: insecureHTTPClient,
		url:                auctioneerURL,
		requireTLS:         requireTLS,
		reqGen:             rata.NewRequestGenerator(auctioneerURL, Routes),
	}, nil
}

func (c *auctioneerClient) RequestLRPAuctions(logger lager.Logger, traceID string, lrpStarts []*LRPStartRequest) error {
	logger = logger.Session("request-lrp-auctions")

	payload, err := json.Marshal(lrpStarts)
	if err != nil {
		return err
	}

	resp, err := c.createRequest(logger, traceID, CreateLRPAuctionsRoute, rata.Params{}, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("http error: status code %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return nil
}

func (c *auctioneerClient) RequestTaskAuctions(logger lager.Logger, traceID string, tasks []*TaskStartRequest) error {
	logger = logger.Session("request-task-auctions")

	payload, err := json.Marshal(tasks)
	if err != nil {
		return err
	}

	resp, err := c.createRequest(logger, traceID, CreateTaskAuctionsRoute, rata.Params{}, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("http error: status code %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return nil
}

func (c *auctioneerClient) createRequest(logger lager.Logger, traceID string, route string, params rata.Params, payload []byte) (*http.Response, error) {
	resp, err := c.doRequest(c.httpClient, traceID, false, route, params, payload)
	if err != nil {
		// Fall back to HTTP and try again if we do not require TLS
		if !c.requireTLS && c.insecureHTTPClient != nil {
			logger.Error("retrying-on-http", err)
			return c.doRequest(c.insecureHTTPClient, traceID, true, route, params, payload)
		}
	}
	return resp, err
}

func (c *auctioneerClient) doRequest(client *http.Client, traceID string, useHttp bool, route string, params rata.Params, payload []byte) (*http.Response, error) {
	req, err := c.reqGen.CreateRequest(route, params, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vcap-Request-Id", traceID)

	if useHttp {
		req.URL.Scheme = "http"
	}
	return client.Do(req)
}
