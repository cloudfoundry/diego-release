package integration_test

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("cells", func() {
	var serverTimeout int

	itValidatesBBSFlags("cells")
	itHasNoArgs("cells", false)

	Context("when cells command is called", func() {
		BeforeEach(func() {
			serverTimeout = 0
		})

		JustBeforeEach(func() {
			response := &models.CellsResponse{
				Cells: []*models.CellPresence{
					{
						CellId:     "cell-1",
						RepAddress: "rep-1",
						Zone:       "zone1",
						Capacity: &models.CellCapacity{
							MemoryMb:   1024,
							DiskMb:     1024,
							Containers: 10,
						},
						RootfsProviders: []*models.Provider{
							{
								Name: "rootfs1",
							},
						},
						RepUrl: "http://rep1.com",
					},
				},
			}
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/cells/list.r1"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.RespondWithProto(200, response.ToProto()),
				),
			)
		})

		It("returns the json encoding of the cell presences", func() {
			sess := RunCFDot("cells")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say(`"rep_url":"http://rep1.com"`))
		})

		Context("when timeout flag is present", func() {
			var sess *gexec.Session

			BeforeEach(func() {
				sess = RunCFDot("--timeout", "1", "cells")
			})

			Context("when request exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})
			})

			Context("when request is within the timeout", func() {
				It("exits with status code of 0", func() {
					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out).To(gbytes.Say(`"rep_url":"http://rep1.com"`))
				})
			})
		})
	})

	Context("when cells command is called with extra arguments", func() {
		It("exits with status code of 3", func() {
			sess := RunCFDot("cells", "extra-args")
			Eventually(sess).Should(gexec.Exit(3))
		})
	})
})
