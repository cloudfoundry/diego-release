package databaseuri

import (
	"encoding/json"
	"net/url"
)

func ParseDatabaseURI(services string) (string, error) {
	if services == "" {
		return "", nil
	}

	data := map[string][]struct {
		Credentials struct {
			Uri string `json:"uri"`
		} `json:"credentials"`
	}{}
	if err := json.Unmarshal([]byte(services), &data); err != nil {
		return "", err
	}

	var creds []string
	for _, v1 := range data {
		for _, v2 := range v1 {
			if v2.Credentials.Uri != "" {
				creds = append(creds, v2.Credentials.Uri)
			}
		}
	}

	schemes := map[string]string{
		"mysql":      "mysql2",
		"mysql2":     "",
		"postgres":   "",
		"postgresql": "postgres",
	}
	for _, service_uri := range creds {
		if uri, err := url.Parse(service_uri); err == nil {
			if val, ok := schemes[uri.Scheme]; ok {
				if val != "" {
					uri.Scheme = val
				}
				return uri.String(), nil
			}
		}
	}
	return "", nil
}
