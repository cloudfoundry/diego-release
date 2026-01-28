package ecrhelper

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"                    //lint:ignore SA1019 - need tot bump to aws-sdk-go-v2
	"github.com/aws/aws-sdk-go/aws/credentials"        //lint:ignore SA1019 - need tot bump to aws-sdk-go-v2
	"github.com/aws/aws-sdk-go/aws/endpoints"          //lint:ignore SA1019 - need tot bump to aws-sdk-go-v2
	awssession "github.com/aws/aws-sdk-go/aws/session" //lint:ignore SA1019 - need tot bump to aws-sdk-go-v2
	"github.com/aws/aws-sdk-go/service/ecr"            //lint:ignore SA1019 - need tot bump to aws-sdk-go-v2
	ecrapi "github.com/awslabs/amazon-ecr-credential-helper/ecr-login/api"
)

const ECR_REPO_REGEX = `[a-zA-Z0-9][a-zA-Z0-9_-]*\.dkr\.ecr(-fips)?\.[a-zA-Z0-9][a-zA-Z0-9_-]*\.amazonaws\.com(\.cn)?[^ ]*`

//go:generate counterfeiter -o fakes/fake_ecrhelper.go . ECRHelper
type ECRHelper interface {
	IsECRRepo(registryURL string) (bool, error)
	GetECRCredentials(registryURL string, username string, password string) (string, string, error)
}

type ecrHelper struct {
}

func NewECRHelper() ECRHelper {
	return ecrHelper{}
}

func (h ecrHelper) IsECRRepo(registryURL string) (bool, error) {
	rECRRepo, err := regexp.Compile(ECR_REPO_REGEX)
	if err != nil {
		return false, err
	}

	isECR := rECRRepo.MatchString(registryURL)

	return isECR, nil
}

func (h ecrHelper) GetECRCredentials(registryURL string, username string, password string) (string, string, error) {
	rootFSURL, err := url.Parse(registryURL)
	if err != nil {
		return "", "", err
	}
	rootFSURL.Scheme = ""
	ecrRequestURL := strings.TrimLeft(rootFSURL.String(), "/")

	registry, err := ecrapi.ExtractRegistry(ecrRequestURL)
	if err != nil {
		return "", "", err
	}

	awsSession, err := awssession.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(username, password, ""),
	})
	if err != nil {
		return "", "", err
	}

	awsConfig := &aws.Config{Region: aws.String(registry.Region)}
	if registry.FIPS {
		resolver := endpoints.DefaultResolver()
		endpoint, err := resolver.EndpointFor("ecr-fips", registry.Region, func(opts *endpoints.Options) {
			opts.ResolveUnknownService = true
		})
		if err != nil {
			return "", "", err
		}
		awsConfig = awsConfig.WithEndpoint(endpoint.URL)
	}

	ecrClient := ecr.New(awsSession, awsConfig)

	input := &ecr.GetAuthorizationTokenInput{}
	output, err := ecrClient.GetAuthorizationToken(input)
	if err != nil {
		return "", "", err
	}
	if output == nil {
		return "", "", fmt.Errorf("missing AuthorizationData in response")
	}

	for _, authData := range output.AuthorizationData {
		if authData.ProxyEndpoint != nil && authData.AuthorizationToken != nil {
			token := aws.StringValue(authData.AuthorizationToken)
			decodedToken, err := base64.StdEncoding.DecodeString(token)
			if err != nil {
				return "", "", err
			}

			parts := strings.SplitN(string(decodedToken), ":", 2)
			if len(parts) < 2 {
				return "", "", fmt.Errorf("invalid authorization token")
			}

			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("no authorization token found")
}
