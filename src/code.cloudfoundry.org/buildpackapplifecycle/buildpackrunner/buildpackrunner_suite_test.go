package buildpackrunner_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"code.cloudfoundry.org/buildpackapplifecycle/test_helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBuildpackrunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Buildpackrunner Suite")
}

var fileGitUrl url.URL
var gitUrl url.URL
var httpServer *httptest.Server
var tmpDir string
var tmpTarPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	gitPath, err := exec.LookPath("git")
	Expect(err).NotTo(HaveOccurred())

	tmpDir, err = os.MkdirTemp("", "tmpDir")
	Expect(err).NotTo(HaveOccurred())

	httpServer = httptest.NewServer(http.FileServer(http.Dir(tmpDir)))

	buildpackDir := filepath.Join(tmpDir, "fake-buildpack")
	err = os.MkdirAll(buildpackDir, os.ModePerm)
	Expect(err).NotTo(HaveOccurred())

	submoduleDir := filepath.Join(tmpDir, "submodule")
	err = os.MkdirAll(submoduleDir, os.ModePerm)
	Expect(err).NotTo(HaveOccurred())

	Expect(os.RemoveAll(filepath.Join(buildpackDir, ".git"))).To(Succeed())
	execute(buildpackDir, gitPath, "init")
	execute(buildpackDir, gitPath, "config", "user.email", "you@example.com")
	execute(buildpackDir, gitPath, "config", "user.name", "your name")
	writeFile(filepath.Join(buildpackDir, "content"), "some content")

	Expect(os.RemoveAll(filepath.Join(submoduleDir, ".git"))).To(Succeed())
	execute(submoduleDir, gitPath, "init")
	execute(submoduleDir, gitPath, "config", "user.email", "you@example.com")
	execute(submoduleDir, gitPath, "config", "user.name", "your name")
	writeFile(filepath.Join(submoduleDir, "README"), "1st commit")
	execute(submoduleDir, gitPath, "add", ".")
	execute(submoduleDir, gitPath, "commit", "-am", "first commit")
	writeFile(filepath.Join(submoduleDir, "README"), "2nd commit")
	execute(submoduleDir, gitPath, "commit", "-am", "second commit")
	execute(submoduleDir, gitPath, "update-server-info")

	execute(buildpackDir, gitPath, "submodule", "add", fmt.Sprintf("http://%s/submodule/.git", httpServer.Listener.Addr().String()), "sub")

	execute(buildpackDir+"/sub", gitPath, "checkout", "HEAD^")
	execute(buildpackDir, gitPath, "add", "-A")
	execute(buildpackDir, gitPath, "commit", "-m", "fake commit")
	execute(buildpackDir, gitPath, "commit", "--allow-empty", "-m", "empty commit")
	execute(buildpackDir, gitPath, "tag", "a_lightweight_tag")
	execute(buildpackDir, gitPath, "checkout", "-b", "a_branch")
	execute(buildpackDir+"/sub", gitPath, "checkout", "master")
	execute(buildpackDir, gitPath, "add", "-A")
	execute(buildpackDir, gitPath, "commit", "-am", "update submodule")
	execute(buildpackDir, gitPath, "checkout", "master")
	execute(buildpackDir, gitPath, "update-server-info")

	tarPath := ""
	if runtime.GOOS == "windows" {
		tarPath = test_helpers.DownloadOrFindWindowsTar()
	}

	return []byte(string(tmpDir+"|"+httpServer.Listener.Addr().String()) + "|" + tarPath)

}, func(data []byte) {
	synchronizedData := strings.Split(string(data), "|")
	tmpDir := synchronizedData[0]
	gitUrlHost := synchronizedData[1]
	tmpTarPath = synchronizedData[2]

	gitUrl = url.URL{
		Scheme: "http",
		Host:   gitUrlHost,
		Path:   "/fake-buildpack/.git",
	}

	fileGitUrl = url.URL{
		Scheme: "file",
		Path:   tmpDir + "/fake-buildpack",
	}
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	if httpServer != nil {
		httpServer.Close()
	}
	Expect(os.RemoveAll(tmpDir)).To(Succeed())
})

func execute(dir string, execCmd string, args ...string) {
	cmd := exec.Command(execCmd, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), fmt.Sprintf("Command failed: '%s' in directory '%s'.\nError output:\n%s\n", execCmd, dir, output))
}

func writeFile(filepath, content string) {
	err := os.WriteFile(filepath,
		[]byte(content), os.ModePerm)
	Expect(err).NotTo(HaveOccurred())
}
