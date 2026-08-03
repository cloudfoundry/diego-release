package uaa

import (
	"net/http"
)

// UserInfo is a protected resource required for OpenID Connect compatibility.
// The response format is defined here: https://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse.
type UserInfo struct {
	UserID            string `json:"user_id"`
	Sub               string `json:"sub"`
	Username          string `json:"user_name"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Email             string `json:"email"`
	PhoneNumber       string `json:"phone_number"`
	PreviousLoginTime int64  `json:"previous_logon_time"`
	Name              string `json:"name"`
}

// GetMe retrieves the UserInfo for the current user.
func (a *API) GetMe() (*UserInfo, error) {
	u := urlWithPath(*a.TargetURL, "/userinfo")
	u.RawQuery = "scheme=openid"

	info := &UserInfo{}
	err := a.doJSON(http.MethodGet, &u, nil, info, true)
	if err != nil {
		return nil, err
	}
	return info, nil
}
