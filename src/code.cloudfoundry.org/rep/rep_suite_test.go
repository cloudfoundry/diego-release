package rep_test

import (
	"testing"
	"time"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	executorfakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	"code.cloudfoundry.org/rep/repfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	cfHttpTimeout time.Duration
	auctionRep    *repfakes.FakeClient
	factory       bbsmodels.RepClientFactory

	fakeExecutorClient *executorfakes.FakeClient
	fakeEvacuatable    *fake_evacuation_context.FakeEvacuatable
)

func TestRep(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rep Suite")
}

var _ = BeforeEach(func() {
	auctionRep = &repfakes.FakeClient{}
	fakeExecutorClient = &executorfakes.FakeClient{}
	fakeEvacuatable = &fake_evacuation_context.FakeEvacuatable{}

	cfHttpTimeout = 1 * time.Second
	var err error
	httpClient := cfhttp.NewClient(
		cfhttp.WithRequestTimeout(cfHttpTimeout),
	)
	factory, err = rep.NewClientFactory(httpClient, httpClient, nil)
	Expect(err).NotTo(HaveOccurred())
})
