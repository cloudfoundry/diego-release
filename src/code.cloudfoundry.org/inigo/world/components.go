package world

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	auctioneerconfig "code.cloudfoundry.org/auctioneer/cmd/auctioneer/config"
	"code.cloudfoundry.org/bbs"
	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	bbsrunner "code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/bbs/test_helpers"
	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	sshproxyconfig "code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy/config"
	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/durationjson"
	executorinit "code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/executor/initializer/configuration"
	fileserverconfig "code.cloudfoundry.org/fileserver/cmd/file-server/config"
	"code.cloudfoundry.org/garden"
	gardenclient "code.cloudfoundry.org/garden/client"
	gardenconnection "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/inigo/helpers/certauthority"
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/locket"
	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	"code.cloudfoundry.org/rep"
	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	routeemitterconfig "code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	routingapi "code.cloudfoundry.org/route-emitter/cmd/route-emitter/runners"
	routingapiconfig "code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/volman"
	volmanclient "code.cloudfoundry.org/volman/vollocal"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"golang.org/x/crypto/ssh"
)

var (
	PreloadedStacks = []string{"red-stack", "blue-stack", "cflinuxfs3"}
	DefaultStack    = PreloadedStacks[0]
)

type (
	BuiltExecutables map[string]string
	BuiltLifecycles  map[string]string
)

const (
	LifecycleFilename = "lifecycle.tar.gz"
)

type BuiltArtifacts struct {
	Executables BuiltExecutables
	Lifecycles  BuiltLifecycles
	Healthcheck string
}

type SSHKeys struct {
	HostKey       ssh.Signer
	HostKeyPem    string
	PrivateKeyPem string
	AuthorizedKey string
}

type SSLConfig struct {
	ServerCert string
	ServerKey  string
	ClientCert string
	ClientKey  string
	CACert     string
}

type GardenSettingsConfig struct {
	GrootFSBinPath            string
	GrootFSStorePath          string
	GardenBinPath             string
	GardenGraphPath           string
	UnprivilegedGrootfsConfig GrootFSConfig
	PrivilegedGrootfsConfig   GrootFSConfig
	NetworkPluginConfig       NetworkPluginConfig
}

type NetworkPluginConfig struct {
	NetworkName    string `json:"network_name"`
	SubnetRange    string `json:"subnet_range"`
	GatewayAddress string `json:"gateway_address"`
}

type GrootFSConfig struct {
	StorePath string `yaml:"store"`
	DraxBin   string `yaml:"drax_bin"`
	LogLevel  string `yaml:"log_level"`
	Create    struct {
		JSON                bool     `yaml:"json"`
		UidMappings         []string `yaml:"uid_mappings"`
		GidMappings         []string `yaml:"gid_mappings"`
		SkipLayerValidation bool     `yaml:"skip_layer_validation"`
	}
}

type ComponentAddresses struct {
	NATS                string
	BBS                 string
	Health              string
	Rep                 string
	FileServer          string
	Router              string
	RouterStatus        string
	RouterRoutes        string
	RouterRouteServices string
	Garden              string
	Auctioneer          string
	SSHProxy            string
	SSHProxyHealthCheck string
	FakeVolmanDriver    string
	Locket              string
	SQL                 string
}

func DBInfo() (string, string) {
	var dbBaseConnectionString string
	var dbDriverName string

	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		user = "diego"
	}

	if test_helpers.UsePostgres() {
		dbDriverName = "postgres"
		password, ok := os.LookupEnv("DB_PASSWORD")
		if !ok {
			password = "diego_pw"
		}
		dbBaseConnectionString = fmt.Sprintf("postgres://%s:%s@127.0.0.1/", user, password)
	} else {
		dbDriverName = "mysql"
		password, ok := os.LookupEnv("DB_PASSWORD")
		if !ok {
			password = "diego_password"
		}
		dbBaseConnectionString = fmt.Sprintf("%s:%s@tcp(localhost:3306)/", user, password)
	}

	return dbDriverName, dbBaseConnectionString
}

func makeCommonComponentMaker(builtArtifacts BuiltArtifacts, worldAddresses ComponentAddresses, allocator portauthority.PortAllocator, certAuthority certauthority.CertAuthority) commonComponentMaker {
	startCheckTimeout := 10 * time.Second
	if timeout, found := os.LookupEnv("START_CHECK_TIMEOUT_DURATION"); found && timeout != "" {
		var err error
		startCheckTimeout, err = time.ParseDuration(timeout)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("%s not a valid duration", timeout))
	}

	tmpDir := TempDir("component-maker")

	grootfsBinPath := os.Getenv("GROOTFS_BINPATH")
	grootfsStorePath := os.Getenv("GROOTFS_STORE_PATH")
	gardenBinPath := os.Getenv("GARDEN_BINPATH")
	gardenRootFSPath := os.Getenv("GARDEN_TEST_ROOTFS")
	gardenGraphPath := os.Getenv("GARDEN_GRAPH_PATH")

	if gardenGraphPath == "" {
		gardenGraphPath = TempDirWithParent(tmpDir, "garden-graph")
	}

	Expect(grootfsBinPath).NotTo(BeEmpty(), "must provide $GROOTFS_BINPATH")
	if runtime.GOOS == "windows" {
		Expect(grootfsStorePath).NotTo(BeEmpty(), "must provide $GROOTFS_STORE_PATH")
	}
	Expect(gardenBinPath).NotTo(BeEmpty(), "must provide $GARDEN_BINPATH")
	Expect(gardenRootFSPath).NotTo(BeEmpty(), "must provide $GARDEN_TEST_ROOTFS")

	// tests depend on this env var to be set
	externalAddress := os.Getenv("EXTERNAL_ADDRESS")
	Expect(externalAddress).NotTo(BeEmpty(), "must provide $EXTERNAL_ADDRESS")

	stackPathMap := make(repconfig.RootFSes, len(PreloadedStacks))
	for i, stack := range PreloadedStacks {
		stackPathMap[i] = repconfig.RootFS{
			Name: stack,
			Path: gardenRootFSPath,
		}
	}

	hostKeyPair, err := keys.RSAKeyPairFactory.NewKeyPair(1024)
	Expect(err).NotTo(HaveOccurred())

	userKeyPair, err := keys.RSAKeyPairFactory.NewKeyPair(1024)
	Expect(err).NotTo(HaveOccurred())

	sshKeys := SSHKeys{
		HostKey:       hostKeyPair.PrivateKey(),
		HostKeyPem:    hostKeyPair.PEMEncodedPrivateKey(),
		PrivateKeyPem: userKeyPair.PEMEncodedPrivateKey(),
		AuthorizedKey: userKeyPair.AuthorizedKey(),
	}

	_, caCert := certAuthority.CAAndKey()
	bbsServerKey, bbsServerCert, err := certAuthority.GenerateSelfSignedCertAndKey("bbs_server", []string{"bbs_server"}, false)
	Expect(err).NotTo(HaveOccurred())
	repServerKey, repServerCert, err := certAuthority.GenerateSelfSignedCertAndKey("rep_server", []string{"cell.service.cf.internal", "*.cell.service.cf.internal"}, false)
	Expect(err).NotTo(HaveOccurred())
	auctioneerServerKey, auctioneerServerCert, err := certAuthority.GenerateSelfSignedCertAndKey("auctioneer_server", []string{"auctioneer_server"}, false)
	Expect(err).NotTo(HaveOccurred())
	routingAPIKey, routingAPICert, err := certAuthority.GenerateSelfSignedCertAndKey("routing_api_server", []string{"routing_api_server"}, false)
	Expect(err).NotTo(HaveOccurred())
	clientKey, clientCert, err := certAuthority.GenerateSelfSignedCertAndKey("client", []string{"client"}, false)
	Expect(err).NotTo(HaveOccurred())

	sqlCACert := filepath.Join("..", "fixtures", "certs", "sql-certs", "server-ca.crt")

	bbsSSLConfig := SSLConfig{
		ServerCert: bbsServerCert,
		ServerKey:  bbsServerKey,
		ClientCert: clientCert,
		ClientKey:  clientKey,
		CACert:     caCert,
	}

	locketSSLConfig := SSLConfig{
		ServerCert: bbsServerCert,
		ServerKey:  bbsServerKey,
		ClientCert: clientCert,
		ClientKey:  clientKey,
		CACert:     caCert,
	}

	repSSLConfig := SSLConfig{
		ServerCert: repServerCert,
		ServerKey:  repServerKey,
		ClientCert: clientCert,
		ClientKey:  clientKey,
		CACert:     caCert,
	}

	auctioneerSSLConfig := SSLConfig{
		ServerCert: auctioneerServerCert,
		ServerKey:  auctioneerServerKey,
		ClientCert: clientCert,
		ClientKey:  clientKey,
		CACert:     caCert,
	}

	routingApiSSLConfig := SSLConfig{
		ServerCert: routingAPICert,
		ServerKey:  routingAPIKey,
		CACert:     caCert,
	}

	storeTimestamp := time.Now().UnixNano()

	unprivilegedGrootfsConfig := GrootFSConfig{
		StorePath: fmt.Sprintf("/mnt/garden-storage/unprivileged-%d-%d", GinkgoParallelProcess(), storeTimestamp),
		DraxBin:   "/usr/local/bin/drax",
		LogLevel:  "debug",
	}
	unprivilegedGrootfsConfig.Create.JSON = true
	unprivilegedGrootfsConfig.Create.UidMappings = []string{"0:4294967294:1", "1:1:4294967293"}
	unprivilegedGrootfsConfig.Create.GidMappings = []string{"0:4294967294:1", "1:1:4294967293"}
	unprivilegedGrootfsConfig.Create.SkipLayerValidation = true

	privilegedGrootfsConfig := GrootFSConfig{
		StorePath: fmt.Sprintf("/mnt/garden-storage/privileged-%d-%d", GinkgoParallelProcess(), storeTimestamp),
		DraxBin:   "/usr/local/bin/drax",
		LogLevel:  "debug",
	}
	privilegedGrootfsConfig.Create.JSON = true
	privilegedGrootfsConfig.Create.SkipLayerValidation = true

	networkPluginConfig := NetworkPluginConfig{
		NetworkName:    "winc-nat",
		SubnetRange:    "172.30.0.0/22",
		GatewayAddress: "172.30.0.1",
	}

	gardenConfig := GardenSettingsConfig{
		GrootFSBinPath:            grootfsBinPath,
		GrootFSStorePath:          grootfsStorePath,
		GardenBinPath:             gardenBinPath,
		GardenGraphPath:           gardenGraphPath,
		UnprivilegedGrootfsConfig: unprivilegedGrootfsConfig,
		PrivilegedGrootfsConfig:   privilegedGrootfsConfig,
		NetworkPluginConfig:       networkPluginConfig,
	}

	guid, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())

	volmanConfigDir := TempDirWithParent(tmpDir, guid.String())

	dbDriverName, dbBaseConnectionString := DBInfo()
	return commonComponentMaker{
		artifacts: builtArtifacts,
		addresses: worldAddresses,

		rootFSes: stackPathMap,

		gardenConfig:           gardenConfig,
		sshConfig:              sshKeys,
		bbsSSL:                 bbsSSLConfig,
		locketSSL:              locketSSLConfig,
		repSSL:                 repSSLConfig,
		auctioneerSSL:          auctioneerSSLConfig,
		routingAPISSL:          routingApiSSLConfig,
		sqlCACertFile:          sqlCACert,
		volmanDriverConfigDir:  volmanConfigDir,
		dbDriverName:           dbDriverName,
		dbBaseConnectionString: dbBaseConnectionString,

		portAllocator: allocator,

		startCheckTimeout: startCheckTimeout,

		tmpDir: tmpDir,
	}
}

