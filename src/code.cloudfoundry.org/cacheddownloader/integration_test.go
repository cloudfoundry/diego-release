package cacheddownloader_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Integration", func() {
	var (
		cache               *cacheddownloader.FileCache
		downloader          *cacheddownloader.Downloader
		cachedDownloader    cacheddownloader.CachedDownloader
		server              *httptest.Server
		serverPath          string
		cachedPath          string
		cacheMaxSizeInBytes int64 = 32000
		downloadTimeout           = time.Second
		checksum            cacheddownloader.ChecksumInfoType
		url                 *url.URL
		logger              *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		var err error

		serverPath, err = os.MkdirTemp("", "cached_downloader_integration_server")
		Expect(err).NotTo(HaveOccurred())

		cachedPath, err = os.MkdirTemp("", "cached_downloader_integration_cache")
		Expect(err).NotTo(HaveOccurred())

		handler := http.FileServer(http.Dir(serverPath))
		server = httptest.NewServer(handler)

		url, err = url.Parse(server.URL + "/file")
		Expect(err).NotTo(HaveOccurred())

		cache = cacheddownloader.NewCache(cachedPath, cacheMaxSizeInBytes, 0)
		downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, nil)
		cachedDownloader, err = cacheddownloader.New(downloader, cache, cacheddownloader.NoopTransform)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(serverPath)
		os.RemoveAll(cachedPath)
		server.Close()
	})

	fetch := func(fileToFetch string) ([]byte, time.Time) {
		url, err := url.Parse(server.URL + "/" + fileToFetch)
		Expect(err).NotTo(HaveOccurred())

		reader, _, err := cachedDownloader.Fetch(logger, url, "the-cache-key", checksum, make(chan struct{}))
		Expect(err).NotTo(HaveOccurred())
		defer reader.Close()

		readData, err := io.ReadAll(reader)
		Expect(err).NotTo(HaveOccurred())

		cacheContents, err := os.ReadDir(cachedPath)
		Expect(err).NotTo(HaveOccurred())
		expectCacheToHaveNEntries(cachedPath, 1)

		content, err := os.ReadFile(filepath.Join(cachedPath, cacheContents[0].Name()))
		Expect(err).NotTo(HaveOccurred())

		Expect(readData).To(Equal(content))

		fdInfo, err := cacheContents[0].Info()
		Expect(err).NotTo(HaveOccurred())

		return content, fdInfo.ModTime()
	}

	fetchAsDirectory := func(fileToFetch string) (string, time.Time) {
		url, err := url.Parse(server.URL + "/" + fileToFetch)
		Expect(err).NotTo(HaveOccurred())

		dirPath, _, err := cachedDownloader.FetchAsDirectory(logger, url, "tar-file-cache-key", checksum, make(chan struct{}))
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			err := cachedDownloader.CloseDirectory(logger, "tar-file-cache-key", dirPath)
			Expect(err).NotTo(HaveOccurred())
		}()

		// For some reason the first stat changes the mod time on Windows.
		// Call this so that we have predictable mod times on Windows
		os.Stat(dirPath)

		cacheContents, err := os.ReadDir(cachedPath)
		Expect(err).NotTo(HaveOccurred())
		expectCacheToHaveNEntries(cachedPath, 1)

		// ReadDir sorts by file name, so the tarfile should come before the directory
		fdInfo, err := cacheContents[0].Info()
		Expect(err).NotTo(HaveOccurred())

		Expect(fdInfo.Mode().IsDir()).To(BeTrue())

		dirPathInCache := filepath.Join(cachedPath, cacheContents[0].Name())
		Expect(dirPath).To(Equal(dirPathInCache))

		return dirPath, fdInfo.ModTime()
	}

	Describe("Fetch", func() {
		It("caches downloads", func() {
			// touch a file on disk
			err := os.WriteFile(filepath.Join(serverPath, "file"), []byte("a"), 0666)
			Expect(err).NotTo(HaveOccurred())

			// download file once
			content, modTimeBefore := fetch("file")
			Expect(content).To(Equal([]byte("a")))

			time.Sleep(time.Second)

			// download again should be cached
			content, modTimeAfter := fetch("file")
			Expect(content).To(Equal([]byte("a")))
			Expect(modTimeBefore).To(Equal(modTimeAfter))

			time.Sleep(time.Second)

			// touch file again
			err = os.WriteFile(filepath.Join(serverPath, "file"), []byte("b"), 0666)
			Expect(err).NotTo(HaveOccurred())

			// download again and we should get a file containing "b"
			content, _ = fetch("file")
			Expect(content).To(Equal([]byte("b")))
		})
	})

	Describe("FetchAsDirectory", func() {
		It("caches downloads", func() {
			// create a valid tar file
			tarByteBuffer := createTarBuffer("original", 0)
			file, err := os.Create(filepath.Join(serverPath, "tarfile"))
			Expect(err).NotTo(HaveOccurred())
			_, err = tarByteBuffer.WriteTo(file)
			Expect(err).NotTo(HaveOccurred())

			// fetch directory once
			dirPath, modTimeBefore := fetchAsDirectory("tarfile")
			Expect(os.ReadFile(filepath.Join(dirPath, "testdir/file.txt"))).To(Equal([]byte("original")))

			time.Sleep(time.Second)

			// download again should be cached
			dirPath, modTimeAfter := fetchAsDirectory("tarfile")
			Expect(os.ReadFile(filepath.Join(dirPath, "testdir/file.txt"))).To(Equal([]byte("original")))
			Expect(modTimeBefore).To(Equal(modTimeAfter))

			time.Sleep(time.Second)

			// touch file again
			tarByteBuffer = createTarBuffer("modified", 0)
			file, err = os.Create(filepath.Join(serverPath, "tarfile"))
			Expect(err).NotTo(HaveOccurred())
			_, err = tarByteBuffer.WriteTo(file)
			Expect(err).NotTo(HaveOccurred())

			// download again and we should get an untarred file with modified contents
			dirPath, _ = fetchAsDirectory("tarfile")
			Expect(os.ReadFile(filepath.Join(dirPath, "testdir/file.txt"))).To(Equal([]byte("modified")))
		})
	})
})
