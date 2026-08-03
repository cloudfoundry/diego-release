package auth

import (
	"net/http"

	"code.cloudfoundry.org/credhub-cli/credhub/auth/uaa"
)

// Config provides the CredHub configuration necessary to build an auth Strategy
//
// The credhub.CredHub struct conforms to this interface
type Config interface {
	AuthURL() (string, error)
	Client() *http.Client
}

// Builder constructs the auth type given a configuration
//
// A builder is required by the credhub.Auth() option for credhub.New()
type Builder func(config Config) (Strategy, error)

// Noop builds a NoopStrategy
var Noop Builder = func(config Config) (Strategy, error) {
	return &NoopStrategy{config.Client()}, nil
}

// UaaPassword builds an OauthStrategy for UAA using password_grant token requests
func UaaPassword(clientId, clientSecret, username, password string) Builder {
	return Uaa(clientId, clientSecret, username, password, "", "", false)
}

// UaaClientCredential builds an OauthStrategy for UAA using client_credential_grant token requests
func UaaClientCredentials(clientId, clientSecret string) Builder {
	return Uaa(clientId, clientSecret, "", "", "", "", true)
}

// Uaa builds an OauthStrategy for a UAA using existing tokens
func Uaa(clientId, clientSecret, username, password, accessToken, refreshToken string, usingClientCrendentials bool) Builder {
	return func(config Config) (Strategy, error) {
		httpClient := config.Client()
		authUrl, err := config.AuthURL()

		if err != nil {
			return nil, err
		}

		uaaClient := uaa.Client{
			AuthURL: authUrl,
			Client:  httpClient,
		}

		oauth := &OAuthStrategy{
			Username:                username,
			Password:                password,
			ClientId:                clientId,
			ClientSecret:            clientSecret,
			ApiClient:               httpClient,
			OAuthClient:             &uaaClient,
			ClientCredentialRefresh: usingClientCrendentials,
		}

		oauth.SetTokens(accessToken, refreshToken)

		return oauth, nil
	}
}
