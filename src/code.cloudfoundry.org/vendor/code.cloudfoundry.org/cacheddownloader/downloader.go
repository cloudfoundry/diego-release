package cacheddownloader

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/lager/v3"
)

const (
	MAX_DOWNLOAD_ATTEMPTS = 4
	IDLE_TIMEOUT          = 10 * time.Second
	RETRY_WAIT_MIN        = 500 * time.Millisecond
	RETRY_WAIT_MAX        = 5 * time.Second
	MAX_JITTER            = 200 * time.Millisecond
	NoBytesReceived       = -1
)

func backoff(minRetry, maxRetry, maxJitter time.Duration, attemptNum int) time.Duration {
	mult := math.Pow(2, float64(attemptNum)) * float64(minRetry)
	sleep := time.Duration(mult)
	if float64(sleep) != mult || sleep > maxRetry {
		sleep = maxRetry
	}
	jitter := (MAX_JITTER / 100) * time.Duration(rand.Intn(100))
	return sleep + jitter
}

type DownloadCancelledError struct {
	source   string
	duration time.Duration
	written  int64

	additionalError error
}

func NewDownloadCancelledError(source string, duration time.Duration, written int64, additionalError error) error {
	return &DownloadCancelledError{
		source:   source,
		duration: duration,
		written:  written,

		additionalError: additionalError,
	}
}

func (e *DownloadCancelledError) Error() string {
	msg := fmt.Sprintf("Download cancelled: source '%s', duration '%s'", e.source, e.duration)
	if e.written != NoBytesReceived {
		msg = fmt.Sprintf("%s, bytes '%d'", msg, e.written)
	}

	if e.additionalError != nil {
		msg = fmt.Sprintf("%s, Error: %s", msg, e.additionalError.Error())
	}
	return msg
}

type idleTimeoutConn struct {
	Timeout time.Duration
	net.Conn
}

func (c *idleTimeoutConn) Read(b []byte) (n int, err error) {
	if err = c.Conn.SetDeadline(time.Now().Add(c.Timeout)); err != nil {
		return
	}
	return c.Conn.Read(b)
}

func (c *idleTimeoutConn) Write(b []byte) (n int, err error) {
	if err = c.Conn.SetDeadline(time.Now().Add(c.Timeout)); err != nil {
		return
	}
	return c.Conn.Write(b)
}

type Downloader struct {
	client                    *http.Client
	concurrentDownloadBarrier chan struct{}
}

func NewDownloader(requestTimeout time.Duration, maxConcurrentDownloads int, tlsConfig *tls.Config) *Downloader {
	return NewDownloaderWithIdleTimeout(requestTimeout, 10*time.Second, maxConcurrentDownloads, tlsConfig)
}

func NewDownloaderWithIdleTimeout(requestTimeout time.Duration, idleTimeout time.Duration, maxConcurrentDownloads int, tlsConfig *tls.Config) *Downloader {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: func(netw, addr string) (net.Conn, error) {
			c, err := net.DialTimeout(netw, addr, 10*time.Second)
			if err != nil {
				return nil, err
			}
			if tc, ok := c.(*net.TCPConn); ok {
				// #nosec G104 - avoid logging this error because it will spam for each request made
				tc.SetKeepAlive(true)
				// #nosec G104 - avoid logging this error because it will spam for each request made
				tc.SetKeepAlivePeriod(30 * time.Second)
			}
			return &idleTimeoutConn{idleTimeout, c}, nil
		},
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
	}
	return &Downloader{
		client:                    client,
		concurrentDownloadBarrier: make(chan struct{}, maxConcurrentDownloads),
	}
}

func (downloader *Downloader) Download(
	logger lager.Logger,
	url *url.URL,
	createDestination func() (*os.File, error),
	cachingInfoIn CachingInfoType,
	checksum ChecksumInfoType,
	cancelChan <-chan struct{},
) (path string, cachingInfoOut CachingInfoType, err error) {

	startTime := time.Now()
	logger = logger.Session("download", lager.Data{"host": url.Host})
	logger.Info("starting")
	defer func() { logger.Info("completed", lager.Data{"duration-ns": time.Since(startTime)}) }()

	select {
	case downloader.concurrentDownloadBarrier <- struct{}{}:
	case <-cancelChan:
		return "", CachingInfoType{}, NewDownloadCancelledError("download-barrier", time.Since(startTime), NoBytesReceived, nil)
	}
	logger.Info("download-barrier", lager.Data{"duration-ns": time.Since(startTime)})

	defer func() {
		<-downloader.concurrentDownloadBarrier
	}()

	for attempt := 0; attempt < MAX_DOWNLOAD_ATTEMPTS; attempt++ {
		path, cachingInfoOut, err = downloader.fetchToFile(logger, url, createDestination, cachingInfoIn, checksum, cancelChan)
		if err == nil {
			break
		}
		if _, ok := err.(*DownloadCancelledError); ok {
			break
		}
		if _, ok := err.(*ChecksumFailedError); ok {
			break
		}
		logger.Info("retrying", lager.Data{"attempt": attempt})
		time.Sleep(backoff(RETRY_WAIT_MIN, RETRY_WAIT_MAX, MAX_JITTER, attempt))
	}

	if err != nil {
		logger.Error("download-failed", err)
		return "", CachingInfoType{}, err
	}

	return
}

