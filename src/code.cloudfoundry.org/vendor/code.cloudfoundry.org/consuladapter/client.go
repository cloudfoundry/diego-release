package consuladapter

import (
	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	"github.com/hashicorp/consul/api"
)

//go:generate counterfeiter -o fakes/fake_client.go . Client

type Client interface {
	Agent() Agent
	Session() Session
	Catalog() Catalog
	KV() KV
	Status() Status

	LockOpts(opts *api.LockOptions) (Lock, error)
}

//go:generate counterfeiter -o fakes/fake_lock.go . Lock

type Lock interface {
	Lock(stopCh <-chan struct{}) (lostLock <-chan struct{}, err error)
}

type client struct {
	client *api.Client
}

func NewConsulClient(c *api.Client) Client {
	return &client{client: c}
}

func NewClientFromUrl(urlString string) (Client, error) {
	scheme, address, err := Parse(urlString)
	if err != nil {
		return nil, err
	}

	config := &api.Config{
		Address:    address,
		Scheme:     scheme,
		HttpClient: cfhttp.NewClient(cfhttp.WithStreamingDefaults()),
	}

	c, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &client{client: c}, nil
}

func NewTLSClientFromUrl(urlString, caCert, clientCert, clientKey string) (Client, error) {
	scheme, address, err := Parse(urlString)
	if err != nil {
		return nil, err
	}

	tlsConfig := api.TLSConfig{
		Address:  address,
		CAFile:   caCert,
		CertFile: clientCert,
		KeyFile:  clientKey,
	}

	tlsClientConfig, err := api.SetupTLSConfig(&tlsConfig)
	if err != nil {
		return nil, err
	}

	httpClient := cfhttp.NewClient(
		cfhttp.WithStreamingDefaults(),
		cfhttp.WithTLSConfig(tlsClientConfig),
	)

	config := &api.Config{
		Address:    address,
		Scheme:     scheme,
		HttpClient: httpClient,
	}

	c, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &client{client: c}, nil
}

func (c *client) Agent() Agent {
	return NewConsulAgent(c.client.Agent())
}

func (c *client) KV() KV {
	return NewConsulKV(c.client.KV())
}

func (c *client) Catalog() Catalog {
	return NewConsulCatalog(c.client.Catalog())
}

func (c *client) Session() Session {
	return NewConsulSession(c.client.Session())
}

func (c *client) LockOpts(opts *api.LockOptions) (Lock, error) {
	return c.client.LockOpts(opts)
}

func (c *client) Status() Status {
	return NewConsulStatus(c.client.Status())
}
