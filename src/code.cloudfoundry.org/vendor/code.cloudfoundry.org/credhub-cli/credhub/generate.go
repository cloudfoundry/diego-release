package credhub

import (
	"encoding/json"
	"io"
	"net/http"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/generate"
)

type GenerateOption func(*GenerateOptions) error

// GeneratePassword generates a password credential based on the provided parameters.
func (ch *CredHub) GeneratePassword(name string, gen generate.Password, overwrite Mode) (credentials.Password, error) {
	var cred credentials.Password
	err := ch.generateCredential(name, "password", gen, overwrite, &cred)
	return cred, err
}

// GenerateUser generates a user credential based on the provided parameters.
func (ch *CredHub) GenerateUser(name string, gen generate.User, overwrite Mode) (credentials.User, error) {
	var cred credentials.User
	err := ch.generateCredential(name, "user", gen, overwrite, &cred)
	return cred, err
}

// GenerateCertificate generates a certificate credential based on the provided parameters.
func (ch *CredHub) GenerateCertificate(name string, gen generate.Certificate, overwrite Mode) (credentials.Certificate, error) {
	var cred credentials.Certificate
	err := ch.generateCredential(name, "certificate", gen, overwrite, &cred)
	return cred, err
}

// GenerateRSA generates an RSA credential based on the provided parameters.
func (ch *CredHub) GenerateRSA(name string, gen generate.RSA, overwrite Mode) (credentials.RSA, error) {
	var cred credentials.RSA
	err := ch.generateCredential(name, "rsa", gen, overwrite, &cred)
	return cred, err
}

// GenerateSSH generates an SSH credential based on the provided parameters.
func (ch *CredHub) GenerateSSH(name string, gen generate.SSH, overwrite Mode) (credentials.SSH, error) {
	var cred credentials.SSH
	err := ch.generateCredential(name, "ssh", gen, overwrite, &cred)
	return cred, err
}

// GenerateCredential generates any credential type based on the credType given provided parameters.
func (ch *CredHub) GenerateCredential(name, credType string, gen interface{}, overwrite Mode, options ...GenerateOption) (credentials.Credential, error) {
	var cred credentials.Credential
	err := ch.generateCredential(name, credType, gen, overwrite, &cred, options...)
	return cred, err
}

type generateRequest struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Value      interface{} `json:"value,omitempty"`
	Parameters interface{} `json:"parameters"`
	Mode       string      `json:"mode,omitempty"`
	Overwrite  bool        `json:"overwrite"`
	GenerateOptions
}

type GenerateOptions struct {
	Metadata credentials.Metadata `json:"metadata,omitempty"`
}

func (ch *CredHub) generateCredential(name, credType string, gen interface{}, overwrite Mode, cred interface{}, options ...GenerateOption) error {
	isOverwrite := overwrite == Overwrite

	request := generateRequest{
		Name:       name,
		Type:       credType,
		Parameters: gen,
	}

	if overwrite == Converge {
		request.Mode = string(overwrite)
	} else {
		request.Overwrite = isOverwrite
	}

	if user, ok := gen.(generate.User); ok {
		request.Value = map[string]string{"username": user.Username}
	}

	for _, option := range options {
		if err := option(&request.GenerateOptions); err != nil {
			return err
		}
	}

	serverVersion, err := ch.ServerVersion()
	if err != nil {
		return err
	}

	if request.Metadata != nil && !supportsMetadata(serverVersion) {
		return ServerDoesNotSupportMetadataError
	}

	resp, err := ch.Request(http.MethodPost, "/api/v1/data", nil, request, true)

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)
	dec := json.NewDecoder(resp.Body)

	if err := ch.checkForServerError(resp); err != nil {
		return err
	}
	return dec.Decode(&cred)
}