func MakeV0ComponentMaker(builtArtifacts BuiltArtifacts, worldAddresses ComponentAddresses, allocator portauthority.PortAllocator, certAuthority certauthority.CertAuthority) ComponentMaker {
	return v0ComponentMaker{commonComponentMaker: makeCommonComponentMaker(builtArtifacts, worldAddresses, allocator, certAuthority)}
}

func MakeComponentMaker(builtArtifacts BuiltArtifacts, worldAddresses ComponentAddresses, allocator portauthority.PortAllocator, certAuthority certauthority.CertAuthority) ComponentMaker {
	return v1ComponentMaker{commonComponentMaker: makeCommonComponentMaker(builtArtifacts, worldAddresses, allocator, certAuthority)}
}

type ComponentMaker interface {
	VolmanDriverConfigDir() string
	SSHConfig() SSHKeys
	Artifacts() BuiltArtifacts
	PortAllocator() portauthority.PortAllocator
	Addresses() ComponentAddresses
	Auctioneer(modifyConfigFuncs ...func(cfg *auctioneerconfig.AuctioneerConfig)) *ginkgomon.Runner
	BBS(modifyConfigFuncs ...func(*bbsconfig.BBSConfig)) *ginkgomon.Runner
	BBSClient() bbs.InternalClient
	RepClientFactory() rep.ClientFactory
	BBSServiceClient(logger lager.Logger) serviceclient.ServiceClient
	BBSURL() string
	BBSSSLConfig() SSLConfig
	DefaultStack() string
	FileServer() (ifrit.Runner, string)
	Garden(fs ...func(*runner.GdnRunnerConfig)) *runner.GardenRunner
	GardenClient() garden.Client
	GardenWithoutDefaultStack() ifrit.Runner
	GrootFSDeleteStore()
	GrootFSInitStore()
	Locket(modifyConfigFuncs ...func(*locketconfig.LocketConfig)) ifrit.Runner
	NATS(argv ...string) ifrit.Runner
	Rep(modifyConfigFuncs ...func(*repconfig.RepConfig)) *ginkgomon.Runner
	RepN(n int, modifyConfigFuncs ...func(*repconfig.RepConfig)) *ginkgomon.Runner
	RepSSLConfig() SSLConfig
	RouteEmitter(fs ...func(config *routeemitterconfig.RouteEmitterConfig)) *ginkgomon.Runner
	RouteEmitterN(n int, fs ...func(config *routeemitterconfig.RouteEmitterConfig)) *ginkgomon.Runner
	Router() *ginkgomon.Runner
	RoutingAPI(modifyConfigFuncs ...func(*routingapi.Config)) *routingapi.RoutingAPIRunner
	SQL(argv ...string) ifrit.Runner
	SSHProxy(modifyConfigFuncs ...func(*sshproxyconfig.SSHProxyConfig)) ifrit.Runner
	Setup()
	Teardown()
	VolmanClient(logger lager.Logger) (volman.Manager, ifrit.Runner)
	VolmanDriver(logger lager.Logger) (ifrit.Runner, dockerdriver.Driver)
}

type commonComponentMaker struct {
	artifacts              BuiltArtifacts
	addresses              ComponentAddresses
	rootFSes               repconfig.RootFSes
	gardenConfig           GardenSettingsConfig
	sshConfig              SSHKeys
	bbsSSL                 SSLConfig
	locketSSL              SSLConfig
	repSSL                 SSLConfig
	auctioneerSSL          SSLConfig
	routingAPISSL          SSLConfig
	sqlCACertFile          string
	volmanDriverConfigDir  string
	dbDriverName           string
	dbBaseConnectionString string
	portAllocator          portauthority.PortAllocator
	startCheckTimeout      time.Duration
	tmpDir                 string
}

func (maker commonComponentMaker) VolmanDriverConfigDir() string {
	return maker.volmanDriverConfigDir
}

func (maker commonComponentMaker) SSHConfig() SSHKeys {
	return maker.sshConfig
}

func (maker commonComponentMaker) PortAllocator() portauthority.PortAllocator {
	return maker.portAllocator
}

func (maker commonComponentMaker) Artifacts() BuiltArtifacts {
	return maker.artifacts
}

func (maker commonComponentMaker) Addresses() ComponentAddresses {
	return maker.addresses
}

func (maker commonComponentMaker) BBSSSLConfig() SSLConfig {
	return maker.bbsSSL
}

func (maker commonComponentMaker) RepSSLConfig() SSLConfig {
	return maker.repSSL
}

func (maker commonComponentMaker) Setup() {
	if runtime.GOOS != "windows" {
		maker.GrootFSInitStore()
	}
}

func (maker commonComponentMaker) Teardown() {
	deleteTmpDir := func() error { return os.RemoveAll(maker.tmpDir) }
	if runtime.GOOS != "windows" {
		maker.GrootFSDeleteStore()
		Eventually(deleteTmpDir, time.Minute).Should(Succeed())
	} else {
		// #nosec G104 - auctioneer is not getting stopped on windows. Catching error will cause the test to fail.
		deleteTmpDir()
	}
}

func (maker commonComponentMaker) NATS(argv ...string) ifrit.Runner {
	host, port, err := net.SplitHostPort(maker.addresses.NATS)
	Expect(err).NotTo(HaveOccurred())

	natsServerPath, exists := os.LookupEnv("NATS_SERVER_BINARY")
	if !exists {
		fmt.Println("You need nats-server install set NATS_SERVER_BINARY env variable")
		os.Exit(1)
	}

	return ginkgomon.New(ginkgomon.Config{
		Name:              "nats-server",
		AnsiColorCode:     "30m",
		StartCheck:        "Server is ready",
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			natsServerPath,
			append([]string{
				"--addr", host,
				"--port", port,
			}, argv...)...,
		),
	})
}

