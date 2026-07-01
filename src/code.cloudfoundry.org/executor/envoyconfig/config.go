package envoyconfig

import (
	"bytes"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"os"

	"code.cloudfoundry.org/executor"
	envoy_accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoy_bootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	envoy_cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_metrics "github.com/envoyproxy/go-control-plane/envoy/config/metrics/v3"
	envoy_file_access_log "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	envoy_tcp_proxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoy_tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	ghodss_yaml "github.com/ghodss/yaml"
	"github.com/golang/protobuf/ptypes/duration"
	pstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	StartProxyPort  = 61001
	EndProxyPort    = 65534
	DefaultHTTPPort = 8080
	C2CTLSPort      = 61443
	AdsClusterName  = "pilot-ads"
)

var (
	ErrNoPortsAvailable   = errors.New("no ports available")
	SupportedCipherSuites = []string{"ECDHE-RSA-AES256-GCM-SHA384", "ECDHE-RSA-AES128-GCM-SHA256"}
	AlpnProtocols         = []string{"h2,http/1.1"}
)

// Credential holds cert and key PEM strings for SDS.
type Credential struct {
	Cert string
	Key  string
}

func (c Credential) IsEmpty() bool {
	return c.Cert == "" && c.Key == ""
}

// GetAvailablePort returns a port not in allocatedPorts or extraKnownPorts.
func GetAvailablePort(allocatedPorts []executor.PortMapping, extraKnownPorts ...uint16) (uint16, error) {
	existingPorts := make(map[uint16]struct{})
	for _, portMap := range allocatedPorts {
		existingPorts[portMap.ContainerPort] = struct{}{}
		existingPorts[portMap.ContainerTLSProxyPort] = struct{}{}
	}
	for _, p := range extraKnownPorts {
		existingPorts[p] = struct{}{}
	}
	for port := uint16(StartProxyPort); port < EndProxyPort; port++ {
		if _, ok := existingPorts[port]; ok {
			continue
		}
		return port, nil
	}
	return 0, ErrNoPortsAvailable
}

// AssignProxyPorts takes PortMappings with only ContainerPort set and returns enriched
// entries with ContainerTLSProxyPort allocated. Port 8080 also gets a C2C TLS mapping on 61443.
// HostPort and HostTLSProxyPort are left at 0.
func AssignProxyPorts(appPorts []executor.PortMapping) ([]executor.PortMapping, error) {
	unique := make(map[uint16]struct{})
	var ordered []uint16
	for _, p := range appPorts {
		if _, seen := unique[p.ContainerPort]; !seen {
			unique[p.ContainerPort] = struct{}{}
			ordered = append(ordered, p.ContainerPort)
		}
	}
	var out []executor.PortMapping
	portIdx := 0
	for port := uint16(StartProxyPort); port < EndProxyPort && portIdx < len(ordered); port++ {
		if port == C2CTLSPort {
			continue
		}
		if _, collision := unique[port]; collision {
			continue
		}
		appPort := ordered[portIdx]
		out = append(out, executor.PortMapping{
			ContainerPort:         appPort,
			ContainerTLSProxyPort: port,
		})
		if appPort == DefaultHTTPPort {
			out = append(out, executor.PortMapping{
				ContainerPort:         appPort,
				ContainerTLSProxyPort: C2CTLSPort,
			})
		}
		portIdx++
	}
	if portIdx < len(ordered) {
		return nil, ErrNoPortsAvailable
	}
	return out, nil
}

// GenerateBootstrap returns envoy bootstrap YAML for the given container and options.
func GenerateBootstrap(
	container executor.Container,
	adminPort uint16,
	requireClientCerts bool,
	adsServers []string,
	http2Enabled bool,
) ([]byte, error) {
	config, err := generateProxyConfig(container, adminPort, requireClientCerts, adsServers, http2Enabled)
	if err != nil {
		return nil, err
	}
	marshaller := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}
	jsonBytes, err := marshaller.Marshal(config)
	if err != nil {
		return nil, err
	}
	return ghodss_yaml.JSONToYAML(jsonBytes)
}

// GenerateSDSCertAndKey returns SDS discovery response YAML for a cert/key secret.
func GenerateSDSCertAndKey(name string, cred Credential) ([]byte, error) {
	secret := generateSDSCertAndKeySecret(name, cred)
	return discoveryResponseToYAML(secret)
}

// GenerateSDSCAResource returns SDS discovery response YAML for the validation context (trusted CA + SAN matchers).
func GenerateSDSCAResource(container executor.Container, idCred Credential, trustedCACerts, verifySubjectAltName []string) ([]byte, error) {
	secret, err := generateSDSCAResourceSecret(container, idCred, trustedCACerts, verifySubjectAltName)
	if err != nil {
		return nil, err
	}
	return discoveryResponseToYAML(secret)
}

