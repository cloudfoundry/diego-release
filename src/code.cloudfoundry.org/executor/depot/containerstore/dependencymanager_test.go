package containerstore_test

import (
	"errors"
	"net/url"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/cacheddownloader/cacheddownloaderfakes"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DependencyManager", func() {
	var (
		dependencyManager   containerstore.DependencyManager
		cache               *cacheddownloaderfakes.FakeCachedDownloader
		dependencies        []executor.CachedDependency
		fakeClient          *mfakes.FakeIngressClient
		logConfig           models.LogConfig
		downloadRateLimiter chan struct{}
	)

	BeforeEach(func() {
		cache = &cacheddownloaderfakes.FakeCachedDownloader{}
		fakeClient = &mfakes.FakeIngressClient{}
		logConfig = models.LogConfig{Guid: "test", SourceName: "test", Index: 0, Tags: map[string]string{}}
		downloadRateLimiter = make(chan struct{}, 2)
		dependencyManager = containerstore.NewDependencyManager(cache, downloadRateLimiter)
		dependencies = []executor.CachedDependency{
			{Name: "name-1", CacheKey: "cache-key-1", LogSource: "log-source-1", From: "https://user:pass@example.com:8080/download-1", To: "/var/data/buildpack-1"},
			{CacheKey: "cache-key-2", LogSource: "log-source-2", From: "http://example.com:1515/download-2", To: "/var/data/buildpack-2"},
		}
	})

	Context("when fetching one of the dependencies fails", func() {
		var dependency2ReceivedCancel, dependency3ReceivedCancel bool

		BeforeEach(func() {
			downloadRateLimiter = make(chan struct{}, 3)
			dependencyManager = containerstore.NewDependencyManager(cache, downloadRateLimiter)
			dependencies = append(dependencies,
				executor.CachedDependency{CacheKey: "cache-key-3", LogSource: "log-source-3", From: "https://example.com:8080/download-3", To: "/var/data/buildpack-3"},
			)

			cache.FetchAsDirectoryStub = func(logger lager.Logger, urlToFetch *url.URL, cacheKey string, checksum cacheddownloader.ChecksumInfoType, cancelChan <-chan struct{}) (dirPath string, size int64, err error) {
				switch cacheKey {
				case "cache-key-1":
					time.Sleep(1 * time.Second)
					return "", 0, errors.New("failed-to-fetch")
				case "cache-key-2":
					<-cancelChan
					dependency2ReceivedCancel = true
					return "", 0, errors.New("canceled")
				case "cache-key-3":
					<-cancelChan
					dependency3ReceivedCancel = true
					return "", 0, errors.New("canceled")
				}
				return "", 0, errors.New("unknown-cache-key")
			}
		})

		It("stops downloading the rest of the dependencies", func() {
			_, err := dependencyManager.DownloadCachedDependencies(logger, dependencies, logConfig, fakeClient)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed-to-fetch"))
			Expect(dependency2ReceivedCancel).To(BeTrue())
			Expect(dependency3ReceivedCancel).To(BeTrue())
		})
	})

	Context("when fetching all of the dependencies succeeds", func() {
		var bindMounts containerstore.BindMounts

		BeforeEach(func() {
			cache.FetchAsDirectoryReturns("/tmp/download/dependencies", 123, nil)
			var err error
			bindMounts, err = dependencyManager.DownloadCachedDependencies(logger, dependencies, logConfig, fakeClient)
			Expect(err).NotTo(HaveOccurred())
		})

		It("emits the download log messages for downloads with names", func() {
			Eventually(fakeClient.SendAppLogCallCount).Should(Equal(2))
			msg, logSource, _ := fakeClient.SendAppLogArgsForCall(0)
			Expect(msg).To(Equal("Downloading name-1..."))
			Expect(logSource).To(Equal("log-source-1"))
			msg, logSource, _ = fakeClient.SendAppLogArgsForCall(1)
			Expect(msg).To(Equal("Downloaded name-1 (123B)"))
			Expect(logSource).To(Equal("log-source-1"))
		})

		It("returns the expected mount information", func() {
			expectedGardenMounts := []garden.BindMount{
				{SrcPath: "/tmp/download/dependencies", DstPath: "/var/data/buildpack-1", Mode: garden.BindMountModeRO, Origin: garden.BindMountOriginHost},
				{SrcPath: "/tmp/download/dependencies", DstPath: "/var/data/buildpack-2", Mode: garden.BindMountModeRO, Origin: garden.BindMountOriginHost},
			}

			expectedCacheKeys := []containerstore.BindMountCacheKey{
				{CacheKey: "cache-key-1", Dir: "/tmp/download/dependencies"},
				{CacheKey: "cache-key-2", Dir: "/tmp/download/dependencies"},
			}

			Expect(bindMounts.GardenBindMounts).To(ConsistOf(expectedGardenMounts))
			Expect(bindMounts.CacheKeys).To(ConsistOf(expectedCacheKeys))
		})

		It("downloads the directories", func() {
			Expect(cache.FetchAsDirectoryCallCount()).To(Equal(2))
			// Again order here will not necessisarily be preserved!
			expectedUrls := []url.URL{
				{Scheme: "https", Host: "example.com:8080", Path: "/download-1", User: url.UserPassword("user", "pass")},
				{Scheme: "http", Host: "example.com:1515", Path: "/download-2"},
			}
			expectedCacheKeys := []string{
				"cache-key-1",
				"cache-key-2",
			}

			downloadURLs := make([]url.URL, 2)
			cacheKeys := make([]string, 2)
			_, downloadUrl, cacheKey, _, _ := cache.FetchAsDirectoryArgsForCall(0)
			downloadURLs[0] = *downloadUrl
			cacheKeys[0] = cacheKey
			_, downloadUrl, cacheKey, _, _ = cache.FetchAsDirectoryArgsForCall(1)
			downloadURLs[1] = *downloadUrl
			cacheKeys[1] = cacheKey
			Expect(downloadURLs).To(ConsistOf(expectedUrls))
			Expect(cacheKeys).To(ConsistOf(expectedCacheKeys))
		})
	})

	Context("When a mount has an invalid 'From' field", func() {
		BeforeEach(func() {
			dependencies = []executor.CachedDependency{
				{Name: "name-1", CacheKey: "cache-key-1", LogSource: "log-source-1", From: "%", To: "/var/data/buildpack-1"},
			}
		})

		It("returns the error", func() {
			_, err := dependencyManager.DownloadCachedDependencies(logger, dependencies, logConfig, fakeClient)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When fetching a directory fails", func() {
		BeforeEach(func() {
			cache.FetchAsDirectoryReturns("", 0, errors.New("nope"))
		})

		It("emits the download events", func() {
			_, _ = dependencyManager.DownloadCachedDependencies(logger, dependencies, logConfig, fakeClient)
			Eventually(fakeClient.SendAppLogCallCount).Should(Equal(2))
			msg, logSource, _ := fakeClient.SendAppLogArgsForCall(0)
			Expect(msg).To(Equal("Downloading name-1..."))
			Expect(logSource).To(Equal("log-source-1"))
			msg, logSource, _ = fakeClient.SendAppLogArgsForCall(1)
			Expect(msg).To(Equal("Downloading name-1 failed"))
			Expect(logSource).To(Equal("log-source-1"))
		})

		It("returns the error", func() {
			_, err := dependencyManager.DownloadCachedDependencies(logger, dependencies, logConfig, fakeClient)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When there are no cached dependencies ", func() {
		It("returns an empty list of bindmounts", func() {
			bindMounts, err := dependencyManager.DownloadCachedDependencies(logger, nil, logConfig, fakeClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(bindMounts.CacheKeys).To(HaveLen(0))
			Expect(bindMounts.GardenBindMounts).To(HaveLen(0))
		})
	})

	Context("rate limiting", func() {
		var downloadBlocker chan struct{}

		BeforeEach(func() {
			downloadBlocker = make(chan struct{})
			cache.FetchAsDirectoryStub = func(_ lager.Logger, downloadUrl *url.URL, cacheKey string, checksum cacheddownloader.ChecksumInfoType, cancelChan <-chan struct{}) (string, int64, error) {
				<-downloadBlocker
				return cacheKey, 0, nil
			}

			dependencies = append(dependencies, executor.CachedDependency{
				Name:      "name3",
				CacheKey:  "cache-key3",
				LogSource: "log-source3",
				From:      "https://user:pass@example.com:8080/download-1",
				To:        "/var/data/buildpack-1",
			})
		})

		It("limits how many downloads can occur concurrently", func() {
			done := make(chan struct{})

			go func() {
				_, err := dependencyManager.DownloadCachedDependencies(logger, dependencies, logConfig, fakeClient)
				Expect(err).NotTo(HaveOccurred())
				close(done)
			}()

			Eventually(cache.FetchAsDirectoryCallCount).Should(Equal(2))
			Consistently(cache.FetchAsDirectoryCallCount).Should(Equal(2))

			Eventually(downloadBlocker).Should(BeSent(struct{}{}))

			Eventually(cache.FetchAsDirectoryCallCount).Should(Equal(3))
			Consistently(cache.FetchAsDirectoryCallCount).Should(Equal(3))

			Eventually(downloadBlocker).Should(BeSent(struct{}{}))
			Eventually(downloadBlocker).Should(BeSent(struct{}{}))

			Eventually(done).Should(BeClosed())
		})
	})

	Context("ReleaseCachedDependencies", func() {
		It("closes cached directories", func() {
			err := dependencyManager.ReleaseCachedDependencies(logger, []containerstore.BindMountCacheKey{
				{
					CacheKey: "some-cache-key-1",
					Dir:      "some-cache-dir-1",
				},
				{
					CacheKey: "some-cache-key-2",
					Dir:      "some-cache-dir-2",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(cache.CloseDirectoryCallCount()).To(Equal(2))
			_, cacheKey, dir := cache.CloseDirectoryArgsForCall(0)
			Expect(cacheKey).To(Equal("some-cache-key-1"))
			Expect(dir).To(Equal("some-cache-dir-1"))
			_, cacheKey, dir = cache.CloseDirectoryArgsForCall(1)
			Expect(cacheKey).To(Equal("some-cache-key-2"))
			Expect(dir).To(Equal("some-cache-dir-2"))
		})

		It("ignores not found error", func() {
			cache.CloseDirectoryReturns(cacheddownloader.EntryNotFound)
			err := dependencyManager.ReleaseCachedDependencies(logger, []containerstore.BindMountCacheKey{
				{
					CacheKey: "some-cache-key-1",
					Dir:      "some-cache-dir-1",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(cache.CloseDirectoryCallCount()).To(Equal(1))
		})

		It("ignores already closed error", func() {
			cache.CloseDirectoryReturns(cacheddownloader.AlreadyClosed)
			err := dependencyManager.ReleaseCachedDependencies(logger, []containerstore.BindMountCacheKey{
				{
					CacheKey: "some-cache-key-1",
					Dir:      "some-cache-dir-1",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(cache.CloseDirectoryCallCount()).To(Equal(1))
		})

		It("fails when closing directory fails", func() {
			cache.CloseDirectoryReturns(errors.New("unknown-error"))
			err := dependencyManager.ReleaseCachedDependencies(logger, []containerstore.BindMountCacheKey{
				{
					CacheKey: "some-cache-key-1",
					Dir:      "some-cache-dir-1",
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(cache.CloseDirectoryCallCount()).To(Equal(1))
		})
	})
})
