package containerstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/envoyconfig"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

const (
	StartProxyPort  = 61001
	EndProxyPort    = 65534
	DefaultHTTPPort = 8080
	C2CTLSPort      = 61443

	TimeOut = 250000000

	IngressListener = "ingress_listener"
	TcpProxy        = "envoy.tcp_proxy"
	AdsClusterName  = "pilot-ads"

	AdminAccessLog = os.DevNull
)

var (
	ErrNoPortsAvailable     = errors.New("no ports available")
	ErrInvalidCertificate   = errors.New("cannot parse invalid certificate")
	ErrC2CTLSPortIsReserved = fmt.Errorf("port %d is reserved for container networking", C2CTLSPort)

	AlpnProtocols         = []string{"h2,http/1.1"}
	SupportedCipherSuites = []string{"ECDHE-RSA-AES256-GCM-SHA384", "ECDHE-RSA-AES128-GCM-SHA256"}
)

var dummyRunner = func(credRotatedChan <-chan Credential) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)
		for {
			select {
			case <-credRotatedChan:
			case <-signals:
				return nil
			}
		}
	})
}

type ProxyConfigHandler struct {
	logger                             lager.Logger
	containerProxyPath                 string
	containerProxyConfigPath           string
	containerProxyTrustedCACerts       []string
	containerProxyVerifySubjectAltName []string
	containerProxyRequireClientCerts   bool

	reloadDuration time.Duration
	reloadClock    clock.Clock

	adsServers []string

	http2Enabled bool
}

type NoopProxyConfigHandler struct{}

func (p *NoopProxyConfigHandler) CreateDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error) {
	return nil, nil, nil
}
func (p *NoopProxyConfigHandler) RemoveDir(logger lager.Logger, container executor.Container) error {
	return nil
}
func (p *NoopProxyConfigHandler) Update(credentials Credentials, container executor.Container) error {
	return nil
}
func (p *NoopProxyConfigHandler) Close(invalidCredentials Credentials, container executor.Container) error {
	return nil
}

func (p *NoopProxyConfigHandler) RemoveProxyConfigDir(logger lager.Logger, container executor.Container) error {
	return nil
}

func (p *NoopProxyConfigHandler) ProxyPorts(lager.Logger, *executor.Container) ([]executor.ProxyPortMapping, []uint16, error) {
	return nil, nil, nil
}

func (p *NoopProxyConfigHandler) Runner(logger lager.Logger, container executor.Container, credRotatedChan <-chan Credential) (ifrit.Runner, error) {
	return dummyRunner(credRotatedChan), nil
}

func NewNoopProxyConfigHandler() *NoopProxyConfigHandler {
	return &NoopProxyConfigHandler{}
}

func NewProxyConfigHandler(
	logger lager.Logger,
	containerProxyPath string,
	containerProxyConfigPath string,
	ContainerProxyTrustedCACerts []string,
	ContainerProxyVerifySubjectAltName []string,
	containerProxyRequireClientCerts bool,
	reloadDuration time.Duration,
	reloadClock clock.Clock,
	adsServers []string,
	http2Enabled bool,
) *ProxyConfigHandler {
	return &ProxyConfigHandler{
		logger:                             logger.Session("proxy-manager"),
		containerProxyPath:                 containerProxyPath,
		containerProxyConfigPath:           containerProxyConfigPath,
		containerProxyTrustedCACerts:       ContainerProxyTrustedCACerts,
		containerProxyVerifySubjectAltName: ContainerProxyVerifySubjectAltName,
		containerProxyRequireClientCerts:   containerProxyRequireClientCerts,
		reloadDuration:                     reloadDuration,
		reloadClock:                        reloadClock,
		adsServers:                         adsServers,
		http2Enabled:                       http2Enabled,
	}
}

