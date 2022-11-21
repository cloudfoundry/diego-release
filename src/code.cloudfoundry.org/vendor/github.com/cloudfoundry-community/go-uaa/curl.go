package uaa

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"strings"
)

// Curl makes a request to the UAA API with the given path, method, data, and
// headers.
func (a *API) Curl(path string, method string, data string, headers []string) (string, string, int, error) {
	u := urlWithPath(*a.TargetURL, path)
	req, err := http.NewRequest(method, u.String(), strings.NewReader(data))
	if err != nil {
		return "", "", -1, err
	}
	err = mergeHeaders(req.Header, strings.Join(headers, "\n"))
	if err != nil {
		return "", "", -1, err
	}

	a.ensureTransport(a.Client.Transport)
	resp, err := a.Client.Do(req)
	if err != nil {
		if a.verbose {
			fmt.Printf("%v\n\n", err)
		}
		return "", "", -1, err
	}
	defer resp.Body.Close()

	headerBytes, _ := httputil.DumpResponse(resp, false)
	resHeaders := string(headerBytes)

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil && a.verbose {
		fmt.Printf("%v\n\n", err)
	}
	resBody := string(bytes)

	return resHeaders, resBody, resp.StatusCode, nil
}

func mergeHeaders(destination http.Header, headerString string) (err error) {
	headerString = strings.TrimSpace(headerString)
	headerString += "\n\n"
	headerReader := bufio.NewReader(strings.NewReader(headerString))
	headers, err := textproto.NewReader(headerReader).ReadMIMEHeader()
	if err != nil {
		return
	}

	for key, values := range headers {
		destination.Del(key)
		for _, value := range values {
			destination.Add(key, value)
		}
	}

	return
}
