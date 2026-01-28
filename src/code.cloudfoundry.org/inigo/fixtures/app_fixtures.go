package fixtures

import (
	"os"
	"runtime"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func GoServerApp() []archive_helper.ArchiveFile {
	originalCGOValue := os.Getenv("CGO_ENABLED")
	err := os.Setenv("CGO_ENABLED", "0")
	Expect(err).NotTo(HaveOccurred())

	serverPath, err := gexec.Build("code.cloudfoundry.org/inigo/fixtures/go-server")
	Expect(err).NotTo(HaveOccurred())

	err = os.Setenv("CGO_ENABLED", originalCGOValue)
	Expect(err).NotTo(HaveOccurred())

	contents, err := os.ReadFile(serverPath)
	Expect(err).NotTo(HaveOccurred())
	return []archive_helper.ArchiveFile{
		{
			Name: getGoServerBinaryName(),
			Body: string(contents),
		}, {
			Name: "staging_info.yml",
			Body: `detected_buildpack: Doesn't Matter
start_command: go-server`,
		},
	}
}

func getGoServerBinaryName() string {
	if runtime.GOOS == "windows" {
		return "go-server.exe"
	}
	return "go-server"
}
