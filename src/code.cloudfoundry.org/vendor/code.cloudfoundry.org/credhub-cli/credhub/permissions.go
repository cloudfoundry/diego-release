package credhub

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"encoding/json"

	"code.cloudfoundry.org/credhub-cli/credhub/permissions"
)

type permissionsResponse struct {
	CredentialName string                      `json:"credential_name"`
	Permissions    []permissions.V1_Permission `json:"permissions"`
}

func (ch *CredHub) GetPermissions(name string) ([]permissions.V1_Permission, error) {
	query := url.Values{}
	query.Set("credential_name", name)

	resp, err := ch.Request(http.MethodGet, "/api/v1/permissions", query, nil, true)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	var response permissionsResponse

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Permissions, err
}

func (ch *CredHub) GetPermissionByUUID(uuid string) (*permissions.Permission, error) {
	path := "/api/v2/permissions/" + uuid

	resp, err := ch.Request(http.MethodGet, path, nil, nil, true)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	var response permissions.Permission

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (ch *CredHub) GetPermissionByPathActor(path string, actor string) (*permissions.Permission, error) {
	apiPath := "/api/v2/permissions"
	query := url.Values{}
	query.Set("actor", actor)
	query.Set("path", path)
	resp, err := ch.Request(http.MethodGet, apiPath, query, nil, true)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	var response permissions.Permission

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (ch *CredHub) addV1Permission(credName string, perms []permissions.V1_Permission) (*http.Response, error) {
	requestBody := map[string]interface{}{}
	requestBody["credential_name"] = credName
	requestBody["permissions"] = perms

	resp, err := ch.Request(http.MethodPost, "/api/v1/permissions", nil, requestBody, true)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (ch *CredHub) addV2Permission(path string, actor string, ops []string) (*http.Response, error) {
	requestBody := map[string]interface{}{}
	requestBody["path"] = path
	requestBody["actor"] = actor
	requestBody["operations"] = ops

	resp, err := ch.Request(http.MethodPost, "/api/v2/permissions", nil, requestBody, true)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (ch *CredHub) AddPermission(path string, actor string, ops []string) (*permissions.Permission, error) {
	serverVersion, err := ch.ServerVersion()
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	isOlderVersion := serverVersion.Segments()[0] < 2

	if isOlderVersion {
		resp, err = ch.addV1Permission(path, []permissions.V1_Permission{{Actor: actor, Operations: ops}})
	} else {
		resp, err = ch.addV2Permission(path, actor, ops)
	}

	if err != nil {
		return nil, err
	}

	if isOlderVersion {
		return nil, nil
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	var response permissions.Permission

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (ch *CredHub) UpdatePermission(uuid string, path string, actor string, ops []string) (*permissions.Permission, error) {
	serverVersion, err := ch.ServerVersion()
	if err != nil {
		return nil, err
	}

	isOlderVersion := serverVersion.Segments()[0] < 2
	if isOlderVersion {
		return nil, errors.New("credhub server version <2.0 not supported")
	}

	requestBody := map[string]interface{}{}

	requestBody["path"] = path
	requestBody["actor"] = actor
	requestBody["operations"] = ops

	resp, err := ch.Request(http.MethodPut, "/api/v2/permissions/"+uuid, nil, requestBody, true)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)

	var response permissions.Permission

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil

}

func (ch *CredHub) DeletePermission(uuid string) (*permissions.Permission, error) {
	serverVersion, err := ch.ServerVersion()
	if err != nil {
		return nil, err
	}

	isOlderVersion := serverVersion.Segments()[0] < 2
	if isOlderVersion {
		return nil, errors.New("credhub server version <2.0 not supported")
	}

	resp, err := ch.Request(http.MethodDelete, "/api/v2/permissions/"+uuid, nil, nil, true)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)

	var response permissions.Permission

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}
