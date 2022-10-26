package uaa

import (
	"net/http"
)

// Info is information about the UAA server.
type Info struct {
	App            uaaApp              `json:"app"`
	Links          uaaLinks            `json:"links"`
	Prompts        map[string][]string `json:"prompts"`
	ZoneName       string              `json:"zone_name"`
	EntityID       string              `json:"entityID"`
	CommitID       string              `json:"commit_id"`
	Timestamp      string              `json:"timestamp"`
	IdpDefinitions map[string]string   `json:"idpDefinitions"`
}

type uaaApp struct {
	Version string `json:"version"`
}

type uaaLinks struct {
	ForgotPassword string `json:"passwd"`
	Uaa            string `json:"uaa"`
	Registration   string `json:"register"`
	Login          string `json:"login"`
}

// GetInfo gets server information
// http://docs.cloudfoundry.org/api/uaa/version/4.14.0/index.html#server-information-2.
func (a *API) GetInfo() (*Info, error) {
	url := urlWithPath(*a.TargetURL, "/info")

	info := &Info{}
	err := a.doJSON(http.MethodGet, &url, nil, info, false)
	return info, err
}
