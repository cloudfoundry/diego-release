package gcrhelper_test

import (
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/gcrhelper"
)

var _ = Describe("Gcrhelper", func() {
	var helper gcrhelper.GCRHelper

	BeforeEach(func() {
		helper = gcrhelper.NewGCRHelper()
	})

	Describe("IsGCRRepo", func() {
		DescribeTable("returns true for GCR/Artifact Registry URLs",
			func(url string) {
				isGCR, err := helper.IsGCRRepo(url)
				Expect(err).NotTo(HaveOccurred())
				Expect(isGCR).To(BeTrue())
			},
			Entry("gcr.io", "gcr.io/my-project/my-image:tag"),
			Entry("gcr.io with docker:// scheme", "docker://gcr.io/my-project/my-image:tag"),
			Entry("us.gcr.io", "us.gcr.io/my-project/my-image:tag"),
			Entry("eu.gcr.io", "eu.gcr.io/my-project/my-image:tag"),
			Entry("asia.gcr.io", "asia.gcr.io/my-project/my-image:tag"),
			Entry("Artifact Registry pkg.dev", "europe-west3-docker.pkg.dev/my-project/my-repo/my-image:tag"),
			Entry("Artifact Registry with docker:// scheme", "docker://europe-west3-docker.pkg.dev/my-project/my-repo/my-image:tag"),
			Entry("us-central1 Artifact Registry", "us-central1-docker.pkg.dev/my-project/my-repo/my-image:latest"),
		)

		DescribeTable("returns false for non-GCR URLs",
			func(url string) {
				isGCR, err := helper.IsGCRRepo(url)
				Expect(err).NotTo(HaveOccurred())
				Expect(isGCR).To(BeFalse())
			},
			Entry("Docker Hub", "docker.io/cloudfoundry/diego-docker-app"),
			Entry("Docker Hub with docker:// scheme", "docker://cloudfoundry/diego-docker-app"),
			Entry("ECR", "555555555.dkr.ecr.us-east-1.amazonaws.com/my-image"),
			Entry("private registry", "internal-registry.example.com:5000/my-repo/my-image:v2"),
			Entry("preloaded rootfs", "preloaded:cflinuxfs4"),
		)
	})

	Describe("GetGCRCredentials", func() {
		Context("when the token fetcher succeeds", func() {
			BeforeEach(func() {
				helper = gcrhelper.NewGCRHelperWithTokenFetcher(func() (string, error) {
					return "ya29.fake-token", nil
				})
			})

			It("returns oauth2accesstoken and the metadata token", func() {
				username, password, err := helper.GetGCRCredentials()
				Expect(err).NotTo(HaveOccurred())
				Expect(username).To(Equal("oauth2accesstoken"))
				Expect(password).To(Equal("ya29.fake-token"))
			})

			It("fetches a fresh token on every call (tokens are short-lived and metadata calls are <1ms on GCE)", func() {
				var calls atomic.Int32
				helper = gcrhelper.NewGCRHelperWithTokenFetcher(func() (string, error) {
					calls.Add(1)
					return "ya29.fake-token", nil
				})

				_, _, _ = helper.GetGCRCredentials()
				_, _, _ = helper.GetGCRCredentials()
				_, _, _ = helper.GetGCRCredentials()
				Expect(calls.Load()).To(Equal(int32(3)))
			})
		})

		Context("when the token fetcher fails (e.g. not running on GCE)", func() {
			BeforeEach(func() {
				helper = gcrhelper.NewGCRHelperWithTokenFetcher(func() (string, error) {
					return "", fmt.Errorf("metadata server unreachable")
				})
			})

			It("returns empty credentials so public images still pull unauthenticated", func() {
				username, password, err := helper.GetGCRCredentials()
				Expect(err).NotTo(HaveOccurred())
				Expect(username).To(Equal(""))
				Expect(password).To(Equal(""))
			})

			It("does not retry the metadata server after the first failure (one dial attempt per process lifetime)", func() {
				var calls atomic.Int32
				helper = gcrhelper.NewGCRHelperWithTokenFetcher(func() (string, error) {
					calls.Add(1)
					return "", fmt.Errorf("metadata server unreachable")
				})

				_, _, _ = helper.GetGCRCredentials()
				_, _, _ = helper.GetGCRCredentials()
				_, _, _ = helper.GetGCRCredentials()
				Expect(calls.Load()).To(Equal(int32(1)))
			})
		})
	})
})
