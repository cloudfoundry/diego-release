//go:build windows && windows2012R2
// +build windows,windows2012R2

package buildpackapplifecycle_test

import (
	"os"

	. "github.com/onsi/gomega"
)

func pathPrefix() string {
	workingDir, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	return workingDir
}
