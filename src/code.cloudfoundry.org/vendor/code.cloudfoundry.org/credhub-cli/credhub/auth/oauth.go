package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// OAuth authentication strategy
type OAuthStrategy struct {
	accessToken  string
	refreshToken string

	mu sync.RWMutex // guards AccessToken & Refresh Token

	Username                string
	Password                string
	ClientId                string
	ClientSecret            string
	ApiClient               *http.Client
	OAuthClient             OAuthClient
	ClientCredentialRefresh bool
}

type OAuthClient interface {
	ClientCredentialGrant(clientId, clientSecret string) (string, error)
	PasswordGrant(clientId, clientSecret, username, password string) (string, string, error)
	RefreshTokenGrant(clientId, clientSecret, refreshToken string) (string, string, error)
	RevokeToken(token string) error
}

// Do submits requests with bearer token authorization, using the AccessToken as the bearer token.
//
// Will automatically refresh the AccessToken and retry the request if the token has expired.
func (a *OAuthStrategy) Do(req *http.Request) (*http.Response, error) {
	if err := a.Login(); err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+a.AccessToken())

	clone, err := cloneRequest(req)

	if err != nil {
		return nil, errors.New("failed to clone request body: " + err.Error())
	}

	req.Header.Set("Authorization", "Bearer "+a.AccessToken())
	resp, err := a.ApiClient.Do(req)

	if err != nil {
		return resp, err
	}

	expired, err := tokenExpired(resp)

	if err != nil || !expired {
		return resp, err
	}

	if err := a.Refresh(); err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+a.AccessToken())
	return a.ApiClient.Do(clone)
}

// Refresh will get a new AccessToken
//
// If RefreshToken is available, a refresh token grant will be used, otherwise
// client credential grant will be used.
func (a *OAuthStrategy) Refresh() error {
	refreshToken := a.RefreshToken()

	if refreshToken == "" {
		return a.requestToken()
	}

	var accessToken string
	var err error

	if a.ClientCredentialRefresh {
		accessToken, err = a.OAuthClient.ClientCredentialGrant(a.ClientId, a.ClientSecret)
	} else {
		accessToken, refreshToken, err = a.OAuthClient.RefreshTokenGrant(a.ClientId, a.ClientSecret, refreshToken)
	}

	if err != nil {
		if strings.Contains(err.Error(), "invalid_token") {
			return errors.New("You are not currently authenticated. Please log in to continue.")
		}
		return err
	}

	a.SetTokens(accessToken, refreshToken)

	return nil
}

// Logout will send a revoke token request
//
// On success, the AccessToken and RefreshToken will be empty
func (a *OAuthStrategy) Logout() error {
	accessToken := a.AccessToken()

	if accessToken == "" {
		return nil
	}

	if err := a.OAuthClient.RevokeToken(a.AccessToken()); err != nil {
		return err
	}

	a.SetTokens("", "")

	return nil
}

// Login will make a token grant request to the OAuth server
//
// The grant type will be password grant if Username is not empty, and client
// credentials grant otherwise.
//
// On success, the AccessToken and RefreshToken (if given) will be populated.
//
// Login will be a no-op if the AccessToken is not empty when invoked.
func (a *OAuthStrategy) Login() error {
	if a.AccessToken() != "" && a.AccessToken() != "revoked" {
		return nil
	}

	return a.requestToken()
}

func (a *OAuthStrategy) requestToken() error {
	var accessToken string
	var refreshToken string
	var err error

	if a.ClientCredentialRefresh {
		accessToken, err = a.OAuthClient.ClientCredentialGrant(a.ClientId, a.ClientSecret)
	} else {
		accessToken, refreshToken, err = a.OAuthClient.PasswordGrant(a.ClientId, a.ClientSecret, a.Username, a.Password)
	}

	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Error getting token. Your token may have expired and could not be refreshed. Please try logging in again. [%s]", err.Error()))
	}

	a.SetTokens(accessToken, refreshToken)

	return nil
}

// AccessToken is the Bearer token to be used for authenticated requests
func (a *OAuthStrategy) AccessToken() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.accessToken
}

// RefreshToken is used to by Refresh() to get a new AccessToken.
// Only applies for password grants.
func (a *OAuthStrategy) RefreshToken() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.refreshToken
}

// SetToken sets the AccessToken and RefreshTokens
func (a *OAuthStrategy) SetTokens(access, refresh string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.accessToken = access
	a.refreshToken = refresh
}

func tokenExpired(resp *http.Response) (bool, error) {
	if resp.StatusCode < 400 {
		return false, nil
	}

	var errResp map[string]string
	buf, err := io.ReadAll(resp.Body)

	if err != nil {
		return false, err
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(buf))

	decoder := json.NewDecoder(bytes.NewBuffer(buf))
	err = decoder.Decode(&errResp)

	if err != nil {
		// Since we fail to decode the error response
		// we cannot ensure that the token is invalid
		return false, nil
	}

	return errResp["error"] == "access_token_expired", nil
}

func cloneRequest(r *http.Request) (*http.Request, error) {
	if r.Body == nil {
		return r, nil
	}

	r2 := new(http.Request)
	*r2 = *r

	// deep copy the body
	buf, err := io.ReadAll(r.Body)

	if err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewBuffer(buf))
	r2.Body = io.NopCloser(bytes.NewBuffer(buf))

	return r2, nil
}

var _ Strategy = new(OAuthStrategy)
