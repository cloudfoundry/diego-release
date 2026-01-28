package config_test

import (
	"os"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/fileserver/cmd/file-server/config"
	"code.cloudfoundry.org/lager/v3/lagerflags"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var configPath, configData string

	BeforeEach(func() {
		configData = `{
			"server_address": "192.168.1.1:8080",
			"static_directory": "/tmp/static",

			"https_server_enabled": true,
			"https_listen_addr": "192.168.1.1:8443",
			"cert_file": "/tmp/cert_file",
			"key_file": "/tmp/key_file",

			"debug_address": "127.0.0.1:17017",
			"log_level": "debug"
		}`
	})

	JustBeforeEach(func() {
		configFile, err := os.CreateTemp("", "file-server-config")
		Expect(err).NotTo(HaveOccurred())

		configPath = configFile.Name()

		n, err := configFile.WriteString(configData)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(configData)))
	})

	AfterEach(func() {
		err := os.RemoveAll(configPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("correctly parses the config file", func() {
		fileserverConfig, err := config.NewFileServerConfig(configPath)
		Expect(err).NotTo(HaveOccurred())

		expectedConfig := config.FileServerConfig{
			ServerAddress:   "192.168.1.1:8080",
			StaticDirectory: "/tmp/static",

			HTTPSServerEnabled: true,
			HTTPSListenAddr:    "192.168.1.1:8443",
			CertFile:           "/tmp/cert_file",
			KeyFile:            "/tmp/key_file",

			DebugServerConfig: debugserver.DebugServerConfig{
				DebugAddress: "127.0.0.1:17017",
			},
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: "debug",
			},
		}

		Expect(fileserverConfig).To(Equal(expectedConfig))
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			_, err := config.NewFileServerConfig("foobar")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the file does not contain valid json", func() {
		BeforeEach(func() {
			configData = "{{"
		})

		It("returns an error", func() {
			_, err := config.NewFileServerConfig(configPath)
			Expect(err).To(HaveOccurred())
		})
	})
})
