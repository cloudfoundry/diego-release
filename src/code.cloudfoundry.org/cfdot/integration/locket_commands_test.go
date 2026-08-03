package integration_test

import (
	"context"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var _ = Describe("Locket commands", func() {
	var (
		logger       *lagertest.TestLogger
		locketClient models.LocketClient
	)

	BeforeEach(func() {
		os.Setenv("CA_CERT_FILE", locketCACertFile)
		os.Setenv("CLIENT_CERT_FILE", locketClientCertFile)
		os.Setenv("CLIENT_KEY_FILE", locketClientKeyFile)

		var err error
		logger = lagertest.NewTestLogger("locket")
		locketClient, err = locket.NewClient(logger, locket.ClientLocketConfig{
			LocketAddress:        locketAPILocation,
			LocketCACertFile:     locketCACertFile,
			LocketClientCertFile: locketClientCertFile,
			LocketClientKeyFile:  locketClientKeyFile,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.Unsetenv("CA_CERT_FILE")
		os.Unsetenv("CLIENT_CERT_FILE")
		os.Unsetenv("CLIENT_KEY_FILE")
	})

	Describe("release-lock", func() {
		itValidatesLocketFlags("release-lock")

		BeforeEach(func() {
			req := &models.LockRequest{
				Resource: &models.Resource{
					Key:      "test-key",
					Owner:    "test-owner",
					Value:    "test-value",
					TypeCode: models.LOCK,
				},
				TtlInSeconds: 10,
			}

			_, err := locketClient.Lock(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("exits successfully with code 0 after releasing the lock", func() {
			cfdotCmd := exec.Command(cfdotPath,
				"--locketAPILocation", locketAPILocation,
				"release-lock",
				"--key", "test-key",
				"--owner", "test-owner",
			)

			sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			_, err = locketClient.Fetch(context.Background(), &models.FetchRequest{Key: "test-key"})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("claim-locks", func() {
		itValidatesLocketFlags("claim-lock")

		It("exits successfully with code 0 after acquiring the lock", func() {
			cfdotCmd := exec.Command(cfdotPath,
				"--locketAPILocation", locketAPILocation,
				"claim-lock",
				"--key", "test-key",
				"--owner", "test-owner",
				"--ttl", "60",
			)

			sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			resp, err := locketClient.Fetch(context.Background(), &models.FetchRequest{Key: "test-key"})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.GetResource()).To(Equal(&models.Resource{
				Key:      "test-key",
				Owner:    "test-owner",
				Type:     "lock",
				TypeCode: models.LOCK,
			}))
		})

		Context("when the server is down", func() {
			BeforeEach(func() {
				ginkgomon.Interrupt(locketProcess)
			})

			It("fails with a relevant error message", func() {
				cfdotCmd := exec.Command(cfdotPath,
					"--locketAPILocation", locketAPILocation,
					"claim-lock",
					"--key", "test-key",
					"--owner", "test-owner",
					"--ttl", "60",
				)

				sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess, 11*time.Second).Should(gexec.Exit(4))
				Expect(sess.Err).To(gbytes.Say("context deadline exceeded"))
			})
		})
	})

	Describe("locks", func() {
		itValidatesLocketFlags("locks")
		itHasNoArgs("locks", true)

		Context("when the server responds with locks", func() {
			BeforeEach(func() {
				req := &models.LockRequest{
					Resource: &models.Resource{
						Key:      "key",
						Owner:    "owner",
						Value:    "value",
						TypeCode: models.LOCK,
					},
					TtlInSeconds: 10,
				}

				_, err := locketClient.Lock(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("prints a json stream of all the locks", func() {
				cfdotCmd := exec.Command(cfdotPath, "--locketAPILocation", locketAPILocation, "locks")

				sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(gbytes.Say(`"key":"key","owner":"owner"`))
			})
		})

		Context("when the server is down", func() {
			BeforeEach(func() {
				ginkgomon.Interrupt(locketProcess)
			})

			It("fails with a relevant error message", func() {
				cfdotCmd := exec.Command(cfdotPath, "--locketAPILocation", locketAPILocation, "locks")

				sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess, 11*time.Second).Should(gexec.Exit(4))
				Expect(sess.Err).To(gbytes.Say("context deadline exceeded"))
			})
		})
	})
})
