package uaa

import (
	"net/http"
)

// Keys is a slice of JSON Web Keys.
type Keys struct {
	Keys []JWK `json:"keys"`
}

// TokenKeys gets the JSON Web Token signing keys for the UAA server.
func (a *API) TokenKeys() ([]JWK, error) {
	url := urlWithPath(*a.TargetURL, "/token_keys")
	keys := &Keys{}
	err := a.doJSON(http.MethodGet, &url, nil, keys, false)
	if err != nil {
		key, e := a.TokenKey()
		if e != nil {
			return nil, e
		}
		return []JWK{*key}, nil
	}
	return keys.Keys, err
}
