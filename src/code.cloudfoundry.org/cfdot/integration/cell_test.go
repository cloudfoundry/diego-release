package integration_test

import (
	"encoding/json"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("cell", func() {
	itValidatesBBSFlags("cell")

	Context("when cell command is called", func() {
		var (
			presence      *models.CellPresence
			serverTimeout int
		)

		BeforeEach(func() {
			presence = &models.CellPresence{
				CellId:     "cell-1",
				RepAddress: "rep-1",
			}
			serverTimeout = 0
		})

		JustBeforeEach(func() {
			response := &models.CellsResponse{
				Cells: []*models.CellPresence{presence},
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
			sess := RunCFDot("cell", "cell-1")
			Eventually(sess).Should(gexec.Exit(0))

			jsonData, err := json.Marshal(presence)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Out).To(gbytes.Say(string(jsonData)))
		})

		Context("when the cell does not exist", func() {
			It("exits with status code of 5", func() {
				sess := RunCFDot("cell", "cell-id-dsafasdklfjasdlkf")
				Eventually(sess).Should(gexec.Exit(5))
			})
		})

		Context("when timeout flag is present", func() {
			var sess *gexec.Session

			BeforeEach(func() {
				sess = RunCFDot("--timeout", "1", "cell", "cell-1")
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

					jsonData, err := json.Marshal(presence)
					Expect(err).NotTo(HaveOccurred())
					Expect(sess.Out).To(gbytes.Say(string(jsonData)))
				})
			})
		})
	})

	Context("when cell command is called with extra arguments", func() {
		It("exits with status code of 3", func() {
			sess := RunCFDot("cell", "cell-id", "extra-argument")
			Eventually(sess).Should(gexec.Exit(3))
		})
	})
})
