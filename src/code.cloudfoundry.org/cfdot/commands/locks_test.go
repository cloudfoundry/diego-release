package commands_test

import (
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/cfdot/commands"
	"code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/locket/models/modelsfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Locks", func() {
	var (
		fakeLocketClient *modelsfakes.FakeLocketClient
		stdout, stderr   *gbytes.Buffer
	)

	BeforeEach(func() {
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		fakeLocketClient = &modelsfakes.FakeLocketClient{}
	})

	Context("when the locket responds with locks", func() {
		var (
			resources *models.FetchAllResponse
		)
		BeforeEach(func() {
			resources = &models.FetchAllResponse{
				Resources: []*models.Resource{
					&models.Resource{
						Key:   "key",
						Owner: "owner",
						Value: "value",
						Type:  "type",
					},
				},
			}
			fakeLocketClient.FetchAllReturns(resources, nil)
		})

		It("prints a json stream of all the locks", func() {
			err := commands.Locks(stdout, stderr, fakeLocketClient)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocketClient.FetchAllCallCount()).To(Equal(1))

			_, req, _ := fakeLocketClient.FetchAllArgsForCall(0)
			Expect(req).To(Equal(&models.FetchAllRequest{TypeCode: models.LOCK}))

			d, err := json.Marshal(resources.Resources[0])
			Expect(err).NotTo(HaveOccurred())
			expectedOutput := string(d) + "\n"

			Expect(string(stdout.Contents())).To(Equal(expectedOutput))
		})
	})

	Context("when the locket errors", func() {
		JustBeforeEach(func() {
			fakeLocketClient.FetchAllReturns(nil, errors.New("boom"))
		})

		It("fails with a relevant error", func() {
			err := commands.Locks(stdout, stderr, fakeLocketClient)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(errors.New("boom")))
		})
	})
})
