package credhub

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/hashicorp/go-version"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
)

// Option can be provided to New() to specify additional parameters for
// connecting to the CredHub server
type SetOption func(*SetOptions) error

// SetValue sets a value credential with a user-provided value.
func (ch *CredHub) SetValue(name string, value values.Value, options ...SetOption) (credentials.Value, error) {
	var cred credentials.Value
	err := ch.setCredential(name, "value", value, &cred, options...)

	return cred, err
}

// SetJSON sets a JSON credential with a user-provided value.
func (ch *CredHub) SetJSON(name string, value values.JSON, options ...SetOption) (credentials.JSON, error) {
	var cred credentials.JSON
	err := ch.setCredential(name, "json", value, &cred, options...)

	return cred, err
}

// SetPassword sets a password credential with a user-provided value.
func (ch *CredHub) SetPassword(name string, value values.Password, options ...SetOption) (credentials.Password, error) {
	var cred credentials.Password
	err := ch.setCredential(name, "password", value, &cred, options...)

	return cred, err
}

// SetUser sets a user credential with a user-provided value.
func (ch *CredHub) SetUser(name string, value values.User, options ...SetOption) (credentials.User, error) {
	var cred credentials.User
	err := ch.setCredential(name, "user", value, &cred, options...)

	return cred, err
}

// SetCertificate sets a certificate credential with a user-provided value.
func (ch *CredHub) SetCertificate(name string, value values.Certificate, options ...SetOption) (credentials.Certificate, error) {
	var cred credentials.Certificate
	err := ch.setCredential(name, "certificate", value, &cred, options...)

	return cred, err
}

// SetRSA sets an RSA credential with a user-provided value.
func (ch *CredHub) SetRSA(name string, value values.RSA, options ...SetOption) (credentials.RSA, error) {
	var cred credentials.RSA
	err := ch.setCredential(name, "rsa", value, &cred, options...)

	return cred, err
}

// SetSSH sets an SSH credential with a user-provided value.
func (ch *CredHub) SetSSH(name string, value values.SSH, options ...SetOption) (credentials.SSH, error) {
	var cred credentials.SSH
	err := ch.setCredential(name, "ssh", value, &cred, options...)

	return cred, err
}

// SetCredential sets a credential of any type with a user-provided value.
func (ch *CredHub) SetCredential(name, credType string, value interface{}, options ...SetOption) (credentials.Credential, error) {
	var cred credentials.Credential
	err := ch.setCredential(name, credType, value, &cred, options...)

	return cred, err
}

type setRequest struct {
	Name  string      `json:"name"`
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
	Mode  string      `json:"mode,omitempty"`
	SetOptions
}

type SetOptions struct {
	Metadata credentials.Metadata `json:"metadata,omitempty"`
}

func (ch *CredHub) setCredential(name, credType string, value, cred interface{}, options ...SetOption) error {
	request := &setRequest{
		Name:  name,
		Type:  credType,
		Value: value,
	}

	serverVersion, err := ch.ServerVersion()
	if err != nil {
		return err
	}
	if serverVersion.Segments()[0] < 2 {
		request.Mode = "overwrite"
	}

	for _, option := range options {
		if err := option(&request.SetOptions); err != nil {
			return err
		}
	}

	if request.Metadata != nil && !supportsMetadata(serverVersion) {
		return ServerDoesNotSupportMetadataError
	}

	resp, err := ch.Request(http.MethodPut, "/api/v1/data", nil, request, true)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	return json.NewDecoder(resp.Body).Decode(cred)
}

func supportsMetadata(v *version.Version) bool {
	supportedVersion := version.Must(version.NewVersion("2.6.0"))
	return v.GreaterThan(supportedVersion) || v.Equal(supportedVersion)
}
