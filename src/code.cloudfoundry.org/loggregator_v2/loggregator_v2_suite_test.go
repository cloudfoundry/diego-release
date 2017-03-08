package loggregator_v2_test

import (
	"io/ioutil"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/grpclog"

	"testing"
)

func TestLoggregatorV2(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LoggregatorV2 Suite")
}

var _ = BeforeSuite(func() {
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))
})
