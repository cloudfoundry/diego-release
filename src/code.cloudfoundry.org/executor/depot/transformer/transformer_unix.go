//go:build !windows
// +build !windows

package transformer

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/bbs/models"
)

func envoyRunAction(envoyArgs []string) models.RunAction {
	args := []string{
		"-c",
		// make sure the entire process group is killed if the shell exits
		// otherwise we ended up in the following situtation for short running tasks:
		// - assuming envoy proxy is still initializing
		// - short running task exits
		// - codependent step tries to signal the envoy proxy process
		// - the wrapper shell script gets signalled and exit
		// - garden's `process.Wait` won't return until both Stdout & Stderr are
		//   closed which causes the rep to assume envoy is hanging and send it a SigKill
		fmt.Sprintf(
			"trap 'kill -9 0' TERM; /etc/cf-assets/envoy/envoy %s& pid=$!; wait $pid",
			strings.Join(envoyArgs, " "),
		),
	}

	return models.RunAction{
		LogSource: "PROXY",
		Path:      "sh",
		Args:      args,
	}

}
