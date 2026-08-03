package ecrhelper

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrapi "github.com/awslabs/amazon-ecr-credential-helper/ecr-login/api"
)

const ECR_REPO_REGEX = `[a-zA-Z0-9][a-zA-Z0-9_-]*\.dkr\.ecr(-fips)?\.[a-zA-Z0-9][a-zA-Z0-9_-]*\.amazonaws\.com(\.cn)?[^ ]*`

//go:generate counterfeiter -o fakes/fake_ecr_client.go . ECRClient
type ECRClient interface {
	GetAuthorizationToken(ctx context.Context, params *ecr.GetAuthorizationTokenInput, optFns ...func(*ecr.Options)) (*ecr.GetAuthorizationTokenOutput, error)
}

//go:generate counterfeiter -o fakes/fake_ecrhelper.go . ECRHelper
type ECRHelper interface {
	IsECRRepo(registryURL string) (bool, error)
	GetECRCredentials(registryURL string, username string, password string) (string, string, error)
}

type ClientFactory func(ctx context.Context, region, username, password string, useFIPS bool) (ECRClient, error)

func DefaultClientFactory(ctx context.Context, region, username, password string, useFIPS bool) (ECRClient, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(username, password, "")),
	}

	if useFIPS {
		opts = append(opts, config.WithUseFIPSEndpoint(aws.FIPSEndpointStateEnabled))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return ecr.NewFromConfig(cfg), nil
}

type ecrHelper struct {
	ecrClientFactory ClientFactory
}

func NewECRHelper() ECRHelper {
	return &ecrHelper{
		ecrClientFactory: DefaultClientFactory,
	}
}

func NewECRHelperWithFactory(factory ClientFactory) ECRHelper {
	if factory == nil {
		factory = DefaultClientFactory
	}

	return &ecrHelper{
		ecrClientFactory: factory,
	}
}

func (h *ecrHelper) IsECRRepo(registryURL string) (bool, error) {
	rECRRepo, err := regexp.Compile(ECR_REPO_REGEX)
	if err != nil {
		return false, err
	}

	isECR := rECRRepo.MatchString(registryURL)

	return isECR, nil
}

func (h *ecrHelper) GetECRCredentials(registryURL string, username string, password string) (string, string, error) {
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

	ecrClient, err := h.ecrClientFactory(context.TODO(), registry.Region, username, password, registry.FIPS)
	if err != nil {
		return "", "", err
	}

	input := &ecr.GetAuthorizationTokenInput{}
	output, err := ecrClient.GetAuthorizationToken(context.TODO(), input)
	if err != nil {
		return "", "", err
	}
	if output == nil {
		return "", "", fmt.Errorf("missing AuthorizationData in response")
	}

	for _, authData := range output.AuthorizationData {
		if authData.ProxyEndpoint != nil && authData.AuthorizationToken != nil {
			token := authData.AuthorizationToken
			decodedToken, err := base64.StdEncoding.DecodeString(*token)
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
