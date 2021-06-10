package consuladapter

import (
	"errors"
	"net/url"
)

func Parse(urlArg string) (string, string, error) {
	u, err := url.Parse(urlArg)
	if err != nil {
		return "", "", err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", errors.New("scheme must be http or https")
	}

	if u.Host == "" {
		return "", "", errors.New("missing address")
	}

	return u.Scheme, u.Host, nil
}
