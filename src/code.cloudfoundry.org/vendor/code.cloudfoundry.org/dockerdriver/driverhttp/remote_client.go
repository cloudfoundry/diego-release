package driverhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/goshims/http_wrap"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/tedsuo/rata"
)

type reqFactory struct {
	reqGen  *rata.RequestGenerator
	route   string
	payload []byte
}

func newReqFactory(reqGen *rata.RequestGenerator, route string, payload []byte) *reqFactory {
	return &reqFactory{
		reqGen:  reqGen,
		route:   route,
		payload: payload,
	}
}

func (r *reqFactory) Request() (*http.Request, error) {
	return r.reqGen.CreateRequest(r.route, nil, bytes.NewBuffer(r.payload))
}

type remoteClient struct {
	HttpClient http_wrap.Client
	reqGen     *rata.RequestGenerator
	clock      clock.Clock
	url        string
	tls        *dockerdriver.TLSConfig
}

func NewRemoteClient(url string, tls *dockerdriver.TLSConfig) (*remoteClient, error) {
	client := cfhttp.NewClient()
	input_url := url

	if strings.Contains(url, ".sock") {
		client = cfhttp.NewUnixClient(url)
		url = fmt.Sprintf("unix://%s", url)
	} else {
		if tls != nil {
			tlsConfig, err := tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(tls.CertFile, tls.KeyFile),
			).Client(tlsconfig.WithAuthorityFromFile(tls.CAFile))
			if err != nil {
				return nil, err
			}

			tlsConfig.InsecureSkipVerify = tls.InsecureSkipVerify

			if tr, ok := client.Transport.(*http.Transport); ok {
				tr.TLSClientConfig = tlsConfig
			} else {
				return nil, errors.New("Invalid transport")
			}
		}

	}

	driver := NewRemoteClientWithClient(url, tls, client, clock.NewClock())
	driver.url = input_url
	return driver, nil
}

func NewRemoteClientWithClient(url string, tls *dockerdriver.TLSConfig, client http_wrap.Client, clock clock.Clock) *remoteClient {
	driver := remoteClient{
		HttpClient: client,
		reqGen:     rata.NewRequestGenerator(url, dockerdriver.Routes),
		clock:      clock,
	}

	driver.tls = tls

	return &driver
}

func (r *remoteClient) Matches(loggerIn lager.Logger, url string, tls *dockerdriver.TLSConfig) bool {
	logger := loggerIn.Session("matches")
	logger.Info("start")
	defer logger.Info("end")

	if url != r.url {
		return false
	}
	var tls1, tls2 []byte
	var err error
	if tls != nil {
		tls1, err = json.Marshal(tls)
		if err != nil {
			logger.Error("failed-json-marshall", err)
			return false
		}
	}
	if r.tls != nil {
		tls2, err = json.Marshal(r.tls)
		if err != nil {
			logger.Error("failed-json-marshall", err)
			return false
		}
	}
	return string(tls1) == string(tls2)
}

func (r *remoteClient) Activate(env dockerdriver.Env) dockerdriver.ActivateResponse {
	logger := env.Logger().Session("activate")
	logger.Info("start")
	defer logger.Info("end")

	request := newReqFactory(r.reqGen, dockerdriver.ActivateRoute, nil)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-activate", err)
		return dockerdriver.ActivateResponse{Err: err.Error()}
	}

	if response == nil {
		return dockerdriver.ActivateResponse{Err: "Invalid response from driver."}
	}

	var activate dockerdriver.ActivateResponse
	if err := json.Unmarshal(response, &activate); err != nil {
		logger.Error("failed-parsing-activate-response", err)
		return dockerdriver.ActivateResponse{Err: err.Error()}
	}

	return activate
}

func (r *remoteClient) Create(env dockerdriver.Env, createRequest dockerdriver.CreateRequest) dockerdriver.ErrorResponse {
	logger := env.Logger().Session("create", lager.Data{"create_request.Name": createRequest.Name})
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(createRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return dockerdriver.ErrorResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, dockerdriver.CreateRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-creating-volume", err)
		return dockerdriver.ErrorResponse{Err: err.Error()}
	}

	var remoteError dockerdriver.ErrorResponse
	if response == nil {
		return dockerdriver.ErrorResponse{Err: "Invalid response from driver."}
	}

	if err := json.Unmarshal(response, &remoteError); err != nil {
		logger.Error("failed-parsing-error-response", err)
		return dockerdriver.ErrorResponse{Err: err.Error()}
	}

	return dockerdriver.ErrorResponse{}
}

func (r *remoteClient) List(env dockerdriver.Env) dockerdriver.ListResponse {
	logger := env.Logger().Session("remoteclient-list")
	logger.Info("start")
	defer logger.Info("end")

	request := newReqFactory(r.reqGen, dockerdriver.ListRoute, nil)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-list", err)
		return dockerdriver.ListResponse{Err: err.Error()}
	}

	if response == nil {
		return dockerdriver.ListResponse{Err: "Invalid response from driver."}
	}

	var list dockerdriver.ListResponse
	if err := json.Unmarshal(response, &list); err != nil {
		logger.Error("failed-parsing-list-response", err)
		return dockerdriver.ListResponse{Err: err.Error()}
	}

	return list
}

