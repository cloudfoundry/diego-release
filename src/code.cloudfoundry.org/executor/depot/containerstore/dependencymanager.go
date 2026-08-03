package containerstore

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bytefmt"
	"code.cloudfoundry.org/cacheddownloader"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter -o containerstorefakes/fake_bindmounter.go . DependencyManager

type DependencyManager interface {
	DownloadCachedDependencies(logger lager.Logger, mounts []executor.CachedDependency, logconfig models.LogConfig, metronClient loggingclient.IngressClient) (BindMounts, error)
	ReleaseCachedDependencies(logger lager.Logger, keys []BindMountCacheKey) error
	Stop(logger lager.Logger)
}

type dependencyManager struct {
	cache               cacheddownloader.CachedDownloader
	downloadRateLimiter chan struct{}
}

func NewDependencyManager(cache cacheddownloader.CachedDownloader, downloadRateLimiter chan struct{}) DependencyManager {
	return &dependencyManager{cache, downloadRateLimiter}
}

func (bm *dependencyManager) Stop(logger lager.Logger) {
	logger.Debug("stopping")
	defer logger.Debug("stopping-complete")
	err := bm.cache.SaveState(logger.Session("downloader"))
	if err != nil {
		logger.Error("failed-saving-cache-state", err, lager.Data{"err": err})
	}
}

func (bm *dependencyManager) DownloadCachedDependencies(logger lager.Logger, mounts []executor.CachedDependency, logConfig models.LogConfig, metronClient loggingclient.IngressClient) (BindMounts, error) {
	logger.Debug("downloading-cached-dependencies")
	defer logger.Debug("downloading-cached-dependencies-complete")

	total := len(mounts)
	completed := 0
	mountChan := make(chan *cachedBindMount, total)
	errChan := make(chan error, total)
	cancelChan := make(chan struct{})
	bindMounts := NewBindMounts(total)

	if total == 0 {
		return bindMounts, nil
	}

	var wg sync.WaitGroup

	for i := range mounts {
		wg.Add(1)
		go func(mount *executor.CachedDependency) {
			defer wg.Done()

			limiterStart := time.Now()
			bm.downloadRateLimiter <- struct{}{}
			limiterTime := time.Since(limiterStart)
			logger.Info("cached-dependency-rate-limiter", lager.Data{"cache-key": mount.CacheKey, "duration-ns": limiterTime})

			defer func() {
				<-bm.downloadRateLimiter
			}()

			cachedMount, err := bm.downloadCachedDependency(logger, mount, logConfig, metronClient, cancelChan)
			if err != nil {
				errChan <- err
			} else {
				mountChan <- cachedMount
			}
		}(&mounts[i])
	}

	for {
		select {
		case err := <-errChan:
			close(cancelChan)
			wg.Wait()
			return bindMounts, err
		case cachedMount := <-mountChan:
			bindMounts.AddBindMount(cachedMount.CacheKey, cachedMount.BindMount)
			completed++
			if total == completed {
				return bindMounts, nil
			}
		}
	}
}

