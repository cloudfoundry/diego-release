package uaa

import (
	"net/http"
)

// JWK represents a JSON Web Key (https://tools.ietf.org/html/rfc7517).
type JWK struct {
	Kty   string `json:"kty"`
	E     string `json:"e,omitempty"`
	Use   string `json:"use"`
	Kid   string `json:"kid"`
	Alg   string `json:"alg"`
	Value string `json:"value"`
	N     string `json:"n,omitempty"`
}

// TokenKey retrieves a JWK from the token_key endpoint
// (http://docs.cloudfoundry.org/api/uaa/version/4.14.0/index.html#token-key-s).
func (a *API) TokenKey() (*JWK, error) {
	url := urlWithPath(*a.TargetURL, "/token_key")

	key := &JWK{}
	err := a.doJSON(http.MethodGet, &url, nil, key, false)
	if err != nil {
		return nil, err
	}
	return key, err
}
