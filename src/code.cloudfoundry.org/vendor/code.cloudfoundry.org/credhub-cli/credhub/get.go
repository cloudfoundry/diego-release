package credhub

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"strconv"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
)

// GetById returns a credential version by ID. The returned credential will be encoded as a map and may be of any type.
func (ch *CredHub) GetById(id string) (credentials.Credential, error) {
	var cred credentials.Credential

	err := ch.makeCredentialGetByIdRequest(id, &cred)

	return cred, err
}

// GetAllVersions returns all credential versions for a given credential name. The returned credentials will be encoded as a list of maps and may be of any type.
func (ch *CredHub) GetAllVersions(name string) ([]credentials.Credential, error) {
	query := url.Values{}
	query.Set("name", name)

	return ch.makeMultiCredentialGetRequest(query)
}

// GetLatestVersion returns the current credential version for a given credential name. The returned credential will be encoded as a map and may be of any type.
func (ch *CredHub) GetLatestVersion(name string) (credentials.Credential, error) {
	var cred credentials.Credential
	err := ch.getCurrentCredential(name, &cred)
	return cred, err
}

// GetNVersions returns the N most recent credential versions for a given credential name. The returned credentials will be encoded as a list of maps and may be of any type.
func (ch *CredHub) GetNVersions(name string, numberOfVersions int) ([]credentials.Credential, error) {
	creds, err := ch.getNVersionsOfCredential(name, numberOfVersions)
	return creds, err
}

// GetLatestValue returns the current credential version for a given credential name. The returned credential will be encoded as a map and must be of type 'value'.
func (ch *CredHub) GetLatestValue(name string) (credentials.Value, error) {
	var cred credentials.Value
	err := ch.getCurrentCredential(name, &cred)

	return cred, err
}

// GetLatestJSON returns the current credential version for a given credential name. The returned credential will be encoded as a map and must be of type 'json'.
func (ch *CredHub) GetLatestJSON(name string) (credentials.JSON, error) {
	var cred credentials.JSON
	err := ch.getCurrentCredential(name, &cred)

	return cred, err
}

// GetLatestPassword returns the current credential version for a given credential name. The returned credential will be encoded as a map and must be of type 'password'.
func (ch *CredHub) GetLatestPassword(name string) (credentials.Password, error) {
	var cred credentials.Password
	err := ch.getCurrentCredential(name, &cred)

	return cred, err
}

// GetLatestUser returns the current credential version for a given credential name. The returned credential will be encoded as a map and must be of type 'user'.
func (ch *CredHub) GetLatestUser(name string) (credentials.User, error) {
	var cred credentials.User
	err := ch.getCurrentCredential(name, &cred)

	return cred, err
}

// GetLatestCertificate returns the current credential version for a given credential name. The returned credential will be encoded as a map and must be of type 'certificate'.
func (ch *CredHub) GetLatestCertificate(name string) (credentials.Certificate, error) {
	var cred credentials.Certificate
	err := ch.getCurrentCredential(name, &cred)

	return cred, err
}

// GetLatestRSA returns the current credential version for a given credential name. The returned credential will be encoded as a map and must be of type 'rsa'.
func (ch *CredHub) GetLatestRSA(name string) (credentials.RSA, error) {
	var cred credentials.RSA
	err := ch.getCurrentCredential(name, &cred)

	return cred, err
}

// GetLatestSSH returns the current credential version for a given credential name. The returned credential will be encoded as a map and must be of type 'ssh'.
func (ch *CredHub) GetLatestSSH(name string) (credentials.SSH, error) {
	var cred credentials.SSH
	err := ch.getCurrentCredential(name, &cred)

	return cred, err
}

func (ch *CredHub) getCurrentCredential(name string, cred interface{}) error {
	query := url.Values{}

	query.Set("current", "true")
	query.Set("name", name)

	return ch.makeCredentialGetRequest(query, cred)
}

func (ch *CredHub) makeCredentialGetRequest(query url.Values, cred interface{}) error {
	resp, err := ch.Request(http.MethodGet, "/api/v1/data", query, nil, true)

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)
	dec := json.NewDecoder(resp.Body)

	response := make(map[string][]json.RawMessage)

	if err := dec.Decode(&response); err != nil {
		return errors.New("The response body could not be decoded: " + err.Error())
	}

	var ok bool
	var data []json.RawMessage

	if data, ok = response["data"]; !ok || len(data) == 0 {
		return newCredhubError("response did not contain any credentials", "")
	}

	rawMessage := data[0]

	return json.Unmarshal(rawMessage, cred)
}

func (ch *CredHub) makeCredentialGetByIdRequest(id string, cred *credentials.Credential) error {
	resp, err := ch.Request(http.MethodGet, "/api/v1/data/"+id, nil, nil, true)

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)
	dec := json.NewDecoder(resp.Body)

	if err := dec.Decode(cred); err != nil {
		return errors.New("The response body could not be decoded: " + err.Error())
	}

	return nil
}

func (ch *CredHub) getNVersionsOfCredential(name string, numberOfVersions int) ([]credentials.Credential, error) {
	query := url.Values{}
	query.Set("name", name)
	query.Set("versions", strconv.Itoa(numberOfVersions))

	return ch.makeMultiCredentialGetRequest(query)
}

func (ch *CredHub) makeMultiCredentialGetRequest(query url.Values) ([]credentials.Credential, error) {
	resp, err := ch.Request(http.MethodGet, "/api/v1/data", query, nil, true)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)
	dec := json.NewDecoder(resp.Body)

	response := make(map[string][]credentials.Credential)

	if err := dec.Decode(&response); err != nil {
		return nil, errors.New("The response body could not be decoded: " + err.Error())
	}

	var ok bool
	var data []credentials.Credential

	if data, ok = response["data"]; !ok || len(data) == 0 {
		return nil, newCredhubError("response did not contain any credentials", "")
	}

	return data, nil
}
