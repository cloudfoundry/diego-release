package uaa

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// GroupsEndpoint is the path to the groups resource.
const GroupsEndpoint string = "/Groups"

// paginatedGroupList is the response from the API for a single page of groups.
type paginatedGroupList struct {
	Page
	Resources []Group  `json:"resources"`
	Schemas   []string `json:"schemas"`
}

// GroupMember is a user or a group.
type GroupMember struct {
	Origin string `json:"origin,omitempty"`
	Type   string `json:"type,omitempty"`
	Value  string `json:"value,omitempty"`
}

// Group is a container for users and groups.
type Group struct {
	ID          string        `json:"id,omitempty"`
	Meta        *Meta         `json:"meta,omitempty"`
	DisplayName string        `json:"displayName,omitempty"`
	ZoneID      string        `json:"zoneId,omitempty"`
	Description string        `json:"description,omitempty"`
	Members     []GroupMember `json:"members,omitempty"`
	Schemas     []string      `json:"schemas,omitempty"`
}

// paginatedGroupMappingList is the response from the API for a single page of group mappings.
type paginatedGroupMappingList struct {
	Page
	Resources []GroupMapping `json:"resources"`
	Schemas   []string       `json:"schemas"`
}

// GroupMapping is a container for external group mapping
type GroupMapping struct {
	GroupID       string   `json:"groupId,omitempty"`
	DisplayName   string   `json:"displayName,omitempty"`
	ExternalGroup string   `json:"externalGroup,omitempty"`
	Origin        string   `json:"origin,omitempty"`
	Meta          *Meta    `json:"meta,omitempty"`
	Schemas       []string `json:"schemas,omitempty"`
}

// Identifier returns the field used to uniquely identify a Group.
func (g Group) Identifier() string {
	return g.ID
}

// AddGroupMember adds the entity with the given memberID to the group with the
// given ID. If no entityType is supplied, the entityType (which can be "USER"
// or "GROUP") will be "USER". If no origin is supplied, the origin will be
// "uaa".
func (a *API) AddGroupMember(groupID string, memberID string, entityType string, origin string) error {
	u := urlWithPath(*a.TargetURL, fmt.Sprintf("%s/%s/members", GroupsEndpoint, groupID))
	if origin == "" {
		origin = "uaa"
	}
	if entityType == "" {
		entityType = "USER"
	}
	membership := GroupMember{Origin: origin, Type: entityType, Value: memberID}
	j, err := json.Marshal(membership)
	if err != nil {
		return err
	}
	err = a.doJSON(http.MethodPost, &u, bytes.NewBuffer([]byte(j)), nil, true)
	if err != nil {
		return err
	}
	return nil
}

// RemoveGroupMember removes the entity with the given memberID from the group
// with the given ID. If no entityType is supplied, the entityType (which can be
// "USER" or "GROUP") will be "USER". If no origin is supplied, the origin will
// be "uaa".
func (a *API) RemoveGroupMember(groupID string, memberID string, entityType string, origin string) error {
	u := urlWithPath(*a.TargetURL, fmt.Sprintf("%s/%s/members/%s", GroupsEndpoint, groupID, memberID))
	if origin == "" {
		origin = "uaa"
	}
	if entityType == "" {
		entityType = "USER"
	}
	membership := GroupMember{Origin: origin, Type: entityType, Value: memberID}
	j, err := json.Marshal(membership)
	if err != nil {
		return err
	}
	err = a.doJSON(http.MethodDelete, &u, bytes.NewBuffer([]byte(j)), nil, true)
	if err != nil {
		return err
	}
	return nil
}

// GetGroupByName gets the group with the given name
// http://docs.cloudfoundry.org/api/uaa/version/4.14.0/index.html#list-4.
func (a *API) GetGroupByName(name string, attributes string) (*Group, error) {
	if name == "" {
		return nil, errors.New("group name may not be blank")
	}

	filter := fmt.Sprintf(`displayName eq "%v"`, name)
	groups, err := a.ListAllGroups(filter, "", attributes, "")
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("group %v not found", name)
	}
	return &groups[0], nil
}

func (a *API) MapGroup(groupID string, externalGroup string, origin string) error {
	u := urlWithPath(*a.TargetURL, fmt.Sprintf("%s/External", GroupsEndpoint))
	if origin == "" {
		origin = "ldap"
	}
	mapped := &GroupMapping{}
	mapping := GroupMapping{Origin: origin, GroupID: groupID, ExternalGroup: externalGroup}
	j, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	err = a.doJSON(http.MethodPost, &u, bytes.NewBuffer([]byte(j)), mapped, true)
	if err != nil {
		return err
	}
	return nil
}

func (a *API) UnmapGroup(groupID string, externalGroup string, origin string) error {
	if origin == "" {
		origin = "ldap"
	}
	u := urlWithPath(*a.TargetURL, fmt.Sprintf("%s/External/groupId/%s/externalGroup/%s/origin/%s", GroupsEndpoint, groupID, externalGroup, origin))
	mapped := &GroupMapping{}
	err := a.doJSON(http.MethodDelete, &u, nil, mapped, true)
	if err != nil {
		return err
	}
	return nil
}

func (a *API) ListGroupMappings(origin string, startIndex int, itemsPerPage int) ([]GroupMapping, Page, error) {
	u := urlWithPath(*a.TargetURL, fmt.Sprintf("%s/External", GroupsEndpoint))
	query := url.Values{}
	if origin != "" {
		query.Set("origin", origin)
	}
	if startIndex == 0 {
		startIndex = 1
	}
	query.Set("startIndex", strconv.Itoa(startIndex))
	if itemsPerPage == 0 {
		itemsPerPage = 100
	}
	query.Set("count", strconv.Itoa(itemsPerPage))
	u.RawQuery = query.Encode()

	mappings := &paginatedGroupMappingList{}
	err := a.doJSON(http.MethodGet, &u, nil, mappings, true)
	if err != nil {
		return nil, Page{}, err
	}
	page := Page{
		StartIndex:   mappings.StartIndex,
		ItemsPerPage: mappings.ItemsPerPage,
		TotalResults: mappings.TotalResults,
	}
	return mappings.Resources, page, err
}

// ListAllGroups retrieves UAA groups
func (a *API) ListAllGroupMappings(origin string) ([]GroupMapping, error) {
	page := Page{
		StartIndex:   1,
		ItemsPerPage: 100,
	}
	var (
		results     []GroupMapping
		currentPage []GroupMapping
		err         error
	)

	for {
		currentPage, page, err = a.ListGroupMappings(origin, page.StartIndex, page.ItemsPerPage)
		if err != nil {
			return nil, err
		}
		results = append(results, currentPage...)

		if (page.StartIndex + page.ItemsPerPage) > page.TotalResults {
			break
		}
		page.StartIndex = page.StartIndex + page.ItemsPerPage
	}
	return results, nil
}
