package uploader

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/lager/v3"
)

var ErrUploadCancelled = errors.New("upload cancelled")

type Uploader interface {
	Upload(fileLocation string, destinationUrl *url.URL, cancel <-chan struct{}) (int64, error)
}

type URLUploader struct {
	httpClient *http.Client
	tlsConfig  *tls.Config
	transport  *http.Transport
	logger     lager.Logger
}

func New(logger lager.Logger, timeout time.Duration, tlsConfig *tls.Config) Uploader {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return &URLUploader{
		httpClient: httpClient,
		tlsConfig:  tlsConfig,
		transport:  transport,
		logger:     logger.Session("URLUploader"),
	}
}

func (uploader *URLUploader) Upload(fileLocation string, url *url.URL, cancel <-chan struct{}) (int64, error) {
	logger := uploader.logger.WithData(lager.Data{"fileLocation": fileLocation})

	sourceFile, bytesToUpload, contentMD5, contentSHA256, err := uploader.prepareFileForUpload(fileLocation, logger)
	if err != nil {
		return 0, err
	}
	defer sourceFile.Close()

UPLOAD_ATTEMPTS:
	for attempt := 0; attempt < 3; attempt++ {
		logger := logger.WithData(lager.Data{"attempt": attempt})
		logger.Info("uploading")
		err = uploader.attemptUpload(
			sourceFile,
			bytesToUpload,
			contentMD5,
			contentSHA256,
			url.String(),
			cancel,
			logger,
		)
		switch err {
		case nil:
			logger.Info("succeeded-uploading")
			break UPLOAD_ATTEMPTS
		case ErrUploadCancelled:
			logger.Info("cancelled-uploading")
			break UPLOAD_ATTEMPTS
		default:
			logger.Error("failed-uploading", err)
		}
	}

	if err != nil {
		logger.Error("failed-all-upload-attempts", err)
		return 0, err
	}

	return int64(bytesToUpload), nil
}

func (uploader *URLUploader) prepareFileForUpload(fileLocation string, logger lager.Logger) (*os.File, int64, string, string, error) {
	sourceFile, err := os.Open(fileLocation)
	if err != nil {
		logger.Error("failed-open", err)
		return nil, 0, "", "", err
	}

	fileInfo, err := sourceFile.Stat()
	if err != nil {
		logger.Error("failed-stat", err)
		return nil, 0, "", "", err
	}

	md5Checksum := md5.New()
	sha256Checksum := sha256.New()
	_, err = io.Copy(io.MultiWriter(md5Checksum, sha256Checksum), bufio.NewReader(sourceFile))
	if err != nil {
		logger.Error("failed-read", err)
		return nil, 0, "", "", err
	}

	contentMD5 := base64.StdEncoding.EncodeToString(md5Checksum.Sum(nil))
	contentSHA256 := base64.StdEncoding.EncodeToString(sha256Checksum.Sum(nil))

	return sourceFile, fileInfo.Size(), contentMD5, contentSHA256, nil
}

func (uploader *URLUploader) attemptUpload(
	sourceFile *os.File,
	bytesToUpload int64,
	contentMD5 string,
	contentSHA256 string,
	url string,
	cancelCh <-chan struct{},
	logger lager.Logger,
) error {
	_, err := sourceFile.Seek(0, 0)
	if err != nil {
		logger.Error("failed-seek", err)
		return err
	}

	request, err := http.NewRequest("POST", url, io.NopCloser(sourceFile))
	if err != nil {
		logger.Error("somehow-failed-to-create-request", err)
		return err
	}

	ctx, reqCancel := context.WithCancel(context.Background())
	defer reqCancel()
	request = request.WithContext(ctx)
	request.ContentLength = bytesToUpload
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Content-MD5", contentMD5)
	request.Header.Set("Content-Digest", fmt.Sprintf("sha-256=:%s:", contentSHA256))

	var resp *http.Response
	reqComplete := make(chan error)
	go func() {
		var err error
		resp, err = uploader.httpClient.Do(request)
		reqComplete <- err
	}()

	select {
	case <-cancelCh:
		logger.Info("canceled-upload")
		reqCancel()
		<-reqComplete
		return ErrUploadCancelled
	case err := <-reqComplete:
		if err != nil {
			return err
		}
	}

	// access to resp has been syncronized via reqComplete
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Upload failed: Status code %d", resp.StatusCode)
	}

	return nil
}
