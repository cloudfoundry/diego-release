package testrunner

import (
	"crypto/tls"
	"encoding/json"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/cmd/locket/certauthority"
	"code.cloudfoundry.org/locket/cmd/locket/config"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/onsi/gomega"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var (
	caCertFile, certFile, keyFile string
)

func init() {
	certDepot, err := os.MkdirTemp("", "cert-depot")
	if err != nil {
		panic(err)
	}
	certAuthority, err := certauthority.NewCertAuthority(certDepot, "ca")
	if err != nil {
		panic(err)
	}
	_, caCertFile = certAuthority.CAAndKey()
	keyFile, certFile, err = certAuthority.GenerateSelfSignedCertAndKey("locket", []string{"localhost"}, false)
	if err != nil {
		panic(err)
	}
}

func NewLocketRunner(locketBinPath string, fs ...func(cfg *config.LocketConfig)) *ginkgomon.Runner {

	cfg := &config.LocketConfig{
		LagerConfig: lagerflags.LagerConfig{
			LogLevel:   lagerflags.INFO,
			TimeFormat: lagerflags.FormatUnixEpoch,
		},
		DatabaseDriver: "mysql",
		ReportInterval: durationjson.Duration(1 * time.Minute),
		CaFile:         caCertFile,
		CertFile:       certFile,
		KeyFile:        keyFile,
	}

	for _, f := range fs {
		f(cfg)
	}

	locketConfig, err := os.CreateTemp("", "locket-config")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	locketConfigFilePath := locketConfig.Name()

	encoder := json.NewEncoder(locketConfig)
	err = encoder.Encode(cfg)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(locketConfig.Close()).To(gomega.Succeed())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "locket",
		StartCheck:        "locket.started",
		StartCheckTimeout: 10 * time.Second,
		Command:           exec.Command(locketBinPath, "-config="+locketConfigFilePath),
		Cleanup: func() {
			err := os.RemoveAll(locketConfigFilePath)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		},
	})
}

func LocketClientTLSConfig() *tls.Config {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certFile, keyFile),
	).Client(tlsconfig.WithAuthorityFromFile(caCertFile))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return tlsConfig
}

func ClientLocketConfig() locket.ClientLocketConfig {
	return locket.ClientLocketConfig{
		LocketCACertFile:     caCertFile,
		LocketClientCertFile: certFile,
		LocketClientKeyFile:  keyFile,
	}
}
