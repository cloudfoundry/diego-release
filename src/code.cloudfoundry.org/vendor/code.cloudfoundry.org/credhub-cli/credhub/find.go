package credhub

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
)

// FindByPartialName retrieves a list of stored credential names which contain the search.
func (ch *CredHub) FindByPartialName(nameLike string) (credentials.FindResults, error) {
	return ch.findByPathOrNameLike("name-like", nameLike)
}

// FindByPath retrieves a list of stored credential names which are within the specified path.
func (ch *CredHub) FindByPath(path string) (credentials.FindResults, error) {
	return ch.findByPathOrNameLike("path", path)
}

func (ch *CredHub) findByPathOrNameLike(key, value string) (credentials.FindResults, error) {
	var creds credentials.FindResults
	body, err := ch.find(key, value)

	if err != nil {
		return creds, err
	}

	err = json.Unmarshal(body, &creds)

	return creds, err
}

func (ch *CredHub) find(key, value string) ([]byte, error) {
	query := url.Values{}
	query.Set(key, value)

	resp, err := ch.Request(http.MethodGet, "/api/v1/data", query, nil, true)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
