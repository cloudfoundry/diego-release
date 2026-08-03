package rundmc

import (
	"bytes"
	"os"
	"os/exec"
)

func (c MountPointChecker) IsMountPoint(path string) (bool, error) {
	return c(path)
}

func IsMountPoint(path string) (bool, error) {
	// append trailing slash to force symlink traversal; symlinking e.g. 'cpu'
	// to 'cpu,cpuacct' is common
	cmd := exec.Command("mountpoint", path+"/")
	cmdOutput, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}

	// According to the mountpoint command implementation, an error means
	// that the path either does not exist, or is not a mountpoint
	if bytes.Contains(cmdOutput, []byte("is not a mountpoint")) {
		return false, nil
	}

	if stat, statErr := os.Stat(path); os.IsNotExist(statErr) || !stat.IsDir() {
		return false, nil
	}

	return false, err
}
