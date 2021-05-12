package commandrunner // import "code.cloudfoundry.org/commandrunner"

import (
	"os"
	"os/exec"
)

type CommandRunner interface {
	Run(*exec.Cmd) error
	Start(*exec.Cmd) error
	Background(*exec.Cmd) error

	Wait(*exec.Cmd) error
	Kill(*exec.Cmd) error
	Signal(*exec.Cmd, os.Signal) error
}