func discoveryResponseToYAML(resourceMsg proto.Message) ([]byte, error) {
	resourceAny, err := anypb.New(resourceMsg)
	if err != nil {
		return nil, err
	}
	dr := &envoy_discovery.DiscoveryResponse{
		VersionInfo: "0",
		Resources:   []*anypb.Any{resourceAny},
	}
	jsonMarshaler := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: false}
	jsonBytes, err := jsonMarshaler.Marshal(dr)
	if err != nil {
		return nil, err
	}
	return ghodss_yaml.JSONToYAML(jsonBytes)
}

func envoyAddr(ip string, port uint16) *envoy_core.Address {
	return &envoy_core.Address{
		Address: &envoy_core.Address_SocketAddress{
			SocketAddress: &envoy_core.SocketAddress{
				Address: ip,
				PortSpecifier: &envoy_core.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}

func generateProxyConfig(
	container executor.Container,
	adminPort uint16,
	requireClientCerts bool,
	adsServers []string,
	http2Enabled bool,
) (*envoy_bootstrap.Bootstrap, error) {
	clusters := []*envoy_cluster.Cluster{}
	uniqueContainerPorts := map[uint16]struct{}{}
	for _, portMap := range container.Ports {
		uniqueContainerPorts[portMap.ContainerPort] = struct{}{}
	}
	for containerPort := range uniqueContainerPorts {
		clusterName := fmt.Sprintf("service-cluster-%d", containerPort)
		clusters = append(clusters, &envoy_cluster.Cluster{
			Name:                 clusterName,
			ClusterDiscoveryType: &envoy_cluster.Cluster_Type{Type: envoy_cluster.Cluster_STATIC},
			ConnectTimeout:       &duration.Duration{Nanos: 250000000},
			LoadAssignment: &envoy_endpoint.ClusterLoadAssignment{
				ClusterName: clusterName,
				Endpoints: []*envoy_endpoint.LocalityLbEndpoints{{
					LbEndpoints: []*envoy_endpoint.LbEndpoint{{
						HostIdentifier: &envoy_endpoint.LbEndpoint_Endpoint{
							Endpoint: &envoy_endpoint.Endpoint{
								Address: envoyAddr(container.InternalIP, containerPort),
							},
						},
					}},
				}},
			},
			CircuitBreakers: &envoy_cluster.CircuitBreakers{
				Thresholds: []*envoy_cluster.CircuitBreakers_Thresholds{
					{MaxConnections: &wrappers.UInt32Value{Value: math.MaxUint32}},
					{MaxPendingRequests: &wrappers.UInt32Value{Value: math.MaxUint32}},
					{MaxRequests: &wrappers.UInt32Value{Value: math.MaxUint32}},
				}},
		})
	}
	listeners, err := generateListeners(container, requireClientCerts, http2Enabled)
	if err != nil {
		return nil, fmt.Errorf("generating listeners: %w", err)
	}
	adminAccessLogAny, err := anypb.New(&envoy_file_access_log.FileAccessLog{Path: os.DevNull})
	if err != nil {
		return nil, fmt.Errorf("creating admin access log config: %w", err)
	}
	config := &envoy_bootstrap.Bootstrap{
		Admin: &envoy_bootstrap.Admin{
			Address: envoyAddr("127.0.0.1", adminPort),
			AccessLog: []*envoy_accesslog.AccessLog{
				{
					Name:       "envoy.access_loggers.file",
					ConfigType: &envoy_accesslog.AccessLog_TypedConfig{TypedConfig: adminAccessLogAny},
				},
			},
		},
		StatsConfig: &envoy_metrics.StatsConfig{
			StatsMatcher: &envoy_metrics.StatsMatcher{
				StatsMatcher: &envoy_metrics.StatsMatcher_RejectAll{RejectAll: true},
			},
		},
		Node: &envoy_core.Node{
			Id:      fmt.Sprintf("sidecar~%s~%s~x", container.InternalIP, container.Guid),
			Cluster: "proxy-cluster",
		},
		StaticResources: &envoy_bootstrap.Bootstrap_StaticResources{
			Listeners: listeners,
		},
		LayeredRuntime: &envoy_bootstrap.LayeredRuntime{
			Layers: []*envoy_bootstrap.RuntimeLayer{{
				Name: "static-layer",
				LayerSpecifier: &envoy_bootstrap.RuntimeLayer_StaticLayer{
					StaticLayer: &pstruct.Struct{
						Fields: map[string]*pstruct.Value{
							"envoy": {
								Kind: &pstruct.Value_StructValue{
									StructValue: &pstruct.Struct{
										Fields: map[string]*pstruct.Value{
											"reloadable_features": {
												Kind: &pstruct.Value_StructValue{
													StructValue: &pstruct.Struct{
														Fields: map[string]*pstruct.Value{
															"new_tcp_connection_pool": {Kind: &pstruct.Value_BoolValue{BoolValue: false}},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}},
		},
	}
	if len(adsServers) > 0 {
		var adsEndpoints []*envoy_endpoint.LbEndpoint
		for _, a := range adsServers {
			address, port, err := splitHost(a)
			if err != nil {
				return nil, err
			}
			adsEndpoints = append(adsEndpoints, &envoy_endpoint.LbEndpoint{
				HostIdentifier: &envoy_endpoint.LbEndpoint_Endpoint{
					Endpoint: &envoy_endpoint.Endpoint{Address: envoyAddr(address, port)},
				},
			})
		}
		clusters = append(clusters, &envoy_cluster.Cluster{
			Name:                 AdsClusterName,
			ClusterDiscoveryType: &envoy_cluster.Cluster_Type{Type: envoy_cluster.Cluster_STATIC},
			ConnectTimeout:       &duration.Duration{Nanos: 250000000},
			LoadAssignment: &envoy_endpoint.ClusterLoadAssignment{
				ClusterName: AdsClusterName,
				Endpoints:   []*envoy_endpoint.LocalityLbEndpoints{{LbEndpoints: adsEndpoints}},
			},
		})
		config.DynamicResources = &envoy_bootstrap.Bootstrap_DynamicResources{
			LdsConfig: &envoy_core.ConfigSource{ConfigSourceSpecifier: &envoy_core.ConfigSource_Ads{Ads: &envoy_core.AggregatedConfigSource{}}},
			CdsConfig: &envoy_core.ConfigSource{ConfigSourceSpecifier: &envoy_core.ConfigSource_Ads{Ads: &envoy_core.AggregatedConfigSource{}}},
			AdsConfig: &envoy_core.ApiConfigSource{
				ApiType: envoy_core.ApiConfigSource_GRPC,
				GrpcServices: []*envoy_core.GrpcService{{
					TargetSpecifier: &envoy_core.GrpcService_EnvoyGrpc_{EnvoyGrpc: &envoy_core.GrpcService_EnvoyGrpc{ClusterName: "pilot-ads"}},
				}},
			},
		}
	}
	config.StaticResources.Clusters = clusters
	return config, nil
}

func splitHost(host string) (string, uint16, error) {
	parts := strings.Split(host, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("ads server address is invalid: %s", host)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("ads server address is invalid: %s", host)
	}
	return parts[0], uint16(port), nil
}

func sdsFileConfigSource(filePath, watchDir string) *envoy_core.ConfigSource {
	return &envoy_core.ConfigSource{
		ConfigSourceSpecifier: &envoy_core.ConfigSource_PathConfigSource{
			PathConfigSource: &envoy_core.PathConfigSource{
				Path:             filePath,
				WatchedDirectory: &envoy_core.WatchedDirectory{Path: watchDir},
			},
		},
	}
}

func generateListeners(container executor.Container, requireClientCerts, http2Enabled bool) ([]*envoy_listener.Listener, error) {
	listeners := []*envoy_listener.Listener{}
	sdsDir := "/etc/cf-assets/envoy_config"
	sdsIDCertPath := sdsDir + "/sds-id-cert-and-key.yaml"
	sdsC2CPath := sdsDir + "/sds-c2c-cert-and-key.yaml"
	sdsValidationPath := sdsDir + "/sds-id-validation-context.yaml"
	for _, portMap := range container.Ports {
		clusterName := fmt.Sprintf("service-cluster-%d", portMap.ContainerPort)
		sdsPath := sdsIDCertPath
		sdsSecretName := "id-cert-and-key"
		if portMap.ContainerTLSProxyPort == C2CTLSPort {
			sdsPath = sdsC2CPath
			sdsSecretName = "c2c-cert-and-key"
		}
		tlsContext := &envoy_tls.DownstreamTlsContext{
			CommonTlsContext: &envoy_tls.CommonTlsContext{
				TlsCertificateSdsSecretConfigs: []*envoy_tls.SdsSecretConfig{{
					Name:      sdsSecretName,
					SdsConfig: sdsFileConfigSource(sdsPath, sdsDir),
				}},
				TlsParams: &envoy_tls.TlsParameters{CipherSuites: SupportedCipherSuites},
			},
		}
		if http2Enabled && portMap.ContainerTLSProxyPort != C2CTLSPort {
			tlsContext.CommonTlsContext.AlpnProtocols = AlpnProtocols
		}
		if requireClientCerts && portMap.ContainerTLSProxyPort != C2CTLSPort {
			tlsContext.RequireClientCertificate = &wrappers.BoolValue{Value: requireClientCerts}
			tlsContext.CommonTlsContext.ValidationContextType = &envoy_tls.CommonTlsContext_ValidationContextSdsSecretConfig{
				ValidationContextSdsSecretConfig: &envoy_tls.SdsSecretConfig{
					Name:      "id-validation-context",
					SdsConfig: sdsFileConfigSource(sdsValidationPath, sdsDir),
				},
			}
		}
		tlsContextAny, err := anypb.New(tlsContext)
		if err != nil {
			return nil, err
		}
		filterConfig, err := anypb.New(&envoy_tcp_proxy.TcpProxy{
			StatPrefix:       fmt.Sprintf("stats-%d-%d", portMap.ContainerPort, portMap.ContainerTLSProxyPort),
			ClusterSpecifier: &envoy_tcp_proxy.TcpProxy_Cluster{Cluster: clusterName},
		})
		if err != nil {
			return nil, err
		}
		listenerName := fmt.Sprintf("listener-%d-%d", portMap.ContainerPort, portMap.ContainerTLSProxyPort)
		listeners = append(listeners, &envoy_listener.Listener{
			Name:    listenerName,
			Address: envoyAddr("0.0.0.0", portMap.ContainerTLSProxyPort),
			FilterChains: []*envoy_listener.FilterChain{{
				Filters: []*envoy_listener.Filter{{
					Name:       "envoy.tcp_proxy",
					ConfigType: &envoy_listener.Filter_TypedConfig{TypedConfig: filterConfig},
				}},
				TransportSocket: &envoy_core.TransportSocket{
					Name:       listenerName,
					ConfigType: &envoy_core.TransportSocket_TypedConfig{TypedConfig: tlsContextAny},
				},
			}},
		})
	}
	return listeners, nil
}

func generateSDSCertAndKeySecret(name string, cred Credential) proto.Message {
	return &envoy_tls.Secret{
		Name: name,
		Type: &envoy_tls.Secret_TlsCertificate{
			TlsCertificate: &envoy_tls.TlsCertificate{
				CertificateChain: &envoy_core.DataSource{Specifier: &envoy_core.DataSource_InlineString{InlineString: cred.Cert}},
				PrivateKey:       &envoy_core.DataSource{Specifier: &envoy_core.DataSource_InlineString{InlineString: cred.Key}},
			},
		},
	}
}

func generateSDSCAResourceSecret(container executor.Container, idCred Credential, trustedCaCerts, subjectAltNames []string) (proto.Message, error) {
	certs, err := pemConcatenate(trustedCaCerts)
	if err != nil {
		return nil, err
	}
	var matchers []*envoy_tls.SubjectAltNameMatcher
	for _, s := range subjectAltNames {
		matchers = append(matchers, &envoy_tls.SubjectAltNameMatcher{
			SanType: envoy_tls.SubjectAltNameMatcher_DNS,
			Matcher: &envoy_matcher.StringMatcher{MatchPattern: &envoy_matcher.StringMatcher_Exact{Exact: s}},
		})
	}
	return &envoy_tls.Secret{
		Name: "id-validation-context",
		Type: &envoy_tls.Secret_ValidationContext{
			ValidationContext: &envoy_tls.CertificateValidationContext{
				TrustedCa:                 &envoy_core.DataSource{Specifier: &envoy_core.DataSource_InlineString{InlineString: certs}},
				MatchTypedSubjectAltNames: matchers,
			},
		},
	}, nil
}

const maxTrustedCABytes = 512 * 1024

func pemConcatenate(certs []string) (string, error) {
	// Reject huge single strings up front (avoids large allocation and catches config bug: entire rep.json or cert bundle in one array element).
	var total int
	for _, cert := range certs {
		if cert == "" {
			continue
		}
		if len(cert) > maxTrustedCABytes {
			return "", fmt.Errorf("single trusted CA cert string length %d exceeds maximum (%d bytes); ensure container_proxy_trusted_ca_certs is an array of individual PEM certs, not one large blob", len(cert), maxTrustedCABytes)
		}
		total += len(cert)
	}
	if total >= maxTrustedCABytes {
		return "", fmt.Errorf("total trusted CA certs input size %d exceeds maximum (%d bytes)", total, maxTrustedCABytes)
	}
	var buf bytes.Buffer
	for _, cert := range certs {
		if cert == "" {
			continue
		}
		rest := []byte(cert)
		totalLen := len(rest)
		for {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				if len(rest) == totalLen {
					return "", errors.New("failed to read certificate")
				}
				break
			}
			if err := pem.Encode(&buf, block); err != nil {
				return "", err
			}
		}
	}
	return buf.String(), nil
}
