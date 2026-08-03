package test_helpers

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/gomega"
)

func GetWindowsTarURL() string {
	return os.Getenv("TAR_URL")
}

func DownloadOrFindWindowsTar() string {
	var tarPath string

	tarUrl := GetWindowsTarURL()

	if tarUrl == "" {
		var err error
		tarPath, err = exec.LookPath("tar.exe")
		Expect(err).NotTo(HaveOccurred(), "tar.exe must either be present on the machine or specified via TAR_URL")
	} else {
		tmpDir, err := os.MkdirTemp("", "tar")
		Expect(err).NotTo(HaveOccurred())
		tarPath = filepath.Join(tmpDir, "tar.exe")

		resp, err := http.Get(tarUrl)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		tarExeFile, err := os.OpenFile(tarPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		Expect(err).NotTo(HaveOccurred())
		defer tarExeFile.Close()

		_, err = io.Copy(tarExeFile, resp.Body)
		Expect(err).NotTo(HaveOccurred())
	}

	return tarPath
}

func CopyFile(src string, dst string) {
	s, err := os.Open(src)
	Expect(err).ToNot(HaveOccurred())

	defer s.Close()

	i, err := s.Stat()
	Expect(err).ToNot(HaveOccurred())

	err = os.MkdirAll(filepath.Dir(dst), 0755)
	Expect(err).ToNot(HaveOccurred())

	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, i.Mode())
	Expect(err).ToNot(HaveOccurred())

	defer f.Close()

	_, err = io.Copy(f, s)
	Expect(err).ToNot(HaveOccurred())

}
