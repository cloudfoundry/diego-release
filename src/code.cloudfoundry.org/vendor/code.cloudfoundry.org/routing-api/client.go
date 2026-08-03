package routing_api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-api/trace"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

const (
	defaultMaxRetries  = uint16(0)
	defaultHttpTimeout = 60 * time.Second
)

//go:generate counterfeiter -o fake_routing_api/fake_client.go . Client

type Client interface {
	SetToken(string)
	UpsertRoutes([]models.Route) error
	Routes() ([]models.Route, error)
	DeleteRoutes([]models.Route) error
	RouterGroups() ([]models.RouterGroup, error)
	RouterGroupWithName(string) (models.RouterGroup, error)
	UpdateRouterGroup(models.RouterGroup) error
	CreateRouterGroup(models.RouterGroup) error
	DeleteRouterGroup(models.RouterGroup) error
	ReservePort(string, string) (int, error)
	UpsertTcpRouteMappings([]models.TcpRouteMapping) error
	DeleteTcpRouteMappings([]models.TcpRouteMapping) error
	TcpRouteMappings() ([]models.TcpRouteMapping, error)
	FilteredTcpRouteMappings([]string) ([]models.TcpRouteMapping, error)

	SubscribeToEvents() (EventSource, error)
	SubscribeToEventsWithMaxRetries(retries uint16) (EventSource, error)
	SubscribeToTcpEvents() (TcpEventSource, error)
	SubscribeToTcpEventsWithMaxRetries(retries uint16) (TcpEventSource, error)
}

func NewClient(url string, skipTLSVerification bool) Client {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipTLSVerification,
	}
	return NewClientWithTLSConfig(url, tlsConfig)
}

func NewClientWithTLSConfig(url string, tlsConfig *tls.Config) Client {
	httpClient := cfhttp.NewClient(
		cfhttp.WithRequestTimeout(defaultHttpTimeout),
		cfhttp.WithTLSConfig(tlsConfig),
	)

	streamingClient := cfhttp.NewClient(
		cfhttp.WithStreamingDefaults(),
		cfhttp.WithTLSConfig(tlsConfig),
	)
	return &client{
		httpClient:          httpClient,
		streamingHTTPClient: streamingClient,

		tokenMutex: &sync.RWMutex{},

		reqGen: rata.NewRequestGenerator(url, Routes()),
	}
}

type client struct {
	httpClient          *http.Client
	streamingHTTPClient *http.Client

	tokenMutex *sync.RWMutex
	authToken  string

	reqGen *rata.RequestGenerator
}

func (c *client) SetToken(token string) {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()
	c.authToken = token
}

func (c *client) UpsertRoutes(routes []models.Route) error {
	return c.doRequest(UpsertRoute, nil, nil, routes, nil)
}

func (c *client) Routes() ([]models.Route, error) {
	var routes []models.Route
	err := c.doRequest(ListRoute, nil, nil, nil, &routes)
	return routes, err
}

func (c *client) UpdateRouterGroup(group models.RouterGroup) error {
	return c.doRequest(UpdateRouterGroup, rata.Params{"guid": group.Guid}, nil, group, nil)
}

func (c *client) CreateRouterGroup(group models.RouterGroup) error {
	return c.doRequest(CreateRouterGroup, nil, nil, group, nil)
}

func (c *client) DeleteRouterGroup(group models.RouterGroup) error {
	return c.doRequest(DeleteRouterGroup, rata.Params{"guid": group.Guid}, nil, nil, nil)
}

func (c *client) RouterGroups() ([]models.RouterGroup, error) {
	var routerGroups []models.RouterGroup
	err := c.doRequest(ListRouterGroups, nil, nil, nil, &routerGroups)
	return routerGroups, err
}

func (c *client) RouterGroupWithName(name string) (models.RouterGroup, error) {
	var routerGroups []models.RouterGroup
	err := c.doRequest(ListRouterGroups, nil, url.Values{"name": []string{name}}, nil, &routerGroups)
	if err != nil {
		return models.RouterGroup{}, err
	}
	return routerGroups[0], err
}

func (c *client) ReservePort(groupName string, portRange string) (int, error) {
	reservablePorts := models.ReservablePorts(portRange)
	ranges, err := reservablePorts.Parse()
	if err != nil {
		return 0, err
	}

	if len(ranges) > 1 {
		return 0, Error{ProcessRequestError, "multiple port ranges are not supported"}
	}

	if start, end := ranges[0].Endpoints(); start == end {
		return 0, Error{ProcessRequestError, "single port is not supported"}
	}

	routerGroups, err := c.RouterGroups()
	if err != nil {
		return 0, err
	}

	reservablePort, err := getNextAvailablePort(routerGroups, ranges[0])
	if err != nil {
		return 0, err
	}

	routerGroup := models.RouterGroup{
		Name:            groupName,
		Type:            models.RouterGroup_TCP,
		ReservablePorts: reservablePort,
	}

	existingRouterGroup, _ := c.RouterGroupWithName(groupName)

	if (existingRouterGroup != models.RouterGroup{}) {
		existingRouterGroup.ReservablePorts = reservablePort
		err = c.UpdateRouterGroup(existingRouterGroup)

		if err != nil {
			return 0, err
		}
	} else {
		err = c.CreateRouterGroup(routerGroup)

		if err != nil {
			return 0, err
		}
	}

	return strconv.Atoi(string(reservablePort))
}

