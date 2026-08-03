package auctioneer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/tedsuo/rata"
)

const ClientRetryCount = 3

const (
	CreateTaskAuctionsRoute = "CreateTaskAuctions"
	CreateLRPAuctionsRoute  = "CreateLRPAuctions"
)

var routes = rata.Routes{
	{Path: "/v1/tasks", Method: "POST", Name: CreateTaskAuctionsRoute},
	{Path: "/v1/lrps", Method: "POST", Name: CreateLRPAuctionsRoute},
}

type auctioneerClient struct {
	httpClient         *http.Client
	insecureHTTPClient *http.Client
	url                string
	requireTLS         bool
	reqGen             *rata.RequestGenerator
}

func NewClient(auctioneerURL string, requestTimeout time.Duration) models.AuctioneerClient {
	return &auctioneerClient{
		httpClient: cfhttp.NewClient(
			cfhttp.WithRequestTimeout(requestTimeout),
		),
		url:    auctioneerURL,
		reqGen: rata.NewRequestGenerator(auctioneerURL, routes),
	}
}

func NewSecureClient(auctioneerURL, caFile, certFile, keyFile string, requireTLS bool, requestTimeout time.Duration) (models.AuctioneerClient, error) {
	insecureHTTPClient := cfhttp.NewClient(
		cfhttp.WithRequestTimeout(requestTimeout),
	)

	tlsConf, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certFile, keyFile),
	).Client(tlsconfig.WithAuthorityFromFile(caFile))
	if err != nil {
		return nil, err
	}

	httpClient := cfhttp.NewClient(
		cfhttp.WithRequestTimeout(requestTimeout),
		cfhttp.WithTLSConfig(tlsConf),
	)

	return &auctioneerClient{
		httpClient:         httpClient,
		insecureHTTPClient: insecureHTTPClient,
		url:                auctioneerURL,
		requireTLS:         requireTLS,
		reqGen:             rata.NewRequestGenerator(auctioneerURL, routes),
	}, nil
}

func (c *auctioneerClient) RequestLRPAuctions(logger lager.Logger, traceID string, lrpStarts []*models.LRPStartRequest) error {
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

func (c *auctioneerClient) RequestTaskAuctions(logger lager.Logger, traceID string, tasks []*models.TaskStartRequest) error {
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
	resp, err := c.doRequest(logger, c.httpClient, traceID, false, route, params, payload)
	if err != nil {
		if !c.requireTLS && c.insecureHTTPClient != nil {
			logger.Error("retrying-on-http", err)
			return c.doRequest(logger, c.insecureHTTPClient, traceID, true, route, params, payload)
		}
	}
	return resp, err
}

func (c *auctioneerClient) doRequest(logger lager.Logger, client *http.Client, traceID string, useHttp bool, route string, params rata.Params, payload []byte) (*http.Response, error) {
	logger = logger.Session("do-request")
	var resp *http.Response
	var err error
	for attempts := 0; attempts < ClientRetryCount; attempts++ {
		logger.Debug("creating-request", lager.Data{"attempt": attempts + 1, "request_name": route})
		var req *http.Request
		req, err = c.reqGen.CreateRequest(route, params, bytes.NewBuffer(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Vcap-Request-Id", traceID)

		if useHttp {
			req.URL.Scheme = "http"
		}

		logger.Debug("doing-request", lager.Data{"attempt": attempts + 1, "request_path": req.URL.Path})
		resp, err = client.Do(req)
		if err != nil {
			logger.Error("failed-doing-request", err)
			time.Sleep(500 * time.Millisecond)
		} else {
			logger.Debug("complete", lager.Data{"request_path": req.URL.Path})
			break
		}
	}
	return resp, err
}
