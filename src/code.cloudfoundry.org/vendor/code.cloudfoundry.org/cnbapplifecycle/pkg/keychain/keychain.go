package keychain

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
)

const CnbCredentialsEnv = "CNB_REGISTRY_CREDS"

type auth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

func (a auth) config() (authn.AuthConfig, error) {
	if !((a.Username != "" && a.Password != "" && a.Token == "") || (a.Token != "" && a.Username == "" && a.Password == "")) {
		return authn.AuthConfig{}, errors.New("invalid credential combination")
	}

	if a.Token != "" {
		return authn.AuthConfig{
			RegistryToken: a.Token,
		}, nil
	}

	return authn.AuthConfig{
		Username: a.Username,
		Password: a.Password,
	}, nil
}

type envKeyChain struct {
	credentials map[string]auth
}

func FromEnv() (authn.Keychain, error) {
	value, ok := os.LookupEnv(CnbCredentialsEnv)
	if !ok {
		return authn.DefaultKeychain, nil
	}

	e := &envKeyChain{}
	if err := json.Unmarshal([]byte(value), &e.credentials); err != nil {
		return nil, err
	}

	return authn.NewMultiKeychain(e, authn.DefaultKeychain), nil
}

func (e *envKeyChain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	creds, ok := e.credentials[resource.RegistryStr()]
	if !ok {
		return authn.Anonymous, nil
	}

	config, err := creds.config()
	if err != nil {
		return nil, err
	}

	return authn.FromConfig(config), nil
}
