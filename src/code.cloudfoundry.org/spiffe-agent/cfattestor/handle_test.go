package cfattestor

import (
	"os"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("containerHandle", func() {
	const pid = 4242
	var procRoot string

	writeCgroup := func(content string) {
		dir := filepath.Join(procRoot, strconv.Itoa(pid))
		Expect(os.MkdirAll(dir, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "cgroup"), []byte(content), 0644)).To(Succeed())
	}

	BeforeEach(func() {
		procRoot = GinkgoT().TempDir()
	})

	DescribeTable("resolves the Garden handle",
		func(content, expected string) {
			writeCgroup(content)
			handle, err := containerHandle(procRoot, pid)
			Expect(err).NotTo(HaveOccurred())
			Expect(handle).To(Equal(expected))
		},
		Entry("cgroup v1 controller line", "12:cpu,cpuacct:/garden/abc123\n", "abc123"),
		Entry("cgroup v2 unified line", "0::/garden/abc123/init\n", "abc123"),
		Entry("mixed multi-line", "11:pids:/system.slice\n12:cpu,cpuacct:/garden/abc123\n0::/garden/abc123/init\n", "abc123"),
	)

	It("errors when no garden segment exists", func() {
		writeCgroup("0::/system.slice/foo.service\n")
		_, err := containerHandle(procRoot, pid)
		Expect(err).To(HaveOccurred())
	})

	It("errors when the cgroup file is unreadable", func() {
		_, err := containerHandle(procRoot, pid)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("handleFromPath", func() {
	DescribeTable("extracts the segment after garden",
		func(path, expected string, ok bool) {
			handle, found := handleFromPath(path)
			Expect(found).To(Equal(ok))
			Expect(handle).To(Equal(expected))
		},
		Entry("v1 path", "/garden/abc123", "abc123", true),
		Entry("v2 init path", "/garden/abc123/init", "abc123", true),
		Entry("no garden", "/system.slice/foo.service", "", false),
		Entry("garden trailing with nothing", "/garden", "", false),
	)
})
