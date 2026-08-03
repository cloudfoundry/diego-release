package auth

import "net/http"

// NoopStrategy will submit requests with no additional authentication
type NoopStrategy struct {
	*http.Client
}

var _ Strategy = new(NoopStrategy)
