package handlers_test

import (
	"net/http"

	"code.cloudfoundry.org/rep"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PingHandler", func() {
	It("responds with 200 OK", func() {
		status, _ := Request(rep.PingRoute, nil, nil)
		Expect(status).To(Equal(http.StatusOK))
	})
})
