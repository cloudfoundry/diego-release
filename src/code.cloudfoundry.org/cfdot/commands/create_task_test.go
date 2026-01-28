package commands_test

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/openzipkin/zipkin-go/model"
)

var _ = Describe("CreateTask", func() {

	var (
		fakeBBSClient  *fake_bbs.FakeClient
		stdout, stderr *gbytes.Buffer
		expectedTask   *models.Task
		spec           []byte
	)

	BeforeEach(func() {
		fakeBBSClient = &fake_bbs.FakeClient{}
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		expectedTask = &models.Task{
			TaskGuid:       "task-guid",
			Domain:         "domain",
			TaskDefinition: model_helpers.NewValidTaskDefinition(),
		}
		var err error
		spec, err = json.Marshal(expectedTask)
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates the task", func() {
		err := commands.CreateTask(stdout, stderr, fakeBBSClient, spec)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeBBSClient.DesireTaskCallCount()).To(Equal(1))
		_, traceID, guid, domain, taskDefinition := fakeBBSClient.DesireTaskArgsForCall(0)
		Expect(guid).To(Equal(expectedTask.TaskGuid))
		Expect(domain).To(Equal(expectedTask.Domain))
		Expect(taskDefinition).To(Equal(expectedTask.TaskDefinition))

		_, err = model.TraceIDFromHex(traceID)
		Expect(err).NotTo(HaveOccurred())
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
			args := []string{"@" + filename}
			actualSpec, err := commands.ValidateCreateTaskArguments(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec).To(Equal(spec))
		})

	})

	Context("when the bbs errors", func() {
		BeforeEach(func() {
			fakeBBSClient.DesireTaskReturns(models.ErrUnknownError)
		})

		It("fails with a relevant error", func() {
			err := commands.CreateTask(stdout, stderr, fakeBBSClient, []byte("{}"))
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(models.ErrUnknownError))
		})
	})

})
