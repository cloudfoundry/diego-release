/*
 * Datadog API for Go
 *
 * Please see the included LICENSE file for licensing information.
 *
 * Copyright 2013 by authors and contributors.
 */

package datadog

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

// uriForAPI is to be called with something like "/v1/events" and it will give
// the proper request URI to be posted to.
func (self *Client) uriForAPI(api string) string {
	if strings.Index(api, "?") > -1 {
		return "https://app.datadoghq.com/api" + api + "&api_key=" +
			self.apiKey + "&application_key=" + self.appKey
	} else {
		return "https://app.datadoghq.com/api" + api + "?api_key=" +
			self.apiKey + "&application_key=" + self.appKey
	}
}

// doJsonRequest is the simplest type of request: a method on a URI that returns
// some JSON result which we unmarshal into the passed interface.
func (self *Client) doJsonRequest(method, api string,
	reqbody, out interface{}) error {
	// Handle the body if they gave us one.
	var bodyreader io.Reader
	if method != "GET" && reqbody != nil {
		bjson, err := json.Marshal(reqbody)
		if err != nil {
			return err
		}
		bodyreader = bytes.NewReader(bjson)
	}

	req, err := http.NewRequest(method, self.uriForAPI(api), bodyreader)
	if err != nil {
		return err
	}
	if bodyreader != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	// Actually do the request, error back if something crazy happened.
	resp, err := self.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.New("API error: " + resp.Status)
	}

	// If they don't care about the body, then we don't care to give them one,
	// so bail out because we're done.
	if out == nil {
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// If we got no body, by default let's just make an empty JSON dict. This
	// saves us some work in other parts of the code.
	if len(body) == 0 {
		body = []byte{'{', '}'}
	}

	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	return nil
}
