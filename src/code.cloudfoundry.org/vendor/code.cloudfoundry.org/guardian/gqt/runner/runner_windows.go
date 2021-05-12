package runner

import (
	"os"
	"os/exec"

	"code.cloudfoundry.org/lager"
)

type UserCredential interface{}

func setUserCredential(runner *GardenRunner) {}

func socket2meCommand(config GdnRunnerConfig) *exec.Cmd {
	panic("Unix sockets are unsupported on Windows")
}

func (r *GardenRunner) setupDirsForUser() {}

func (r *RunningGarden) Cleanup() error {
	r.logger.Info("cleanup-tempdirs")
	if err := os.RemoveAll(r.TmpDir); err != nil {
		r.logger.Error("cleanup-tempdirs-failed", err, lager.Data{"tmpdir": r.TmpDir})
		return err
	} else {
		r.logger.Info("tempdirs-removed")
	}

	return nil
}
