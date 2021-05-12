// UAA client for token grants and revocation
package uaa

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// Client makes requests to the UAA server at AuthURL
type Client struct {
	AuthURL string
	Client  *http.Client
}

type token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

type responseError struct {
	Name        string `json:"error"`
	Description string `json:"error_description"`
}

// Metadata captures the data returned by the GET /info on a UAA server
// This fields are not exhaustive and can added to over time.
// See: https://docs.cloudfoundry.org/api/uaa/version/4.6.0/index.html#server-information
type Metadata struct {
	Links struct {
		Login string `json:"login"`
	} `json:"links"`
	Prompts struct {
		Passcode []string `json:"passcode"`
	} `json:"prompts"`
}

// PasscodePrompt returns a prompt to tell the user where to get a passcode from.
// If not present in the metadata (PCF installation don't seem to return it), will attempt to
// contruct a plausible URL.
func (md *Metadata) PasscodePrompt() string {
	// Give default in case server doesn't tell us
	if len(md.Prompts.Passcode) == 2 && md.Prompts.Passcode[1] != "" {
		return md.Prompts.Passcode[1]
	}
	var loginURL string
	if md.Links.Login != "" {
		loginURL = md.Links.Login
	} else {
		loginURL = "https://login.system.example.com"
	}
	return fmt.Sprintf("One Time Code ( Get one at %s/passcode )", loginURL)
}

func (e *responseError) Error() string {
	if e.Description == "" {
		return e.Name
	}

	return fmt.Sprintf("%s %s", e.Name, e.Description)
}

func (u *Client) Metadata() (*Metadata, error) {
	request, err := http.NewRequest("GET", u.AuthURL+"/info", nil)
	if err != nil {
		return nil, err
	}

	request.Header.Add("Accept", "application/json")
	response, err := u.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)

	if response.StatusCode != 200 {
		return nil, errors.New("unable to fetch metadata successfully")
	}

	var rv Metadata
	err = json.NewDecoder(response.Body).Decode(&rv)
	if err != nil {
		return nil, err
	}

	return &rv, nil
}

// ClientCredentialGrant requests a token using client_credentials grant type
func (u *Client) ClientCredentialGrant(clientId, clientSecret string) (string, error) {
	values := url.Values{
		"grant_type":    {"client_credentials"},
		"response_type": {"token"},
		"client_id":     {clientId},
		"client_secret": {clientSecret},
	}

	token, err := u.tokenGrantRequest(values)

	return token.AccessToken, err
}

// PasswordGrant requests an access token and refresh token using password grant type
func (u *Client) PasswordGrant(clientId, clientSecret, username, password string) (string, string, error) {
	values := url.Values{
		"grant_type":    {"password"},
		"response_type": {"token"},
		"username":      {username},
		"password":      {password},
		"client_id":     {clientId},
		"client_secret": {clientSecret},
	}

	token, err := u.tokenGrantRequest(values)

	return token.AccessToken, token.RefreshToken, err
}

// PasscodeGrant requests an access token and refresh token using passcode grant type
func (u *Client) PasscodeGrant(clientId, clientSecret, passcode string) (string, string, error) {
	values := url.Values{
		"grant_type":    {"password"},
		"response_type": {"token"},
		"passcode":      {passcode},
		"client_id":     {clientId},
		"client_secret": {clientSecret},
	}

	token, err := u.tokenGrantRequest(values)

	return token.AccessToken, token.RefreshToken, err
}

// RefreshTokenGrant requests a new access token and refresh token using refresh_token grant type
func (u *Client) RefreshTokenGrant(clientId, clientSecret, refreshToken string) (string, string, error) {
	values := url.Values{
		"grant_type":    {"refresh_token"},
		"response_type": {"token"},
		"client_id":     {clientId},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
	}

	token, err := u.tokenGrantRequest(values)

	return token.AccessToken, token.RefreshToken, err
}

func (u *Client) tokenGrantRequest(headers url.Values) (token, error) {
	var t token

	request, err := http.NewRequest("POST", u.AuthURL+"/oauth/token", bytes.NewBufferString(headers.Encode()))
	if err != nil {
		return t, err
	}

	request.Header.Add("Accept", "application/json")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := u.Client.Do(request)

	if err != nil {
		return t, err
	}

	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)

	decoder := json.NewDecoder(response.Body)

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		err = decoder.Decode(&t)
		return t, err
	}

	respErr := responseError{}

	if err := decoder.Decode(&respErr); err != nil {
		return t, err
	}

	return t, &respErr
}

// RevokeToken revokes the given access token
func (u *Client) RevokeToken(accessToken string) error {
	segments := strings.Split(accessToken, ".")

	if len(segments) < 2 {
		return errors.New("access token missing segments")
	}

	jsonPayload, err := base64.RawURLEncoding.DecodeString(segments[1])

	if err != nil {
		return errors.New("could not base64 decode token payload")
	}

	payload := make(map[string]interface{})
	json.Unmarshal(jsonPayload, &payload)
	jti, ok := payload["jti"].(string)

	if !ok {
		return errors.New("could not parse jti from payload")
	}

	request, err := http.NewRequest(http.MethodDelete, u.AuthURL+"/oauth/token/revoke/"+jti, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := u.Client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("Received HTTP %d error while revoking token from auth server: %q", resp.StatusCode, body)
	}

	return nil
}
