package internal

import (
	"archive/tar"
	"fmt"
	"io"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
)

const MAX_RESULT_SIZE = 1024 * 50

var ErrResultFileTooLarge = fmt.Errorf("result file is too large (over %d bytes)", MAX_RESULT_SIZE)

//go:generate counterfeiter -o fake_internal/fake_container_delegate.go container_delegate.go ContainerDelegate

type ContainerDelegate interface {
	GetContainer(logger lager.Logger, guid string) (executor.Container, bool)
	RunContainer(logger lager.Logger, traceID string, req *executor.RunRequest) bool
	StopContainer(logger lager.Logger, traceID string, guid string) bool
	DeleteContainer(logger lager.Logger, traceID string, guid string) bool
	FetchContainerResultFile(logger lager.Logger, guid string, filename string) (string, error)
}

type containerDelegate struct {
	client executor.Client
}

func NewContainerDelegate(client executor.Client) ContainerDelegate {
	return &containerDelegate{
		client: client,
	}
}

func (d *containerDelegate) GetContainer(logger lager.Logger, guid string) (executor.Container, bool) {
	logger.Debug("fetch-container")
	container, err := d.client.GetContainer(logger, guid)
	if err != nil {
		logInfoOrError(logger, "failed-fetch-container", err)
		return container, false
	}
	logger.Debug("succeeded-fetch-container")
	return container, true
}

func (d *containerDelegate) RunContainer(logger lager.Logger, traceID string, req *executor.RunRequest) bool {
	logger.Info("running-container")
	err := d.client.RunContainer(logger, traceID, req)
	if err != nil {
		logInfoOrError(logger, "failed-running-container", err)
		d.DeleteContainer(logger, traceID, req.Guid)
		return false
	}
	logger.Info("succeeded-running-container")
	return true
}

func (d *containerDelegate) StopContainer(logger lager.Logger, traceID string, guid string) bool {
	logger.Info("stopping-container")
	err := d.client.StopContainer(logger, traceID, guid)
	if err != nil {
		logInfoOrError(logger, "failed-stopping-container", err)
		return false
	}
	logger.Info("succeeded-stopping-container")
	return true
}

func (d *containerDelegate) DeleteContainer(logger lager.Logger, traceID string, guid string) bool {
	logger.Info("deleting-container")
	err := d.client.DeleteContainer(logger, traceID, guid)
	if err != nil {
		logInfoOrError(logger, "failed-deleting-container", err)
		return false
	}
	logger.Info("succeeded-deleting-container")
	return true
}

func (d *containerDelegate) FetchContainerResultFile(logger lager.Logger, guid string, filename string) (string, error) {
	logger.Info("fetching-container-result")
	stream, err := d.client.GetFiles(logger, guid, filename)
	if err != nil {
		logInfoOrError(logger, "failed-fetching-container-result-stream-from-executor", err)
		return "", err
	}

	defer stream.Close()

	tarReader := tar.NewReader(stream)

	_, err = tarReader.Next()
	if err != nil {
		return "", err
	}

	// make the buffer 1 byte larger than the MAX_RESULT_SIZE so we can
	// tell when we are at the exact size limit vs. over the limit
	// tarReader.Read will return '0, nil' when it has filled our buffer
	// instead of '0, io.EOF'
	buf := make([]byte, MAX_RESULT_SIZE+1)
	var readErr error
	numBytesRead := 0
	for readErr == nil {
		n := 0
		n, readErr = tarReader.Read(buf[numBytesRead:])
		numBytesRead += n
		if numBytesRead > MAX_RESULT_SIZE {
			logger.Error("failed-fetching-container-result-too-large", readErr)
			return "", ErrResultFileTooLarge
		}
	}
	if readErr != io.EOF {
		logger.Error("failed-reading-container-result-file", readErr)
		return "", readErr
	}

	logger.Info("succeeded-fetching-container-result")
	return string(buf[:numBytesRead]), nil
}

func logInfoOrError(logger lager.Logger, msg string, err error) {
	if err == executor.ErrContainerNotFound {
		logger.Info(msg, lager.Data{"error": err.Error()})
	} else {
		logger.Error(msg, err)
	}
}