func (maker commonComponentMaker) SQL(argv ...string) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		defer GinkgoRecover()

		logger := lagertest.NewTestLogger("component-maker")

		db, err := helpers.Connect(logger, maker.dbDriverName, maker.dbBaseConnectionString, "", false)
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		Eventually(db.Ping).Should(Succeed())

		sqlDBName := fmt.Sprintf("diego_%d", GinkgoParallelProcess())
		// #nosec G104 - ignore errors dropping databases that don't exist, we just want a clean slate
		db.Exec(fmt.Sprintf("DROP DATABASE %s", sqlDBName))
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", sqlDBName))
		Expect(err).NotTo(HaveOccurred())

		dbWithDatabaseNameConnectionString := fmt.Sprintf("%s%s", maker.dbBaseConnectionString, sqlDBName)
		db, err = helpers.Connect(logger, maker.dbDriverName, dbWithDatabaseNameConnectionString, "", false)
		Expect(err).NotTo(HaveOccurred())
		Eventually(db.Ping).Should(Succeed())

		Expect(db.Close()).To(Succeed())

		close(ready)

		<-signals
		db, err = helpers.Connect(logger, maker.dbDriverName, maker.dbBaseConnectionString, "", false)
		Expect(err).NotTo(HaveOccurred())
		Eventually(db.Ping).ShouldNot(HaveOccurred())

		_, err = db.Exec(fmt.Sprintf("DROP DATABASE %s", sqlDBName))
		Expect(err).NotTo(HaveOccurred())

		return nil
	})
}

func (maker commonComponentMaker) GrootFSInitStore() {
	err := maker.grootfsInitStore(maker.gardenConfig.UnprivilegedGrootfsConfig)
	Expect(err).NotTo(HaveOccurred())

	err = maker.grootfsInitStore(maker.gardenConfig.PrivilegedGrootfsConfig)
	Expect(err).NotTo(HaveOccurred())
}

func (maker commonComponentMaker) grootfsInitStore(grootfsConfig GrootFSConfig) error {
	grootfsArgs := []string{}
	grootfsArgs = append(grootfsArgs, "--config", maker.grootfsConfigPath(grootfsConfig))
	grootfsArgs = append(grootfsArgs, "init-store")
	for _, mapping := range grootfsConfig.Create.UidMappings {
		grootfsArgs = append(grootfsArgs, "--uid-mapping", mapping)
	}
	for _, mapping := range grootfsConfig.Create.GidMappings {
		grootfsArgs = append(grootfsArgs, "--gid-mapping", mapping)
	}

	return maker.grootfsRunner(grootfsArgs)
}

func (maker commonComponentMaker) GrootFSDeleteStore() {
	err := maker.grootfsDeleteStore(maker.gardenConfig.UnprivilegedGrootfsConfig)
	Expect(err).NotTo(HaveOccurred())

	err = maker.grootfsDeleteStore(maker.gardenConfig.PrivilegedGrootfsConfig)
	Expect(err).NotTo(HaveOccurred())
}

func (maker commonComponentMaker) grootfsDeleteStore(grootfsConfig GrootFSConfig) error {
	grootfsArgs := []string{}
	grootfsArgs = append(grootfsArgs, "--config", maker.grootfsConfigPath(grootfsConfig))
	grootfsArgs = append(grootfsArgs, "delete-store")
	return maker.grootfsRunner(grootfsArgs)
}

func (maker commonComponentMaker) grootfsRunner(args []string) error {
	cmd := exec.Command(filepath.Join(maker.gardenConfig.GardenBinPath, "grootfs"), args...)
	cmd.Stderr = GinkgoWriter
	cmd.Stdout = GinkgoWriter
	return cmd.Run()
}

func (maker commonComponentMaker) grootfsConfigPath(grootfsConfig GrootFSConfig) string {
	configFile, err := os.CreateTemp("", "grootfs-config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()
	data, err := yaml.Marshal(&grootfsConfig)
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.Write(data)
	Expect(err).NotTo(HaveOccurred())

	return configFile.Name()
}

func (maker commonComponentMaker) networkPluginConfigPath(networkPluginConfig NetworkPluginConfig) string {
	configFile, err := os.CreateTemp(TempDirWithParent(maker.tmpDir, "network-plugin"), "network-plugin-config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()
	data, err := json.Marshal(&networkPluginConfig)
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.Write(data)
	Expect(err).NotTo(HaveOccurred())

	return configFile.Name()
}

func (maker commonComponentMaker) GardenWithoutDefaultStack() ifrit.Runner {
	return maker.garden(false)
}

func (maker commonComponentMaker) Garden(fs ...func(*runner.GdnRunnerConfig)) *runner.GardenRunner {
	return maker.garden(true, fs...)
}

func (maker commonComponentMaker) garden(includeDefaultStack bool, fs ...func(*runner.GdnRunnerConfig)) *runner.GardenRunner {
	defaultRootFS := ""
	if includeDefaultStack {
		defaultRootFS = maker.rootFSes.StackPathMap()[maker.DefaultStack()]
	}

	config := runner.DefaultGdnRunnerConfig(runner.Binaries{
		Tardis: filepath.Join(maker.gardenConfig.GardenBinPath, "tardis"),
	})

	config.GdnBin = maker.artifacts.Executables["garden"]

	if runtime.GOOS == "windows" {
		config.TarBin = filepath.Join(maker.gardenConfig.GardenBinPath, "tar.exe")
		config.InitBin = filepath.Join(maker.gardenConfig.GardenBinPath, "init.exe")
		config.RuntimePluginBin = filepath.Join(maker.gardenConfig.GardenBinPath, "winc.exe")
		config.NSTarBin = filepath.Join(maker.gardenConfig.GardenBinPath, "nstar.exe")
		config.ImagePluginBin = filepath.Join(maker.gardenConfig.GardenBinPath, "grootfs.exe")
		config.ImagePluginExtraArgs = []string{
			"\"--driver-store\"",
			maker.gardenConfig.GrootFSStorePath,
		}
		config.NetworkPluginBin = filepath.Join(maker.gardenConfig.GardenBinPath, "winc-network.exe")
		config.NetworkPluginExtraArgs = []string{
			"\"--configFile\"",
			maker.networkPluginConfigPath(maker.gardenConfig.NetworkPluginConfig),
		}

		maxContainers := uint64(20)
		config.MaxContainers = &maxContainers
	} else {
		config.TarBin = filepath.Join(maker.gardenConfig.GardenBinPath, "tar")
		config.InitBin = filepath.Join(maker.gardenConfig.GardenBinPath, "init")
		config.ExecRunnerBin = filepath.Join(maker.gardenConfig.GardenBinPath, "dadoo")
		config.RuntimePluginBin = filepath.Join(maker.gardenConfig.GardenBinPath, "runc")
		config.NSTarBin = filepath.Join(maker.gardenConfig.GardenBinPath, "nstar")
		config.ImagePluginBin = filepath.Join(maker.gardenConfig.GardenBinPath, "grootfs")
		config.PrivilegedImagePluginBin = filepath.Join(maker.gardenConfig.GardenBinPath, "grootfs")

		// TODO: this is overriding the guardian runner args, which is fine since we
		// don't use tardis (tardis is only required for overlay+xfs)
		config.ImagePluginExtraArgs = []string{
			"\"--config\"",
			maker.grootfsConfigPath(maker.gardenConfig.UnprivilegedGrootfsConfig),
		}

		// TODO: this is overriding the guardian runner args, which is fine since we
		// don't use tardis (tardis is only required for overlay+xfs)
		config.PrivilegedImagePluginExtraArgs = []string{
			"\"--config\"",
			maker.grootfsConfigPath(maker.gardenConfig.PrivilegedGrootfsConfig),
		}

		config.DenyNetworks = []string{"0.0.0.0/0"}
		config.AllowHostAccess = boolPtr(true)

		poolSize := 10
		config.PortPoolSize = &poolSize
		ports, err := maker.portAllocator.ClaimPorts(*config.PortPoolSize)
		startPort := int(ports)
		Expect(err).NotTo(HaveOccurred())
		config.PortPoolStart = &startPort
	}

	config.DefaultRootFS = defaultRootFS

	host, port, err := net.SplitHostPort(maker.addresses.Garden)
	Expect(err).NotTo(HaveOccurred())

	config.BindSocket = ""
	config.BindIP = host

	intPort, err := strconv.Atoi(port)
	Expect(err).NotTo(HaveOccurred())
	config.BindPort = intPtr(intPort)

	for _, f := range fs {
		f(&config)
	}

	gardenRunner := runner.NewGardenRunner(config)
	gardenRunner.Runner.StartCheck = "guardian.started"
	gardenRunner.Runner.StartCheckTimeout = maker.startCheckTimeout

	return gardenRunner
}

func (maker commonComponentMaker) RoutingAPI(modifyConfigFuncs ...func(*routingapi.Config)) *routingapi.RoutingAPIRunner {
	binPath := maker.artifacts.Executables["routing-api"]

	sqlConfig := routingapi.SQLConfig{
		DriverName: maker.dbDriverName,
		DBName:     fmt.Sprintf("routingapi_%d", GinkgoParallelProcess()),
	}

	port, err := maker.portAllocator.ClaimPorts(2)
	Expect(err).NotTo(HaveOccurred())

	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		user = "diego"
	}

	if maker.dbDriverName == "mysql" {
		password, ok := os.LookupEnv("DB_PASSWORD")
		if !ok {
			password = "diego_password"
		}
		sqlConfig.Port = 3306
		sqlConfig.Username = user
		sqlConfig.Password = password
	} else {
		password, ok := os.LookupEnv("DB_PASSWORD")
		if !ok {
			password = "diego_pw"
		}
		sqlConfig.Port = 5432
		sqlConfig.Username = user
		sqlConfig.Password = password
	}

	modifyConfigFuncs = append(modifyConfigFuncs, func(c *routingapi.Config) {
		c.Locket = maker.locketClientConfig()
	})

	modifyConfigFuncs = append(modifyConfigFuncs, func(c *routingapi.Config) {
		c.API = routingapiconfig.APIConfig{
			ListenPort:  port,
			HTTPEnabled: true,

			MTLSListenPort:     port + 2,
			MTLSClientCAPath:   maker.routingAPISSL.CACert,
			MTLSServerCertPath: maker.routingAPISSL.ServerCert,
			MTLSServerKeyPath:  maker.routingAPISSL.ServerKey,
		}
	})

	runner, err := routingapi.NewRoutingAPIRunner(binPath, port+1, sqlConfig, modifyConfigFuncs...)
	Expect(err).NotTo(HaveOccurred())
	return runner
}

func (maker commonComponentMaker) Locket(modifyConfigFuncs ...func(*locketconfig.LocketConfig)) ifrit.Runner {
	return locketrunner.NewLocketRunner(maker.artifacts.Executables["locket"], func(cfg *locketconfig.LocketConfig) {
		cfg.CertFile = maker.locketSSL.ServerCert
		cfg.KeyFile = maker.locketSSL.ServerKey
		cfg.CaFile = maker.locketSSL.CACert
		cfg.DatabaseConnectionString = maker.addresses.SQL
		cfg.DatabaseDriver = maker.dbDriverName
		cfg.ListenAddress = maker.addresses.Locket
		cfg.SQLCACertFile = maker.sqlCACertFile
		cfg.LagerConfig = lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		}

		for _, modifyConfig := range modifyConfigFuncs {
			modifyConfig(cfg)
		}
	})
}

