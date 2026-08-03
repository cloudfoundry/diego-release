package ecrhelper_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/ecrhelper"
	"code.cloudfoundry.org/ecrhelper/fakes"
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
		var fakeECRClient *fakes.FakeECRClient
		var useFIPSEndpoint bool

		BeforeEach(func() {
			awsAccessKeyId = "fake_access_key_id"
			awsSecretAccessKey = "fake_secret_access_key"
			ecrRepoRef = "123456789012.dkr.ecr.us-east-1.amazonaws.com"

			validAuthToken := "AWS:fake_password"
			encodedToken := base64.StdEncoding.EncodeToString([]byte(validAuthToken))

			fakeECRClient = &fakes.FakeECRClient{}
			useFIPSEndpoint = false
			ecrHelper = ecrhelper.NewECRHelperWithFactory(func(ctx context.Context, region, username, password string, useFIPS bool) (ecrhelper.ECRClient, error) {
				useFIPSEndpoint = useFIPS
				return fakeECRClient, nil
			})

			fakeECRClient.GetAuthorizationTokenReturns(
				&ecr.GetAuthorizationTokenOutput{
					AuthorizationData: []types.AuthorizationData{
						{
							AuthorizationToken: aws.String(encodedToken),
							ProxyEndpoint:      aws.String(fmt.Sprintf("https://%s", ecrRepoRef)),
						},
					},
				},
				nil,
			)
		})

		It("sets username and password to ECR provided username and password", func() {
			url := fmt.Sprintf("%s/%s", ecrRepoRef, "diego-docker-app")
			username, password, err := ecrHelper.GetECRCredentials(url, awsAccessKeyId, awsSecretAccessKey)

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeECRClient.GetAuthorizationTokenCallCount()).To(Equal(1))
			Expect(useFIPSEndpoint).To(BeFalse())
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
				Expect(fakeECRClient.GetAuthorizationTokenCallCount()).To(Equal(1))
				Expect(username).To(Equal("AWS"))
				Expect(password).ToNot(BeEmpty())
				Expect(password).ToNot(Equal(awsSecretAccessKey))
			})
		})

		Context("when ECR repo ref is a FIPS repo", func() {
			BeforeEach(func() {
				ecrRepoRef = "123456789012.dkr.ecr-fips.us-east-1.amazonaws.com"
			})

			It("should configure the ecr client with FIPS endpoint", func() {
				url := fmt.Sprintf("%s/%s", ecrRepoRef, "diego-docker-app")
				_, _, err := ecrHelper.GetECRCredentials(url, awsAccessKeyId, awsSecretAccessKey)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeECRClient.GetAuthorizationTokenCallCount()).To(Equal(1))
				Expect(useFIPSEndpoint).To(BeTrue())
			})
		})
	})

	Describe("GetECRCredentials integration", func() {
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
