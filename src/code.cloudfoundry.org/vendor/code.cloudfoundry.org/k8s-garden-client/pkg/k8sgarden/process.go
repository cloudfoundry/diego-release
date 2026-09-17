package k8sgarden

import (
	"context"
	"errors"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	ctrdclient "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type process struct {
	log     lager.Logger
	id      string
	io      garden.ProcessIO
	process ctrdclient.Process

	task ctrdclient.Task
	spec *specs.Process
}

type Process interface {
	garden.Process
	Spec() *specs.Process
	Task() ctrdclient.Task
}

func NewProcess(
	log lager.Logger,
	id string,
	spec *specs.Process,
	io garden.ProcessIO,
	task ctrdclient.Task,
) Process {
	return &process{
		log:  log,
		id:   id,
		spec: spec,
		io:   io,
		task: task,
	}
}

// ID implements [garden.Process].
func (p *process) ID() string {
	return p.id
}

// Signal implements [garden.Process].
func (p *process) Signal(signal garden.Signal) error {
	s := syscall.SIGTERM
	if signal == garden.SignalKill {
		s = syscall.SIGKILL
	}

	p.log.Info("signaling-process", lager.Data{"signal": s, "pid": p.process.Pid()})
	return p.process.Kill(context.Background(), s)
}

// Wait implements [garden.Process].
func (p *process) Wait() (int, error) {
	p.log.Info("waiting-for-process-to-exit")
	defer p.log.Info("process-exited")
	var err error

	p.process, err = p.task.Exec(context.Background(), p.id, p.spec, cio.NewCreator(cio.WithStreams(p.io.Stdin, p.io.Stdout, p.io.Stderr), cio.WithFIFODir("/var/lib/rep/containerd_fifo")))
	if err != nil {
		return -1, err
	}

	if err := p.process.Start(context.Background()); err != nil {
		return -1, err
	}

	// The shim keeps its own writer open on the stdin FIFO, so the process never
	// sees EOF on stdin until we explicitly close it. Without this, stdin-reading
	// processes such as the `tar -xf -` used by StreamIn block forever. The tar
	// bytes still flow through cio's own stdin writer; this only drops the shim's
	// redundant keep-alive writer.
	if p.io.Stdin != nil {
		go p.closeStdin()
	}

	statusChan, err := p.process.Wait(context.Background())
	if err != nil {
		return -1, err
	}
	exitStatus := <-statusChan

	// wait for io to also catch daemon processes
	var closeErr error
	if io := p.process.IO(); io != nil {
		p.log.Info("waiting-for-io-to-finish")
		io.Wait()
		p.log.Info("io-finished")
		closeErr = io.Close()
	}
	_, err = p.process.Delete(context.Background())

	return int(exitStatus.ExitCode()), errors.Join(exitStatus.Error(), err, closeErr)
}

// closeStdin closes the process's stdin so stdin-reading processes get EOF,
// retrying with exponential backoff as the shim may not yet have wired up IO.
func (p *process) closeStdin() {
	backoff := 100 * time.Millisecond
	for i := 0; i < 10; i++ {
		if err := p.process.CloseIO(context.Background(), ctrdclient.WithStdinCloser); err != nil {
			p.log.Error("failed-closing-stdin", err)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return
	}
}

func (p *process) Spec() *specs.Process {
	return p.spec
}

func (p *process) Task() ctrdclient.Task {
	return p.task
}

func (p *process) SetTTY(garden.TTYSpec) error {
	return ErrNotSupported
}