func (maker commonComponentMaker) RouteEmitterN(n int, fs ...func(config *routeemitterconfig.RouteEmitterConfig)) *ginkgomon.Runner {
	name := "route-emitter-" + strconv.Itoa(n)

	configFile, err := os.CreateTemp(TempDirWithParent(maker.tmpDir, "route-emitter"), "route-emitter-config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()

	cfg := routeemitterconfig.RouteEmitterConfig{
		LocketSessionName: name,
		NATSAddresses:     maker.addresses.NATS,
		BBSAddress:        maker.BBSURL(),
		LockRetryInterval: durationjson.Duration(time.Second),
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
		BBSClientCertFile:            maker.bbsSSL.ClientCert,
		BBSClientKeyFile:             maker.bbsSSL.ClientKey,
		BBSCACertFile:                maker.bbsSSL.CACert,
		CommunicationTimeout:         durationjson.Duration(30 * time.Second),
		LockTTL:                      durationjson.Duration(locket.DefaultSessionTTL),
		NATSUsername:                 "nats",
		NATSPassword:                 "nats",
		RouteEmittingWorkers:         20,
		SyncInterval:                 durationjson.Duration(time.Minute),
		TCPRouteTTL:                  durationjson.Duration(2 * time.Minute),
		UnregistrationInterval:       durationjson.Duration(30 * time.Second),
		UnregistrationSendCount:      20,
		EnableTCPEmitter:             false,
		EnableInternalEmitter:        false,
		RegisterDirectInstanceRoutes: false,
		ClientLocketConfig:           maker.locketClientConfig(),
	}

	for _, f := range fs {
		f(&cfg)
	}

	encoder := json.NewEncoder(configFile)
	err = encoder.Encode(&cfg)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              name,
		AnsiColorCode:     "36m",
		StartCheck:        `"` + name + `.watcher.sync.complete"`,
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			maker.artifacts.Executables["route-emitter"],
			"-config", configFile.Name(),
		),
		Cleanup: func() {
			err := os.RemoveAll(configFile.Name())
			Expect(err).ToNot(HaveOccurred())
		},
	})
}

func (maker commonComponentMaker) FileServer() (ifrit.Runner, string) {
	servedFilesDir := TempDirWithParent(maker.tmpDir, "file-server-files")

	configFile, err := os.CreateTemp(TempDirWithParent(maker.tmpDir, "file-server"), "file-server-config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()

	cfg := fileserverconfig.FileServerConfig{
		ServerAddress: maker.addresses.FileServer,
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
		StaticDirectory: servedFilesDir,
	}

	buildpackAppLifeCycleDir := filepath.Join(servedFilesDir, "buildpack_app_lifecycle")
	err = os.Mkdir(buildpackAppLifeCycleDir, 0755)
	Expect(err).NotTo(HaveOccurred())
	file := maker.artifacts.Lifecycles["buildpackapplifecycle"]
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("powershell", "-Command", fmt.Sprintf("cp -Recurse -Force %s %s", file, filepath.Join(buildpackAppLifeCycleDir, "buildpack_app_lifecycle.tgz")))
		} else {
			cmd = exec.Command("cp", file, filepath.Join(buildpackAppLifeCycleDir, "buildpack_app_lifecycle.tgz"))
		}
		err = cmd.Run()
		Expect(err).NotTo(HaveOccurred())
	}

	dockerAppLifeCycleDir := filepath.Join(servedFilesDir, "docker_app_lifecycle")
	err = os.Mkdir(dockerAppLifeCycleDir, 0755)
	Expect(err).NotTo(HaveOccurred())
	file = maker.artifacts.Lifecycles["dockerapplifecycle"]
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("powershell", "-Command", fmt.Sprintf("cp -Recurse -Force %s %s", file, filepath.Join(buildpackAppLifeCycleDir, "docker_app_lifecycle.tgz")))
		} else {
			cmd = exec.Command("cp", file, filepath.Join(dockerAppLifeCycleDir, "docker_app_lifecycle.tgz"))
		}
		err = cmd.Run()
		Expect(err).NotTo(HaveOccurred())
	}

	encoder := json.NewEncoder(configFile)
	err = encoder.Encode(&cfg)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "file-server",
		AnsiColorCode:     "92m",
		StartCheck:        `"file-server.ready"`,
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			maker.artifacts.Executables["file-server"],
			"-config", configFile.Name(),
		),
		Cleanup: func() {
			err := os.RemoveAll(servedFilesDir)
			Expect(err).ToNot(HaveOccurred())
			err = os.RemoveAll(configFile.Name())
			Expect(err).ToNot(HaveOccurred())
		},
	}), servedFilesDir
}

