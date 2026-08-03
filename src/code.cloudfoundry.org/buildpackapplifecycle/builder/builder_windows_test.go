package main_test

import (
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var buildpackFixtures = filepath.Join("fixtures", "buildpacks", "windows")
var appFixtures = filepath.Join("fixtures", "apps")

func cp(src string, dst string) {
	session, err := gexec.Start(exec.Command("powershell", "-Command", "Copy-Item", "-Recurse", "-Force", src, dst),
		GinkgoWriter,
		GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	Eventually(session, 5*time.Second).Should(gexec.Exit(0))
}
