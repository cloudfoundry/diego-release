package steps

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bytefmt"
	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

type downloadStep struct {
	container        garden.Container
	model            models.DownloadAction
	cachedDownloader cacheddownloader.CachedDownloader
	streamer         log_streamer.LogStreamer
	rateLimiter      chan struct{}
	cancelDownload   chan struct{}

	logger lager.Logger
}

func NewDownload(
	container garden.Container,
	model models.DownloadAction,
	cachedDownloader cacheddownloader.CachedDownloader,
	rateLimiter chan struct{},
	streamer log_streamer.LogStreamer,
	logger lager.Logger,
) ifrit.Runner {
	logger = logger.Session("download-step", lager.Data{
		"to":       model.To,
		"cacheKey": model.CacheKey,
		"user":     model.User,
	})

	return &downloadStep{
		container:        container,
		model:            model,
		cachedDownloader: cachedDownloader,
		streamer:         streamer,
		rateLimiter:      rateLimiter,
		logger:           logger,
		cancelDownload:   make(chan struct{}),
	}
}

func (step *downloadStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	step.logger.Info("acquiring-limiter")
	select {
	case step.rateLimiter <- struct{}{}:
	case <-signals:
		return new(CancelledError)
	}
	defer func() {
		<-step.rateLimiter
	}()
	step.logger.Info("acquired-limiter")

	errCh := make(chan error, 1)
	go func() {
		errCh <- step.perform()
	}()

	select {
	case err := <-errCh:
		return err
	case <-signals:
		close(step.cancelDownload)
		return new(CancelledError)
	}
}

func (step *downloadStep) perform() error {
	step.emit("Downloading %s...\n", step.model.Artifact)

	downloadedFile, downloadedSize, err := step.fetch()
	if err != nil {
		var errString string
		if step.model.Artifact != "" {
			errString = fmt.Sprintf("Downloading %s failed", step.model.Artifact)
		} else {
			errString = "Downloading failed"
		}

		step.emitError("%s\n", errString)
		return NewEmittableError(err, "%s", errString)
	}

	err = step.streamIn(step.model.To, downloadedFile)
	if err != nil {
		var errString string
		if step.model.Artifact != "" {
			errString = fmt.Sprintf("Copying %s into the container failed: %v", step.model.Artifact, err)
		} else {
			errString = fmt.Sprintf("Copying into the container failed: %v", err)
		}

		step.emitError("%s\n", errString)
		return NewEmittableError(err, "%s", errString)
	}

	if downloadedSize != 0 {
		step.emit("Downloaded %s (%s)\n", step.model.Artifact, bytefmt.ByteSize(uint64(downloadedSize)))
	} else {
		step.emit("Downloaded %s\n", step.model.Artifact)
	}

	return nil
}

func (step *downloadStep) fetch() (io.ReadCloser, int64, error) {
	step.logger.Info("fetch-starting")
	url, err := url.ParseRequestURI(step.model.From)
	if err != nil {
		step.logger.Error("parse-request-uri-error", err)
		return nil, 0, err
	}

	tarStream, downloadedSize, err := step.cachedDownloader.Fetch(
		step.logger.Session("downloader"),
		url,
		step.model.CacheKey,
		cacheddownloader.ChecksumInfoType{
			Algorithm: step.model.GetChecksumAlgorithm(),
			Value:     step.model.GetChecksumValue(),
		},
		step.cancelDownload,
	)
	if err != nil {
		step.logger.Error("fetch-failed", err)
		return nil, 0, err
	}

	step.logger.Info("fetch-complete", lager.Data{"size": downloadedSize})
	return tarStream, downloadedSize, nil
}

func (step *downloadStep) streamIn(destination string, reader io.ReadCloser) error {
	step.logger.Info("stream-in-starting")

	wrappedReader := &ReadSizer{Reader: reader}

	// StreamIn will close the reader
	err := step.container.StreamIn(garden.StreamInSpec{Path: destination, TarStream: wrappedReader, User: step.model.User})
	if err != nil {
		step.logger.Error("stream-in-failed", err, lager.Data{
			"destination": destination,
		})
		return err
	}

	step.logger.Info("stream-in-complete", lager.Data{"size": wrappedReader.BytesRead()})
	return nil
}

func (step *downloadStep) emit(format string, a ...interface{}) {
	if step.model.Artifact != "" {
		fmt.Fprintf(step.streamer.Stdout(), format, a...)
	}
}

func (step *downloadStep) emitError(format string, a ...interface{}) {
	err_bytes := []byte(fmt.Sprintf(format, a...))
	if len(err_bytes) > 1024 {
		truncation_length := 1024 - len([]byte(" (error truncated)"))
		err_bytes = append(err_bytes[:truncation_length], []byte(" (error truncated)")...)
	}

	fmt.Fprint(step.streamer.Stderr(), string(err_bytes))
}

type ReadSizer struct {
	bytesRead int
	io.Reader
}

func (r *ReadSizer) Read(dest []byte) (int, error) {
	n, err := r.Reader.Read(dest)
	r.bytesRead += n
	return n, err
}

func (r *ReadSizer) BytesRead() int {
	return r.bytesRead
}
