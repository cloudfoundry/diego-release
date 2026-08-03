//go:build !windows2012R2
// +build !windows2012R2

package main_test

import (
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func buildLauncher() string {
	launcherPath, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/launcher", "-race")
	Expect(err).NotTo(HaveOccurred())

	return launcherPath
}
