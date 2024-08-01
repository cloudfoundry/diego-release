package uaa

import (
	"fmt"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

type RequestError struct {
	Url           string
	ErrorResponse []byte
}

func (r RequestError) Error() string {
	return fmt.Sprintf("An error occurred while calling %s %s", r.Url, string(r.ErrorResponse))
}

func requestErrorFromOauthError(err error) error {
	oauthErrorResponse, isRetrieveError := err.(*oauth2.RetrieveError)
	if isRetrieveError {
		tokenUrl := oauthErrorResponse.Response.Request.URL.String()
		return requestErrorWithBody(tokenUrl, oauthErrorResponse.Body)
	}
	return err
}

func requestErrorWithBody(url string, body []byte) error {
	return RequestError{url, body}
}

func requestError(url string) error {
	return errors.Errorf("An error occurred while calling %s", url)
}

func parseError(err error, url string, body []byte) error {
	return errors.Wrapf(err, "An unknown error occurred while parsing response from %s. Response was %s", url, string(body))
}
