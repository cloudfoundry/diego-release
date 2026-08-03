package cacheddownloader_test

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/archiver/extractor/test_helper"
	. "code.cloudfoundry.org/cacheddownloader"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TarTransformer", func() {
	var (
		scratch string

		sourcePath      string
		destinationPath string
	)

	archiveFiles := []test_helper.ArchiveFile{
		{Name: "some-file", Body: "some-contents"},
	}

	BeforeEach(func() {
		var err error

		scratch, err = os.MkdirTemp("", "tar-transformer-scratch")
		Expect(err).ShouldNot(HaveOccurred())

		destinationFile, err := os.CreateTemp("", "destination")
		Expect(err).ShouldNot(HaveOccurred())

		err = destinationFile.Close()
		Expect(err).ShouldNot(HaveOccurred())

		destinationPath = destinationFile.Name()
	})

	AfterEach(func() {
		err := os.RemoveAll(scratch)
		Expect(err).ShouldNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		_, err := TarTransform(sourcePath, destinationPath)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when the file is a .zip", func() {
		BeforeEach(func() {
			sourcePath = filepath.Join(scratch, "file.zip")

			test_helper.CreateZipArchive(sourcePath, archiveFiles)
		})

		It("closes the tarfile", func() {
			// On Windows, you can't remove files that are still open.  On Linux, you can.
			err := os.Remove(destinationPath)

			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