// This modifies the container pointer in order to create garden NetIn rules in the storenode.Create
func (p *ProxyConfigHandler) ProxyPorts(logger lager.Logger, container *executor.Container) ([]executor.ProxyPortMapping, []uint16, error) {
	if !container.EnableContainerProxy {
		return nil, nil, nil
	}

	proxyPortMapping := []executor.ProxyPortMapping{}

	existingPorts := make(map[uint16]interface{})
	containerPorts := make([]uint16, len(container.Ports))
	for i, portMap := range container.Ports {
		if portMap.ContainerPort == C2CTLSPort {
			return nil, nil, ErrC2CTLSPortIsReserved
		}
		existingPorts[portMap.ContainerPort] = struct{}{}
		containerPorts[i] = portMap.ContainerPort
	}

	extraPorts := []uint16{}

	portCount := 0
	for port := uint16(StartProxyPort); port < EndProxyPort; port++ {
		if portCount == len(existingPorts) {
			break
		}

		if existingPorts[port] != nil {
			continue
		}

		if port == C2CTLSPort {
			continue
		}

		extraPorts = append(extraPorts, port)
		proxyPortMapping = append(proxyPortMapping, executor.ProxyPortMapping{
			AppPort:   containerPorts[portCount],
			ProxyPort: port,
		})

		if containerPorts[portCount] == DefaultHTTPPort {
			proxyPortMapping = append(proxyPortMapping, executor.ProxyPortMapping{
				AppPort:   containerPorts[portCount],
				ProxyPort: C2CTLSPort,
			})
			extraPorts = append(extraPorts, C2CTLSPort)
		}

		portCount++
	}

	return proxyPortMapping, extraPorts, nil
}

func (p *ProxyConfigHandler) CreateDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error) {
	if !container.EnableContainerProxy {
		return nil, nil, nil
	}

	logger.Info("adding-container-proxy-bindmounts")
	proxyConfigDir := filepath.Join(p.containerProxyConfigPath, container.Guid)
	mounts := []garden.BindMount{
		{
			Origin:  garden.BindMountOriginHost,
			SrcPath: p.containerProxyPath,
			DstPath: "/etc/cf-assets/envoy",
		},
		{
			Origin:  garden.BindMountOriginHost,
			SrcPath: proxyConfigDir,
			DstPath: "/etc/cf-assets/envoy_config",
		},
	}

	err := os.MkdirAll(proxyConfigDir, 0755)
	if err != nil {
		return nil, nil, err
	}

	return mounts, nil, nil
}

func (p *ProxyConfigHandler) RemoveDir(logger lager.Logger, container executor.Container) error {
	if !container.EnableContainerProxy {
		return nil
	}

	logger.Info("removing-container-proxy-config-dir")
	proxyConfigDir := filepath.Join(p.containerProxyConfigPath, container.Guid)
	return os.RemoveAll(proxyConfigDir)
}

func (p *ProxyConfigHandler) Update(credentials Credentials, container executor.Container) error {
	if !container.EnableContainerProxy {
		return nil
	}

	return p.writeConfig(credentials, container)
}

func (p *ProxyConfigHandler) Close(invalidCredentials Credentials, container executor.Container) error {
	if !container.EnableContainerProxy {
		return nil
	}

	err := p.writeConfig(invalidCredentials, container)
	if err != nil {
		return err
	}

	p.reloadClock.Sleep(p.reloadDuration)
	return nil
}

func (p *ProxyConfigHandler) writeConfig(credentials Credentials, container executor.Container) error {
	dir := filepath.Join(p.containerProxyConfigPath, container.Guid)
	adminPort, err := envoyconfig.GetAvailablePort(container.Ports)
	if err != nil {
		return err
	}
	bootstrapYAML, err := envoyconfig.GenerateBootstrap(
		container, adminPort, p.containerProxyRequireClientCerts, p.adsServers, p.http2Enabled)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "envoy.yaml"), bootstrapYAML, 0644); err != nil {
		return err
	}
	idCred := envoyconfig.Credential{Cert: credentials.InstanceIdentityCredential.Cert, Key: credentials.InstanceIdentityCredential.Key}
	writeSDS := func(name string, data []byte) error {
		tmpPath := filepath.Join(dir, name+".tmp")
		if err := os.WriteFile(tmpPath, data, 0644); err != nil {
			return err
		}
		return os.Rename(tmpPath, filepath.Join(dir, name))
	}
	if !idCred.IsEmpty() {
		sdsID, err := envoyconfig.GenerateSDSCertAndKey("id-cert-and-key", idCred)
		if err != nil {
			return err
		}
		if err := writeSDS("sds-id-cert-and-key.yaml", sdsID); err != nil {
			return err
		}
	}
	c2cCred := envoyconfig.Credential{Cert: credentials.C2CCredential.Cert, Key: credentials.C2CCredential.Key}
	sdsC2C, err := envoyconfig.GenerateSDSCertAndKey("c2c-cert-and-key", c2cCred)
	if err != nil {
		return err
	}
	if err := writeSDS("sds-c2c-cert-and-key.yaml", sdsC2C); err != nil {
		return err
	}
	sdsValidation, err := envoyconfig.GenerateSDSCAResource(
		container, idCred, p.containerProxyTrustedCACerts, p.containerProxyVerifySubjectAltName)
	if err != nil {
		return err
	}
	return writeSDS("sds-id-validation-context.yaml", sdsValidation)
}
