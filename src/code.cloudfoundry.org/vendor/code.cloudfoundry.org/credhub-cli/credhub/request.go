package credhub

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// Request sends an authenticated request to the CredHub server.
//
// The pathStr should include the full path (eg. /api/v1/data).
// The request body should be marshallable to JSON, but can be left nil for GET requests.
//
// Request() is used by other CredHub client methods to send authenticated requests to the CredHub server.
//
// Use Request() directly to send authenticated requests to the CredHub server.
// For unauthenticated requests (eg. /health), use Config.Client() instead.
func (ch *CredHub) Request(method string, pathStr string, query url.Values, body interface{}, checkServerErr bool) (*http.Response, error) {
	return ch.request(ch.Auth, method, pathStr, query, body, checkServerErr)
}

type requester interface {
	Do(req *http.Request) (*http.Response, error)
}

func (ch *CredHub) request(client requester, method string, pathStr string, query url.Values, body interface{}, checkServerErr bool) (*http.Response, error) {
	u := *ch.baseURL // clone
	u.Path = pathStr
	u.RawQuery = query.Encode()

	var req *http.Request

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req, err = http.NewRequest(method, u.String(), bytes.NewReader(jsonBody))
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if os.Getenv("CREDHUB_DEBUG") == "true" {
		dumpRequest(req)
	}

	resp, err := client.Do(req)
	if err != nil {
		if os.Getenv("CREDHUB_DEBUG") == "true" {
			fmt.Println(fmt.Sprintf("[DEBUG] %s: %v", "An error occurred during the data request.", err))
		}
		return resp, err
	}

	if os.Getenv("CREDHUB_DEBUG") == "true" {
		dumpResponse(resp)
	}

	if checkServerErr {
		if err := ch.checkForServerError(resp); err != nil {
			return nil, err
		}
	}

	return resp, err
}

func (ch *CredHub) checkForServerError(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.New("The response body could not be read: " + err.Error())
		}

		var respErr error

		switch resp.StatusCode {
		case http.StatusNotFound:
			respErr = &NotFoundError{}
		default:
			respErr = &Error{}
		}

		if err := json.Unmarshal(body, &respErr); err != nil {
			return errors.New("The response body could not be decoded: " + err.Error())
		}
		return respErr
	}

	return nil
}

func dumpRequest(req *http.Request) {
	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		fmt.Println("[DEBUG] An error occurred during request dump.", err.Error())
	}
	fmt.Println("[DEBUG]", string(dump))
}

func dumpResponse(resp *http.Response) {
	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		fmt.Println("[DEBUG] An error occurred during response dump.", err.Error())
	}
	fmt.Println("[DEBUG]", string(dump))
}
