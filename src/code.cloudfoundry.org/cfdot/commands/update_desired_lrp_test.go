package commands_test

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/openzipkin/zipkin-go/model"
)

var _ = Describe("UpdateDesiredLRP", func() {
	var (
		fakeBBSClient     *fake_bbs.FakeClient
		stdout, stderr    *gbytes.Buffer
		updatedDesiredLRP *models.DesiredLRPUpdate
		processGuid       string
		spec              []byte
	)

	BeforeEach(func() {
		fakeBBSClient = &fake_bbs.FakeClient{}
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		processGuid = "some-process-guid"
		initialDesiredLRP := &models.DesiredLRP{
			ProcessGuid: processGuid,
			Instances:   1,
		}

		var err error
		initialSpec, err := json.Marshal(initialDesiredLRP)
		Expect(err).NotTo(HaveOccurred())
		err = commands.CreateDesiredLRP(stdout, stderr, fakeBBSClient, initialSpec)
		Expect(err).NotTo(HaveOccurred())

		updatedInstanceCount := int32(4)
		dlu := models.DesiredLRPUpdate{}
		dlu.SetInstances(updatedInstanceCount)
		updatedDesiredLRP = &dlu

		spec, err = json.Marshal(updatedDesiredLRP)
		Expect(err).NotTo(HaveOccurred())
	})

	It("updates the desired lrp", func() {
		err := commands.UpdateDesiredLRP(stdout, stderr, fakeBBSClient, processGuid, spec)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeBBSClient.UpdateDesiredLRPCallCount()).To(Equal(1))
		_, traceID, guid, lrp := fakeBBSClient.UpdateDesiredLRPArgsForCall(0)

		_, err = model.TraceIDFromHex(traceID)
		Expect(err).NotTo(HaveOccurred())
		Expect(lrp).To(Equal(updatedDesiredLRP))
		Expect(guid).To(Equal(processGuid))
	})

	Context("when a file is passed as an argument", func() {
		var filename string

		BeforeEach(func() {
			f, err := os.CreateTemp(os.TempDir(), "spec_file")
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()
			_, err = f.Write(spec)
			Expect(err).NotTo(HaveOccurred())
			filename = f.Name()
		})

		It("validates the input file successfully", func() {
			args := []string{processGuid, "@" + filename}
			actualProcessGuid, actualSpec, err := commands.ValidateUpdateDesiredLRPArguments(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec).To(Equal(spec))
			Expect(actualProcessGuid).To(Equal(processGuid))
		})
	})

	Context("when the bbs errors", func() {
		BeforeEach(func() {
			fakeBBSClient.UpdateDesiredLRPReturns(models.ErrUnknownError)
		})

		It("fails with a relevant error", func() {
			err := commands.UpdateDesiredLRP(stdout, stderr, fakeBBSClient, processGuid, []byte("{}"))
			Expect(err).To(MatchError(models.ErrUnknownError))
		})
	})
})
