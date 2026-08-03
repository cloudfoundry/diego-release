package rep_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"os"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resources", func() {
	var (
		cellState      models.CellState
		linuxRootFSURL string
	)

	BeforeEach(func() {
		linuxOnlyRootFSProviders := models.RootFSProviders{models.PreloadedRootFSScheme: models.NewFixedSetRootFSProvider("linux")}
		total := models.NewResources(1000, 2000, 10)
		avail := models.NewResources(950, 1900, 3)
		linuxRootFSURL = models.PreloadedRootFS("linux")

		lrps := []models.SchedulingLRP{
			*buildLRP("ig-1", "pg-1", "domain", 0, linuxRootFSURL, 10, 20, 30, []string{}, []string{}, models.ActualLRPStateClaimed),
			*buildLRP("ig-2", "pg-1", "domain", 1, linuxRootFSURL, 10, 20, 30, []string{}, []string{}, models.ActualLRPStateClaimed),
			*buildLRP("ig-3", "pg-2", "domain", 0, linuxRootFSURL, 10, 20, 30, []string{}, []string{}, models.ActualLRPStateClaimed),
			*buildLRP("ig-4", "pg-3", "domain", 0, linuxRootFSURL, 10, 20, 30, []string{}, []string{}, models.ActualLRPStateClaimed),
			*buildLRP("ig-5", "pg-4", "domain", 0, linuxRootFSURL, 10, 20, 30, []string{}, []string{}, models.ActualLRPStateClaimed),
		}

		tasks := []models.SchedulingTask{
			*buildTask("tg-big", "domain", linuxRootFSURL, 20, 10, 10, []string{}, []string{}, models.Task_Running, false),
			*buildTask("tg-small", "domain", linuxRootFSURL, 10, 10, 10, []string{}, []string{}, models.Task_Running, false),
		}

		cellState = models.NewCellState(
			"cell-id",
			0,
			"https://foo.cell.service.cf.internal",
			linuxOnlyRootFSProviders,
			avail,
			total,
			lrps,
			tasks,
			"my-zone",
			7,
			false,
			nil,
			nil,
			nil,
			0,
		)
	})

	Describe("MatchPlacementTags", func() {
		Context("when cell state does not have placement tags", func() {
			It("does not allow lrps with placement tags", func() {
				state := models.CellState{
					PlacementTags:         []string{},
					OptionalPlacementTags: []string{},
				}
				Expect(state.MatchPlacementTags([]string{"foo"})).To(BeFalse())
				Expect(state.MatchPlacementTags([]string{})).To(BeTrue())
			})
		})

		Context("when it has require placement tags", func() {
			It("requires the placement tags to be present in the lrp", func() {
				state := models.CellState{
					PlacementTags:         []string{"foo", "bar"},
					OptionalPlacementTags: []string{},
				}
				Expect(state.MatchPlacementTags([]string{})).To(BeFalse())
				Expect(state.MatchPlacementTags([]string{"foo"})).To(BeFalse())
				Expect(state.MatchPlacementTags([]string{"foo", "bar"})).To(BeTrue())
			})
		})

		Context("when it has optional placement tags", func() {
			It("does not require placement tags to be present on the desired lrp", func() {
				state := models.CellState{
					PlacementTags:         []string{},
					OptionalPlacementTags: []string{"foo"},
				}
				Expect(state.MatchPlacementTags([]string{})).To(BeTrue())
				Expect(state.MatchPlacementTags([]string{"foo"})).To(BeTrue())
			})

			It("does not allow extra placement tags to be defined in the lrp", func() {
				state := models.CellState{
					PlacementTags:         []string{},
					OptionalPlacementTags: []string{"foo"},
				}
				Expect(state.MatchPlacementTags([]string{"bar"})).To(BeFalse())
			})
		})

		Context("when both placement tags and optional placement tags are present", func() {
			It("requires all required placement tags to be on the lrp", func() {
				state := models.CellState{
					PlacementTags:         []string{"foo"},
					OptionalPlacementTags: []string{"bar"},
				}
				Expect(state.MatchPlacementTags([]string{})).To(BeFalse())
				Expect(state.MatchPlacementTags([]string{"bar"})).To(BeFalse())
				Expect(state.MatchPlacementTags([]string{"foo"})).To(BeTrue())
				Expect(state.MatchPlacementTags([]string{"foo", "bar"})).To(BeTrue())
				Expect(state.MatchPlacementTags([]string{"foo", "bar", "baz"})).To(BeFalse())
			})
		})
	})

	Describe("Resource Matching", func() {
		var requiredResource models.Resource
		var err error
		BeforeEach(func() {
			requiredResource = models.NewResource(10, 10, 10)
		})

		JustBeforeEach(func() {
			err = cellState.ResourceMatch(&requiredResource)
		})

		Context("when insufficient memory", func() {
			BeforeEach(func() {
				requiredResource.MemoryMB = 5000
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: memory"))
			})
		})

		Context("when insufficient disk", func() {
			BeforeEach(func() {
				requiredResource.DiskMB = 5000
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: disk"))
			})
		})

		Context("when insufficient disk and memory", func() {
			BeforeEach(func() {
				requiredResource.MemoryMB = 5000
				requiredResource.DiskMB = 5000
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: disk, memory"))
			})
		})

		Context("when insufficient disk, memory and containers", func() {
			BeforeEach(func() {
				requiredResource.MemoryMB = 5000
				requiredResource.DiskMB = 5000
				cellState.AvailableResources.Containers = 0
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: containers, disk, memory"))
			})
		})

		Context("when there are no available containers", func() {
			BeforeEach(func() {
				cellState.AvailableResources.Containers = 0
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("insufficient resources: containers"))
			})
		})

		Context("when there is sufficient room", func() {
			It("does not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("StackPathMap", func() {
		Describe("PathForRootFS", func() {
			var stackPathMap rep.StackPathMap
			BeforeEach(func() {
				stackPathMap = rep.StackPathMap{
					"cflinuxfs3": "cflinuxfs3:/var/vcap/packages/cflinuxfs3/rootfs.tar",
				}
			})
			It("returns the resolved path if the RootFS URL scheme is preloaded", func() {
				p, err := stackPathMap.PathForRootFS("preloaded:cflinuxfs3")
				Expect(err).NotTo(HaveOccurred())
				Expect(p).To(Equal("cflinuxfs3:/var/vcap/packages/cflinuxfs3/rootfs.tar"))
			})
			It("returns the correct URL if the RootFS URL scheme is preloaded+layer", func() {
				queryString := "?layer=https://blobstore.internal/layer1.tgz?layer_path=/tmp/asset1&layer_digest=alkjsdflkj"
				p, err := stackPathMap.PathForRootFS(fmt.Sprintf("preloaded+layer:cflinuxfs3%s", queryString))
				Expect(err).NotTo(HaveOccurred())
				Expect(p).To(Equal(fmt.Sprintf("preloaded+layer:cflinuxfs3:/var/vcap/packages/cflinuxfs3/rootfs.tar%s", queryString)))
			})
			It("returns a blank string and no error if the RootFS URL is blank", func() {
				p, err := stackPathMap.PathForRootFS("")
				Expect(err).NotTo(HaveOccurred())
				Expect(p).To(Equal(""))
			})
			It("returns the same URL and no error if the RootFS scheme is docker", func() {
				p, err := stackPathMap.PathForRootFS("docker:///cloudfoundry/grace")
				Expect(err).NotTo(HaveOccurred())
				Expect(p).To(Equal("docker:///cloudfoundry/grace"))
			})
			It("returns an error if the RootFS URL is invalid", func() {
				_, err := stackPathMap.PathForRootFS("%x")
				Expect(err).To(HaveOccurred())
			})
			It("returns an error if the Preloaded RootFS path could not be found in the map", func() {
				_, err := stackPathMap.PathForRootFS("preloaded:not-on-cell")
				Expect(err).To(MatchError(rep.ErrPreloadedRootFSNotFound))
			})
		})

		Describe("StackVersionList", func() {
			It("returns an empty list when the map is empty", func() {
				m := rep.StackPathMap{}
				Expect(m.StackVersionList()).To(Equal([]string{}))
			})

			It("returns only the stack name when version is empty", func() {
				m := rep.StackPathMap{
					"cflinuxfs3": "/nonexistent/path/to/rootfs.tar",
				}
				list := m.StackVersionList()
				Expect(list).To(HaveLen(1))
				Expect(list[0]).To(Equal("cflinuxfs3"))
			})

			It("returns name@version when version is non-empty", func() {
				tarPath := createTarWithVersion("1.2.3")
				defer os.Remove(tarPath)

				m := rep.StackPathMap{
					"cflinuxfs3": tarPath,
				}
				list := m.StackVersionList()
				Expect(list).To(HaveLen(1))
				Expect(list[0]).To(Equal("cflinuxfs3@1.2.3"))
			})

			It("returns mixed name and name@version for multiple stacks", func() {
				tarPath := createTarWithVersion("2.0.0")
				defer os.Remove(tarPath)

				m := rep.StackPathMap{
					"with-version":    tarPath,
					"without-version": "/nonexistent/path.tar",
				}
				list := m.StackVersionList()
				Expect(list).To(HaveLen(2))
				Expect(list).To(ContainElement("with-version@2.0.0"))
				Expect(list).To(ContainElement("without-version"))
			})
		})
	})
})

