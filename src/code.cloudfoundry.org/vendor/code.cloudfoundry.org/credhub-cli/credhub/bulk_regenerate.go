package credhub

import (
	"encoding/json"
	"io"
	"net/http"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
)

func (ch *CredHub) BulkRegenerate(signedBy string) (credentials.BulkRegenerateResults, error) {
	var creds credentials.BulkRegenerateResults

	bulkRegenerateEndpoint := "/api/v1/bulk-regenerate"

	requestBody := map[string]interface{}{}
	requestBody["signed_by"] = signedBy

	resp, err := ch.Request(http.MethodPost, bulkRegenerateEndpoint, nil, requestBody, true)

	if err != nil {
		return credentials.BulkRegenerateResults{}, err
	}

	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&creds)

	return creds, err
}