func (r *remoteClient) Mount(env dockerdriver.Env, mountRequest dockerdriver.MountRequest) dockerdriver.MountResponse {
	logger := env.Logger().Session("remoteclient-mount", lager.Data{"mount_request": mountRequest})
	logger.Info("start")
	defer logger.Info("end")

	sendingJson, err := json.Marshal(mountRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return dockerdriver.MountResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, dockerdriver.MountRoute, sendingJson)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-mounting-volume", err)
		return dockerdriver.MountResponse{Err: err.Error()}
	}

	if response == nil {
		return dockerdriver.MountResponse{Err: "Invalid response from driver."}
	}

	var mountPoint dockerdriver.MountResponse
	if err := json.Unmarshal(response, &mountPoint); err != nil {
		logger.Error("failed-parsing-mount-response", err)
		return dockerdriver.MountResponse{Err: err.Error()}
	}

	return mountPoint
}

func (r *remoteClient) Path(env dockerdriver.Env, pathRequest dockerdriver.PathRequest) dockerdriver.PathResponse {
	logger := env.Logger().Session("path")
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(pathRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return dockerdriver.PathResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, dockerdriver.PathRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-volume-path", err)
		return dockerdriver.PathResponse{Err: err.Error()}
	}

	if response == nil {
		return dockerdriver.PathResponse{Err: "Invalid response from driver."}
	}

	var mountPoint dockerdriver.PathResponse
	if err := json.Unmarshal(response, &mountPoint); err != nil {
		logger.Error("failed-parsing-path-response", err)
		return dockerdriver.PathResponse{Err: err.Error()}
	}

	return mountPoint
}

func (r *remoteClient) Unmount(env dockerdriver.Env, unmountRequest dockerdriver.UnmountRequest) dockerdriver.ErrorResponse {
	logger := env.Logger().Session("mount")
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(unmountRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return dockerdriver.ErrorResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, dockerdriver.UnmountRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-unmounting-volume", err)
		return dockerdriver.ErrorResponse{Err: err.Error()}
	}

	if response == nil {
		return dockerdriver.ErrorResponse{Err: "Invalid response from driver."}
	}

	var remoteErrorResponse dockerdriver.ErrorResponse
	if err := json.Unmarshal(response, &remoteErrorResponse); err != nil {
		logger.Error("failed-parsing-error-response", err)
		return dockerdriver.ErrorResponse{Err: err.Error()}
	}
	return remoteErrorResponse
}

func (r *remoteClient) Remove(env dockerdriver.Env, removeRequest dockerdriver.RemoveRequest) dockerdriver.ErrorResponse {
	logger := env.Logger().Session("remove")
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(removeRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return dockerdriver.ErrorResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, dockerdriver.RemoveRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-removing-volume", err)
		return dockerdriver.ErrorResponse{Err: err.Error()}
	}

	if response == nil {
		return dockerdriver.ErrorResponse{Err: "Invalid response from driver."}
	}

	var remoteErrorResponse dockerdriver.ErrorResponse
	if err := json.Unmarshal(response, &remoteErrorResponse); err != nil {
		logger.Error("failed-parsing-error-response", err)
		return dockerdriver.ErrorResponse{Err: err.Error()}
	}

	return remoteErrorResponse
}

func (r *remoteClient) Get(env dockerdriver.Env, getRequest dockerdriver.GetRequest) dockerdriver.GetResponse {
	logger := env.Logger().Session("get")
	logger.Info("start")
	defer logger.Info("end")

	payload, err := json.Marshal(getRequest)
	if err != nil {
		logger.Error("failed-marshalling-request", err)
		return dockerdriver.GetResponse{Err: err.Error()}
	}

	request := newReqFactory(r.reqGen, dockerdriver.GetRoute, payload)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-getting-volume", err)
		return dockerdriver.GetResponse{Err: err.Error()}
	}

	if response == nil {
		return dockerdriver.GetResponse{Err: "Invalid response from driver."}
	}

	var remoteResponse dockerdriver.GetResponse
	if err := json.Unmarshal(response, &remoteResponse); err != nil {
		logger.Error("failed-parsing-error-response", err)
		return dockerdriver.GetResponse{Err: err.Error()}
	}

	return remoteResponse
}

func (r *remoteClient) Capabilities(env dockerdriver.Env) dockerdriver.CapabilitiesResponse {
	logger := env.Logger().Session("capabilities")
	logger.Info("start")
	defer logger.Info("end")

	request := newReqFactory(r.reqGen, dockerdriver.CapabilitiesRoute, nil)

	response, err := r.do(env.Context(), logger, request)
	if err != nil {
		logger.Error("failed-capabilities", err)
		return dockerdriver.CapabilitiesResponse{}
	}

	var remoteError dockerdriver.CapabilitiesResponse
	if response == nil {
		return remoteError
	}

	var capabilities dockerdriver.CapabilitiesResponse
	if err := json.Unmarshal(response, &capabilities); err != nil {
		logger.Error("failed-parsing-capabilities-response", err)
		return dockerdriver.CapabilitiesResponse{}
	}

	return capabilities
}

func (r *remoteClient) GetVoldriver() dockerdriver.Driver {
	return r
}

func (r *remoteClient) do(ctx context.Context, logger lager.Logger, requestFactory *reqFactory) ([]byte, error) {

	var data []byte

	request, err := requestFactory.Request()
	if err != nil {
		logger.Error("request-gen-failed", err)
		return data, err
	}
	request = request.WithContext(ctx)

	response, err := r.HttpClient.Do(request)
	if err != nil {
		logger.Error("request-failed", err)
		return data, err
	}
	logger.Debug("response", lager.Data{"response": response.Status})

	data, err = io.ReadAll(response.Body)
	if err != nil {
		return data, err
	}

	var remoteErrorResponse dockerdriver.ErrorResponse
	if err := json.Unmarshal(data, &remoteErrorResponse); err != nil {
		logger.Error("failed-parsing-http-response-body", err)
		return data, err
	}

	if remoteErrorResponse.Err != "" {
		return data, errors.New(remoteErrorResponse.Err)
	}

	return data, nil

}
