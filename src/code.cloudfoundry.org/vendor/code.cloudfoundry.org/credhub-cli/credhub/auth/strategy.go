// CredHub authentication strategies
package auth

import (
	"net/http"
)

// Strategy provides http.Client-like interface to send authenticated requests to the server
//
// Modifies the request and client to include authentication based on the authentication strategy
type Strategy interface {
	Do(req *http.Request) (*http.Response, error)
}
