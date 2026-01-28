package world

import (
	"os"

	. "github.com/onsi/gomega"
)

func TempDir(prefix string) string {
	tmpDir, err := os.MkdirTemp(os.TempDir(), prefix)
	Expect(err).NotTo(HaveOccurred())

	err = os.Chmod(tmpDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	return tmpDir
}

func TempDirWithParent(parentDir string, prefix string) string {
	tmpDir, err := os.MkdirTemp(parentDir, prefix)
	Expect(err).NotTo(HaveOccurred())

	err = os.Chmod(tmpDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	return tmpDir
}
