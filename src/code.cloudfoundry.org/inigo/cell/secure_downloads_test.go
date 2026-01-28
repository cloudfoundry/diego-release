package cell_test

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/rep/cmd/rep/config"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Secure Downloading and Uploading", func() {
	var (
		processGuid         string
		archiveFiles        []archive_helper.ArchiveFile
		fileServerStaticDir string
		ifritRuntime        ifrit.Process
		tlsFileServer       *httptest.Server
		cfgs                []func(cfg *config.RepConfig)
		fileServer          ifrit.Runner
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet working on windows")
		}
		processGuid = helpers.GenerateGuid()

		fileServer, fileServerStaticDir = componentMaker.FileServer()

		archiveFiles = fixtures.GoServerApp()
	})

	JustBeforeEach(func() {
		ifritRuntime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "file-server", Runner: fileServer},
			{Name: "rep", Runner: componentMaker.Rep(cfgs...)},
			{Name: "auctioneer", Runner: componentMaker.Auctioneer()},
		}))
		archive_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			archiveFiles,
		)
	})

	AfterEach(func() {
		helpers.StopProcesses(ifritRuntime)
	})

	Describe("downloading", func() {
		var lrp *models.DesiredLRP

		BeforeEach(func() {
			fileServerURL, err := url.Parse(fmt.Sprintf("http://%s", componentMaker.Addresses().FileServer))
			Expect(err).NotTo(HaveOccurred())
			proxy := httputil.NewSingleHostReverseProxy(fileServerURL)
			tlsFileServer = httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				proxy.ServeHTTP(rw, req)
			}))
			tlsConfig, err := tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(componentMaker.BBSSSLConfig().ServerCert, componentMaker.BBSSSLConfig().ServerKey),
			).Server(
				tlsconfig.WithClientAuthenticationFromFile(componentMaker.BBSSSLConfig().CACert),
			)
			Expect(err).NotTo(HaveOccurred())
			tlsFileServer.TLS = tlsConfig
		})

		JustBeforeEach(func() {
			tlsFileServer.StartTLS()

			lrp = helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)
			err := bbsClient.DesireLRP(lgr, "", lrp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("eventually runs", func() {
			Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
		})

		Context("when CaCertForDownload is present", func() {
			BeforeEach(func() {
				cfgs = append(cfgs, func(cfg *config.RepConfig) {
					cfg.PathToCACertsForDownloads = cfg.PathToTLSCACert
				})
			})

			Context("when TLSCaCert is empty", func() {
				BeforeEach(func() {
					tlsFileServer.TLS.ClientAuth = tls.NoClientCert

					cfgs = append(cfgs, func(cfg *config.RepConfig) {
						cfg.PathToTLSCACert = ""
					})
				})

				It("eventually runs", func() {
					Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				})
			})
		})

		Context("when skip cert verify is set to true and the ca cert isn't set", func() {
			BeforeEach(func() {
				cfgs = append(cfgs, func(cfg *config.RepConfig) {
					cfg.PathToTLSCACert = "../fixtures/certs/wrong-ca.crt"
					cfg.SkipCertVerify = true
				})
			})

			It("eventually runs", func() {
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
			})
		})
	})

	Describe("uploading", func() {
		var (
			guid       string
			server     *httptest.Server
			gotRequest chan struct{}
		)

		BeforeEach(func() {
			guid = helpers.GenerateGuid()

			gotRequest = make(chan struct{})

			server, _ = helpers.Callback(os.Getenv("EXTERNAL_ADDRESS"), ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/thingy"),
				func(w http.ResponseWriter, r *http.Request) {
					contents, err := io.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(contents)).To(Equal("tasty thingy\n"))

					close(gotRequest)
				},
			))

			serverURL, err := url.Parse(server.URL)
			Expect(err).NotTo(HaveOccurred())
			proxy := httputil.NewSingleHostReverseProxy(serverURL)
			tlsFileServer = httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				proxy.ServeHTTP(rw, req)
			}))
			tlsConfig, err := tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(componentMaker.BBSSSLConfig().ServerCert, componentMaker.BBSSSLConfig().ServerKey),
			).Server(
				tlsconfig.WithClientAuthenticationFromFile(componentMaker.BBSSSLConfig().CACert),
			)
			Expect(err).NotTo(HaveOccurred())
			tlsFileServer.TLS = tlsConfig
			tlsFileServer.StartTLS()
		})

		AfterEach(func() {
			server.Close()
		})

		It("uploads the specified files", func() {
			expectedTask := helpers.TaskCreateRequest(
				guid,
				models.Serial(
					&models.RunAction{
						User: "vcap",
						Path: "sh",
						Args: []string{"-c", "echo tasty thingy > thingy"},
					},
					&models.UploadAction{
						From: "thingy",
						To:   fmt.Sprintf("%s/thingy", tlsFileServer.URL),
						User: "vcap",
					},
				),
			)

			err := bbsClient.DesireTask(lgr, "", expectedTask.TaskGuid, expectedTask.Domain, expectedTask.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())

			Eventually(gotRequest).Should(BeClosed())
		})
	})
})
