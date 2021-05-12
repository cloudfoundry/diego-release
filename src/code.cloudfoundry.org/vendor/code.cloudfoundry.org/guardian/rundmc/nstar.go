package rundmc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/lager"
)

type nstar struct {
	NstarBinPath string
	TarBinPath   string

	CommandRunner commandrunner.CommandRunner
}

func NewNstarRunner(nstarPath, tarPath string, runner commandrunner.CommandRunner) NstarRunner {
	return &nstar{
		NstarBinPath:  nstarPath,
		TarBinPath:    tarPath,
		CommandRunner: runner,
	}
}

func (n *nstar) StreamIn(logger lager.Logger, pid int, path, user string, tarStream io.Reader) error {
	buff := new(bytes.Buffer)
	cmd := exec.Command(n.NstarBinPath, n.TarBinPath, fmt.Sprintf("%d", pid), n.streamUser(user), path)
	cmd.Stdout = buff
	cmd.Stderr = buff
	cmd.Stdin = tarStream

	if err := n.CommandRunner.Run(cmd); err != nil {
		return fmt.Errorf("error streaming in: %v. Output: %s", err, buff.String())
	}

	return nil
}

func (n *nstar) StreamOut(log lager.Logger, pid int, path, user string) (io.ReadCloser, error) {
	sourcePath := filepath.Dir(path)
	compressPath := filepath.Base(path)
	if strings.HasSuffix(path, "/") {
		sourcePath = path
		compressPath = "."
	}

	errOut := new(bytes.Buffer)
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(n.NstarBinPath, n.TarBinPath, fmt.Sprintf("%d", pid), n.streamUser(user), sourcePath, compressPath)
	cmd.Stdout = writer
	cmd.Stderr = errOut

	if err := n.CommandRunner.Background(cmd); err != nil {
		return nil, fmt.Errorf("error streaming out: %v. Output: %s", err, errOut.String())
	}

	writer.Close()

	go func() {
		if err := n.CommandRunner.Wait(cmd); err != nil {
			log.Error("wait", err, lager.Data{
				"pid":    pid,
				"path":   path,
				"user":   user,
				"stdout": errOut.String()})
		}
	}()

	return reader, nil
}

func (n *nstar) streamUser(usr string) string {
	if usr == "" {
		usr = "root"
	}
	return usr
}
