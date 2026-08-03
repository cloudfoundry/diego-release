package uaa

// IsHealthy returns true if the UAA is healthy, false if it is unhealthy, and
// an error if there is an issue making a request to the /healthz endpoint.
func (a *API) IsHealthy() (bool, error) {
	u := urlWithPath(*a.TargetURL, "/healthz")
	resp, err := a.Client.Get(u.String())
	if err != nil {
		return false, err
	}
	if resp.StatusCode == 200 {
		return true, nil
	}

	return false, nil
}
