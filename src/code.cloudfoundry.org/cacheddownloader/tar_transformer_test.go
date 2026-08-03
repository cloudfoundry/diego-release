package cacheddownloader_test

import (
	"archive/tar"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/cacheddownloader"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TarTransformer", func() {
	var (
		scratch string

		sourcePath      string
		destinationPath string

		transformedSize int64
		transformErr    error
	)

	archiveFiles := []test_helper.ArchiveFile{
		{Name: "some-file", Body: "some-contents"},
		{Name: "some-symlink", Link: "some-symlink-target"},
		{Name: "some-symlink-target", Body: "some-other-contents"},
	}

	verifyTarFile := func(path string) {
		file, err := os.Open(path)
		Expect(err).NotTo(HaveOccurred())

		tr := tar.NewReader(file)

		entry, err := tr.Next()
		Expect(err).NotTo(HaveOccurred())

		Expect(entry.Name).To(Equal("some-file"))
		Expect(entry.Size).To(Equal(int64(len("some-contents"))))
	}

	BeforeEach(func() {
		var err error

		scratch, err = os.MkdirTemp("", "tar-transformer-scratch")
		Expect(err).NotTo(HaveOccurred())

		destinationFile, err := os.CreateTemp("", "destination")
		Expect(err).NotTo(HaveOccurred())

		err = destinationFile.Close()
		Expect(err).NotTo(HaveOccurred())

		destinationPath = destinationFile.Name()
	})

	AfterEach(func() {
		err := os.RemoveAll(scratch)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		transformedSize, transformErr = cacheddownloader.TarTransform(sourcePath, destinationPath)
	})

	Context("when the file is already a .tar", func() {
		BeforeEach(func() {
			sourcePath = filepath.Join(scratch, "file.tar")

			test_helper.CreateTarArchive(sourcePath, archiveFiles)
		})

		It("renames the file to the destination", func() {
			verifyTarFile(destinationPath)
		})

		It("removes the source file", func() {
			_, err := os.Stat(sourcePath)
			Expect(err).To(HaveOccurred())
		})

		It("returns its size", func() {
			fi, err := os.Stat(destinationPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(transformedSize).To(Equal(fi.Size()))
		})
	})

	Context("when the file is a .tar.gz", func() {
		BeforeEach(func() {
			sourcePath = filepath.Join(scratch, "file.tar.gz")

			test_helper.CreateTarGZArchive(sourcePath, archiveFiles)
		})

		Context("when gunzip is not available on the PATH", func() {
			var oldPATH string

			BeforeEach(func() {
				oldPATH = os.Getenv("PATH")
				os.Setenv("PATH", "/dev/null")

				_, err := exec.LookPath("gunzip")
				Expect(err).To(HaveOccurred())
			})

			AfterEach(func() {
				os.Setenv("PATH", oldPATH)
			})

			It("does not error", func() {
				Expect(transformErr).NotTo(HaveOccurred())
			})

			It("gzip uncompresses it to a .tar", func() {
				verifyTarFile(destinationPath)
			})

			It("deletes the original file", func() {
				_, err := os.Stat(sourcePath)
				Expect(err).To(HaveOccurred())
			})

			It("returns the correct number of bytes written", func() {
				fi, err := os.Stat(destinationPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(fi.Size()).To(Equal(transformedSize))
			})
		})

		Context("when gunzip is available on the PATH", func() {
			BeforeEach(func() {
				_, err := exec.LookPath("gunzip")
				if err != nil {
					Skip("skipping gunzip tests because gunzip was not found on the PATH")
				}
			})

			It("does not error", func() {
				Expect(transformErr).NotTo(HaveOccurred())
			})

			It("gzip uncompresses it to a .tar", func() {
				verifyTarFile(destinationPath)
			})

			It("deletes the original file", func() {
				_, err := os.Stat(sourcePath)
				Expect(err).To(HaveOccurred())
			})

			It("returns the correct number of bytes written", func() {
				fi, err := os.Stat(destinationPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(fi.Size()).To(Equal(transformedSize))
			})
		})
	})

	Context("when the file is a .zip", func() {
		BeforeEach(func() {
			sourcePath = filepath.Join(scratch, "file.zip")

			test_helper.CreateZipArchive(sourcePath, archiveFiles)
		})

		It("does not error", func() {
			Expect(transformErr).NotTo(HaveOccurred())
		})

		It("transforms it to a .tar", func() {
			verifyTarFile(destinationPath)
		})

		It("deletes the original file", func() {
			_, err := os.Stat(sourcePath)
			Expect(err).To(HaveOccurred())
		})

		It("returns the correct number of bytes written", func() {
			fi, err := os.Stat(destinationPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(fi.Size()).To(Equal(transformedSize))
		})
	})

	Context("when the file is a .mp3", func() {
		BeforeEach(func() {
			sourcePath = filepath.Join(scratch, "bogus")

			err := os.WriteFile(sourcePath, []byte("bogus"), 0755)
			Expect(err).NotTo(HaveOccurred())
		})

		It("blows up horribly", func() {
			Expect(transformErr).To(Equal(cacheddownloader.ErrUnknownArchiveFormat))
		})
	})
})
