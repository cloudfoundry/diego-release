package handlers_test

import (
	"bytes"
	"net/http"

	"code.cloudfoundry.org/rep"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("LogWrap", func() {
	It("emits a log line for every request", func() {
		status, _ := Request(rep.PingRoute, nil, bytes.NewBufferString(""))
		Expect(status).To(Equal(http.StatusOK))
		Expect(logger.Buffer()).To(gbytes.Say("serving"))
		Expect(logger.Buffer()).To(gbytes.Say("done"))
	})
})