func (maker commonComponentMaker) Router() *ginkgomon.Runner {
	_, routerPort, err := net.SplitHostPort(maker.addresses.Router)
	Expect(err).NotTo(HaveOccurred())

	routerPortInt, err := strconv.Atoi(routerPort)
	Expect(err).NotTo(HaveOccurred())

	_, routerStatusPort, err := net.SplitHostPort(maker.addresses.RouterStatus)
	Expect(err).NotTo(HaveOccurred())

	routerStatusPortInt, err := strconv.Atoi(routerStatusPort)
	Expect(err).NotTo(HaveOccurred())

	_, routerRoutesPort, err := net.SplitHostPort(maker.addresses.RouterRoutes)
	Expect(err).NotTo(HaveOccurred())

	routerRoutesPortInt, err := strconv.Atoi(routerRoutesPort)
	Expect(err).NotTo(HaveOccurred())

	_, routerRouteServicesPort, err := net.SplitHostPort(maker.addresses.RouterRouteServices)
	Expect(err).NotTo(HaveOccurred())

	routerRouteServicesPortInt, err := strconv.Atoi(routerRouteServicesPort)
	Expect(err).NotTo(HaveOccurred())
	natsHost, natsPort, err := net.SplitHostPort(maker.addresses.NATS)
	Expect(err).NotTo(HaveOccurred())

	natsPortInt, err := strconv.Atoi(natsPort)
	Expect(err).NotTo(HaveOccurred())

	routerConfig := `
status:
  port: %d
  user: ""
  pass: ""
  routes:
    port: %d
nats:
  hosts:
  - hostname: %s
    port: %d
  user: ""
  pass: ""
logging:
  file: /dev/stdout
  syslog: ""
  level: info
  loggregator_enabled: false
  metron_address: 127.0.0.1:65534
port: %d
index: 0
zone: ""
tracing:
  enable_zipkin: false
trace_key: ""
access_log:
  file: ""
  enable_streaming: false
enable_access_log_streaming: false
debug_addr: ""
enable_proxy: false
enable_ssl: false
ssl_port: 0
ssl_cert_path: ""
ssl_key_path: ""
sslcertificate:
  certificate: []
  privatekey: null
  ocspstaple: []
  signedcertificatetimestamps: []
  leaf: null
skip_ssl_validation: false
cipher_suites: "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256"
ciphersuites: []
load_balancer_healthy_threshold: 0s
publish_start_message_interval: 0s
suspend_pruning_if_nats_unavailable: false
prune_stale_droplets_interval: 5s
droplet_stale_threshold: 10s
publish_active_apps_interval: 0s
start_response_delay_interval: 1s
endpoint_timeout: 0s
route_services_timeout: 0s
secure_cookies: false
oauth:
  token_endpoint: ""
  port: 0
  skip_ssl_validation: false
  client_name: ""
  client_secret: ""
  ca_certs: ""
routing_api:
  uri: ""
  port: 0
  auth_disabled: false
route_services_internal_server_port: %d
route_services_secret: ""
route_services_secret_decrypt_only: ""
route_services_recommend_https: false
extra_headers_to_log: []
token_fetcher_max_retries: 0
token_fetcher_retry_interval: 0s
token_fetcher_expiration_buffer_time: 0
pid_file: ""
`
	routerConfig = fmt.Sprintf(routerConfig, uint16(routerStatusPortInt), uint16(routerRoutesPortInt), natsHost, uint16(natsPortInt), uint16(routerPortInt), uint16(routerRouteServicesPortInt))

	configFile, err := os.CreateTemp(TempDirWithParent(maker.tmpDir, "router-config"), "router-config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()
	_, err = configFile.Write([]byte(routerConfig))
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "router",
		AnsiColorCode:     "93m",
		StartCheck:        "router.started",
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			maker.artifacts.Executables["router"],
			"-c", configFile.Name(),
		),
		Cleanup: func() {
			err := os.Remove(configFile.Name())
			Expect(err).NotTo(HaveOccurred())
		},
	})
}

func (maker commonComponentMaker) SSHProxy(modifyConfigFuncs ...func(*sshproxyconfig.SSHProxyConfig)) ifrit.Runner {
	sshProxyConfig := sshproxyconfig.SSHProxyConfig{
		Address:            maker.addresses.SSHProxy,
		HealthCheckAddress: maker.addresses.SSHProxyHealthCheck,
		BBSAddress:         maker.BBSURL(),
		BBSCACert:          maker.bbsSSL.CACert,
		BBSClientCert:      maker.bbsSSL.ClientCert,
		BBSClientKey:       maker.bbsSSL.ClientKey,
		EnableDiegoAuth:    true,
		HostKey:            maker.sshConfig.HostKeyPem,
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
		CommunicationTimeout:     durationjson.Duration(10 * time.Second),
		ConnectToInstanceAddress: false,
		IdleConnectionTimeout:    durationjson.Duration(5 * time.Minute),
	}

	for _, f := range modifyConfigFuncs {
		f(&sshProxyConfig)
	}

	configFile, err := os.CreateTemp("", "ssh-proxy-config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()

	encoder := json.NewEncoder(configFile)
	err = encoder.Encode(&sshProxyConfig)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "ssh-proxy",
		AnsiColorCode:     "96m",
		StartCheck:        "ssh-proxy.started",
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			maker.artifacts.Executables["ssh-proxy"],
			[]string{
				"-config", configFile.Name(),
			}...,
		),
	})
}

func (maker commonComponentMaker) DefaultStack() string {
	Expect(maker.rootFSes).NotTo(BeEmpty())
	return maker.rootFSes.Names()[0]
}

func (maker commonComponentMaker) GardenClient() garden.Client {
	return gardenclient.New(gardenconnection.New("tcp", maker.addresses.Garden))
}

func (maker commonComponentMaker) BBSClient() bbs.InternalClient {
	client, err := bbs.NewClient(
		maker.BBSURL(),
		maker.bbsSSL.CACert,
		maker.bbsSSL.ClientCert,
		maker.bbsSSL.ClientKey,
		0, 0,
	)
	Expect(err).NotTo(HaveOccurred())
	return client
}

func (maker commonComponentMaker) RepClientFactory() rep.ClientFactory {
	Expect(maker.repSSL.CACert).To(BeAnExistingFile())

	tlsConfig := rep.TLSConfig{
		RequireTLS:      true,
		CertFile:        maker.repSSL.ClientCert,
		KeyFile:         maker.repSSL.ClientKey,
		CaCertFile:      maker.repSSL.CACert,
		ClientCacheSize: 100,
	}

	client := cfhttp.NewClient(cfhttp.WithRequestTimeout(10 * time.Second))
	factory, err := rep.NewClientFactory(client, client, &tlsConfig)
	Expect(err).NotTo(HaveOccurred())
	return factory
}

func (maker commonComponentMaker) BBSServiceClient(logger lager.Logger) serviceclient.ServiceClient {
	locketClient, err := locket.NewClient(logger, maker.locketClientConfig())
	Expect(err).NotTo(HaveOccurred())

	return serviceclient.NewServiceClient(locketClient)
}

func (maker commonComponentMaker) BBSURL() string {
	return "https://" + maker.addresses.BBS
}

func (maker commonComponentMaker) VolmanClient(logger lager.Logger) (volman.Manager, ifrit.Runner) {
	driverConfig := volmanclient.NewDriverConfig()
	driverConfig.DriverPaths = []string{path.Join(maker.volmanDriverConfigDir, fmt.Sprintf("node-%d", GinkgoParallelProcess()))}

	metronClient, err := loggingclient.NewIngressClient(loggingclient.Config{})
	Expect(err).NotTo(HaveOccurred())
	return volmanclient.NewServer(logger, metronClient, driverConfig)
}

func (maker commonComponentMaker) VolmanDriver(logger lager.Logger) (ifrit.Runner, dockerdriver.Driver) {
	debugServerPort, err := maker.portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	debugServerAddress := fmt.Sprintf("0.0.0.0:%d", debugServerPort)
	fakeDriverRunner := ginkgomon.New(ginkgomon.Config{
		Name: "local-driver",
		Command: exec.Command(
			maker.artifacts.Executables["local-driver"],
			"-listenAddr", maker.addresses.FakeVolmanDriver,
			"-debugAddr", debugServerAddress,
			"-mountDir", maker.volmanDriverConfigDir,
			"-logLevel", "debug",
			"-driversPath", path.Join(maker.volmanDriverConfigDir, fmt.Sprintf("node-%d", GinkgoParallelProcess())),
			"-transport", "tcp-json",
			"-uniqueVolumeIds",
		),
		StartCheck: "localdriver-server.started",
	})

	client, err := driverhttp.NewRemoteClient("http://"+maker.addresses.FakeVolmanDriver, nil)
	Expect(err).NotTo(HaveOccurred())

	return fakeDriverRunner, client
}

func (maker commonComponentMaker) locketClientConfig() locket.ClientLocketConfig {
	return locket.ClientLocketConfig{
		LocketAddress:        maker.addresses.Locket,
		LocketCACertFile:     maker.locketSSL.CACert,
		LocketClientCertFile: maker.locketSSL.ClientCert,
		LocketClientKeyFile:  maker.locketSSL.ClientKey,
	}
}

type v1ComponentMaker struct {
	commonComponentMaker
}

type v0ComponentMaker struct {
	commonComponentMaker
}

func (maker v0ComponentMaker) Auctioneer(modifyConfigFuncs ...func(*auctioneerconfig.AuctioneerConfig)) *ginkgomon.Runner {
	cfg := auctioneerconfig.AuctioneerConfig{
		BBSAddress:        maker.BBSURL(),
		BBSCACertFile:     maker.bbsSSL.CACert,
		BBSClientCertFile: maker.bbsSSL.ClientCert,
		BBSClientKeyFile:  maker.bbsSSL.ClientKey,
		ListenAddress:     maker.addresses.Auctioneer,
		LockRetryInterval: durationjson.Duration(1 * time.Second),
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
		RepCACert:               maker.repSSL.CACert,
		RepClientCert:           maker.repSSL.ClientCert,
		RepClientKey:            maker.repSSL.ClientKey,
		StartingContainerWeight: 0.33,
	}

	for _, f := range modifyConfigFuncs {
		f(&cfg)
	}

	args := []string{
		"-bbsAddress", cfg.BBSAddress,
		"-bbsCACert", cfg.BBSCACertFile,
		"-bbsClientCert", cfg.BBSClientCertFile,
		"-bbsClientKey", cfg.BBSClientKeyFile,
		"-listenAddr", cfg.ListenAddress,
		"-lockRetryInterval", time.Duration(cfg.LockRetryInterval).String(),
		"-logLevel", cfg.LagerConfig.LogLevel,
		"-repCACert", cfg.RepCACert,
		"-repClientCert", cfg.RepClientCert,
		"-repClientKey", cfg.RepClientKey,
		"-startingContainerWeight", strconv.FormatFloat(cfg.StartingContainerWeight, 'f', -1, 64),
	}

	return ginkgomon.New(ginkgomon.Config{
		Name:              "auctioneer",
		AnsiColorCode:     "35m",
		StartCheck:        `"auctioneer.started"`,
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			maker.artifacts.Executables["auctioneer"],
			args...,
		),
	})
}

