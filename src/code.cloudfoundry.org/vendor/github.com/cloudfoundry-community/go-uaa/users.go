package uaa

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// UsersEndpoint is the path to the users resource.
const UsersEndpoint string = "/Users"

// Meta describes the version and timestamps for a resource.
type Meta struct {
	Version      int    `json:"version,omitempty"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
}

// UserName is a person's name.
type UserName struct {
	FamilyName string `json:"familyName,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
}

// Email is an email address.
type Email struct {
	Value   string `json:"value,omitempty"`
	Primary *bool  `json:"primary,omitempty"`
}

// UserGroup is a group that a user belongs to.
type UserGroup struct {
	Value   string `json:"value,omitempty"`
	Display string `json:"display,omitempty"`
	Type    string `json:"type,omitempty"`
}

// Approval is a record of the user's explicit approval or rejection for an
// application's request for delegated permissions.
type Approval struct {
	UserID        string `json:"userId,omitempty"`
	ClientID      string `json:"clientId,omitempty"`
	Scope         string `json:"scope,omitempty"`
	Status        string `json:"status,omitempty"`
	LastUpdatedAt string `json:"lastUpdatedAt,omitempty"`
	ExpiresAt     string `json:"expiresAt,omitempty"`
}

// PhoneNumber is a phone number for a user.
type PhoneNumber struct {
	Value string `json:"value"`
}

// User is a UAA user
// http://docs.cloudfoundry.org/api/uaa/version/4.14.0/index.html#get-3.
type User struct {
	ID                   string        `json:"id,omitempty"`
	Password             string        `json:"password,omitempty"`
	ExternalID           string        `json:"externalId,omitempty"`
	Meta                 *Meta         `json:"meta,omitempty"`
	Username             string        `json:"userName,omitempty"`
	Name                 *UserName     `json:"name,omitempty"`
	Emails               []Email       `json:"emails,omitempty"`
	Groups               []UserGroup   `json:"groups,omitempty"`
	Approvals            []Approval    `json:"approvals,omitempty"`
	PhoneNumbers         []PhoneNumber `json:"phoneNumbers,omitempty"`
	Active               *bool         `json:"active,omitempty"`
	Verified             *bool         `json:"verified,omitempty"`
	Origin               string        `json:"origin,omitempty"`
	ZoneID               string        `json:"zoneId,omitempty"`
	PasswordLastModified string        `json:"passwordLastModified,omitempty"`
	PreviousLogonTime    int           `json:"previousLogonTime,omitempty"`
	LastLogonTime        int           `json:"lastLogonTime,omitempty"`
	Schemas              []string      `json:"schemas,omitempty"`
}

// Identifier returns the field used to uniquely identify a User.
func (u User) Identifier() string {
	return u.ID
}

// paginatedUserList is the response from the API for a single page of users.
type paginatedUserList struct {
	Page
	Resources []User   `json:"resources"`
	Schemas   []string `json:"schemas"`
}

// GetUserByUsername gets the user with the given username
// http://docs.cloudfoundry.org/api/uaa/version/4.14.0/index.html#list-with-attribute-filtering.
func (a *API) GetUserByUsername(username, origin, attributes string) (*User, error) {
	if username == "" {
		return nil, errors.New("username cannot be blank")
	}

	filter := fmt.Sprintf(`userName eq "%v"`, username)
	help := fmt.Sprintf("user %v not found", username)

	if origin != "" {
		filter = fmt.Sprintf(`%s and origin eq "%v"`, filter, origin)
		help = fmt.Sprintf(`%s in origin %v`, help, origin)
	}

	users, err := a.ListAllUsers(filter, "", attributes, "")
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, errors.New(help)
	}
	if len(users) > 1 && origin == "" {
		var foundOrigins []string
		for _, user := range users {
			foundOrigins = append(foundOrigins, user.Origin)
		}

		msgTmpl := "Found users with username %v in multiple origins %v."
		msg := fmt.Sprintf(msgTmpl, username, "["+strings.Join(foundOrigins, ", ")+"]")
		return nil, errors.New(msg)
	}
	return &users[0], nil
}

// DeactivateUser deactivates the user with the given user ID
// http://docs.cloudfoundry.org/api/uaa/version/4.14.0/index.html#patch.
func (a *API) DeactivateUser(userID string, userMetaVersion int) error {
	return a.setActive(false, userID, userMetaVersion)
}

// ActivateUser activates the user with the given user ID
// http://docs.cloudfoundry.org/api/uaa/version/4.14.0/index.html#patch.
func (a *API) ActivateUser(userID string, userMetaVersion int) error {
	return a.setActive(true, userID, userMetaVersion)
}

func (a *API) setActive(active bool, userID string, userMetaVersion int) error {
	if userID == "" {
		return errors.New("userID cannot be blank")
	}
	u := urlWithPath(*a.TargetURL, fmt.Sprintf("%s/%s", UsersEndpoint, userID))
	user := &User{}
	user.Active = &active

	extraHeaders := map[string]string{"If-Match": strconv.Itoa(userMetaVersion)}
	j, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return a.doJSONWithHeaders(http.MethodPatch, &u, extraHeaders, bytes.NewBuffer([]byte(j)), nil, true)
}
