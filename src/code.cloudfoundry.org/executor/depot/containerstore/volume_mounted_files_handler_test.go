package containerstore_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/depot/containerstore/containerstorefakes"
	"code.cloudfoundry.org/garden"
)

var _ = Describe("VolumeMountedFilesHandler", func() {
	var (
		tmpdir            string
		fakeContainerUUID string

		handler     *containerstore.VolumeMountedFilesHandler
		fakeHandler *containerstore.VolumeMountedFilesHandler
		container   executor.Container

		fakeFSOperations *containerstorefakes.FakeFSOperator
	)

	AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	BeforeEach(func() {
		fakeContainerUUID = "E62613F8-7E85-4F49-B3EF-690BD2AE7EF2"

		tmpdir = filepath.Join(os.TempDir(), "volume_mounted_files")
		err := os.MkdirAll(tmpdir, os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		container = executor.Container{Guid: fakeContainerUUID}

		container.VolumeMountedFiles = append(container.VolumeMountedFiles, executor.VolumeMountedFiles{
			Path: "/redis/username", Content: "username",
		})

		container.VolumeMountedFiles = append(container.VolumeMountedFiles, executor.VolumeMountedFiles{
			Path: "/redis/password", Content: "password",
		})

		container.VolumeMountedFiles = append(container.VolumeMountedFiles, executor.VolumeMountedFiles{
			Path: "/httpd/password", Content: "password",
		})

		container.VolumeMountedFiles = append(container.VolumeMountedFiles, executor.VolumeMountedFiles{
			Path: "/httpd/username", Content: "username",
		})

		container.VolumeMountedFiles = append(container.VolumeMountedFiles, executor.VolumeMountedFiles{
			Path: "/kafka/domain.com/password", Content: "password",
		})

		container.VolumeMountedFiles = append(container.VolumeMountedFiles, executor.VolumeMountedFiles{
			Path: "/kafka/domain.com/username", Content: "username",
		})

		container.VolumeMountedFiles = append(container.VolumeMountedFiles, executor.VolumeMountedFiles{
			Path: "/kafka/domain.org/username", Content: "username",
		})

		container.VolumeMountedFiles = append(container.VolumeMountedFiles, executor.VolumeMountedFiles{
			Path: "/kafka/domain.org/password", Content: "password",
		})

		handler = containerstore.NewVolumeMountedFilesHandler(
			containerstore.NewFSOperations(),
			tmpdir,
			fakeContainerUUID,
		)

		fakeFSOperations = &containerstorefakes.FakeFSOperator{}
		fakeHandler = containerstore.NewVolumeMountedFilesHandler(
			fakeFSOperations,
			tmpdir,
			fakeContainerUUID,
		)
	})

	Context("CreateDir", func() {
		It("returns a valid bind mount", func() {
			mount, err := handler.CreateDir(logger, container)
			Expect(err).To(Succeed())

			Expect(mount).To(HaveLen(1))
			Expect(mount[0].SrcPath).To(BeADirectory())
			Expect(mount[0].DstPath).To(Equal(fakeContainerUUID))
			Expect(mount[0].Mode).To(Equal(garden.BindMountModeRO))
			Expect(mount[0].Origin).To(Equal(garden.BindMountOriginHost))
		})

		It("returns a valid volume mounted files configuration directory", func() {
			_, err := handler.CreateDir(logger, container)
			Expect(err).To(Succeed())

			Expect(filepath.Join(tmpdir, fakeContainerUUID, "redis")).To(BeADirectory())

			Expect(filepath.Join(tmpdir, fakeContainerUUID, "redis", "username")).To(BeAnExistingFile())
			Expect(filepath.Join(tmpdir, fakeContainerUUID, "redis", "username")).To(BeARegularFile())

			Expect(filepath.Join(tmpdir, fakeContainerUUID, "redis", "password")).To(BeAnExistingFile())
			Expect(filepath.Join(tmpdir, fakeContainerUUID, "redis", "password")).To(BeARegularFile())

			Expect(filepath.Join(tmpdir, fakeContainerUUID, "httpd", "username")).To(BeAnExistingFile())
			Expect(filepath.Join(tmpdir, fakeContainerUUID, "httpd", "username")).To(BeARegularFile())

			Expect(filepath.Join(tmpdir, fakeContainerUUID, "httpd", "password")).To(BeAnExistingFile())
			Expect(filepath.Join(tmpdir, fakeContainerUUID, "httpd", "password")).To(BeARegularFile())
		})

		It("when CreateDir/Chdir return an error", func() {
			fakeFSOperations.ChdirReturns(&os.PathError{Op: "chdir", Err: errors.New("directory does not exist")})
			_, err := fakeHandler.CreateDir(logger, container)
			Expect(err).To(HaveOccurred())

			var pathErr *os.PathError
			Expect(errors.As(err, &pathErr)).To(BeTrue())
			Expect(pathErr).NotTo(BeNil())
			Expect(pathErr.Op).To(Equal("chdir"))
			Expect(pathErr.Err).To(MatchError("directory does not exist"))
		})

		It("when CreateDir/Mkdir return an error", func() {
			container.Guid = ""
			fakeFSOperations.MkdirAllReturns(&os.PathError{Op: "mkdir", Err: errors.New("directory exists")})

			_, err := fakeHandler.CreateDir(logger, container)
			Expect(err).To(HaveOccurred())

			var pathErr *os.PathError
			Expect(errors.As(err, &pathErr)).To(BeTrue())
		})

		It("when CreateDir/volumeMountedFilesForServices dirName/fileName return an error", func() {
			container.VolumeMountedFiles = []executor.VolumeMountedFiles{{
				Path:    "",
				Content: "content",
			}}

			_, err := handler.CreateDir(logger, container)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to extract volume-mounted-files directory. format is: /serviceName/fileName"))
		})

		Context("VolumeMountedFilesHandler Error Cases", func() {
			DescribeTable("handles error cases properly", func(setupMock func(), expectedError string) {
				setupMock()

				_, err := fakeHandler.CreateDir(logger, container)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedError))
			},

				Entry("fails when os.WriteFile returns an error due to file being non-writable",
					func() { fakeFSOperations.WriteFileReturns(errors.New("permission denied")) },
					"permission denied",
				),
				Entry("fails when os.Create returns an error due to file being non-writable",
					func() { fakeFSOperations.CreateFileReturns(nil, errors.New("permission denied")) },
					"permission denied",
				),
				Entry("when CreateDir/volumeMountedFilesForServices MkdirAll returns an error",
					func() { fakeFSOperations.MkdirAllReturns(errors.New("file name too long")) },
					"file name too long",
				),
			)
		})

		Context("File Content Validation", func() {
			DescribeTable("validates the content of the service file",
				func(service, fileType, expected string) {
					_, err := handler.CreateDir(logger, container)
					Expect(err).To(Succeed())

					filePath := filepath.Join(tmpdir, fakeContainerUUID, service, fileType)
					Expect(filePath).To(BeAnExistingFile())

					content, err := os.ReadFile(filePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(content)).To(Equal(expected))
				},

				Entry("validates the content of the redis username file", "redis", "username", "username"),
				Entry("validates the content of the redis password file", "redis", "password", "password"),
				Entry("validates the content of the httpd username file", "httpd", "username", "username"),
				Entry("validates the content of the httpd password file", "httpd", "password", "password"),
			)
		})

		Context("when volume mounted files directory does not exist", func() {
			BeforeEach(func() {
				handler = containerstore.NewVolumeMountedFilesHandler(
					containerstore.NewFSOperations(),
					"/some/fake/path",
					"mount_path",
				)
			})

			It("when trying to change directory to volume mount fail", func() {
				_, err := handler.CreateDir(logger, container)
				Expect(err).To(HaveOccurred())

				Expect(err.Error()).To(ContainSubstring("volume mount path does not exist"))
			})
		})

		Context("RemoveDir volume mount directory", func() {
			It("when removed succeed", func() {
				err := handler.RemoveDir(logger, container)
				Expect(err).NotTo(HaveOccurred())

				volumeMountedFiles := filepath.Join(tmpdir, fakeContainerUUID)
				Eventually(volumeMountedFiles).ShouldNot(BeADirectory())
			})

			It("when removed fail", func() {
				fakeFSOperations.RemoveAllReturns(errors.New("remove error"))

				err := fakeHandler.RemoveDir(logger, container)
				Expect(err).To(HaveOccurred())

				Expect(err.Error()).To(ContainSubstring("failed-to-remove-volume-mounted-files-directory"))
			})
		})
	})
})
