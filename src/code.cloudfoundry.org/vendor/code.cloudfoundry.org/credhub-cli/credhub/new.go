package credhub

import (
	"net/url"

	"code.cloudfoundry.org/credhub-cli/credhub/auth"
)

// New provides a CredHub API client for the target server. Options can be
// provided to specify additional parameters, including authentication.
// See the Option type for a list of supported options.
//
// When targeting multiple CredHub servers, use a new CredHub API client
// for each target server.
func New(target string, options ...Option) (*CredHub, error) {
	baseURL, err := url.Parse(target)

	if err != nil {
		return nil, err
	}

	credhub := &CredHub{
		ApiURL:      target,
		baseURL:     baseURL,
		authBuilder: auth.Noop,
	}

	for _, option := range options {
		if err := option(credhub); err != nil {
			return nil, err
		}
	}

	credhub.Auth, err = credhub.authBuilder(credhub)
	if err != nil {
		return nil, err
	}

	return credhub, nil
}
