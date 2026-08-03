package helpers

import (
	"os/exec"

	. "github.com/onsi/gomega"
)

func Copy(sourcePath, destinationPath string) {
	Expect(sourcePath).NotTo(BeEmpty())
	Expect(destinationPath).NotTo(BeEmpty())
	err := exec.Command("cp", "-a", sourcePath, destinationPath).Run()
	Expect(err).NotTo(HaveOccurred())
}
