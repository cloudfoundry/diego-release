//go:build !windows2012R2

package main_test

import (
	"fmt"

	"code.cloudfoundry.org/diego-ssh/cmd/sshd/testrunner"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
)

func buildSshd() string {
	sshd, err := gexec.Build("code.cloudfoundry.org/diego-ssh/cmd/sshd", "-race")
	Expect(err).NotTo(HaveOccurred())
	return sshd
}

func startSshd(sshdPath string, args testrunner.Args, address string, port int) (ifrit.Runner, ifrit.Process) {
	args.Address = fmt.Sprintf("%s:%d", address, port)
	runner := testrunner.New(sshdPath, args)
	process := ifrit.Invoke(runner)
	return runner, process
}
