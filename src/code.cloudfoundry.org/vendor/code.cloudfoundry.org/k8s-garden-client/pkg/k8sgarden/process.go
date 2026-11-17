package k8sgarden

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	ctrdclient "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var noSuchExecutable = regexp.MustCompile(`(?:starting container process caused|unable to start container process): exec: .*: (?:stat .*: no such file or directory|executable file not found in \$PATH)`)

type process struct {
	log  lager.Logger
	id   string
	io   garden.ProcessIO
	task ctrdclient.Task
	spec *specs.Process

	mu   sync.Mutex
	proc ctrdclient.Process

	waitOnce sync.Once
	exitCode int
	waitErr  error
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
	return newProcess(log, id, spec, io, task)
}

func newProcess(
	log lager.Logger,
	id string,
	spec *specs.Process,
	io garden.ProcessIO,
	task ctrdclient.Task,
) *process {
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
	proc := p.ctrdProcess()
	if proc == nil {
		return errors.New("process not started")
	}

	s := syscall.SIGTERM
	if signal == garden.SignalKill {
		s = syscall.SIGKILL
	}

	p.log.Info("signaling-process", lager.Data{"signal": s, "pid": proc.Pid()})
	return proc.Kill(context.Background(), s)
}

// Wait implements [garden.Process].
func (p *process) Wait() (int, error) {
	p.waitOnce.Do(func() {
		p.exitCode, p.waitErr = p.wait()
	})
	return p.exitCode, p.waitErr
}

func (p *process) start() (err error) {
	defer func() {
		if err != nil && noSuchExecutable.MatchString(err.Error()) {
			err = garden.ExecutableNotFoundError{Message: err.Error()}
		}
	}()

	proc, err := p.task.Exec(context.Background(), p.id, p.spec, cio.NewCreator(cio.WithStreams(p.io.Stdin, p.io.Stdout, p.io.Stderr), cio.WithFIFODir("/var/lib/rep/containerd_fifo")))
	if err != nil {
		return err
	}
	if err := proc.Start(context.Background()); err != nil {
		if _, deleteErr := proc.Delete(context.Background()); deleteErr != nil {
			p.log.Error("failed-deleting-process", deleteErr)
		}
		return err
	}
	p.setCtrdProcess(proc)

	// The shim keeps its own writer open on the stdin FIFO, so the process never
	// sees EOF on stdin until we explicitly close it. Without this, stdin-reading
	// processes such as the `tar -xf -` used by StreamIn block forever. The tar
	// bytes still flow through cio's own stdin writer; this only drops the shim's
	// redundant keep-alive writer.
	if p.io.Stdin != nil {
		go p.closeStdin(proc)
	}
	return nil
}

func (p *process) wait() (int, error) {
	p.log.Info("waiting-for-process-to-exit")
	defer p.log.Info("process-exited")

	proc := p.ctrdProcess()
	if proc == nil {
		if err := p.start(); err != nil {
			return -1, err
		}
		proc = p.ctrdProcess()
	}

	statusChan, err := proc.Wait(context.Background())
	if err != nil {
		return -1, err
	}
	exitStatus := <-statusChan

	// wait for io to also catch daemon processes
	if io := proc.IO(); io != nil {
		p.log.Info("waiting-for-io-to-finish")
		io.Wait()
		p.log.Info("io-finished")
		if err := io.Close(); err != nil {
			p.log.Error("failed-closing-process-io", err)
		}
	}
	if _, err := proc.Delete(context.Background()); err != nil {
		p.log.Error("failed-deleting-process", err)
	}

	return int(exitStatus.ExitCode()), exitStatus.Error()
}

func (p *process) ctrdProcess() ctrdclient.Process {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.proc
}

func (p *process) setCtrdProcess(proc ctrdclient.Process) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.proc = proc
}

// closeStdin closes the process's stdin so stdin-reading processes get EOF,
// retrying with exponential backoff as the shim may not yet have wired up IO.
func (p *process) closeStdin(proc ctrdclient.Process) {
	backoff := 100 * time.Millisecond
	for range 10 {
		if err := proc.CloseIO(context.Background(), ctrdclient.WithStdinCloser); err != nil {
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
