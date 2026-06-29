package cfattestor

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildSpiffeID", func() {
	It("builds the golden CF SPIFFE ID", func() {
		s := Selectors{OrgID: "org-1", SpaceID: "space-2", AppID: "app-3", ProcessType: "web"}
		Expect(BuildSpiffeID("example.org", s)).To(Equal(
			"spiffe://example.org/cf/org/org-1/space/space-2/app/app-3/process/web"))
	})
})