func getNextAvailablePort(groups models.RouterGroups, portRange models.Range) (models.ReservablePorts, error) {
	portSet := make(map[uint16]bool)

	for _, group := range groups {
		groupPortRanges, err := group.ReservablePorts.Parse()
		if err != nil {
			return "", err // not tested
		}

		for _, grp := range groupPortRanges {
			groupStart, groupEnd := grp.Endpoints()
			for p := groupStart; p <= groupEnd; p++ {
				portSet[p] = true
			}
		}
	}

	start, end := portRange.Endpoints()
	for i := start; i <= end; i++ {
		if _, ok := portSet[i]; !ok {
			return models.ReservablePorts(strconv.Itoa(int(i))), nil
		}
	}

	return "", Error{Type: PortRangeExhaustedError, Message: fmt.Sprintf("There are no free ports in range: %s", portRange)}
}

func (c *client) DeleteRoutes(routes []models.Route) error {
	return c.doRequest(DeleteRoute, nil, nil, routes, nil)
}

func (c *client) UpsertTcpRouteMappings(tcpRouteMappings []models.TcpRouteMapping) error {
	return c.doRequest(UpsertTcpRouteMapping, nil, nil, tcpRouteMappings, nil)
}

func (c *client) TcpRouteMappings() ([]models.TcpRouteMapping, error) {
	var tcpRouteMappings []models.TcpRouteMapping
	err := c.doRequest(ListTcpRouteMapping, nil, nil, nil, &tcpRouteMappings)
	return tcpRouteMappings, err
}

func (c *client) FilteredTcpRouteMappings(isolationSegments []string) ([]models.TcpRouteMapping, error) {
	var tcpRouteMappings []models.TcpRouteMapping
	err := c.doRequest(ListTcpRouteMapping, nil, url.Values{"isolation_segment": isolationSegments}, nil, &tcpRouteMappings)
	return tcpRouteMappings, err
}

func (c *client) DeleteTcpRouteMappings(tcpRouteMappings []models.TcpRouteMapping) error {
	return c.doRequest(DeleteTcpRouteMapping, nil, nil, tcpRouteMappings, nil)
}

func (c *client) SubscribeToEvents() (EventSource, error) {
	eventSource, err := c.doSubscribe(EventStreamRoute, defaultMaxRetries)
	if err != nil {
		return nil, err
	}
	return NewEventSource(eventSource), nil
}

func (c *client) SubscribeToTcpEvents() (TcpEventSource, error) {
	eventSource, err := c.doSubscribe(EventStreamTcpRoute, defaultMaxRetries)
	if err != nil {
		return nil, err
	}
	return NewTcpEventSource(eventSource), nil
}

func (c *client) SubscribeToEventsWithMaxRetries(retries uint16) (EventSource, error) {
	eventSource, err := c.doSubscribe(EventStreamRoute, retries)
	if err != nil {
		return nil, err
	}
	return NewEventSource(eventSource), nil
}

func (c *client) SubscribeToTcpEventsWithMaxRetries(retries uint16) (TcpEventSource, error) {
	eventSource, err := c.doSubscribe(EventStreamTcpRoute, retries)
	if err != nil {
		return nil, err
	}
	return NewTcpEventSource(eventSource), nil
}

func (c *client) doSubscribe(routeName string, retries uint16) (RawEventSource, error) {
	config := sse.Config{
		Client: c.streamingHTTPClient,
		RetryParams: sse.RetryParams{
			MaxRetries:    retries,
			RetryInterval: time.Second,
		},
		RequestCreator: func() *http.Request {
			request, err := c.reqGen.CreateRequest(routeName, nil, nil)
			c.tokenMutex.RLock()
			defer c.tokenMutex.RUnlock()
			request.Header.Add("Authorization", "bearer "+c.authToken)
			if err != nil {
				panic(err) // totally shouldn't happen
			}

			trace.DumpRequest(request)
			return request
		},
	}
	eventSource, err := config.Connect()
	if err != nil {
		bre, ok := err.(sse.BadResponseError)
		if ok && bre.Response.StatusCode == http.StatusUnauthorized {
			return nil, Error{Type: "unauthorized", Message: "unauthorized"}
		}
		return nil, err
	}

	return eventSource, nil
}

func (c *client) createRequest(requestName string, params rata.Params, queryParams url.Values, request interface{}) (*http.Request, error) {
	var (
		bodyBytes []byte
		err       error
	)

	if request != nil {
		bodyBytes, err = json.Marshal(request)
		if err != nil {
			return nil, err
		}
	}

	req, err := c.reqGen.CreateRequest(requestName, params, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = queryParams.Encode()
	req.ContentLength = int64(len(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	c.tokenMutex.RLock()
	defer c.tokenMutex.RUnlock()
	req.Header.Add("Authorization", "bearer "+c.authToken)

	return req, nil
}

func (c *client) doRequest(requestName string, params rata.Params, queryParams url.Values, request, response interface{}) error {
	req, err := c.createRequest(requestName, params, queryParams, request)
	if err != nil {
		return err
	}
	return c.do(req, response)
}

func (c *client) do(req *http.Request, response interface{}) error {
	trace.DumpRequest(req)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	trace.DumpResponse(res)

	if res.StatusCode == http.StatusUnauthorized {
		return NewError(UnauthorizedError, "unauthorized")
	}

	if res.StatusCode > 299 {
		return transformResponseError(res)
	}

	if response != nil {
		return json.NewDecoder(res.Body).Decode(response)
	}

	return nil
}

func transformResponseError(res *http.Response) error {
	errResponse := Error{}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return NewError(ResponseError, "failed to read response body")
	}

	err = json.Unmarshal(data, &errResponse)
	if err != nil {
		return NewError(ResponseError, string(data))
	}

	if errResponse.Type == "" {
		return NewError(ResponseError, string(data))
	}
	return errResponse
}
