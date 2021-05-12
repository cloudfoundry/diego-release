package credhub

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
)

func (ch *CredHub) GetAllCertificatesMetadata() ([]credentials.CertificateMetadata, error) {
	query := url.Values{}

	return ch.makeGetCertificatesRequest(query)
}

func (ch *CredHub) GetCertificateMetadataByName(name string) (credentials.CertificateMetadata, error) {
	query := url.Values{}
	query.Set("name", name)

	certs, err := ch.makeGetCertificatesRequest(query)
	if err != nil {
		return credentials.CertificateMetadata{}, err
	}

	return certs[0], nil
}

func (ch *CredHub) makeGetCertificatesRequest(query url.Values) ([]credentials.CertificateMetadata, error) {
	resp, err := ch.Request(http.MethodGet, "/api/v1/certificates/", query, nil, true)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)

	dec := json.NewDecoder(resp.Body)
	response := make(map[string][]credentials.CertificateMetadata)

	if err := dec.Decode(&response); err != nil {
		return nil, errors.New("The response body could not be decoded: " + err.Error())
	}

	var ok bool
	var data []credentials.CertificateMetadata

	if data, ok = response["certificates"]; !ok || len(data) == 0 {
		return []credentials.CertificateMetadata{}, nil
	}

	return data, nil
}