func (downloader *Downloader) fetchToFile(
	logger lager.Logger,
	url *url.URL,
	createDestination func() (*os.File, error),
	cachingInfoIn CachingInfoType,
	checksum ChecksumInfoType,
	cancelChan <-chan struct{},
) (string, CachingInfoType, error) {
	var req *http.Request
	var err error

	req, err = http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return "", CachingInfoType{}, err
	}
	if cachingInfoIn.ETag != "" {
		req.Header.Add("If-None-Match", cachingInfoIn.ETag)
	}
	if cachingInfoIn.LastModified != "" {
		req.Header.Add("If-Modified-Since", cachingInfoIn.LastModified)
	}

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	req = req.WithContext(ctx)

	completeChan := make(chan struct{})
	defer close(completeChan)

	go func() {
		select {
		case <-completeChan:
		case <-cancelChan:
			cancel()
		}
	}()

	startTime := time.Now()

	var resp *http.Response
	reqStart := time.Now()
	resp, err = downloader.client.Do(req)
	logger.Info("fetch-request", lager.Data{"duration-ns": time.Since(reqStart)})

	if err != nil {
		select {
		case <-cancelChan:
			err = NewDownloadCancelledError("fetch-request", time.Since(startTime), NoBytesReceived, err)
		default:
		}
		return "", CachingInfoType{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return "", CachingInfoType{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return "", CachingInfoType{}, fmt.Errorf("Download failed: Status code %d", resp.StatusCode)
	}

	var destinationFile *os.File
	destinationFile, err = createDestination()
	if err != nil {
		return "", CachingInfoType{}, err
	}

	go func() {
		select {
		case <-completeChan:
		case <-cancelChan:
			// #nosec G104 - ignore errors from closing response bodies to avoid spamming logs with network issues
			resp.Body.Close()
		}
	}()

	return copyToDestinationFile(logger, destinationFile, resp, checksum, cancelChan)
}

func copyToDestinationFile(
	logger lager.Logger,
	destinationFile *os.File,
	resp *http.Response,
	checksum ChecksumInfoType,
	cancelChan <-chan struct{},
) (string, CachingInfoType, error) {
	var err error
	var checksumValidator *hashValidator
	logger = logger.Session("copy-to-destination-file", lager.Data{"destination": destinationFile.Name()})

	defer func() {
		closeErr := destinationFile.Close()
		if closeErr != nil {
			logger.Debug("failed-to-close-destination-file", lager.Data{"error": closeErr, "destination": destinationFile.Name()})
		}
		if err != nil {
			rmErr := os.Remove(destinationFile.Name())
			if rmErr != nil {
				logger.Debug("failed-to-delete-destination-file", lager.Data{"error": rmErr, "destination": destinationFile.Name()})
			}
		}
	}()

	_, err = destinationFile.Seek(0, 0)
	if err != nil {
		return "", CachingInfoType{}, err
	}

	err = destinationFile.Truncate(0)
	if err != nil {
		return "", CachingInfoType{}, err
	}

	ioWriters := []io.Writer{destinationFile}

	// if checksum data is provided, create the checksum validator
	if checksum.Algorithm != "" || checksum.Value != "" {
		checksumValidator, err = NewHashValidator(checksum.Algorithm)
		if err != nil {
			return "", CachingInfoType{}, err
		}
		ioWriters = append(ioWriters, checksumValidator.hash)
	}

	startTime := time.Now()
	written, err := io.Copy(io.MultiWriter(ioWriters...), resp.Body)

	if err != nil {
		logger.Error("copy-failed", err, lager.Data{"duration-ns": time.Since(startTime), "bytes-written": written})
		select {
		case <-cancelChan:
			err = NewDownloadCancelledError("copy-body", time.Since(startTime), written, err)
		default:
		}
		return "", CachingInfoType{}, err
	}
	logger.Info("copy-finished", lager.Data{"duration-ns": time.Since(startTime), "bytes-written": written})

	cachingInfoOut := CachingInfoType{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	// validate checksum
	if checksumValidator != nil {
		err = checksumValidator.Validate(checksum.Value)
		if err != nil {
			return "", CachingInfoType{}, err
		}
	}

	return destinationFile.Name(), cachingInfoOut, nil
}
