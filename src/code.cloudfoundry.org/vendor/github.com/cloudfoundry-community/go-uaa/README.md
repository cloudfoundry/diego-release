# `go-uaa` [![Travis-CI](https://travis-ci.org/cloudfoundry-community/go-uaa.svg)](https://travis-ci.org/cloudfoundry-community/go-uaa) [![godoc](https://godoc.org/github.com/cloudfoundry-community/go-uaa?status.svg)](http://godoc.org/github.com/cloudfoundry-community/go-uaa) [![Report card](https://goreportcard.com/badge/github.com/cloudfoundry-community/go-uaa)](https://goreportcard.com/report/github.com/cloudfoundry-community/go-uaa)

### Overview

`go-uaa` is a client library for the [UAA API](https://docs.cloudfoundry.org/api/uaa/). It is a [`go module`](https://github.com/golang/go/wiki/Modules).

### Usage

#### Step 1: Add `go-uaa` As A Dependency
```
$ go mod init # optional
$ go get -u github.com/cloudfoundry-community/go-uaa
$ cat go.mod
```

```
module github.com/cloudfoundry-community/go-uaa/cmd/test

go 1.13

require github.com/cloudfoundry-community/go-uaa latest
```

#### Step 2: Construct and Use `uaa.API`

Construct a `uaa.API` by using `uaa.New(target string, authOpt AuthenticationOption, opts ...Option)`:
* The target is the URL of your UAA API (for example, https://uaa.run.pivotal.io); *do not* include `/oauth/token` suffix
* You must choose one authentication method and supply it as the third argument. There are a number of authentication methods available:
  * [`uaa.WithClientCredentials(clientID string, clientSecret string, tokenFormat TokenFormat)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithClientCredentials)
  * [`uaa.WithPasswordCredentials(clientID string, clientSecret string, username string, password string, tokenFormat TokenFormat)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithPasswordCredentials)
  * [`uaa.WithAuthorizationCode(clientID string, clientSecret string, authorizationCode string, tokenFormat TokenFormat, redirectURL *url.URL)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithAuthorizationCode)
  * [`uaa.WithRefreshToken(clientID string, clientSecret string, refreshToken string, tokenFormat TokenFormat)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithRefreshToken)
  * [`uaa.WithToken(token *oauth2.Token)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithToken) (this is the only authentication methods that **cannot** automatically refresh the token when it expires)
* You can optionally supply one or more options:
  * [`uaa.WithZoneID(zoneID string)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithZoneID) if you want to specify your own [zone ID](https://docs.cloudfoundry.org/uaa/uaa-concepts.html#iz)
  * [`uaa.WithClient(client *http.Client)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithClient) if you want to specify your own `http.Client`
  * [`uaa.WithSkipSSLValidation(skipSSLValidation bool)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithSkipSSLValidation) if you want to ignore SSL validation issues; this is not recommended, and you should instead ensure you trust the certificate authority that issues the certificates used by UAA
	* [`uaa.WithUserAgent(userAgent string)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithUserAgent) if you want to supply your own user agent for requests to the UAA API
	* [`uaa.WithVerbosity(verbose bool)`](https://godoc.org/github.com/cloudfoundry-community/go-uaa#WithVerbosity) if you want to enable verbose logging

```bash
$ cat main.go
```

```go
package main

import (
	"log"

	uaa "github.com/cloudfoundry-community/go-uaa"
)

func main() {
	// construct the API
	api, err := uaa.New(
		"https://uaa.example.net",
		uaa.WithClientCredentials("client-id", "client-secret", uaa.JSONWebToken),
	)
	if err != nil {
		log.Fatal(err)
	}

	// use the API to fetch a user
	user, err := api.GetUserByUsername("test@example.net", "uaa", "")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Hello, %s\n", user.Name.GivenName)
}
```

### Experimental

* For the foreseeable future, releases will be in the `v0.x.y` range
* You should expect breaking changes until `v1.x.y` releases occur
* Notifications of breaking changes will be made via release notes associated with each tag
* You should [use `go modules`](https://blog.golang.org/using-go-modules) with this package

### Contributing

Pull requests welcome.