func (maker v0ComponentMaker) RouteEmitter(modifyConfigFuncs ...func(config *routeemitterconfig.RouteEmitterConfig)) *ginkgomon.Runner {
	cfg := routeemitterconfig.RouteEmitterConfig{
		NATSAddresses:     maker.addresses.NATS,
		BBSAddress:        maker.BBSURL(),
		BBSClientCertFile: maker.bbsSSL.ClientCert,
		BBSClientKeyFile:  maker.bbsSSL.ClientKey,
		BBSCACertFile:     maker.bbsSSL.CACert,
		LockRetryInterval: durationjson.Duration(1 * time.Second),
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
	}

	for _, f := range modifyConfigFuncs {
		f(&cfg)
	}

	return ginkgomon.New(ginkgomon.Config{
		Name:              "route-emitter",
		AnsiColorCode:     "36m",
		StartCheck:        `"route-emitter.started"`,
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			maker.artifacts.Executables["route-emitter"],
			[]string{
				"-natsAddresses", cfg.NATSAddresses,
				"-bbsAddress", cfg.BBSAddress,
				"-lockRetryInterval", time.Duration(cfg.LockRetryInterval).String(),
				"-logLevel", cfg.LogLevel,
				"-bbsClientCert", cfg.BBSClientCertFile,
				"-bbsClientKey", cfg.BBSClientKeyFile,
				"-bbsCACert", cfg.BBSCACertFile,
			}...,
		),
	})
}

func (maker v0ComponentMaker) FileServer() (ifrit.Runner, string) {
	servedFilesDir := TempDirWithParent(maker.tmpDir, fmt.Sprintf("file-server-files-%d-", GinkgoParallelProcess()))

	return ginkgomon.New(ginkgomon.Config{
		Name:              "file-server",
		AnsiColorCode:     "92m",
		StartCheck:        `"file-server.ready"`,
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			maker.artifacts.Executables["file-server"],
			[]string{
				"-address", maker.addresses.FileServer,
				"-logLevel", "debug",
				"-staticDirectory", servedFilesDir,
			}...,
		),
		Cleanup: func() {
			err := os.RemoveAll(servedFilesDir)
			Expect(err).NotTo(HaveOccurred())
		},
	}), servedFilesDir
}

func (maker v0ComponentMaker) BBS(modifyConfigFuncs ...func(*bbsconfig.BBSConfig)) *ginkgomon.Runner {
	cfg := bbsconfig.BBSConfig{
		AdvertiseURL:             maker.BBSURL(),
		AuctioneerAddress:        "http://" + maker.addresses.Auctioneer,
		CaFile:                   maker.bbsSSL.CACert,
		CertFile:                 maker.bbsSSL.ServerCert,
		KeyFile:                  maker.bbsSSL.ServerKey,
		DatabaseConnectionString: maker.addresses.SQL,
		DatabaseDriver:           maker.dbDriverName,
		EncryptionConfig: encryption.EncryptionConfig{
			ActiveKeyLabel: "secure-key-1",
			EncryptionKeys: map[string]string{"secure-key-1": "secure-passphrase"},
		},
		HealthAddress: maker.addresses.Health,
		ListenAddress: maker.addresses.BBS,
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
		RepCACert:     maker.repSSL.CACert,
		RepClientCert: maker.repSSL.ClientCert,
		RepClientKey:  maker.repSSL.ClientKey,
	}

	for _, f := range modifyConfigFuncs {
		f(&cfg)
	}

	akl := cfg.ActiveKeyLabel
	encryptionKey := fmt.Sprintf("%s:%s", akl, cfg.EncryptionKeys[akl])

	args := []string{
		"-activeKeyLabel", akl,
		"-advertiseURL", cfg.AdvertiseURL,
		"-auctioneerAddress", cfg.AuctioneerAddress,
		"-caFile", cfg.CaFile,
		"-certFile", cfg.CertFile,
		"-keyFile", cfg.KeyFile,
		"-databaseConnectionString", cfg.DatabaseConnectionString,
		"-databaseDriver", cfg.DatabaseDriver,
		"-encryptionKey", encryptionKey,
		"-healthAddress", cfg.HealthAddress,
		"-listenAddress", cfg.ListenAddress,
		"-logLevel", cfg.LogLevel,
		"-repCACert", cfg.RepCACert,
		"-repClientCert", cfg.RepClientCert,
		"-repClientKey", cfg.RepClientKey,
		"-requireSSL",
	}

	return ginkgomon.New(ginkgomon.Config{
		Name:              "bbs",
		AnsiColorCode:     "32m",
		StartCheck:        "bbs.started",
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			maker.artifacts.Executables["bbs"],
			args...,
		),
	})
}

func (maker v0ComponentMaker) Rep(modifyConfigFuncs ...func(*repconfig.RepConfig)) *ginkgomon.Runner {
	return maker.RepN(0, modifyConfigFuncs...)
}

func (maker v0ComponentMaker) RepN(n int, modifyConfigFuncs ...func(*repconfig.RepConfig)) *ginkgomon.Runner {
	host, portString, err := net.SplitHostPort(maker.addresses.Rep)
	Expect(err).NotTo(HaveOccurred())
	port, err := strconv.Atoi(portString)
	Expect(err).NotTo(HaveOccurred())

	name := "rep-" + strconv.Itoa(n)

	executorTempDir := TempDirWithParent(maker.tmpDir, "executor")
	cachePath := TempDirWithParent(executorTempDir, "cache")

	cfg := repconfig.RepConfig{
		SessionName:               name,
		BBSAddress:                maker.BBSURL(),
		BBSCACertFile:             maker.bbsSSL.CACert,
		BBSClientCertFile:         maker.bbsSSL.ClientCert,
		BBSClientKeyFile:          maker.bbsSSL.ClientKey,
		CaCertFile:                maker.repSSL.CACert,
		ServerCertFile:            maker.repSSL.ServerCert,
		ServerKeyFile:             maker.repSSL.ServerKey,
		CellID:                    "cell_z1" + "-" + strconv.Itoa(n) + "-" + strconv.Itoa(GinkgoParallelProcess()),
		Zone:                      "z1",
		EvacuationPollingInterval: durationjson.Duration(1 * time.Second),
		EvacuationTimeout:         durationjson.Duration(10 * time.Second),
		ExecutorConfig: executorinit.ExecutorConfig{
			CachePath:                    cachePath,
			ContainerMaxCpuShares:        1024,
			ExportNetworkEnvVars:         true,
			GardenAddr:                   maker.addresses.Garden,
			GardenHealthcheckProcessUser: "vcap",
			GardenNetwork:                "tcp",
			TempDir:                      executorTempDir,
			VolmanDriverPaths:            path.Join(maker.volmanDriverConfigDir, fmt.Sprintf("node-%d", GinkgoParallelProcess())),
		},
		ListenAddr:          fmt.Sprintf("%s:%d", host, offsetPort(port, n)),
		ListenAddrSecurable: fmt.Sprintf("%s:%d", host, offsetPort(port+100, n)),
		LockRetryInterval:   durationjson.Duration(1 * time.Second),
		LockTTL:             durationjson.Duration(10 * time.Second),
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
		PollingInterval:    durationjson.Duration(1 * time.Second),
		ReportInterval:     durationjson.Duration(1 * time.Minute),
		SupportedProviders: []string{"docker"},
	}

	if runtime.GOOS == "windows" {
		cfg.GardenHealthcheckProcessPath = "C:\\windows\\system32\\cmd.exe"
		cfg.GardenHealthcheckProcessArgs = []string{"/c", "dir"}
	} else {
		cfg.GardenHealthcheckProcessPath = "/bin/sh"
		cfg.GardenHealthcheckProcessArgs = []string{"-c", "echo", "foo"}
	}

	// for _, rootfs := range maker.rootFSes {
	// 	cfg.PreloadedRootFS = append(cfg.PreloadedRootFS, repconfig.RootFS{Name: rootfs.Name, Path: rootfs.Path})
	// }

	for _, f := range modifyConfigFuncs {
		f(&cfg)
	}

	args := []string{
		"-sessionName", cfg.SessionName,
		"-bbsAddress", cfg.BBSAddress,
		"-bbsCACert", cfg.BBSCACertFile,
		"-bbsClientCert", cfg.BBSClientCertFile,
		"-bbsClientKey", cfg.BBSClientKeyFile,
		"-caFile", cfg.CaCertFile,
		"-certFile", cfg.ServerCertFile,
		"-keyFile", cfg.ServerKeyFile,
		"-cachePath", cfg.CachePath,
		"-cellID", cfg.CellID,
		"--zone", cfg.Zone,
		"-containerMaxCpuShares", strconv.FormatUint(cfg.ContainerMaxCpuShares, 10),
		"-enableLegacyApiServer=false",
		"-evacuationPollingInterval", time.Duration(cfg.EvacuationPollingInterval).String(),
		"-evacuationTimeout", time.Duration(cfg.EvacuationTimeout).String(),
		fmt.Sprintf("-exportNetworkEnvVars=%t", cfg.ExportNetworkEnvVars),
		"-gardenAddr", cfg.GardenAddr,
		"-gardenHealthcheckProcessPath", cfg.GardenHealthcheckProcessPath,
		"-gardenHealthcheckProcessArgs", strings.Join(cfg.GardenHealthcheckProcessArgs, ","),
		"-gardenHealthcheckProcessUser", cfg.GardenHealthcheckProcessUser,
		"-gardenNetwork", cfg.GardenNetwork,
		"-listenAddr", cfg.ListenAddr,
		"-listenAddrSecurable", cfg.ListenAddrSecurable,
		"-lockRetryInterval", time.Duration(cfg.LockRetryInterval).String(),
		"-lockTTL", time.Duration(cfg.LockTTL).String(),
		"-logLevel", cfg.LogLevel,
		"-pollingInterval", time.Duration(cfg.PollingInterval).String(),
		"-requireTLS=true",
		"-rootFSProvider", cfg.SupportedProviders[0],
		"-tempDir", cfg.TempDir,
		"-volmanDriverPaths", cfg.VolmanDriverPaths,
	}

	for _, rootfs := range maker.rootFSes {
		args = append(args, "-preloadedRootFS", fmt.Sprintf("%s:%s", rootfs.Name, rootfs.Path))
	}

	return ginkgomon.New(ginkgomon.Config{
		Name:          name,
		AnsiColorCode: "33m",
		StartCheck:    `"` + name + `.started"`,
		// rep is not started until it can ping an executor and run a healthcheck
		// container on garden; this can take a bit to start, so account for it
		StartCheckTimeout: 2 * time.Minute,
		Command: exec.Command(
			maker.artifacts.Executables["rep"],
			args...,
		),
	})
}

