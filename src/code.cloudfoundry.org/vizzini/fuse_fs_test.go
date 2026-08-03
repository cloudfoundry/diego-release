package vizzini_test

import (
	"io"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FuseFS", func() {
	var lrp *models.DesiredLRP
	var url string

	BeforeEach(func() {
		if !config.EnablePrivilegedContainerTests {
			Skip("privileged container tests are disabled")
		}
		lrp = DesiredLRPWithGuid(guid)
		lrp.Privileged = true
		url = "http://" + RouteForGuid(guid) + "/env"

		Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
	})

	It("should support FuseFS", func() {
		resp, err := http.Post("http://"+RouteForGuid(guid)+"/fuse-fs/mount", "application/json", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		resp, err = http.Get("http://" + RouteForGuid(guid) + "/fuse-fs/ls")
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		contents, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(ContainSubstring("fuse-fs-works.txt"))
	})
})
