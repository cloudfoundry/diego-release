package steps

import (
	"archive/tar"
	"fmt"
	"io"
	"net/url"
	"os"

	"code.cloudfoundry.org/archiver/compressor"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bytefmt"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/executor/depot/uploader"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

type uploadStep struct {
	container   garden.Container
	model       models.UploadAction
	uploader    uploader.Uploader
	compressor  compressor.Compressor
	tempDir     string
	streamer    log_streamer.LogStreamer
	rateLimiter chan struct{}
	logger      lager.Logger

	cancelUpload chan struct{}
}

func NewUpload(
	container garden.Container,
	model models.UploadAction,
	uploader uploader.Uploader,
	compressor compressor.Compressor,
	tempDir string,
	streamer log_streamer.LogStreamer,
	rateLimiter chan struct{},
	logger lager.Logger,
) ifrit.Runner {
	logger = logger.Session("upload-step", lager.Data{
		"from": model.From,
	})

	return &uploadStep{
		container:   container,
		model:       model,
		uploader:    uploader,
		compressor:  compressor,
		tempDir:     tempDir,
		streamer:    streamer,
		rateLimiter: rateLimiter,
		logger:      logger,

		cancelUpload: make(chan struct{}),
	}
}

const (
	ErrCreateTmpDir    = "Failed to create temp dir"
	ErrEstablishStream = "Failed to establish stream from container"
	ErrReadTar         = "Failed to find first item in tar stream"
	ErrCreateTmpFile   = "Failed to create temp file"
	ErrCopyStreamToTmp = "Failed to copy stream contents into temp file"
	ErrParsingURL      = "Failed to parse URL"
)

func (step *uploadStep) Run(signals <-chan os.Signal, ready chan<- struct{}) (err error) {
	close(ready)

	step.rateLimiter <- struct{}{}
	defer func() {
		<-step.rateLimiter
	}()

	step.logger.Info("upload-starting")
	step.emit("Uploading %s...\n", step.model.Artifact)

	url, err := url.ParseRequestURI(step.model.To)
	if err != nil {
		step.logger.Error("failed-to-parse-url", err)
		step.emitError(step.artifactErrString(ErrParsingURL))
		// Do not emit error in case it leaks sensitive data in URL
		return err
	}

	tempDir, err := os.MkdirTemp(step.tempDir, "upload")
	if err != nil {
		step.logger.Error("failed-to-create-tmp-dir", err)
		errString := step.artifactErrString(ErrCreateTmpDir)
		step.emitError(errString)
		return NewEmittableError(err, "%s", errString)
	}

	defer os.RemoveAll(tempDir)

	outStream, err := step.container.StreamOut(garden.StreamOutSpec{Path: step.model.From, User: step.model.User})
	if err != nil {
		step.logger.Error("failed-to-stream-out", err)
		errString := step.artifactErrString(ErrEstablishStream)
		step.emitError(errString)
		return NewEmittableError(err, "%s", errString)
	}
	defer outStream.Close()

	tarStream := tar.NewReader(outStream)
	_, err = tarStream.Next()

	if err != nil {
		step.logger.Error("failed-to-read-stream", err)
		errString := step.artifactErrString(ErrReadTar)
		step.emitError(errString)
		return NewEmittableError(err, "%s", errString)
	}

	tempFile, err := os.CreateTemp(step.tempDir, "compressed")
	if err != nil {
		step.logger.Error("failed-to-create-tmp-dir", err)
		errString := step.artifactErrString(ErrCreateTmpFile)
		step.emitError(errString)
		return NewEmittableError(err, "%s", errString)
	}
	finalFileLocation := tempFile.Name()
	defer func() {
		deferErr := tempFile.Close()
		if deferErr != nil {
			step.logger.Debug("failed-to-close-file", lager.Data{"error": deferErr, "file": finalFileLocation})
		}
		deferErr = os.Remove(finalFileLocation)
		if deferErr != nil {
			step.logger.Debug("failed-to-remove-file", lager.Data{"error": deferErr, "file": finalFileLocation})
		}
	}()

	// #nosec - G110 - We're fine with unbounded file decompression here as we have container filesystem quotas that will prevent this from eating up the entire diego cell disk space
	_, err = io.Copy(tempFile, tarStream)
	if err != nil {
		step.logger.Error("failed-to-copy-stream", err)
		errString := step.artifactErrString(ErrCopyStreamToTmp)
		step.emitError(errString)
		return NewEmittableError(err, "%s", errString)
	}

	finished := make(chan struct{})
	defer close(finished)
	go step.cancelUploadOnSignal(finished, signals)

	uploadedBytes, err := step.uploader.Upload(finalFileLocation, url, step.cancelUpload)
	if err != nil {
		select {
		case <-step.cancelUpload:
			return new(CancelledError)

		default:
			step.logger.Error("failed-to-upload", err)

			// Do not emit error in case it leaks sensitive data in URL
			step.emitError(step.artifactErrString("Failed to upload payload"))

			return err
		}
	}

	step.emit("Uploaded %s (%s)\n", step.model.Artifact, bytefmt.ByteSize(uint64(uploadedBytes)))

	step.logger.Info("upload-successful")
	return nil
}

func (step *uploadStep) cancelUploadOnSignal(finished chan struct{}, signals <-chan os.Signal) {
	select {
	case <-signals:
		close(step.cancelUpload)
	case <-finished:
	}
}

func (step *uploadStep) emit(format string, a ...interface{}) {
	if step.model.Artifact != "" {
		fmt.Fprintf(step.streamer.Stdout(), format, a...)
	}
}

func (step *uploadStep) artifactErrString(errString string) string {
	if step.model.Artifact != "" {
		return fmt.Sprintf("%s for %s", errString, step.model.Artifact)
	}
	return errString
}

func (step *uploadStep) emitError(errString string) {
	if step.model.Artifact != "" {
		fmt.Fprintln(step.streamer.Stderr(), errString)
	}
}
