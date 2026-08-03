//go:build !windows

package cell_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/inigo/helpers/certauthority"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/inigo/world"
	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"
)

func buildFakeApp() (string, string) {
	// Build the fake app from the assets directory
	fakeAppPath := filepath.Join(".", "assets", "fake_app")

	// Create temporary output file
	tempDir := world.TempDirWithParent("", "fake-app-build")
	binPath := filepath.Join(tempDir, "fake-app")

	cmd := exec.Command("go", "build", "-a", "-tags", "netgo", "-ldflags", "-extldflags=-static", "-o", binPath)
	cmd.Dir = fakeAppPath // Set working directory to the source directory
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH=amd64")

	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	return binPath, tempDir
}

func buildFakeProxy() string {
	dir := world.TempDirWithParent(suiteTempDir, "fake-proxy")
	err := os.Chmod(dir, 0777)
	Expect(err).NotTo(HaveOccurred())

	// Build the fake proxy from the assets directory
	fakeProxyPath := filepath.Join(".", "assets", "fake_proxy")

	// Create temporary build output file
	tempBinPath := filepath.Join(dir, "fake-proxy-temp")

	cmd := exec.Command("go", "build", "-a", "-tags", "netgo", "-ldflags", "-extldflags=-static", "-o", tempBinPath)
	cmd.Dir = fakeProxyPath // Set working directory to the source directory
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH=amd64")

	_, err = cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred())

	envoyPath := filepath.Join(dir, "envoy")
	srcFile, err := os.Open(tempBinPath)
	Expect(err).NotTo(HaveOccurred())
	defer srcFile.Close()

	newEnvoy, err := os.OpenFile(envoyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	Expect(err).NotTo(HaveOccurred())
	defer newEnvoy.Close()

	_, err = io.Copy(newEnvoy, srcFile)
	Expect(err).NotTo(HaveOccurred())

	err = os.Chmod(envoyPath, 0755)
	Expect(err).NotTo(HaveOccurred())

	// Verify the fake-proxy binary was created correctly
	_, err = os.Stat(envoyPath)
	Expect(err).To(BeNil())

	// Clean up temp binary
	os.Remove(tempBinPath)

	return dir
}