func (bm *dependencyManager) downloadCachedDependency(logger lager.Logger, mount *executor.CachedDependency, logConfig models.LogConfig, metronClient loggingclient.IngressClient, cancelChan <-chan struct{}) (*cachedBindMount, error) {
	sourceName, tags := logConfig.GetSourceNameAndTagsForLogging()
	if mount.LogSource != "" {
		sourceName = mount.LogSource
	}
	var metricErr error
	if mount.Name != "" {
		metricErr = metronClient.SendAppLog(fmt.Sprintf("Downloading %s...", mount.Name), sourceName, tags)
		if metricErr != nil {
			logger.Debug("failed-to-emit-appLog", lager.Data{"error": metricErr})
		}
	}

	downloadURL, err := url.Parse(mount.From)
	if err != nil {
		logger.Error("failed-parsing-bind-mount-download-url", err, lager.Data{"download-url": mount.From, "cache-key": mount.CacheKey})
		if mount.Name != "" {
			metricErr = metronClient.SendAppLog(fmt.Sprintf("Downloading %s failed", mount.Name), sourceName, tags)
			if metricErr != nil {
				logger.Debug("failed-to-emit-appLog", lager.Data{"error": metricErr})
			}
		}
		return nil, err
	}

	logger.Debug("fetching-cache-dependency", lager.Data{"download-url": downloadURL.String(), "cache-key": mount.CacheKey})
	dirPath, downloadedSize, err := bm.cache.FetchAsDirectory(
		logger.Session("downloader"),
		downloadURL,
		mount.CacheKey,
		cacheddownloader.ChecksumInfoType{
			Algorithm: mount.ChecksumAlgorithm,
			Value:     mount.ChecksumValue,
		},
		cancelChan,
	)
	if err != nil {
		logger.Error("failed-fetching-cache-dependency", err, lager.Data{"download-url": downloadURL.String(), "cache-key": mount.CacheKey})
		if mount.Name != "" {
			metricErr = metronClient.SendAppLog(fmt.Sprintf("Downloading %s failed", mount.Name), sourceName, tags)
			if metricErr != nil {
				logger.Debug("failed-to-emit-appLog", lager.Data{"error": metricErr})
			}
		}
		return nil, err
	}
	logger.Debug("fetched-cache-dependency", lager.Data{"download-url": downloadURL.String(), "cache-key": mount.CacheKey, "size": downloadedSize})

	if downloadedSize != 0 {
		if mount.Name != "" {
			metricErr = metronClient.SendAppLog(fmt.Sprintf("Downloaded %s (%s)", mount.Name, bytefmt.ByteSize(uint64(downloadedSize))), sourceName, tags)
			if metricErr != nil {
				logger.Debug("failed-to-emit-appLog", lager.Data{"error": metricErr})
			}
		}
	} else {
		if mount.Name != "" {
			metricErr = metronClient.SendAppLog(fmt.Sprintf("Downloaded %s", mount.Name), sourceName, tags)
			if metricErr != nil {
				logger.Debug("failed-to-emit-appLog", lager.Data{"error": metricErr})
			}
		}
	}
	return newCachedBindMount(mount.CacheKey, newBindMount(dirPath, mount.To)), nil
}

func (bm *dependencyManager) ReleaseCachedDependencies(logger lager.Logger, keys []BindMountCacheKey) error {
	for i := range keys {
		key := &keys[i]
		logger.Debug("releasing-cache-key", lager.Data{"cache-key": key.CacheKey, "dir": key.Dir})
		err := bm.cache.CloseDirectory(logger, key.CacheKey, key.Dir)
		if err != nil && err != cacheddownloader.AlreadyClosed && err != cacheddownloader.EntryNotFound {
			logger.Error("failed-releasing-cache-key", err, lager.Data{"cache-key": key.CacheKey, "dir": key.Dir})
			return err
		}
	}
	return nil
}

type cachedBindMount struct {
	CacheKey  string
	BindMount garden.BindMount
}

func newCachedBindMount(key string, mount garden.BindMount) *cachedBindMount {
	return &cachedBindMount{
		CacheKey:  key,
		BindMount: mount,
	}
}

type BindMounts struct {
	CacheKeys        []BindMountCacheKey
	GardenBindMounts []garden.BindMount
}

func NewBindMounts(capacity int) BindMounts {
	return BindMounts{
		CacheKeys:        make([]BindMountCacheKey, 0, capacity),
		GardenBindMounts: make([]garden.BindMount, 0, capacity),
	}
}

func (b *BindMounts) AddBindMount(cacheKey string, mount garden.BindMount) {
	b.CacheKeys = append(b.CacheKeys, NewbindMountCacheKey(cacheKey, mount.SrcPath))
	b.GardenBindMounts = append(b.GardenBindMounts, mount)
}

type BindMountCacheKey struct {
	CacheKey string
	Dir      string
}

func NewbindMountCacheKey(cacheKey, dir string) BindMountCacheKey {
	return BindMountCacheKey{CacheKey: cacheKey, Dir: dir}
}
