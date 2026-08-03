package cacheddownloader

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/v3"
)

// called after a new object has entered the cache.
// it is assumed that `path` will be removed, if a new path is returned.
// a noop transformer returns the given path and its detected size.
type CacheTransformer func(source, destination string) (newSize int64, err error)

//go:generate counterfeiter -o cacheddownloaderfakes/fake_cached_downloader.go . CachedDownloader

// CachedDownloader is responsible for downloading and caching files and maintaining reference counts for each cache entry.
// Entries in the cache with no active references are ejected from the cache when new space is needed.
type CachedDownloader interface {
	// Fetch downloads the file at the given URL and stores it in the cache with the given cacheKey.
	// If cacheKey is empty, the file will not be saved in the cache.
	//
	// Fetch returns a stream that can be used to read the contents of the downloaded file. While this stream is active (i.e., not yet closed),
	// the associated cache entry will be considered in use and will not be ejected from the cache.
	Fetch(logger lager.Logger, urlToFetch *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (stream io.ReadCloser, size int64, err error)

	// FetchAsDirectory downloads the tarfile pointed to by the given URL, expands the tarfile into a directory, and returns the path of that directory as well as the total number of bytes downloaded.
	FetchAsDirectory(logger lager.Logger, urlToFetch *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (dirPath string, size int64, err error)

	// CloseDirectory decrements the usage counter for the given cacheKey/directoryPath pair.
	// It should be called when the directory returned by FetchAsDirectory is no longer in use.
	// In this way, FetchAsDirectory and CloseDirectory should be treated as a pair of operations,
	// and a process that calls FetchAsDirectory should make sure a corresponding CloseDirectory is eventually called.
	CloseDirectory(logger lager.Logger, cacheKey, directoryPath string) error

	// SaveState writes the current state of the cache metadata to a file so that it can be recovered
	// later. This should be called on process shutdown.
	SaveState(logger lager.Logger) error

	// RecoverState checks to see if a state file exists (from a previous SaveState call), and restores
	// the cache state from that information if such a file exists. This should be called on startup.
	RecoverState(logger lager.Logger) error
}

func NoopTransform(source, destination string) (int64, error) {
	err := os.Rename(source, destination)
	if err != nil {
		return 0, err
	}

	fi, err := os.Stat(destination)
	if err != nil {
		return 0, err
	}

	return fi.Size(), nil
}

type CachingInfoType struct {
	ETag         string
	LastModified string
}

type ChecksumInfoType struct {
	Algorithm string
	Value     string
}

type cachedDownloader struct {
	downloader    *Downloader
	uncachedPath  string
	cache         *FileCache
	transformer   CacheTransformer
	cacheLocation string

	lock       *sync.Mutex
	inProgress map[string]chan struct{}
}

func (c CachingInfoType) isCacheable() bool {
	return c.ETag != "" || c.LastModified != ""
}

func (c CachingInfoType) Equal(other CachingInfoType) bool {
	return c.ETag == other.ETag && c.LastModified == other.LastModified
}

// A transformer function can be used to do post-download
// processing on the file before it is stored in the cache.
func New(
	downloader *Downloader,
	cache *FileCache,
	transformer CacheTransformer,
) (*cachedDownloader, error) {
	err := os.MkdirAll(cache.CachedPath, 0750)
	if err != nil {
		return nil, fmt.Errorf("could not create cache path %s: %s", cache.CachedPath, err)
	}

	uncachedPath, err := createTempCachedDir(cache.CachedPath)
	if err != nil {
		return nil, err
	}
	return &cachedDownloader{
		cache:         cache,
		cacheLocation: filepath.Join(cache.CachedPath, "saved_cache.json"),
		uncachedPath:  uncachedPath,
		downloader:    downloader,
		transformer:   transformer,

		lock:       &sync.Mutex{},
		inProgress: map[string]chan struct{}{},
	}, nil
}

func createTempCachedDir(path string) (string, error) {
	workDir := filepath.Join(path, "temp")
	err := os.RemoveAll(workDir)
	if err != nil {
		return "", fmt.Errorf("could not remove %s: %s", path, err)
	}

	err = os.MkdirAll(workDir, 0755)
	if err != nil {
		return "", fmt.Errorf("could not create path %s: %s", path, err)
	}
	return workDir, nil
}

func (c *cachedDownloader) SaveState(logger lager.Logger) error {
	json, err := json.Marshal(c.cache)
	if err != nil {
		return err
	}

	return os.WriteFile(c.cacheLocation, json, 0600)
}

func (c *cachedDownloader) RecoverState(logger lager.Logger) error {
	file, err := os.Open(c.cacheLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	defer file.Close()

	if err == nil {
		// parse the file only if it exists
		// #nosec G104 - we explicitly don't want json decoding errors to propagate here
		json.NewDecoder(file).Decode(c.cache)
		// #nosec G104 - we explicitly don't want file.Close errors to propagate here
		file.Close()
	}

	// set the inuse count to 0 since all containers will be recreated
	for _, entry := range c.cache.Entries {
		// inuseCount starts at 1 (i.e. 1 == no references to the entry)
		entry.directoryInUseCount = 0
		entry.fileInUseCount = 0
	}

	// delete files that aren't in the cache. **note** if there is no
	// saved_cache.json, then all files will be deleted
	trackedFiles := map[string]struct{}{}

	for _, entry := range c.cache.Entries {
		trackedFiles[entry.FilePath] = struct{}{}
		trackedFiles[entry.ExpandedDirectoryPath] = struct{}{}
	}

	files, err := os.ReadDir(c.cache.CachedPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, file := range files {
		path := filepath.Join(c.cache.CachedPath, file.Name())
		if _, ok := trackedFiles[path]; ok {
			continue
		}

		err = os.RemoveAll(path)
		if err != nil {
			return err
		}
	}

	// free some disk space in case the maxSizeInBytes was changed
	c.cache.makeRoom(logger, 0, "")

	if err := os.Mkdir(c.uncachedPath, 0755); err != nil {
		return err
	}

	return err
}

func (c *cachedDownloader) CloseDirectory(logger lager.Logger, cacheKey, directoryPath string) error {
	cacheKey = fmt.Sprintf("%x", md5.Sum([]byte(cacheKey)))
	return c.cache.CloseDirectory(logger, cacheKey, directoryPath)
}

func (c *cachedDownloader) Fetch(logger lager.Logger, url *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (io.ReadCloser, int64, error) {
	if cacheKey == "" {
		return c.fetchUncachedFile(logger, url, checksum, cancelChan)
	}

	cacheKey = fmt.Sprintf("%x", md5.Sum([]byte(cacheKey)))
	return c.fetchCachedFile(logger, url, cacheKey, checksum, cancelChan)
}

func (c *cachedDownloader) fetchUncachedFile(logger lager.Logger, url *url.URL, checksum ChecksumInfoType, cancelChan <-chan struct{}) (*CachedFile, int64, error) {
	download, _, size, err := c.populateCache(logger, url, "uncached", CachingInfoType{}, checksum, c.transformer, cancelChan)
	if err != nil {
		return nil, 0, err
	}

	file, err := tempFileRemoveOnClose(download.path)
	return file, size, err
}

func (c *cachedDownloader) fetchCachedFile(logger lager.Logger, url *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (*CachedFile, int64, error) {
	rateLimiter, err := c.acquireLimiter(logger, cacheKey, cancelChan)
	if err != nil {
		return nil, 0, err
	}
	defer c.releaseLimiter(cacheKey, rateLimiter)

	// lookup cache entry
	currentReader, currentCachingInfo, getErr := c.cache.Get(logger, cacheKey)

	// download (short circuits if endpoint respects etag/etc.)
	download, cacheIsWarm, size, err := c.populateCache(logger, url, cacheKey, currentCachingInfo, checksum, c.transformer, cancelChan)
	if err != nil {
		if currentReader != nil {
			// #nosec G104 - ignore errors closing a cache we're disregarding
			currentReader.Close()
		}
		return nil, 0, err
	}

	// nothing had to be downloaded; return the cached entry
	if cacheIsWarm {
		logger.Info("file-found-in-cache", lager.Data{"cache_key": cacheKey, "size": size})
		return currentReader, 0, getErr
	}

	// current cache is not fresh; disregard it
	if currentReader != nil {
		// #nosec G104 - ignore errors closing a cache we're disregarding
		currentReader.Close()
	}

	// fetch uncached data
	var newReader *CachedFile
	if download.cachingInfo.isCacheable() {
		newReader, err = c.cache.Add(logger, cacheKey, download.path, download.size, download.cachingInfo)
	} else {
		c.cache.Remove(logger, cacheKey)
		newReader, err = tempFileRemoveOnClose(download.path)
	}

	// return newly fetched file
	return newReader, size, err
}

func (c *cachedDownloader) FetchAsDirectory(logger lager.Logger, url *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (string, int64, error) {
	if cacheKey == "" {
		return "", 0, MissingCacheKeyErr
	}

	cacheKey = fmt.Sprintf("%x", md5.Sum([]byte(cacheKey)))
	return c.fetchCachedDirectory(logger, url, cacheKey, checksum, cancelChan)
}

func (c *cachedDownloader) fetchCachedDirectory(logger lager.Logger, url *url.URL, cacheKey string, checksum ChecksumInfoType, cancelChan <-chan struct{}) (string, int64, error) {
	rateLimiter, err := c.acquireLimiter(logger, cacheKey, cancelChan)
	if err != nil {
		return "", 0, err
	}
	defer c.releaseLimiter(cacheKey, rateLimiter)

	// lookup cache entry
	currentDirectory, currentCachingInfo, getErr := c.cache.GetDirectory(logger, cacheKey)

	// download (short circuits if endpoint respects etag/etc.)
	download, cacheIsWarm, size, err := c.populateCache(logger, url, cacheKey, currentCachingInfo, checksum, TarTransform, cancelChan)
	if err != nil {
		if currentDirectory != "" {
			closeErr := c.cache.CloseDirectory(logger, cacheKey, currentDirectory)
			if closeErr != nil {
				logger.Debug("failed-to-close-cached-dir", lager.Data{"error": closeErr, "dir": currentDirectory})
			}
		}
		return "", 0, err
	}

	// nothing had to be downloaded; return the cached entry
	if cacheIsWarm {
		logger.Info("directory-found-in-cache", lager.Data{"cache_key": cacheKey, "size": size})
		return currentDirectory, 0, getErr
	}

	// current cache is not fresh; disregard it
	if currentDirectory != "" {
		closeErr := c.cache.CloseDirectory(logger, cacheKey, currentDirectory)
		if closeErr != nil {
			logger.Debug("failed-to-close-cached-dir", lager.Data{"error": closeErr, "dir": currentDirectory})
		}
	}

	// fetch uncached data
	var newDirectory string
	if download.cachingInfo.isCacheable() {
		newDirectory, err = c.cache.AddDirectory(logger, cacheKey, download.path, download.size, download.cachingInfo)
		// return newly fetched directory
		return newDirectory, size, err
	}

	c.cache.Remove(logger, cacheKey)
	return "", 0, MissingCacheHeadersErr
}

func (c *cachedDownloader) acquireLimiter(logger lager.Logger, cacheKey string, cancelChan <-chan struct{}) (chan struct{}, error) {
	startTime := time.Now()
	logger = logger.Session("acquire-rate-limiter", lager.Data{"cache-key": cacheKey})
	logger.Info("starting")
	defer func() {
		logger.Info("completed", lager.Data{"duration-ns": time.Since(startTime)})
	}()

	for {
		c.lock.Lock()
		rateLimiter := c.inProgress[cacheKey]
		if rateLimiter == nil {
			rateLimiter = make(chan struct{})
			c.inProgress[cacheKey] = rateLimiter
			c.lock.Unlock()
			return rateLimiter, nil
		}
		c.lock.Unlock()

		select {
		case <-rateLimiter:
		case <-cancelChan:
			return nil, NewDownloadCancelledError("acquire-limiter", time.Since(startTime), NoBytesReceived, nil)
		}
	}
}

func (c *cachedDownloader) releaseLimiter(cacheKey string, limiter chan struct{}) {
	c.lock.Lock()
	delete(c.inProgress, cacheKey)
	close(limiter)
	c.lock.Unlock()
}

func tempFileRemoveOnClose(path string) (*CachedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return NewFileCloser(f, func(path string) {
		// #nosec G104 - we're just trying to clean up temp files that we've created. best effort at clean up, ignore errors
		os.RemoveAll(path)
	}), nil
}

type download struct {
	path        string
	size        int64
	cachingInfo CachingInfoType
}

// Currently populateCache takes a transformer due to the fact that a fetchCachedDirectory
// uses only a TarTransformer, which overwrites what is currently set. This way one transformer
// can be used to call Fetch and FetchAsDirectory
func (c *cachedDownloader) populateCache(
	logger lager.Logger,
	url *url.URL,
	name string,
	cachingInfo CachingInfoType,
	checksum ChecksumInfoType,
	transformer CacheTransformer,
	cancelChan <-chan struct{},
) (download, bool, int64, error) {
	filename, cachingInfo, err := c.downloader.Download(logger, url, func() (*os.File, error) {
		return os.CreateTemp(c.uncachedPath, name+"-")
	}, cachingInfo, checksum, cancelChan)
	if err != nil {
		return download{}, false, 0, err
	}

	if filename == "" {
		return download{}, true, 0, nil
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		return download{}, false, 0, err
	}
	defer os.Remove(filename)

	cachedFile, err := os.CreateTemp(c.uncachedPath, "transformed")
	if err != nil {
		return download{}, false, 0, err
	}

	err = cachedFile.Close()
	if err != nil {
		return download{}, false, 0, err
	}

	cachedSize, err := transformer(filename, cachedFile.Name())
	if err != nil {
		return download{}, false, 0, err
	}

	return download{
		path:        cachedFile.Name(),
		size:        cachedSize,
		cachingInfo: cachingInfo,
	}, false, fileInfo.Size(), nil
}