func (maker v1ComponentMaker) BBS(modifyConfigFuncs ...func(*bbsconfig.BBSConfig)) *ginkgomon.Runner {
	config := bbsconfig.BBSConfig{
		SessionName:                 "bbs",
		CommunicationTimeout:        durationjson.Duration(10 * time.Second),
		DesiredLRPCreationTimeout:   durationjson.Duration(1 * time.Minute),
		ExpireCompletedTaskDuration: durationjson.Duration(2 * time.Minute),
		ExpirePendingTaskDuration:   durationjson.Duration(30 * time.Minute),
		ConvergeRepeatInterval:      durationjson.Duration(30 * time.Second),
		KickTaskDuration:            durationjson.Duration(30 * time.Second),
		LockTTL:                     durationjson.Duration(locket.DefaultSessionTTL),
		LockRetryInterval:           durationjson.Duration(locket.RetryInterval),
		ReportInterval:              durationjson.Duration(1 * time.Minute),
		ConvergenceWorkers:          20,
		UpdateWorkers:               1000,
		TaskCallbackWorkers:         1000,
		MaxOpenDatabaseConnections:  200,
		MaxIdleDatabaseConnections:  200,
		RepClientSessionCacheSize:   0,

		AdvertiseURL: maker.BBSURL(),
		EncryptionConfig: encryption.EncryptionConfig{
			ActiveKeyLabel: "secure-key-1",
			EncryptionKeys: map[string]string{
				"secure-key-1": "secure-passphrase",
			},
		},
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
		AuctioneerAddress:        "https://" + maker.addresses.Auctioneer,
		ListenAddress:            maker.addresses.BBS,
		HealthAddress:            maker.addresses.Health,
		RequireSSL:               true,
		CertFile:                 maker.bbsSSL.ServerCert,
		KeyFile:                  maker.bbsSSL.ServerKey,
		CaFile:                   maker.bbsSSL.CACert,
		RepCACert:                maker.repSSL.CACert,
		RepClientCert:            maker.repSSL.ClientCert,
		RepClientKey:             maker.repSSL.ClientKey,
		AuctioneerCACert:         maker.auctioneerSSL.CACert,
		AuctioneerClientCert:     maker.auctioneerSSL.ClientCert,
		AuctioneerClientKey:      maker.auctioneerSSL.ClientKey,
		DatabaseConnectionString: maker.addresses.SQL,
		DatabaseDriver:           maker.dbDriverName,
		AuctioneerRequireTLS:     true,
		SQLCACertFile:            maker.sqlCACertFile,
		ClientLocketConfig:       maker.locketClientConfig(),
		UUID:                     "bbs-inigo-lock-owner",
	}

	for _, modifyConfig := range modifyConfigFuncs {
		modifyConfig(&config)
	}

	runner := bbsrunner.New(maker.artifacts.Executables["bbs"], config)
	runner.AnsiColorCode = "32m"
	runner.StartCheckTimeout = maker.startCheckTimeout
	return runner
}

func (maker v1ComponentMaker) Rep(modifyConfigFuncs ...func(*repconfig.RepConfig)) *ginkgomon.Runner {
	return maker.RepN(0, modifyConfigFuncs...)
}

