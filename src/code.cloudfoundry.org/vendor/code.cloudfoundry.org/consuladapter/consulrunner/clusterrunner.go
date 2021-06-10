package consulrunner

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/consuladapter"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type ClusterRunner struct {
	startingPort      int
	numNodes          int
	consulProcesses   []ifrit.Process
	running           bool
	dataDir           string
	configDir         string
	scheme            string
	verifyConnections bool
	caCert            string
	clientCert        string
	clientKey         string
	sessionTTL        time.Duration

	mutex *sync.RWMutex
}

type ClusterRunnerConfig struct {
	StartingPort int
	NumNodes     int
	Scheme       string
	CACert       string
	ClientCert   string
	ClientKey    string
}

const defaultDataDirPrefix = "consul_data"
const defaultConfigDirPrefix = "consul_config"

func NewClusterRunner(c ClusterRunnerConfig) *ClusterRunner {
	Expect(c.StartingPort).To(BeNumerically(">", 0))
	Expect(c.StartingPort).To(BeNumerically("<", 1<<16))
	Expect(c.NumNodes).To(BeNumerically(">", 0))

	verifyConnections := (c.Scheme == "https")
	return &ClusterRunner{
		startingPort:      c.StartingPort,
		numNodes:          c.NumNodes,
		sessionTTL:        5 * time.Second,
		scheme:            c.Scheme,
		verifyConnections: verifyConnections,
		caCert:            c.CACert,
		clientCert:        c.ClientCert,
		clientKey:         c.ClientKey,

		mutex: &sync.RWMutex{},
	}
}

func (cr *ClusterRunner) SessionTTL() time.Duration {
	return cr.sessionTTL
}

func (cr *ClusterRunner) ConsulVersion() string {
	cmd := exec.Command("consul", "-v")
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session).Should(gexec.Exit(0))
	Expect(session.Out).To(gbytes.Say("Consul "))
	lines := strings.Split(string(session.Out.Contents()), "\n")
	versionLine := lines[0]
	//Consul in 'dev' mode does not contain the prefix 'v', only 'Consul 0.7.1-dev'
	return strings.TrimPrefix(strings.TrimPrefix(versionLine, "Consul "), "v")
}

func (cr *ClusterRunner) HasPerformanceFlag() bool {
	return !strings.HasPrefix(cr.ConsulVersion(), "0.6")
}

func (cr *ClusterRunner) Start() {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()

	if cr.running {
		return
	}

	tmpDir, err := ioutil.TempDir("", defaultDataDirPrefix)
	Expect(err).NotTo(HaveOccurred())
	cr.dataDir = tmpDir

	tmpDir, err = ioutil.TempDir("", defaultConfigDirPrefix)
	Expect(err).NotTo(HaveOccurred())
	cr.configDir = tmpDir

	cr.consulProcesses = make([]ifrit.Process, cr.numNodes)

	for i := 0; i < cr.numNodes; i++ {
		iStr := fmt.Sprintf("%d", i)
		nodeDataDir := path.Join(cr.dataDir, iStr)
		os.MkdirAll(nodeDataDir, 0700)

		configFilePath := writeConfigFile(
			cr.HasPerformanceFlag(),
			cr.configDir,
			nodeDataDir,
			iStr,
			cr.startingPort,
			i,
			cr.numNodes,
			cr.sessionTTL,
			cr.verifyConnections,
			cr.caCert,
			cr.clientCert,
			cr.clientKey,
		)

		process := ginkgomon.Invoke(ginkgomon.New(ginkgomon.Config{
			Name:              fmt.Sprintf("consul_cluster[%d]", i),
			AnsiColorCode:     "35m",
			StartCheck:        "agent: Join completed.",
			StartCheckTimeout: 10 * time.Second,
			Command: exec.Command(
				"consul",
				"agent",
				"--log-level", "trace",
				"--config-file", configFilePath,
			),
		}))
		cr.consulProcesses[i] = process

		ready := process.Ready()
		Eventually(ready, 10, 0.05).Should(BeClosed(), "Expected consul to be up and running")
	}

	cr.running = true
}

func (cr *ClusterRunner) NewClient() consuladapter.Client {
	var consulClient consuladapter.Client
	var err error

	if cr.scheme == "https" {
		consulClient, err = consuladapter.NewTLSClientFromUrl(cr.URL(), cr.caCert, cr.clientCert, cr.clientKey)
		Expect(err).NotTo(HaveOccurred())
	} else {
		consulClient, err = consuladapter.NewClientFromUrl(cr.URL())
		Expect(err).NotTo(HaveOccurred())
	}

	return consulClient
}

func (cr *ClusterRunner) WaitUntilReady() {
	client := cr.NewClient()
	catalog := client.Catalog()

	Eventually(func() error {
		_, qm, err := catalog.Nodes(nil)
		if err != nil {
			return err
		}
		if qm.KnownLeader && qm.LastIndex > 0 {
			return nil
		}
		return errors.New("not ready")
	}, 10, 100*time.Millisecond).Should(BeNil())
}

func (cr *ClusterRunner) Stop() {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()

	if !cr.running {
		return
	}

	for i := 0; i < cr.numNodes; i++ {
		stopSignal(cr.consulProcesses[i], 5*time.Second)
	}

	os.RemoveAll(cr.dataDir)
	os.RemoveAll(cr.configDir)
	cr.consulProcesses = nil
	cr.running = false
}

func (cr *ClusterRunner) portOffset() int {
	if cr.scheme == "https" {
		return PortOffsetHTTPS
	} else {
		return PortOffsetHTTP
	}
}

func (cr *ClusterRunner) ConsulCluster() string {
	urls := make([]string, cr.numNodes)

	for i := 0; i < cr.numNodes; i++ {
		urls[i] = fmt.Sprintf("%s://127.0.0.1:%d", cr.scheme, cr.startingPort+i*PortOffsetLength+cr.portOffset())
	}

	return strings.Join(urls, ",")
}

func (cr *ClusterRunner) Address() string {
	return fmt.Sprintf("127.0.0.1:%d", cr.startingPort+cr.portOffset())
}

func (cr *ClusterRunner) URL() string {
	return fmt.Sprintf("%s://%s", cr.scheme, cr.Address())
}

func (cr *ClusterRunner) Reset() error {
	client := cr.NewClient()

	sessions, _, err := client.Session().List(nil)
	if err == nil {
		for _, session := range sessions {
			_, err1 := client.Session().Destroy(session.ID, nil)
			if err1 != nil {
				err = err1
			}
		}
	}

	if err != nil {
		return err
	}

	services, err := client.Agent().Services()
	if err == nil {
		for _, service := range services {
			if service.Service == "consul" {
				continue
			}
			err1 := client.Agent().ServiceDeregister(service.ID)
			if err1 != nil {
				err = err1
			}
		}
	}

	if err != nil {
		return err
	}

	checks, err := client.Agent().Checks()
	if err == nil {
		for _, check := range checks {
			err1 := client.Agent().CheckDeregister(check.CheckID)
			if err1 != nil {
				err = err1
			}
		}
	}

	if err != nil {
		return err
	}

	_, err1 := client.KV().DeleteTree("", nil)

	return err1
}
