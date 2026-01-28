//go:build windows2012R2

package main_test

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/diego-ssh/cmd/sshd/testrunner"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
)

func buildSshd() string {
	sshd, err := gexec.Build("code.cloudfoundry.org/diego-ssh/cmd/sshd", "-race", "-tags", "windows2012R2")
	Expect(err).NotTo(HaveOccurred())
	return sshd
}

func startSshd(sshdPath string, args testrunner.Args, address string, port int) (ifrit.Runner, ifrit.Process) {
	args.Address = fmt.Sprintf("%s:2222", address)
	runner := testrunner.New(sshdPath, args)
	runner.Command.Env = append(
		os.Environ(),
		fmt.Sprintf(`CF_INSTANCE_PORTS=[{"external":%d,"internal":%d}]`, port, 2222),
	)
	process := ifrit.Invoke(runner)
	return runner, process
}
