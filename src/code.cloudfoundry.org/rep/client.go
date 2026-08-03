package rep

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/tedsuo/rata"
)

// capture the behavior described in the comment of this story
// https://www.pivotaltracker.com/story/show/130664747/comments/152863773
type TLSConfig struct {
	RequireTLS                    bool
	CertFile, KeyFile, CaCertFile string
	ClientCacheSize               int // the tls client cache size, 0 means use golang default value
}

// return true if all the certs files are set in the struct, i.e. not ""
func (config *TLSConfig) hasCreds() bool {
	return config.CaCertFile != "" &&
		config.KeyFile != "" &&
		config.CertFile != ""
}

// pick either the old address or the new rep_url depending on the announced
// addresses and the tls config
func (config *TLSConfig) pickURL(address, repURL string) (string, error) {
	secure := false
	if repURL != "" {
		url, err := url.Parse(repURL)
		if err != nil {
			return "", err
		}

		if url.Scheme == "https" {
			secure = true
		}
	}

	if !config.RequireTLS && !config.hasCreds() {
		// cannot use tls
		if secure {
			return "", errors.New("https scheme not supported since certificates aren't provided")
		}
		// prefer repURL
		if repURL != "" {
			return repURL, nil
		}
		return address, nil
	} else if !config.RequireTLS {
		// prefer tls but don't require it
		if repURL != "" {
			return repURL, nil
		}
		return address, nil
	} else {
		// must use tls
		if !secure {
			return "", errors.New("https scheme is required but none of the addresses support it")
		}
		return repURL, nil
	}
}

func (tlsConfig *TLSConfig) modifyTransport(client *http.Client) error {
	if !tlsConfig.hasCreds() {
		return nil
	}

	if transport, ok := client.Transport.(*http.Transport); ok {
		config, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(tlsConfig.CertFile, tlsConfig.KeyFile),
		).Client(tlsconfig.WithAuthorityFromFile(tlsConfig.CaCertFile))
		if err != nil {
			return err
		}

		config.ClientSessionCache = tls.NewLRUClientSessionCache(tlsConfig.ClientCacheSize)

		transport.TLSClientConfig = config
	}
	return nil
}

//go:generate counterfeiter -o repfakes/fake_sim_client.go code.cloudfoundry.org/bbs/models RepSimClient

type clientFactory struct {
	httpClient  *http.Client
	stateClient *http.Client
	tlsConfig   *TLSConfig
}

func NewClientFactory(httpClient, stateClient *http.Client, tlsConfig *TLSConfig) (models.RepClientFactory, error) {
	if tlsConfig == nil {
		// zero values tls config
		tlsConfig = &TLSConfig{}
	}

	if err := tlsConfig.modifyTransport(httpClient); err != nil {
		return nil, err
	}

	if err := tlsConfig.modifyTransport(stateClient); err != nil {
		return nil, err
	}

	return &clientFactory{
		httpClient:  httpClient,
		stateClient: stateClient,
		tlsConfig:   tlsConfig,
	}, nil
}

func (factory *clientFactory) CreateClient(address, url, traceID string) (models.RepClient, error) {
	urlToUse, err := factory.tlsConfig.pickURL(address, url)
	if err != nil {
		return nil, err
	}

	return newClient(factory.httpClient, factory.stateClient, urlToUse, traceID), nil
}

type client struct {
	client           *http.Client
	stateClient      *http.Client
	address          string
	requestGenerator *rata.RequestGenerator
}

func newClient(httpClient, stateClient *http.Client, address string, traceID string) models.RepClient {
	requestGenerator := rata.NewRequestGenerator(address, Routes)
	if traceID != "" {
		requestGenerator.Header.Add(trace.RequestIdHeader, traceID)
	}
	return &client{
		client:           httpClient,
		stateClient:      stateClient,
		address:          address,
		requestGenerator: requestGenerator,
	}
}

func (c *client) SetStateClient(stateClient *http.Client) {
	c.stateClient = stateClient
}

func (c *client) StateClientTimeout() time.Duration {
	return c.stateClient.Timeout
}

