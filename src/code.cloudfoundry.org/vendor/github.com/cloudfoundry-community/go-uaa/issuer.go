package uaa

import (
	"net/http"
)

type OpenIDConfig struct {
	Issuer string `json:"issuer"`
}

// Issuer retrieves an issuer name from openid configuration
func (a *API) Issuer() (string, error) {
	url := urlWithPath(*a.TargetURL, "/.well-known/openid-configuration")

	config := &OpenIDConfig{}
	err := a.doJSON(http.MethodGet, &url, nil, config, false)
	if err != nil {
		return "", err
	}
	return config.Issuer, nil
}