func (maker v1ComponentMaker) RepN(n int, modifyConfigFuncs ...func(*repconfig.RepConfig)) *ginkgomon.Runner {
	host, portString, err := net.SplitHostPort(maker.addresses.Rep)
	Expect(err).NotTo(HaveOccurred())
	port, err := strconv.Atoi(portString)
	Expect(err).NotTo(HaveOccurred())

	name := "rep-" + strconv.Itoa(n)

	executorTempDir := TempDirWithParent(maker.tmpDir, "executor")
	cachePath := TempDirWithParent(executorTempDir, "cache")

	// garden 1.16.5 checks the source of the bind mount for mount options.
	// Furthermore Rep in version 1.25.2 bind mounted the healthcheck binaries
	// unconditionally without paying attention to
	// EnableDeclarativeHealthcheck.  We need to ensure that the source exist.
	// see
	// https://github.com/cloudfoundry/guardian/commit/1407257d989b483c64ea7d7cb6ea7d071fa75e84
	healthcheckDummyDir := TempDirWithParent(maker.tmpDir, "healthcheck")

	repConfig := repconfig.RepConfig{
		AdvertiseDomain:           "cell.service.cf.internal",
		BBSClientSessionCacheSize: 0,
		BBSMaxIdleConnsPerHost:    0,
		CommunicationTimeout:      durationjson.Duration(10 * time.Second),

		SessionName:               name,
		SupportedProviders:        []string{"docker"},
		BBSAddress:                maker.BBSURL(),
		BBSClientCertFile:         maker.bbsSSL.ClientCert,
		BBSClientKeyFile:          maker.bbsSSL.ClientKey,
		BBSCACertFile:             maker.bbsSSL.CACert,
		ListenAddr:                fmt.Sprintf("%s:%d", host, offsetPort(port, n)),
		CellID:                    "the-cell-id-" + strconv.Itoa(GinkgoParallelProcess()) + "-" + strconv.Itoa(n),
		PollingInterval:           durationjson.Duration(1 * time.Second),
		ReportInterval:            durationjson.Duration(1 * time.Minute),
		EvacuationPollingInterval: durationjson.Duration(1 * time.Second),
		EvacuationTimeout:         durationjson.Duration(1 * time.Second),
		LockTTL:                   durationjson.Duration(10 * time.Second),
		LockRetryInterval:         durationjson.Duration(1 * time.Second),
		ServerCertFile:            maker.repSSL.ServerCert,
		ServerKeyFile:             maker.repSSL.ServerKey,
		CertFile:                  maker.repSSL.ServerCert,
		KeyFile:                   maker.repSSL.ServerKey,
		CaCertFile:                maker.repSSL.CACert,
		ListenAddrSecurable:       fmt.Sprintf("%s:%d", host, offsetPort(port+100, n)),
		PreloadedRootFS:           maker.rootFSes,
		ClientLocketConfig:        maker.locketClientConfig(),
		ExecutorConfig: executorinit.ExecutorConfig{
			MemoryMB:                           configuration.Automatic,
			DiskMB:                             configuration.Automatic,
			ReservedExpirationTime:             durationjson.Duration(time.Minute),
			ContainerReapInterval:              durationjson.Duration(time.Minute),
			ContainerInodeLimit:                200000,
			EnableDeclarativeHealthcheck:       false,
			DeclarativeHealthcheckPath:         healthcheckDummyDir,
			MaxCacheSizeInBytes:                10 * 1024 * 1024 * 1024,
			SkipCertVerify:                     false,
			HealthyMonitoringInterval:          durationjson.Duration(30 * time.Second),
			UnhealthyMonitoringInterval:        durationjson.Duration(500 * time.Millisecond),
			CreateWorkPoolSize:                 32,
			DeleteWorkPoolSize:                 32,
			ReadWorkPoolSize:                   64,
			MetricsWorkPoolSize:                8,
			HealthCheckWorkPoolSize:            64,
			MaxConcurrentDownloads:             5,
			GardenHealthcheckInterval:          durationjson.Duration(10 * time.Minute),
			GardenHealthcheckEmissionInterval:  durationjson.Duration(30 * time.Second),
			GardenHealthcheckTimeout:           durationjson.Duration(10 * time.Minute),
			GardenHealthcheckCommandRetryPause: durationjson.Duration(time.Second),
			GardenHealthcheckProcessEnv:        []string{},
			GracefulShutdownInterval:           durationjson.Duration(10 * time.Second),
			ContainerMetricsReportInterval:     durationjson.Duration(15 * time.Second),
			EnvoyConfigRefreshDelay:            durationjson.Duration(time.Second),
			EnvoyDrainTimeout:                  durationjson.Duration(15 * time.Minute),

			EnableUnproxiedPortMappings:   true,
			GardenNetwork:                 "tcp",
			GardenAddr:                    maker.addresses.Garden,
			ContainerMaxCpuShares:         1024,
			CachePath:                     cachePath,
			TempDir:                       executorTempDir,
			GardenHealthcheckProcessUser:  "vcap",
			VolmanDriverPaths:             path.Join(maker.volmanDriverConfigDir, fmt.Sprintf("node-%d", GinkgoParallelProcess())),
			ContainerOwnerName:            "executor-" + strconv.Itoa(n),
			HealthCheckContainerOwnerName: "executor-health-check-" + strconv.Itoa(n),
			PathToTLSCert:                 maker.repSSL.ServerCert,
			PathToTLSKey:                  maker.repSSL.ServerKey,
			PathToTLSCACert:               maker.repSSL.CACert,
		},
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
	}

	if runtime.GOOS == "windows" {
		repConfig.GardenHealthcheckProcessPath = "C:\\windows\\system32\\cmd.exe"
		repConfig.GardenHealthcheckProcessArgs = []string{"/c", "dir"}
	} else {
		repConfig.GardenHealthcheckProcessPath = "/bin/sh"
		repConfig.GardenHealthcheckProcessArgs = []string{"-c", "echo", "foo"}
		repConfig.ExtraRootfsDir = filepath.Join("..", "fixtures")
	}

	for _, modifyConfig := range modifyConfigFuncs {
		modifyConfig(&repConfig)
	}

	configFile, err := os.CreateTemp(TempDirWithParent(maker.tmpDir, "rep-config"), "rep-config")
	Expect(err).NotTo(HaveOccurred())

	defer configFile.Close()

	err = json.NewEncoder(configFile).Encode(repConfig)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:          name,
		AnsiColorCode: "33m",
		StartCheck:    `"` + name + `.started"`,
		// rep is not started until it can ping an executor and run a healthcheck
		// container on garden; this can take a bit to start, so account for it
		StartCheckTimeout: 2 * time.Minute,
		Command: exec.Command(
			maker.artifacts.Executables["rep"],
			"-config", configFile.Name()),
	})
}

func (maker v1ComponentMaker) Auctioneer(modifyConfigFuncs ...func(cfg *auctioneerconfig.AuctioneerConfig)) *ginkgomon.Runner {
	auctioneerConfig := auctioneerconfig.AuctioneerConfig{
		AuctionRunnerWorkers:          1000,
		CellStateTimeout:              durationjson.Duration(1 * time.Second),
		CommunicationTimeout:          durationjson.Duration(10 * time.Second),
		LockTTL:                       durationjson.Duration(locket.DefaultSessionTTL),
		ReportInterval:                durationjson.Duration(1 * time.Minute),
		StartingContainerCountMaximum: 0,

		BBSAddress:              maker.BBSURL(),
		ListenAddress:           maker.addresses.Auctioneer,
		LockRetryInterval:       durationjson.Duration(time.Second),
		BBSClientCertFile:       maker.bbsSSL.ClientCert,
		BBSClientKeyFile:        maker.bbsSSL.ClientKey,
		BBSCACertFile:           maker.bbsSSL.CACert,
		StartingContainerWeight: 0.33,
		RepCACert:               maker.repSSL.CACert,
		RepClientCert:           maker.repSSL.ClientCert,
		RepClientKey:            maker.repSSL.ClientKey,
		CACertFile:              maker.auctioneerSSL.CACert,
		ServerCertFile:          maker.auctioneerSSL.ServerCert,
		ServerKeyFile:           maker.auctioneerSSL.ServerKey,
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   "debug",
			TimeFormat: lagerflags.FormatRFC3339,
		},
		ClientLocketConfig: maker.locketClientConfig(),
		UUID:               "auctioneer-inigo-lock-owner",
	}

	for _, modifyConfig := range modifyConfigFuncs {
		modifyConfig(&auctioneerConfig)
	}

	configFile, err := os.CreateTemp(TempDirWithParent(maker.tmpDir, "auctioneer-"), "auctioneer-config-")
	Expect(err).NotTo(HaveOccurred())

	err = json.NewEncoder(configFile).Encode(auctioneerConfig)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "auctioneer",
		AnsiColorCode:     "35m",
		StartCheck:        `"auctioneer.started"`,
		StartCheckTimeout: maker.startCheckTimeout,
		Command: exec.Command(
			maker.artifacts.Executables["auctioneer"],
			"-config", configFile.Name(),
		),
	})
}

func (maker v1ComponentMaker) RouteEmitter(modifyConfigFuncs ...func(config *routeemitterconfig.RouteEmitterConfig)) *ginkgomon.Runner {
	return maker.RouteEmitterN(0, modifyConfigFuncs...)
}

func (blc *BuiltLifecycles) BuildLifecycles(lifeCycle string, tmpDir string) {
	lifeCyclePath := filepath.Join("code.cloudfoundry.org", lifeCycle)

	builderPath, err := gexec.Build(filepath.Join(lifeCyclePath, "builder"), "-race")
	Expect(err).NotTo(HaveOccurred())

	launcherPath, err := gexec.Build(filepath.Join(lifeCyclePath, "launcher"), "-race")
	Expect(err).NotTo(HaveOccurred())

	healthcheckPath, err := gexec.Build("code.cloudfoundry.org/healthcheck/cmd/healthcheck", "-race")
	Expect(err).NotTo(HaveOccurred())

	err = os.Setenv("CGO_ENABLED", "0")
	Expect(err).ToNot(HaveOccurred())

	diegoSSHPath, err := gexec.Build("code.cloudfoundry.org/diego-ssh/cmd/sshd", "-a", "-installsuffix", "static")
	os.Unsetenv("CGO_ENABLED")
	Expect(err).NotTo(HaveOccurred())

	lifecycleDir := TempDirWithParent(tmpDir, lifeCycle)

	err = os.Rename(builderPath, filepath.Join(lifecycleDir, "builder"))
	Expect(err).NotTo(HaveOccurred())

	err = os.Rename(healthcheckPath, filepath.Join(lifecycleDir, "healthcheck"))
	Expect(err).NotTo(HaveOccurred())

	err = os.Rename(launcherPath, filepath.Join(lifecycleDir, "launcher"))
	Expect(err).NotTo(HaveOccurred())

	err = os.Rename(diegoSSHPath, filepath.Join(lifecycleDir, "diego-sshd"))
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("tar", "-czf", "lifecycle.tar.gz", "builder", "launcher", "healthcheck", "diego-sshd")
	cmd.Stderr = GinkgoWriter
	cmd.Stdout = GinkgoWriter
	cmd.Dir = lifecycleDir
	err = cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	(*blc)[lifeCycle] = filepath.Join(lifecycleDir, LifecycleFilename)
}

// offsetPort retuns a new port offest by a given number in such a way
// that it does not interfere with the ginkgo parallel node offest in the base port.
func offsetPort(basePort, offset int) int {
	return basePort + (10 * offset)
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