func buildLRP(instanceGuid,
	guid,
	domain string,
	index int,
	rootFS string,
	memoryMB,
	diskMB,
	maxPids int32,
	placementTags,
	volumeDrivers []string,
	state string,
) *models.SchedulingLRP {
	lrpKey := models.NewActualLRPKey(guid, int32(index), domain)
	lrp := models.NewSchedulingLRP(instanceGuid, lrpKey, models.NewResource(memoryMB, diskMB, maxPids), models.PlacementConstraint{RootFs: rootFS,
		PlacementTags: placementTags,
		VolumeDrivers: volumeDrivers,
	})
	lrp.State = state
	return &lrp
}

func buildTask(taskGuid, domain, rootFS string, memoryMB, diskMB, maxPids int32, placementTags, volumeDrivers []string, state models.Task_State, failed bool) *models.SchedulingTask {
	task := models.NewSchedulingTask(taskGuid, domain, models.NewResource(memoryMB, diskMB, maxPids), models.PlacementConstraint{RootFs: rootFS, VolumeDrivers: volumeDrivers})
	return &task
}

func createTarWithVersion(version string) string {
	tmpDir := GinkgoT().TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "stack-*.tar")
	Expect(err).NotTo(HaveOccurred())
	tarPath := tmpFile.Name()
	Expect(tmpFile.Close()).To(Succeed())

	buf := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buf)

	// Match loadVersionFromPath: header.Name after TrimPrefix(., ".") must equal rep.StackVersionFile ("/etc/stack-version")
	Expect(tarWriter.WriteHeader(&tar.Header{
		Name: rep.StackVersionFile,
		Size: int64(len(version)),
		Mode: 0644,
	})).To(Succeed())
	_, err = tarWriter.Write([]byte(version))
	Expect(err).NotTo(HaveOccurred())
	Expect(tarWriter.Close()).To(Succeed())

	Expect(os.WriteFile(tarPath, buf.Bytes(), 0644)).To(Succeed())
	return tarPath
}
