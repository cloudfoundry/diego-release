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

var _ = Describe("Presences", func() {
	itValidatesLocketFlags("presences")
	itHasNoArgs("presences", true)

	var (
		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		os.Setenv("CA_CERT_FILE", locketCACertFile)
		os.Setenv("CLIENT_CERT_FILE", locketClientCertFile)
		os.Setenv("CLIENT_KEY_FILE", locketClientKeyFile)

		logger = lagertest.NewTestLogger("locket")
	})

	AfterEach(func() {
		os.Unsetenv("CA_CERT_FILE")
		os.Unsetenv("CLIENT_CERT_FILE")
		os.Unsetenv("CLIENT_KEY_FILE")
	})

	Context("when the server responds with presences", func() {
		BeforeEach(func() {
			locketConfig := locket.ClientLocketConfig{
				LocketAddress:        locketAPILocation,
				LocketCACertFile:     locketCACertFile,
				LocketClientCertFile: locketClientCertFile,
				LocketClientKeyFile:  locketClientKeyFile,
			}
			locketClient, err := locket.NewClient(logger, locketConfig)
			Expect(err).NotTo(HaveOccurred())

			req := &models.LockRequest{
				Resource: &models.Resource{
					Key:      "key",
					Owner:    "owner",
					Value:    "value",
					TypeCode: models.TypeCode_PRESENCE,
				},
				TtlInSeconds: 10,
			}

			_, err = locketClient.Lock(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("locks prints a json stream of all the locks", func() {
			cfdotCmd := exec.Command(cfdotPath, "--locketAPILocation", locketAPILocation, "presences")

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

		It("presences fails with a relevant error message", func() {
			cfdotCmd := exec.Command(cfdotPath, "--locketAPILocation", locketAPILocation, "presences")

			sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess, 11*time.Second).Should(gexec.Exit(4))
			Expect(sess.Err).To(gbytes.Say("context deadline exceeded"))
		})
	})
})
