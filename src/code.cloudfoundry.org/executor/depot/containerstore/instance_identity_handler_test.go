package containerstore_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/garden"
)

var _ = Describe("InstanceIdentityHandler", func() {
	var (
		tmpdir    string
		handler   *containerstore.InstanceIdentityHandler
		container executor.Container
	)

	AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	BeforeEach(func() {
		container = executor.Container{Guid: "some-guid"}
		var err error
		tmpdir, err = os.MkdirTemp("", "credsmanager")
		Expect(err).NotTo(HaveOccurred())
		handler = containerstore.NewInstanceIdentityHandler(
			tmpdir,
			"containerpath",
		)
	})

	Context("CreateDir", func() {
		It("returns a valid directory path", func() {
			mount, _, err := handler.CreateDir(logger, container)
			Expect(err).To(Succeed())

			Expect(mount).To(HaveLen(1))
			Expect(mount[0].SrcPath).To(BeADirectory())
			Expect(mount[0].DstPath).To(Equal("containerpath"))
			Expect(mount[0].Mode).To(Equal(garden.BindMountModeRO))
			Expect(mount[0].Origin).To(Equal(garden.BindMountOriginHost))
		})

		It("returns CF_INSTANCE_CERT and CF_INSTANCE_KEY environment variable values", func() {
			_, envVariables, err := handler.CreateDir(logger, container)
			Expect(err).To(Succeed())

			Expect(envVariables).To(HaveLen(2))
			values := map[string]string{}
			values[envVariables[0].Name] = envVariables[0].Value
			values[envVariables[1].Name] = envVariables[1].Value
			Expect(values).To(Equal(map[string]string{
				"CF_INSTANCE_CERT": "containerpath/instance.crt",
				"CF_INSTANCE_KEY":  "containerpath/instance.key",
			}))
		})

		Context("when making directory fails", func() {
			BeforeEach(func() {
				handler = containerstore.NewInstanceIdentityHandler(
					"/invalid/path",
					"containerpath",
				)
			})

			It("returns an error", func() {
				_, _, err := handler.CreateDir(logger, executor.Container{Guid: "somefailure"})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			_, _, err := handler.CreateDir(logger, container)
			Expect(err).To(BeNil())
		})

		It("noops if no Credential is passed", func() {
			By("updating the Credential with a known value")
			err := handler.Update(containerstore.Credentials{InstanceIdentityCredential: containerstore.Credential{Cert: "cert", Key: "key"}}, container)
			Expect(err).NotTo(HaveOccurred())

			certPath := filepath.Join(tmpdir, "some-guid")
			certFile := filepath.Join(certPath, "instance.crt")
			Expect(certFile).To(BeARegularFile())

			data, err := os.ReadFile(certFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(data)).To(Equal("cert"))

			keyFile := filepath.Join(certPath, "instance.key")
			Expect(keyFile).To(BeARegularFile())

			data, err = os.ReadFile(keyFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("key"))

			By("calling update with with an empty Credential")
			err = handler.Update(containerstore.Credentials{}, container)
			Expect(err).NotTo(HaveOccurred())

			By("checking that the old Credential is still there")
			data, err = os.ReadFile(certFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("cert"))

			data, err = os.ReadFile(keyFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("key"))
		})

		It("puts private key into container directory", func() {
			err := handler.Update(containerstore.Credentials{InstanceIdentityCredential: containerstore.Credential{Cert: "cert", Key: "key"}}, container)
			Expect(err).NotTo(HaveOccurred())

			certPath := filepath.Join(tmpdir, "some-guid")
			keyFile := filepath.Join(certPath, "instance.key")
			Expect(keyFile).To(BeARegularFile())

			data, err := os.ReadFile(keyFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(data)).To(Equal("key"))
		})

		It("puts the certificate into container directory", func() {
			err := handler.Update(containerstore.Credentials{InstanceIdentityCredential: containerstore.Credential{Cert: "cert", Key: "key"}}, container)
			Expect(err).NotTo(HaveOccurred())

			certPath := filepath.Join(tmpdir, "some-guid")
			certFile := filepath.Join(certPath, "instance.crt")
			Expect(certFile).To(BeARegularFile())

			data, err := os.ReadFile(certFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(data)).To(Equal("cert"))
		})
	})

	Describe("Close", func() {
		BeforeEach(func() {
			_, _, err := handler.CreateDir(logger, container)
			Expect(err).To(BeNil())
			err = handler.Update(containerstore.Credentials{InstanceIdentityCredential: containerstore.Credential{Cert: "cert", Key: "key"}}, container)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not put private key into container directory", func() {
			err := handler.Close(containerstore.Credentials{InstanceIdentityCredential: containerstore.Credential{Cert: "invalid-cert", Key: "invalid-key"}}, container)
			Expect(err).NotTo(HaveOccurred())

			certPath := filepath.Join(tmpdir, "some-guid")
			keyFile := filepath.Join(certPath, "instance.key")
			Expect(keyFile).To(BeARegularFile())

			data, err := os.ReadFile(keyFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(data)).NotTo(Equal("invalid-key"))
		})

		It("does not put the certificate into container directory", func() {
			err := handler.Close(containerstore.Credentials{InstanceIdentityCredential: containerstore.Credential{Cert: "invalid-cert", Key: "invalid-key"}}, container)
			Expect(err).NotTo(HaveOccurred())

			certPath := filepath.Join(tmpdir, "some-guid")
			certFile := filepath.Join(certPath, "instance.crt")
			Expect(certFile).To(BeARegularFile())

			data, err := os.ReadFile(certFile)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(data)).NotTo(Equal("invalid-cert"))
		})
	})

	Context("RemoveDir", func() {
		BeforeEach(func() {
			_, _, err := handler.CreateDir(logger, container)
			Expect(err).NotTo(HaveOccurred())
			certPath := filepath.Join(tmpdir, "some-guid")
			Eventually(certPath).Should(BeADirectory())
		})

		It("removes the directory created", func() {
			err := handler.RemoveDir(logger, container)
			Expect(err).NotTo(HaveOccurred())
			certPath := filepath.Join(tmpdir, "some-guid")
			Eventually(certPath).ShouldNot(BeADirectory())
		})
	})
})
