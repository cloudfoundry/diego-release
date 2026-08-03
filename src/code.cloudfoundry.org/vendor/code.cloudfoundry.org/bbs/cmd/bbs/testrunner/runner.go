package testrunner

import (
	"encoding/json"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/bbs/cmd/bbs/config"
	"code.cloudfoundry.org/durationjson"

	"github.com/onsi/gomega"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

func New(binPath string, bbsConfig config.BBSConfig) *ginkgomon.Runner {
	if bbsConfig.ReportInterval == 0 {
		bbsConfig.ReportInterval = durationjson.Duration(time.Minute)
	}

	f, err := os.CreateTemp("", "bbs.config")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	err = json.NewEncoder(f).Encode(bbsConfig)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "bbs",
		Command:           exec.Command(binPath, "-config", f.Name()),
		StartCheck:        "bbs.started",
		StartCheckTimeout: 1 * time.Minute,
		Cleanup: func() {
			// #nosec G104 - do not use Expect otherwise a race condition will happen
			os.RemoveAll(f.Name())
		},
	})
}

func WaitForMigration(binPath string, bbsConfig config.BBSConfig) *ginkgomon.Runner {
	runner := New(binPath, bbsConfig)
	runner.StartCheck = "finished-migrations"
	return runner
}
