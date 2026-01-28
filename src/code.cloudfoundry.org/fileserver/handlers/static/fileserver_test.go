package static_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/fileserver/handlers/static"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FileServer", func() {
	var (
		servedDirectory                   string
		fileServer                        *httptest.Server
		expectedShaTest, expectedShaTest2 string
	)

	BeforeEach(func() {
		var err error
		servedDirectory, err = os.MkdirTemp("", "fileserver-test")
		Expect(err).NotTo(HaveOccurred())
		os.Mkdir(filepath.Join(servedDirectory, "testdir"), os.ModePerm)

		tenHoursAgo := time.Now().Add(-10 * time.Hour)

		os.WriteFile(filepath.Join(servedDirectory, "test"), []byte("hello"), os.ModePerm)
		sha256bytes := sha256.Sum256([]byte("hello"))
		expectedShaTest = hex.EncodeToString(sha256bytes[:])
		os.Chtimes(filepath.Join(servedDirectory, "test"), tenHoursAgo, tenHoursAgo)

		os.WriteFile(filepath.Join(servedDirectory, "test2.."), []byte("world"), os.ModePerm)
		sha256bytes = sha256.Sum256([]byte("world"))
		expectedShaTest2 = hex.EncodeToString(sha256bytes[:])

		fileServer = httptest.NewServer(static.NewFileServer(servedDirectory))
	})

	AfterEach(func() {
		fileServer.Close()
		os.RemoveAll(servedDirectory)
	})

	Context("when the file exists", func() {
		It("returns a 200 OK, the file content and its ETag", func() {
			resp, err := http.Get(fmt.Sprintf("%s/test", fileServer.URL))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("ETag")).To(Equal(fmt.Sprintf(`"%s"`, expectedShaTest)))

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("hello"))
		})

		Context("when requesting the file multiple times", func() {
			It("returns the same ETag on every attempt", func() {
				Consistently(func() string {
					resp, err := http.Get(fmt.Sprintf("%s/test", fileServer.URL))
					Expect(err).NotTo(HaveOccurred())
					defer resp.Body.Close()

					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					return resp.Header.Get("ETag")
				}).Should(Equal(fmt.Sprintf(`"%s"`, expectedShaTest)))
			})
		})

		Context("when the file name contains dot dot", func() {
			It("returns a 200 OK and the file and its ETag as the sha1sum", func() {
				resp, err := http.Get(fmt.Sprintf("%s/test2..", fileServer.URL))
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(resp.Header.Get("ETag")).To(Equal(fmt.Sprintf(`"%s"`, expectedShaTest2)))

				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(Equal("world"))
			})
		})

		Context("when the request provides an 'If-None-Match' header that matches the sha1sum", func() {
			It("returns a 304 Not Modfied and does not return the file", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("%s/test", fileServer.URL), nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, expectedShaTest))

				resp, err := http.DefaultClient.Do(req)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				Expect(resp.StatusCode).To(Equal(http.StatusNotModified))
				Expect(resp.Header.Get("ETag")).To(Equal(fmt.Sprintf(`"%s"`, expectedShaTest)))

				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(Equal(""))

			})
		})

		Context("when the request provides an 'If-None-Match' header that does not match the sha1sum", func() {
			It("returns a 200 OK and the file even if the file had an older mtime than the time in If-Modified-Since", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("%s/test", fileServer.URL), nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("If-None-Match", `"different-hash"`)
				req.Header.Set("If-Modified-Since", time.Now().Add(-2*time.Hour).Format(http.TimeFormat))

				resp, err := http.DefaultClient.Do(req)
				Expect(err).ToNot(HaveOccurred())
				defer resp.Body.Close()

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(resp.Header.Get("ETag")).To(Equal(fmt.Sprintf(`"%s"`, expectedShaTest)))

				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(Equal("hello"))
			})
		})
	})

	It("returns 400 on filepaths with dot dot", func() {
		resp, err := http.Get(fmt.Sprintf("%s/../protected-file", fileServer.URL))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("returns 404 on files that don't exist", func() {
		resp, err := http.Get(fmt.Sprintf("%s/does-not-exist", fileServer.URL))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})

	It("returns 401 when accessing a directory", func() {
		resp, err := http.Get(fmt.Sprintf("%s/testdir", fileServer.URL))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
	})

})
