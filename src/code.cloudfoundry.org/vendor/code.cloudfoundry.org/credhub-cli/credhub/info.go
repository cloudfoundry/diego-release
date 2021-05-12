package credhub

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	"code.cloudfoundry.org/credhub-cli/credhub/server"
)

// Info returns the targeted CredHub server information.
func (ch *CredHub) Info() (*server.Info, error) {
	//This uses a the private 'request' as it makes an https call but it does not require authentication
	response, err := ch.request(ch.Client(), "GET", "/info", nil, nil, true)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)

	info := &server.Info{}
	decoder := json.NewDecoder(response.Body)

	if err = decoder.Decode(&info); err != nil {
		return nil, err
	}

	return info, nil
}

// AuthURL returns the targeted CredHub server's trusted authentication server URL.
func (ch *CredHub) AuthURL() (string, error) {
	if ch.authURL != nil {
		return ch.authURL.String(), nil
	}

	info, err := ch.Info()

	if err != nil {
		return "", err
	}

	authURL := info.AuthServer.URL

	if authURL == "" {
		return "", errors.New("AuthURL not found")
	}

	return authURL, nil
}