func (c *client) State(logger lager.Logger) (models.CellState, error) {
	req, err := c.requestGenerator.CreateRequest(StateRoute, nil, nil)
	if err != nil {
		return models.CellState{}, err
	}

	resp, err := c.stateClient.Do(req)
	if err != nil {
		return models.CellState{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.CellState{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var state models.CellState
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.CellState{}, err
	}
	err = json.Unmarshal(bs, &state)
	if err != nil {
		return models.CellState{}, err
	}

	return state, nil
}

func (c *client) Perform(logger lager.Logger, work models.Work) (models.Work, error) {
	body, err := json.Marshal(work)
	if err != nil {
		return models.Work{}, err
	}

	req, err := c.requestGenerator.CreateRequest(PerformRoute, nil, bytes.NewReader(body))
	if err != nil {
		return models.Work{}, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return models.Work{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.Work{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var failedWork models.Work
	err = json.NewDecoder(resp.Body).Decode(&failedWork)
	if err != nil {
		return models.Work{}, err
	}

	return failedWork, nil
}

func (c *client) Reset() error {
	req, err := c.requestGenerator.CreateRequest(SimResetRoute, nil, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *client) UpdateLRPInstance(
	logger lager.Logger,
	update models.LRPUpdate,
) error {
	start := time.Now()
	loggerCopy := logger
	logger = logger.Session("update-lrp", lager.Data{"process-guid": update.ProcessGuid,
		"index":         update.Index,
		"domain":        update.Domain,
		"instance-guid": update.InstanceGUID,
	})
	logger.Info("starting")

	params := rata.Params{
		"process_guid":  update.ProcessGuid,
		"instance_guid": update.InstanceGUID,
	}
	body, err := json.Marshal(update)
	if err != nil {
		logger.Error("marshal-failed", err)
		return err
	}
	req, err := c.requestGenerator.CreateRequest(UpdateLRPInstanceRoute, params, bytes.NewReader(body))
	if err != nil {
		logger.Error("connection-failed", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("request-failed", err)
		return err
	}
	defer resp.Body.Close()

	// We are assuming that a 404 means that the rep has not updated
	// yet, but will roll soon.  This is for backwards compatibility.
	if resp.StatusCode == http.StatusNotFound {
		logger.Error("failed-with-status", err, lager.Data{"status-code": resp.StatusCode, "msg": http.StatusText(resp.StatusCode)})
		// The v1 UpdateLRPInstance route is only for updating InternalRoutes
		// on old versions of rep. This is for backwards compatibility.
		if update.InternalRoutes != nil {
			update.MetricTags = nil
			return c.updateLRPInstanceRoute_r0(loggerCopy, update)
		}
		return nil
	}

	if resp.StatusCode != http.StatusAccepted {
		err := fmt.Errorf("http error: status code %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
		logger.Error("failed-with-status", err, lager.Data{"status-code": resp.StatusCode, "msg": http.StatusText(resp.StatusCode)})
		return err
	}

	logger.Info("completed", lager.Data{"duration": time.Since(start)})
	return nil
}

func (c *client) updateLRPInstanceRoute_r0(
	logger lager.Logger,
	update models.LRPUpdate) error {
	start := time.Now()
	logger = logger.Session("update-lrp-r0", lager.Data{"process-guid": update.ProcessGuid,
		"index":         update.Index,
		"domain":        update.Domain,
		"instance-guid": update.InstanceGUID,
	})
	logger.Info("starting")

	body, err := json.Marshal(update)
	if err != nil {
		logger.Error("marshal-failed", err)
		return err
	}

	params := rata.Params{
		"process_guid":  update.ProcessGuid,
		"instance_guid": update.InstanceGUID,
	}
	req, err := c.requestGenerator.CreateRequest(UpdateLRPInstanceRoute_r0, params, bytes.NewReader(body))
	if err != nil {
		logger.Error("connection-failed", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("request-failed", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		logger.Error("failed-with-status", err, lager.Data{"status-code": resp.StatusCode, "msg": http.StatusText(resp.StatusCode)})
		return nil
	}

	if resp.StatusCode != http.StatusAccepted {
		err := fmt.Errorf("http error: status code %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
		logger.Error("failed-with-status", err, lager.Data{"status-code": resp.StatusCode, "msg": http.StatusText(resp.StatusCode)})
		return err
	}

	logger.Info("completed", lager.Data{"duration": time.Since(start)})
	return nil
}

func (c *client) StopLRPInstance(
	logger lager.Logger,
	key models.ActualLRPKey,
	instanceKey models.ActualLRPInstanceKey,
) error {
	start := time.Now()
	logger = logger.Session("stop-lrp", lager.Data{"process-guid": key.ProcessGuid,
		"index":        key.Index,
		"domain":       key.Domain,
		"instance-key": instanceKey,
	})
	logger.Info("starting")

	req, err := c.requestGenerator.CreateRequest(StopLRPInstanceRoute, stopParamsFromLRP(key, instanceKey), nil)
	if err != nil {
		logger.Error("connection-failed", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("request-failed", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		err := fmt.Errorf("http error: status code %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
		logger.Error("failed-with-status", err, lager.Data{"status-code": resp.StatusCode, "msg": http.StatusText(resp.StatusCode)})
		return err
	}

	logger.Info("completed", lager.Data{"duration": time.Since(start)})
	return nil
}

func (c *client) CancelTask(logger lager.Logger, taskGuid string) error {
	start := time.Now()
	logger = logger.Session("cancel-task", lager.Data{"task-guid": taskGuid})
	logger.Info("starting")

	req, err := c.requestGenerator.CreateRequest(CancelTaskRoute, rata.Params{"task_guid": taskGuid}, nil)
	if err != nil {
		logger.Error("connection-failed", err)
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error("request-failed", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		err := fmt.Errorf("http error: status code %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
		logger.Error("failed-with-status", err, lager.Data{"status-code": resp.StatusCode, "msg": http.StatusText(resp.StatusCode)})
		return err
	}

	logger.Info("completed", lager.Data{"duration": time.Since(start)})
	return nil
}

func stopParamsFromLRP(
	key models.ActualLRPKey,
	instanceKey models.ActualLRPInstanceKey,
) rata.Params {
	return rata.Params{
		"process_guid":  key.ProcessGuid,
		"instance_guid": instanceKey.InstanceGuid,
		"index":         strconv.Itoa(int(key.Index)),
	}
}