var _ = Describe("Codependency", Serial, func() {
	var (
		processGuid     string
		ifritRuntime    ifrit.Process
		fakeProxyDir    string
		fakeAppTempDirs []string // Track temp directories created by buildFakeApp for cleanup
	)

	var startRuntime func()

	BeforeEach(func() {
		processGuid = helpers.GenerateGuid()
		fakeAppTempDirs = []string{} // Reset temp dir tracking for each test

		startRuntime = func() {
			var fileServer ifrit.Runner
			var fileServerStaticDir string
			fileServer, fileServerStaticDir = componentMaker.FileServer(modifyFunFileServerLoggregatorConfig)

			fakeAppPath, tempDir := buildFakeApp()
			fakeAppTempDirs = append(fakeAppTempDirs, tempDir)
			fakeAppContents, err := os.ReadFile(fakeAppPath)
			Expect(err).NotTo(HaveOccurred())

			archive_helper.CreateZipArchive(
				filepath.Join(fileServerStaticDir, "lrp.zip"),
				[]archive_helper.ArchiveFile{
					{
						Name: "fake-app",
						Body: string(fakeAppContents),
						Mode: 0755,
					},
				},
			)

			credDir := world.TempDirWithParent(suiteTempDir, "instance-creds")
			certAuthority, err := certauthority.NewCertAuthority(credDir, "ca-with-no-max-path-length")
			Expect(err).NotTo(HaveOccurred())
			intermediateKeyPath, intermediateCACertPath, err := certAuthority.GenerateSelfSignedCertAndKey("instance-identity", []string{"instance-identity"}, true)
			Expect(err).NotTo(HaveOccurred())

			modifyRepConfig := func(cfg *repconfig.RepConfig) {
				modifyFunRepLoggregatorConfig(cfg)
				cfg.EnableContainerProxy = true
				cfg.ContainerProxyPath = fakeProxyDir
				cfg.ContainerProxyConfigPath = world.TempDirWithParent(suiteTempDir, "envoy_config")
				cfg.InstanceIdentityCredDir = credDir
				cfg.InstanceIdentityCAPath = intermediateCACertPath
				cfg.InstanceIdentityPrivateKeyPath = intermediateKeyPath
				cfg.InstanceIdentityValidityPeriod = durationjson.Duration(time.Minute)
			}

			ifritRuntime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
				{Name: "router", Runner: componentMaker.Router()},
				{Name: "file-server", Runner: fileServer},
				{Name: "rep", Runner: componentMaker.Rep(modifyRepConfig)},
				{Name: "auctioneer", Runner: componentMaker.Auctioneer(modifyFunAuctioneerLoggregatorConfig)},
				{Name: "route-emitter", Runner: componentMaker.RouteEmitter(modifyFunRouteEmitterLoggregatorConfig)},
			}))
		}
	})

	AfterEach(func() {
		helpers.StopProcesses(ifritRuntime)
		if fakeProxyDir != "" {
			if err := os.RemoveAll(fakeProxyDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to cleanup fake proxy directory: %v\n", err)
			}
		}
		// Clean up all temporary directories created by buildFakeApp
		for _, tempDir := range fakeAppTempDirs {
			if err := os.RemoveAll(tempDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to cleanup fake app temp directory %s: %v\n", tempDir, err)
			}
		}
	})

	DescribeTable("when a process exits",
		func(processToExit string, exitCode int) {
			fakeProxyDir = buildFakeProxy()

			// Construct DesiredLRP
			lrp := helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)
			lrp.Setup = models.WrapAction(&models.DownloadAction{
				User: "vcap",
				From: fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.zip"),
				To:   "/tmp",
			})
			lrp.Monitor = models.WrapAction(&models.RunAction{
				User: "vcap",
				Path: "sh",
				Args: []string{"-c", "exit 0"},
			})

			lrp.Action = models.WrapAction(&models.RunAction{
				User: "vcap",
				Path: "sh",
				Args: []string{"-c", "PORT=8080 /tmp/fake-app"},
			})
			lrp.Ports = []uint32{8080, 8081}

			lrp.Sidecars = []*models.Sidecar{
				{
					Action: models.WrapAction(&models.RunAction{
						User: "vcap",
						Path: "sh",
						Args: []string{"-c", "PORT=8081 /tmp/fake-app"},
					}),
					MemoryMb: 128,
				},
			}

			startRuntime()

			err := bbsClient.DesireLRP(lgr, "", lrp)
			Expect(err).NotTo(HaveOccurred())

			getLatestStateForProcess := func(processGuid string) string {
				lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
				if err != nil || len(lrps) == 0 {
					return "UNKNOWN"
				}
				return lrps[0].State
			}
			Eventually(func() string { return getLatestStateForProcess(processGuid) }, 3*time.Minute).Should(Equal(models.ActualLRPStateRunning))

			lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
			Expect(err).NotTo(HaveOccurred())
			actualLRP := lrps[0]

			var port uint32
			for _, p := range actualLRP.ActualLRPNetInfo.Ports {
				fmt.Printf("  ContainerPort: %d, HostPort: %d, ContainerTlsProxyPort: %d\n",
					p.ContainerPort, p.HostPort, p.ContainerTlsProxyPort)
			}

			if processToExit == "main" {
				for _, p := range actualLRP.ActualLRPNetInfo.Ports {
					if p.ContainerPort == 8080 {
						port = p.ContainerPort
						break
					}
				}
			} else if processToExit == "sidecar" {
				for _, p := range actualLRP.ActualLRPNetInfo.Ports {
					if p.ContainerPort == 8081 {
						port = p.ContainerPort
						break
					}
				}
			} else if processToExit == "proxy" {
				// fake-proxy listens directly on port 61001 for HTTP requests
				port = 61001
			}

			// Make the exit request
			exitURL := fmt.Sprintf("http://%s:%d/exit?code=%d", actualLRP.ActualLRPNetInfo.InstanceAddress, port, exitCode)
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(exitURL)
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()

			// Check that the LRP crashed (crash_count > 0). We check crash_count
			// rather than state==CRASHED because the BBS restarts the LRP immediately
			// on first crash (zero back-off), so the CRASHED state is extremely brief.
			Eventually(func() int32 {
				lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
				if err != nil || len(lrps) == 0 {
					return 0
				}
				return lrps[0].CrashCount
			}, 60*time.Second).Should(BeNumerically(">", 0))
		},
		Entry("container should crash when main process exits 0", "main", 0),
		Entry("container should crash when main process exits 1", "main", 1),
		Entry("container should crash when main process exits 134 (SIGABRT)", "main", 134),
		Entry("container should crash when sidecar process exits 0", "sidecar", 0),
		Entry("container should crash when sidecar process exits 1", "sidecar", 1),
		Entry("container should crash when sidecar process exits 134 (SIGABRT)", "sidecar", 134),
		Entry("container should crash when proxy process exits 0", "proxy", 0),
		Entry("container should crash when proxy process exits 1", "proxy", 1),
		Entry("container should crash when proxy process exits 134 (SIGABRT)", "proxy", 134),
	)
})
