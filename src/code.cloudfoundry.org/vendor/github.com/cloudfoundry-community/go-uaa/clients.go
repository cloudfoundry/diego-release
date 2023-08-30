package uaa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ClientsEndpoint is the path to the clients resource.
const ClientsEndpoint string = "/oauth/clients"

// paginatedClientList is the response from the API for a single page of clients.
type paginatedClientList struct {
	Page
	Resources []Client `json:"resources"`
	Schemas   []string `json:"schemas"`
}

// Client is a UAA client
// http://docs.cloudfoundry.org/api/uaa/version/4.19.0/index.html#clients.
type Client struct {
	ClientID             string      `json:"client_id,omitempty" generator:"id"`
	AuthorizedGrantTypes []string    `json:"authorized_grant_types,omitempty"`
	RedirectURI          []string    `json:"redirect_uri,omitempty"`
	Scope                []string    `json:"scope,omitempty"`
	ResourceIDs          []string    `json:"resource_ids,omitempty"`
	Authorities          []string    `json:"authorities,omitempty"`
	AutoApproveRaw       interface{} `json:"autoapprove,omitempty"`
	AccessTokenValidity  int64       `json:"access_token_validity,omitempty"`
	RefreshTokenValidity int64       `json:"refresh_token_validity,omitempty"`
	AllowedProviders     []string    `json:"allowedproviders,omitempty"`
	DisplayName          string      `json:"name,omitempty"`
	TokenSalt            string      `json:"token_salt,omitempty"`
	CreatedWith          string      `json:"createdwith,omitempty"`
	ApprovalsDeleted     bool        `json:"approvals_deleted,omitempty"`
	RequiredUserGroups   []string    `json:"required_user_groups,omitempty"`
	ClientSecret         string      `json:"client_secret,omitempty"`
	LastModified         int64       `json:"lastModified,omitempty"`
	AllowPublic          bool        `json:"allowpublic,omitempty"`
}

// Identifier returns the field used to uniquely identify a Client.
func (c Client) Identifier() string {
	return c.ClientID
}

func (c Client) AutoApprove() []string {
	switch t := c.AutoApproveRaw.(type) {
	case bool:
		return []string{strconv.FormatBool(t)}
	case string:
		return []string{t}
	case []string:
		return t
	}
	return []string{}
}

// GrantType is a type of oauth2 grant.
type GrantType string

// Valid GrantType values.
const (
	REFRESHTOKEN      = GrantType("refresh_token")
	AUTHCODE          = GrantType("authorization_code")
	IMPLICIT          = GrantType("implicit")
	PASSWORD          = GrantType("password")
	CLIENTCREDENTIALS = GrantType("client_credentials")
)

func errorMissingValueForGrantType(value string, grantType GrantType) error {
	return fmt.Errorf("%v must be specified for %v grant type", value, grantType)
}

func errorMissingValue(value string) error {
	return fmt.Errorf("%v must be specified in the client definition", value)
}

func requireRedirectURIForGrantType(c *Client, grantType GrantType) error {
	if contains(c.AuthorizedGrantTypes, string(grantType)) {
		if len(c.RedirectURI) == 0 {
			return errorMissingValueForGrantType("redirect_uri", grantType)
		}
	}
	return nil
}

func requireClientSecretForGrantType(c *Client, grantType GrantType) error {
	if contains(c.AuthorizedGrantTypes, string(grantType)) {
		if c.ClientSecret == "" {
			return errorMissingValueForGrantType("client_secret", grantType)
		}
	}
	return nil
}

func knownGrantTypesStr() string {
	grantTypeStrings := []string{}
	knownGrantTypes := []GrantType{AUTHCODE, IMPLICIT, PASSWORD, CLIENTCREDENTIALS}
	for _, grant := range knownGrantTypes {
		grantTypeStrings = append(grantTypeStrings, string(grant))
	}

	return "[" + strings.Join(grantTypeStrings, ", ") + "]"
}

// Validate returns nil if the client is valid, or an error if it is invalid.
func (c *Client) Validate() error {
	if len(c.AuthorizedGrantTypes) == 0 {
		return fmt.Errorf("grant type must be one of %v", knownGrantTypesStr())
	}

	if c.ClientID == "" {
		return errorMissingValue("client_id")
	}

	if err := requireRedirectURIForGrantType(c, AUTHCODE); err != nil {
		return err
	}
	if err := requireClientSecretForGrantType(c, AUTHCODE); err != nil {
		return err
	}

	if err := requireClientSecretForGrantType(c, CLIENTCREDENTIALS); err != nil {
		return err
	}

	if err := requireRedirectURIForGrantType(c, IMPLICIT); err != nil {
		return err
	}

	return nil
}

type changeSecretBody struct {
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"secret,omitempty"`
}

// ChangeClientSecret updates the secret with the given value for the client
// with the given id
// http://docs.cloudfoundry.org/api/uaa/version/4.14.0/index.html#change-secret.
func (a *API) ChangeClientSecret(id string, newSecret string) error {
	u := urlWithPath(*a.TargetURL, fmt.Sprintf("%s/%s/secret", ClientsEndpoint, id))
	change := &changeSecretBody{ClientID: id, ClientSecret: newSecret}
	j, err := json.Marshal(change)
	if err != nil {
		return err
	}
	err = a.doJSON(http.MethodPut, &u, bytes.NewBuffer([]byte(j)), nil, true)
	if err != nil {
		return err
	}
	return nil
}
