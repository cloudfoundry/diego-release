package volman_test

import (
	"os"

	"path/filepath"

	"code.cloudfoundry.org/garden"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Given garden, volman, localdriver", func() {
	var mountPoint string

	Context("and a mounted volume", func() {
		BeforeEach(func() {
			someConfig := map[string]interface{}{"volume_id": "volman_garden_test-someID"}
			mountPointResponse, err := volmanClient.Mount(logger, "localdriver", "someVolume", "someContainer", someConfig)
			Expect(err).NotTo(HaveOccurred())
			mountPoint = mountPointResponse.Path
		})

		AfterEach(func() {
			err := volmanClient.Unmount(logger, "localdriver", "someVolume", "someContainer")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the container bind mounts with \"host origin\"", func() {
			var bindMount garden.BindMount
			var container garden.Container

			BeforeEach(func() {

				bindMount = garden.BindMount{
					SrcPath: mountPoint,
					DstPath: "/mnt/testmount",
					Mode:    garden.BindMountModeRW,
					Origin:  garden.BindMountOriginHost,
				}
			})

			JustBeforeEach(func() {
				container = createContainer(bindMount)
			})

			It("should be able to write to that volume", func() {
				dir := "/mnt/testmount"
				fileName := "bind-mount-test-file"
				filePath := filepath.Join(dir, fileName)
				createContainerTestFileIn(container, filePath)

				expectContainerTestFileExists(container, filePath)

				By("expecting the file it wrote to be available outside of the container")
				files, err := filepath.Glob(mountPoint + "/*")
				Expect(err).ToNot(HaveOccurred())
				Expect(len(files)).To(Equal(1))
				Expect(files[0]).To(Equal(mountPoint + "/" + fileName))
			})

			Context("when a second container bind mounts the same \"host origin\"", func() {
				var bindMount2 garden.BindMount
				var container2 garden.Container

				BeforeEach(func() {
					bindMount2 = garden.BindMount{
						SrcPath: mountPoint,
						DstPath: "/mnt/testmount2",
						Mode:    garden.BindMountModeRW,
						Origin:  garden.BindMountOriginHost,
					}
				})

				JustBeforeEach(func() {
					container2 = createContainer(bindMount2)
				})

				It("should be able to read the file written by the first", func() {
					dir := "/mnt/testmount"
					fileName := "bind-mount-test-file"
					filePath := filepath.Join(dir, fileName)
					createContainerTestFileIn(container, filePath)
					expectContainerTestFileExists(container, filePath)

					dir = "/mnt/testmount2"
					filePath = filepath.Join(dir, fileName)
					expectContainerTestFileExists(container2, filePath)
				})
			})
		})
	})
})

func createContainer(bindMount garden.BindMount) garden.Container {
	mounts := []garden.BindMount{}
	mounts = append(mounts, bindMount)
	gardenContainerSpec := garden.ContainerSpec{
		Privileged: true,
		BindMounts: mounts,
	}

	container, err := gardenClient.Create(gardenContainerSpec)
	Expect(err).NotTo(HaveOccurred())

	return container
}

func createContainerTestFileIn(container garden.Container, filePath string) {
	process, err := container.Run(garden.ProcessSpec{
		Path: "touch",
		Args: []string{filePath},
		User: "root",
	}, garden.ProcessIO{Stdin: nil, Stdout: os.Stdout, Stderr: os.Stderr})
	Expect(err).ToNot(HaveOccurred())
	Expect(process.Wait()).To(Equal(0))

	process, err = container.Run(garden.ProcessSpec{
		Path: "chmod",
		Args: []string{"0777", filePath},
		User: "root",
	}, garden.ProcessIO{Stdin: nil, Stdout: os.Stdout, Stderr: os.Stderr})
	Expect(err).ToNot(HaveOccurred())
	Expect(process.Wait()).To(Equal(0))

}

func expectContainerTestFileExists(container garden.Container, filePath string) {
	out := gbytes.NewBuffer()
	defer out.Close()

	proc, err := container.Run(garden.ProcessSpec{
		Path: "ls",
		Args: []string{filePath},
		User: "root",
	}, garden.ProcessIO{Stdin: nil, Stdout: out, Stderr: out})
	Expect(err).NotTo(HaveOccurred())
	Expect(proc.Wait()).To(Equal(0))
	Expect(out).To(gbytes.Say(filePath))
}
