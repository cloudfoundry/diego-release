package main_test

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Main", func() {
	var (
		session *gexec.Session
		command *exec.Cmd
		err     error
	)

	BeforeEach(func() {
		command = exec.Command(driverPath)
	})

	JustBeforeEach(func() {
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		session.Kill().Wait("2s")
	})

	Context("with a driver path", func() {
		var dir string

		BeforeEach(func() {
			var err error
			dir, err = os.MkdirTemp("", "driversPath")
			Expect(err).ToNot(HaveOccurred())

			command.Args = append(command.Args, "-driversPath="+dir)
			command.Args = append(command.Args, "-transport=tcp-json")
		})

		It("listens on tcp/9750 by default", func() {
			EventuallyWithOffset(1, func() error {
				_, err := net.Dial("tcp", "0.0.0.0:9750")
				return err
			}, 5).ShouldNot(HaveOccurred())

			specFile := filepath.Join(dir, "localdriver.json")
			specFileContents, err := os.ReadFile(specFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(specFileContents)).To(MatchJSON(`{
				"Name": "localdriver",
				"Addr": "http://0.0.0.0:9750",
				"TLSConfig": null,
				"UniqueVolumeIds": false
			}`))
		})

		Context("in another context", func() {
			BeforeEach(func() {
				command.Args = append(command.Args, "-listenAddr=0.0.0.0:9751")
			})

			It("listens on tcp/9751", func() {
				EventuallyWithOffset(1, func() error {
					_, err := net.Dial("tcp", "0.0.0.0:9751")
					return err
				}, 5).ShouldNot(HaveOccurred())
			})
		})

		Context("with unique volume IDs enabled", func() {
			BeforeEach(func() {
				command.Args = append(command.Args, "-uniqueVolumeIds")
			})

			It("sets the UniqueVolumeIds flag in the spec file", func() {
				specFile := filepath.Join(dir, "localdriver.json")
				Eventually(func() error {
					_, err := os.Stat(specFile)
					return err
				}, 5).ShouldNot(HaveOccurred())

				specFileContents, err := os.ReadFile(specFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(specFileContents)).To(MatchJSON(`{
					"Name": "localdriver",
					"Addr": "http://0.0.0.0:9750",
					"TLSConfig": null,
					"UniqueVolumeIds": true
				}`))
			})
		})
	})
})
