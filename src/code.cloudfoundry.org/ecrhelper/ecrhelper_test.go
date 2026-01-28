package ecrhelper_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/ecrhelper"
)

var _ = Describe("Ecrhelper", func() {
	var ecrHelper ecrhelper.ECRHelper

	BeforeEach(func() {
		ecrHelper = ecrhelper.NewECRHelper()
	})

	Describe("IsECRRepo", func() {
		Context("when ECR repo URL is passed in", func() {
			It("returns true", func() {
				isECRREpo, err := ecrHelper.IsECRRepo("555555555.dkr.ecr.us-east-1.amazonaws.com/diego-docker-app")
				Expect(err).NotTo(HaveOccurred())
				Expect(isECRREpo).To(BeTrue())
			})
		})

		Context("when FIPS ECR repo URL is passed in", func() {
			It("returns true", func() {
				isECRREpo, err := ecrHelper.IsECRRepo("555555555.dkr.ecr-fips.us-east-1.amazonaws.com/diego-docker-app")
				Expect(err).NotTo(HaveOccurred())
				Expect(isECRREpo).To(BeTrue())
			})
		})

		Context("when not ECR repo URL is passed in", func() {
			It("returns false", func() {
				isECRRepo, err := ecrHelper.IsECRRepo("docker.io/cloudfoundry/diego-docker-app")
				Expect(err).NotTo(HaveOccurred())
				Expect(isECRRepo).To(BeFalse())
			})
		})
	})

	Describe("GetECRCredentials", func() {
		var awsAccessKeyId, awsSecretAccessKey, ecrRepoRef string

		BeforeEach(func() {
			awsAccessKeyId = os.Getenv("ECR_TEST_AWS_ACCESS_KEY_ID")
			awsSecretAccessKey = os.Getenv("ECR_TEST_AWS_SECRET_ACCESS_KEY")
			ecrRepoRef = os.Getenv("ECR_TEST_REPO_URI")

			if awsAccessKeyId == "" ||
				awsSecretAccessKey == "" ||
				ecrRepoRef == "" {
				Skip("ECR_TEST_AWS_ACCESS_KEY_ID, ECR_TEST_AWS_SECRET_ACCESS_KEY and ECR_TEST_REPO_URI should be set")
			}
		})

		It("sets username and password to ECR provided username and password", func() {
			username, password, err := ecrHelper.GetECRCredentials(ecrRepoRef, awsAccessKeyId, awsSecretAccessKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(username).To(Equal("AWS"))
			Expect(password).ToNot(BeEmpty())
			Expect(password).ToNot(Equal(awsSecretAccessKey))
		})

		Context("when ECR repo ref contains scheme", func() {
			BeforeEach(func() {
				ecrRepoRef = fmt.Sprintf("docker://%s", ecrRepoRef)
			})

			It("sets username and password to ECR provided username and password", func() {
				username, password, err := ecrHelper.GetECRCredentials(ecrRepoRef, awsAccessKeyId, awsSecretAccessKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(username).To(Equal("AWS"))
				Expect(password).ToNot(BeEmpty())
				Expect(password).ToNot(Equal(awsSecretAccessKey))
			})
		})
	})
})
