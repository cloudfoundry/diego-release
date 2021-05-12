package credhub

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
)

//InterpolateString translates credhub refs in a VCAP_SERVICES object into actual credentials
func (ch *CredHub) InterpolateString(vcapServicesBody string) (string, error) {
	if !strings.Contains(vcapServicesBody, `"credhub-ref"`) {
		return vcapServicesBody, nil
	}

	requestBody := map[string]interface{}{}
	if err := json.Unmarshal([]byte(vcapServicesBody), &requestBody); err != nil {
		return "", err
	}

	resp, err := ch.Request(http.MethodPost, "/api/v1/interpolate", nil, requestBody, true)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if err := ch.checkForServerError(resp); err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
